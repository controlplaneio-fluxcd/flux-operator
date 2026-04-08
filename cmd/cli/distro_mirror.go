// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/spf13/cobra"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/builder"
)

var distroMirrorCmd = &cobra.Command{
	Use:   "mirror <destination>",
	Short: "Mirror the Flux distribution to a destination registry",
	Long: `The distro mirror command copies a complete Flux distribution from the
upstream registries to a destination registry. This is intended for users
running Flux in air-gapped or private-registry environments.

The command performs the following steps:
  1. Pulls the Flux distribution manifests OCI artifact to read the image list.
  2. Resolves the requested version against the available distribution releases.
  3. Mirrors every controller image and (optionally) the Flux Operator image
     and Helm chart to the destination registry.

Authentication for the destination registry uses the local Docker config file.
The source registry (ghcr.io) can be authenticated with --pull-token or
--pull-token-stdin, otherwise the Docker config is used as well.`,
	Example: `  # Mirror the latest Flux 2.x distribution to a private registry
  flux-operator distro mirror registry.example.com/flux

  # Mirror a specific Flux version
  flux-operator distro mirror registry.example.com/flux --version 2.8.x

  # Mirror the enterprise distroless variant
  echo "${GITHUB_TOKEN}" | flux-operator distro mirror registry.example.com/flux \
    --variant enterprise-distroless \
    --pull-token-stdin

  # List the source/destination pairs without copying images
  flux-operator distro mirror registry.example.com/flux --dry-run

  # Mirror only a subset of controllers
  flux-operator distro mirror registry.example.com/flux \
    --components source-controller,kustomize-controller,helm-controller

  # Mirror to an immutable registry (push by digest with a unique tag suffix)
  flux-operator distro mirror registry.example.com/flux --immutable

  # Mirror without the Flux Operator image and chart (controllers only)
  flux-operator distro mirror registry.example.com/flux \
    --include-operator-image=false \
    --include-operator-chart=false
`,
	Args: cobra.ExactArgs(1),
	RunE: distroMirrorCmdRun,
}

type distroMirrorFlags struct {
	version              string
	components           []string
	variant              string
	includeOperatorImage bool
	includeOperatorChart bool
	dryRun               bool
	immutable            bool
	pullToken            string
	pullTokenStdin       bool
}

var distroMirrorArgs = distroMirrorFlags{
	version:              "2.x",
	variant:              builder.UpstreamAlpine,
	includeOperatorImage: true,
	includeOperatorChart: true,
}

const (
	distroMirrorSrcRegistry       = "ghcr.io"
	distroMirrorManifestsArtifact = "oci://ghcr.io/controlplaneio-fluxcd/flux-operator-manifests:latest"
	distroMirrorOperatorRepo      = "ghcr.io/controlplaneio-fluxcd/flux-operator"
	distroMirrorChartRepo         = "ghcr.io/controlplaneio-fluxcd/charts/flux-operator"
	// distroMirrorMinTimeout is the minimum operation timeout. The default
	// rootArgs.timeout (1m) is too short to pull the manifests artifact and
	// copy every controller image, so we floor it.
	distroMirrorMinTimeout = 10 * time.Minute
)

// distroMirrorVariantRegistry maps a distribution variant to its source registry.
var distroMirrorVariantRegistry = map[string]string{
	builder.UpstreamAlpine:           "ghcr.io/fluxcd",
	builder.EnterpriseAlpine:         "ghcr.io/controlplaneio-fluxcd/alpine",
	builder.EnterpriseDistroless:     "ghcr.io/controlplaneio-fluxcd/distroless",
	builder.EnterpriseDistrolessFIPS: "ghcr.io/controlplaneio-fluxcd/distroless-fips",
}

func init() {
	distroMirrorCmd.Flags().StringVar(&distroMirrorArgs.version, "version", distroMirrorArgs.version,
		"Flux distribution version, e.g. 2.8.5 or 2.8.x")
	distroMirrorCmd.Flags().StringSliceVar(&distroMirrorArgs.components, "components", nil,
		"comma-separated list of components to mirror (defaults to all controllers, plus source-watcher for Flux 2.7+)")
	distroMirrorCmd.Flags().StringVar(&distroMirrorArgs.variant, "variant", distroMirrorArgs.variant,
		"distribution variant: upstream-alpine, enterprise-alpine, enterprise-distroless, enterprise-distroless-fips")
	distroMirrorCmd.Flags().BoolVar(&distroMirrorArgs.includeOperatorImage, "include-operator-image", distroMirrorArgs.includeOperatorImage,
		"also mirror the Flux Operator container image")
	distroMirrorCmd.Flags().BoolVar(&distroMirrorArgs.includeOperatorChart, "include-operator-chart", distroMirrorArgs.includeOperatorChart,
		"also mirror the Flux Operator Helm chart")
	distroMirrorCmd.Flags().BoolVar(&distroMirrorArgs.dryRun, "dry-run", false,
		"list source→destination pairs without writing anything")
	distroMirrorCmd.Flags().BoolVar(&distroMirrorArgs.immutable, "immutable", distroMirrorArgs.immutable,
		"treat destination tags as immutable; never overwrite, copy by digest to a unique tag instead")
	distroMirrorCmd.Flags().StringVar(&distroMirrorArgs.pullToken, "pull-token", "",
		"GHCR token for the source registry (basic-auth password)")
	distroMirrorCmd.Flags().BoolVar(&distroMirrorArgs.pullTokenStdin, "pull-token-stdin", false,
		"read the GHCR token for the source registry from stdin")

	distroCmd.AddCommand(distroMirrorCmd)
}

func distroMirrorCmdRun(_ *cobra.Command, args []string) error {
	dstPrefix := strings.TrimSuffix(strings.TrimPrefix(args[0], "oci://"), "/")
	if dstPrefix == "" {
		return errors.New("destination registry is required")
	}

	srcRegistry, ok := distroMirrorVariantRegistry[distroMirrorArgs.variant]
	if !ok {
		return fmt.Errorf("unsupported variant %q, must be one of: %s",
			distroMirrorArgs.variant,
			strings.Join(slices.Sorted(maps.Keys(distroMirrorVariantRegistry)), ", "))
	}

	// --pull-token and --pull-token-stdin are mutually exclusive.
	pullToken := distroMirrorArgs.pullToken
	if distroMirrorArgs.pullTokenStdin {
		if pullToken != "" {
			return errors.New("--pull-token and --pull-token-stdin are mutually exclusive")
		}
		var input string
		if _, err := fmt.Scan(&input); err != nil {
			return fmt.Errorf("unable to read pull token from stdin: %w", err)
		}
		pullToken = input
	}

	keychain := buildMirrorKeychain(distroMirrorSrcRegistry, pullToken)
	ctx, cancel := context.WithTimeout(context.Background(), max(rootArgs.timeout, distroMirrorMinTimeout))
	defer cancel()
	craneOpts := []crane.Option{
		crane.WithContext(ctx),
		crane.WithAuthFromKeychain(keychain),
	}

	rootCmd.Println(`◎`, "Pulling distribution manifests from", distroMirrorManifestsArtifact)
	tmpDir, err := builder.MkdirTempAbs("", "flux-mirror")
	if err != nil {
		return fmt.Errorf("failed to create tmp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if _, err := builder.PullArtifact(ctx, distroMirrorManifestsArtifact, tmpDir, keychain); err != nil {
		return fmt.Errorf("failed to pull distribution manifests: %w", err)
	}

	ver, err := builder.MatchVersion(filepath.Join(tmpDir, "flux"), distroMirrorArgs.version)
	if err != nil {
		return fmt.Errorf("failed to resolve version: %w", err)
	}

	components, err := resolveMirrorComponents(distroMirrorArgs.components, ver)
	if err != nil {
		return err
	}

	opts := builder.MakeDefaultOptions()
	opts.Version = ver
	opts.Variant = distroMirrorArgs.variant
	opts.Registry = srcRegistry
	opts.Components = components

	images, err := builder.ExtractComponentImagesWithDigest(filepath.Join(tmpDir, "flux-images"), opts)
	if err != nil {
		return fmt.Errorf("failed to extract component images: %w", err)
	}

	jobs := buildMirrorJobs(images, dstPrefix,
		distroMirrorArgs.includeOperatorImage,
		distroMirrorArgs.includeOperatorChart)

	rootCmd.Printf("◎ Mirroring Flux %s (%s) to %s\n", ver, distroMirrorArgs.variant, dstPrefix)

	var copied, skipped, byDigest int
	for _, j := range jobs {
		result, err := runMirrorJob(j, distroMirrorArgs.immutable, distroMirrorArgs.dryRun, craneOpts)
		if err != nil {
			return err
		}
		switch result.action {
		case mirrorCopied:
			rootCmd.Printf("→ %s → %s (copied)\n", result.src, result.dst)
			copied++
		case mirrorOverwritten:
			rootCmd.Printf("→ %s → %s (overwritten)\n", result.src, result.dst)
			copied++
		case mirrorByDigest:
			rootCmd.Printf("→ %s → %s (copied by digest)\n", result.src, result.dst)
			byDigest++
		case mirrorSkipped:
			rootCmd.Printf("≡ %s (skipped: %s)\n", result.dst, result.reason)
			skipped++
		case mirrorDryRun:
			rootCmd.Printf("✎ %s → %s (dry-run)\n", result.src, result.dst)
		}
	}

	if distroMirrorArgs.dryRun {
		rootCmd.Printf("✔ Dry-run complete: %d image(s) would be processed\n", len(jobs))
		return nil
	}
	rootCmd.Printf("✔ Mirror complete: %d copied, %d skipped, %d copied by digest\n",
		copied, skipped, byDigest)
	return nil
}

// resolveMirrorComponents returns the user-supplied components if non-empty,
// otherwise the default controller set augmented with source-watcher when the
// resolved version supports it. The result is validated against AllComponents
// and against the version constraint for source-watcher.
func resolveMirrorComponents(input []string, ver string) ([]string, error) {
	v, err := semver.NewVersion(ver)
	if err != nil {
		return nil, fmt.Errorf("failed to parse version %q: %w", ver, err)
	}

	if len(input) == 0 {
		// All controllers; source-watcher only exists in Flux >= 2.7.
		out := slices.DeleteFunc(slices.Clone(builder.AllComponents), func(c string) bool {
			return c == fluxcdv1.FluxSourceWatcher
		})
		if v.Minor() >= 7 {
			out = append(out, fluxcdv1.FluxSourceWatcher)
		}
		return out, nil
	}

	for _, c := range input {
		if !slices.Contains(builder.AllComponents, c) {
			return nil, fmt.Errorf("invalid component %q", c)
		}
		if c == fluxcdv1.FluxSourceWatcher && v.Minor() < 7 {
			return nil, fmt.Errorf("%s is only supported in Flux versions >= 2.7.0", fluxcdv1.FluxSourceWatcher)
		}
	}
	return input, nil
}

// mirrorJob describes a single image to mirror.
type mirrorJob struct {
	srcRepo string // e.g. ghcr.io/fluxcd/source-controller
	dstRepo string // e.g. registry.example.com/flux/source-controller
	tag     string // e.g. v1.6.2
	digest  string // src digest (sha256:...) — empty when not pre-resolved
}

func (j mirrorJob) src() string { return j.srcRepo + ":" + j.tag }
func (j mirrorJob) dst() string { return j.dstRepo + ":" + j.tag }

// buildMirrorJobs constructs the ordered list of mirror jobs from the
// extracted component images, optionally including the Flux Operator
// container image and Helm chart.
func buildMirrorJobs(images []builder.ComponentImage, dstPrefix string, includeOperatorImage, includeOperatorChart bool) []mirrorJob {
	jobs := make([]mirrorJob, 0, len(images)+2)
	for _, img := range images {
		jobs = append(jobs, mirrorJob{
			srcRepo: img.Repository,
			dstRepo: dstPrefix + "/" + path.Base(img.Repository),
			tag:     img.Tag,
			digest:  img.Digest,
		})
	}

	// Operator image is published with a "v" prefix (e.g. v0.46.0),
	// Helm chart is published without it (e.g. 0.46.0).
	bareVersion := strings.TrimPrefix(VERSION, "v")

	if includeOperatorImage {
		jobs = append(jobs, mirrorJob{
			srcRepo: distroMirrorOperatorRepo,
			dstRepo: dstPrefix + "/flux-operator",
			tag:     "v" + bareVersion,
		})
	}

	if includeOperatorChart {
		jobs = append(jobs, mirrorJob{
			srcRepo: distroMirrorChartRepo,
			dstRepo: dstPrefix + "/charts/flux-operator",
			tag:     bareVersion,
		})
	}

	return jobs
}

// mirrorAction describes the outcome of a mirror job.
type mirrorAction int

const (
	mirrorCopied mirrorAction = iota
	mirrorOverwritten
	mirrorByDigest
	mirrorSkipped
	mirrorDryRun
)

// mirrorResult is the result of executing a single mirror job.
type mirrorResult struct {
	action mirrorAction
	src    string // src ref actually used (may be "<repo>@<digest>")
	dst    string // dst ref actually written (may differ from j.dst() in immutable mode)
	reason string // human-readable reason for skip
}

// runMirrorJob copies a single image to the destination registry, honoring the
// immutable and dry-run flags. The function is idempotent: if the destination
// already holds the source digest under the requested tag, the job is skipped.
//
// The source digest is resolved lazily: jobs that don't carry a pre-resolved
// digest only pay the extra round trip when the destination tag exists and we
// need to compare. The common first-mirror path (dst returns 404) avoids it.
func runMirrorJob(j mirrorJob, immutable, dryRun bool, craneOpts []crane.Option) (mirrorResult, error) {
	src, dst := j.src(), j.dst()
	if dryRun {
		return mirrorResult{action: mirrorDryRun, src: src, dst: dst}, nil
	}

	desc, err := crane.Head(dst, craneOpts...)
	switch {
	case isNotFound(err):
		if err := crane.Copy(src, dst, craneOpts...); err != nil {
			return mirrorResult{}, fmt.Errorf("failed to copy %s to %s: %w", src, dst, err)
		}
		return mirrorResult{action: mirrorCopied, src: src, dst: dst}, nil
	case err != nil:
		return mirrorResult{}, fmt.Errorf("failed to head %s: %w", dst, err)
	}

	srcDigest := j.digest
	if srcDigest == "" {
		d, err := crane.Digest(src, craneOpts...)
		if err != nil {
			return mirrorResult{}, fmt.Errorf("failed to resolve digest for %s: %w", src, err)
		}
		srcDigest = d
	}

	if desc.Digest.String() == srcDigest {
		return mirrorResult{action: mirrorSkipped, src: src, dst: dst, reason: "up-to-date"}, nil
	}

	if !immutable {
		if err := crane.Copy(src, dst, craneOpts...); err != nil {
			return mirrorResult{}, fmt.Errorf("failed to copy %s to %s: %w", src, dst, err)
		}
		return mirrorResult{action: mirrorOverwritten, src: src, dst: dst}, nil
	}

	// Immutable mode: don't touch the existing tag. Skip entirely if the
	// source digest is already stored under any tag at the destination.
	if _, err := crane.Digest(j.dstRepo+"@"+srcDigest, craneOpts...); err == nil {
		return mirrorResult{action: mirrorSkipped, src: src, dst: dst, reason: "digest already present"}, nil
	} else if !isNotFound(err) {
		return mirrorResult{}, fmt.Errorf("failed to check digest %s@%s: %w", j.dstRepo, srcDigest, err)
	}

	srcByDigest := j.srcRepo + "@" + srcDigest
	uniqueDst := fmt.Sprintf("%s:%s-%d", j.dstRepo, j.tag, time.Now().Unix())
	if err := crane.Copy(srcByDigest, uniqueDst, craneOpts...); err != nil {
		return mirrorResult{}, fmt.Errorf("failed to copy %s to %s: %w", srcByDigest, uniqueDst, err)
	}
	return mirrorResult{action: mirrorByDigest, src: srcByDigest, dst: uniqueDst}, nil
}

// isNotFound returns true when err is a transport.Error with HTTP 404.
func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	var terr *transport.Error
	if errors.As(err, &terr) {
		return terr.StatusCode == 404
	}
	return false
}

// buildMirrorKeychain returns a keychain that authenticates the given source
// registry hostname with the supplied token (as a basic-auth password) and
// falls back to the default Docker config keychain for everything else.
func buildMirrorKeychain(srcRegistry, token string) authn.Keychain {
	if token == "" {
		return authn.DefaultKeychain
	}
	src := &mirrorTokenKeychain{
		registry: srcRegistry,
		auth: authn.FromConfig(authn.AuthConfig{
			Username: "flux",
			Password: token,
		}),
	}
	return authn.NewMultiKeychain(src, authn.DefaultKeychain)
}

// mirrorTokenKeychain authenticates a single registry hostname with a static
// authenticator and returns anonymous credentials for everything else. It is
// chained with the default keychain via authn.NewMultiKeychain.
type mirrorTokenKeychain struct {
	registry string
	auth     authn.Authenticator
}

// Resolve implements authn.Keychain.
func (k *mirrorTokenKeychain) Resolve(target authn.Resource) (authn.Authenticator, error) {
	if target.RegistryStr() == k.registry {
		return k.auth, nil
	}
	return authn.Anonymous, nil
}
