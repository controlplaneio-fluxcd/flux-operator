// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package agentops

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/fluxcd/pkg/tar"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

// PushArtifactOptions holds configuration for pushing an OCI artifact.
type PushArtifactOptions struct {
	// Tags is the list of tags to apply to the artifact.
	Tags []string

	// Annotations is the map of OCI manifest annotations.
	Annotations map[string]string
}

// PushArtifact creates a Flux OCI artifact from the pre-built data
// and pushes it to the specified repository. It returns the artifact digest.
func PushArtifact(ctx context.Context, repo string, data []byte, opts PushArtifactOptions) (string, error) {
	if len(opts.Tags) == 0 {
		return "", fmt.Errorf("at least one tag is required")
	}

	img := mutate.MediaType(empty.Image, types.OCIManifestSchema1)
	img = mutate.ConfigMediaType(img, types.MediaType(fluxConfigMediaType))

	layer := static.NewLayer(data, types.MediaType(fluxContentMediaType))
	img, err := mutate.Append(img, mutate.Addendum{Layer: layer})
	if err != nil {
		return "", fmt.Errorf("appending layer: %w", err)
	}

	if len(opts.Annotations) > 0 {
		img = mutate.Annotations(img, opts.Annotations).(v1.Image)
	}

	ref := fmt.Sprintf("%s:%s", repo, opts.Tags[0])
	craneOpts := []crane.Option{crane.WithContext(ctx), crane.WithAuthFromKeychain(authn.DefaultKeychain)}

	if err := crane.Push(img, ref, craneOpts...); err != nil {
		return "", fmt.Errorf("pushing artifact to %s: %w", ref, err)
	}

	digest, err := img.Digest()
	if err != nil {
		return "", fmt.Errorf("getting artifact digest: %w", err)
	}

	// Tag additional tags.
	for _, tag := range opts.Tags[1:] {
		if err := crane.Tag(ref, tag, craneOpts...); err != nil {
			return "", fmt.Errorf("tagging artifact with %s: %w", tag, err)
		}
	}

	return digest.String(), nil
}

// BuildArtifact creates a tar+gzip archive containing only the specified skill
// directories from srcDir. It strips environment metadata (uid, gid, timestamps)
// for reproducibility, and skips symlinks and non-regular files.
func BuildArtifact(srcDir string, skillNames []string) ([]byte, error) {
	if len(skillNames) == 0 {
		return nil, fmt.Errorf("no skills to archive")
	}

	absSrc, err := filepath.Abs(srcDir)
	if err != nil {
		return nil, fmt.Errorf("resolving source directory: %w", err)
	}

	skills := make(map[string]struct{}, len(skillNames))
	for _, name := range skillNames {
		skillDir := filepath.Join(absSrc, name)
		if _, err := os.Stat(skillDir); err != nil {
			return nil, fmt.Errorf("archiving skill %s: %w", name, err)
		}
		skills[name] = struct{}{}
	}

	filter := func(path string, _ os.FileInfo) bool {
		rel, err := filepath.Rel(absSrc, path)
		if err != nil || rel == "." {
			return false
		}
		top, _, _ := strings.Cut(filepath.ToSlash(rel), "/")
		_, ok := skills[top]
		return !ok
	}

	var buf bytes.Buffer
	if _, err := tar.Tar(absSrc, &buf, tar.WithFilter(filter)); err != nil {
		return nil, fmt.Errorf("creating archive: %w", err)
	}

	return buf.Bytes(), nil
}

// ParseAnnotations parses a list of key=value strings into a map.
// It splits on the first '=' only, allowing values to contain '='.
func ParseAnnotations(args []string) (map[string]string, error) {
	annotations := make(map[string]string, len(args))
	for _, arg := range args {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid annotation %q: must be in key=value format", arg)
		}
		annotations[parts[0]] = parts[1]
	}
	return annotations, nil
}

// semverRegex matches semver versions with or without a 'v' prefix.
var semverRegex = regexp.MustCompile(`^v?\d+\.\d+\.\d+`)

// AppendGitMetadata auto-populates OCI annotations from git metadata.
// It only sets annotations that are not already present in the map.
// Errors are silently ignored (git not installed, not a repo, etc.).
func AppendGitMetadata(repoPath string, annotations map[string]string) {
	gitTimeout := 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()

	runGit := func(args ...string) (string, error) {
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Dir = repoPath
		out, err := cmd.Output()
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(out)), nil
	}

	if _, ok := annotations[AnnotationCreated]; !ok {
		if ts, err := runGit("log", "-1", "--format=%cI"); err == nil && ts != "" {
			annotations[AnnotationCreated] = ts
		} else {
			annotations[AnnotationCreated] = time.Now().UTC().Format(time.RFC3339)
		}
	}

	if _, ok := annotations[AnnotationSource]; !ok {
		if url, err := runGit("config", "--get", "remote.origin.url"); err == nil && url != "" {
			annotations[AnnotationSource] = NormalizeGitURL(url)
		}
	}

	// Resolve the exact tag once; reused for both revision and version annotations.
	exactTag, _ := runGit("describe", "--tags", "--exact-match", "HEAD")

	if _, ok := annotations[AnnotationRevision]; !ok {
		if sha, err := runGit("rev-parse", "HEAD"); err == nil && sha != "" {
			annotations[AnnotationRevision] = resolveGitRevision(runGit, sha, exactTag)
		}
	}

	if _, ok := annotations[AnnotationVersion]; !ok {
		if exactTag != "" && semverRegex.MatchString(exactTag) {
			annotations[AnnotationVersion] = exactTag
		}
	}
}

// resolveGitRevision builds the revision string in the format <ref>@sha1:<sha>.
// If exactTag is non-empty, it is used directly instead of re-running git describe.
func resolveGitRevision(runGit func(args ...string) (string, error), sha string, exactTag string) string {
	if exactTag != "" {
		return fmt.Sprintf("refs/tags/%s@sha1:%s", exactTag, sha)
	}

	// Try symbolic ref (branch name).
	if ref, err := runGit("rev-parse", "--symbolic-full-name", "HEAD"); err == nil && ref != "" && ref != "HEAD" {
		return fmt.Sprintf("%s@sha1:%s", ref, sha)
	}

	// Detached HEAD, no tag.
	return fmt.Sprintf("sha1:%s", sha)
}

// NormalizeGitURL converts git URLs to HTTPS format.
// It handles git://, git@host:path SSH URLs, and strips .git suffixes.
func NormalizeGitURL(url string) string {
	// Handle git:// protocol.
	url = strings.Replace(url, "git://", "https://", 1)

	// Handle git@host:path SSH URLs.
	if strings.HasPrefix(url, "git@") {
		url = strings.TrimPrefix(url, "git@")
		url = strings.Replace(url, ":", "/", 1)
		url = "https://" + url
	}

	// Strip .git suffix.
	url = strings.TrimSuffix(url, ".git")

	return url
}
