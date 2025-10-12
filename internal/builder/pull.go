// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package builder

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	untar "github.com/fluxcd/pkg/tar"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/hashicorp/go-retryablehttp"
)

// PullArtifact downloads an artifact from an OCI repository and extracts the content
// of the first tgz layer to the given destination directory.
// It returns the digest of the artifact.
func PullArtifact(ctx context.Context, ociURL, dstDir string, keyChain authn.Keychain) (string, error) {
	img, err := crane.Pull(strings.TrimPrefix(ociURL, "oci://"), crane.WithContext(ctx), crane.WithAuthFromKeychain(keyChain))
	if err != nil {
		return "", fmt.Errorf("pulling artifact %s failed: %w", ociURL, err)
	}

	digest, err := img.Digest()
	if err != nil {
		return "", fmt.Errorf("parsing digest for artifact %s failed: %w", ociURL, err)
	}

	layers, err := img.Layers()
	if err != nil {
		return "", fmt.Errorf("listing layers in artifact %s failed: %w", ociURL, err)
	}

	if len(layers) < 1 {
		return "", fmt.Errorf("no layers found in artifact %s", ociURL)
	}

	blob, err := layers[0].Compressed()
	if err != nil {
		return "", fmt.Errorf("extracting layer from artifact %s failed: %w", ociURL, err)
	}

	if err = untar.Untar(blob, dstDir, untar.WithMaxUntarSize(-1)); err != nil {
		return "", fmt.Errorf("extracting layer from artifact %s failed: %w", ociURL, err)
	}

	return digest.String(), nil
}

// ExtractFileFromArtifact downloads an artifact from an OCI repository and
// returns the content of a specific file from its first layer.
// The operation is performed in-memory without writing to the filesystem.
func ExtractFileFromArtifact(ctx context.Context, ociURL, filepath string, keyChain authn.Keychain) ([]byte, error) {
	// Pull the OCI image/artifact from the repository.
	img, err := crane.Pull(strings.TrimPrefix(ociURL, "oci://"), crane.WithContext(ctx), crane.WithAuthFromKeychain(keyChain))
	if err != nil {
		return nil, fmt.Errorf("pulling artifact %s failed: %w", ociURL, err)
	}

	// Get the layers of the image.
	layers, err := img.Layers()
	if err != nil {
		return nil, fmt.Errorf("listing layers in artifact %s failed: %w", ociURL, err)
	}

	// Ensure there is at least one layer.
	if len(layers) < 1 {
		return nil, fmt.Errorf("no layers found in artifact %s", ociURL)
	}

	// Get the compressed blob of the first layer.
	blob, err := layers[0].Compressed()
	if err != nil {
		return nil, fmt.Errorf("reading layer from artifact %s failed: %w", ociURL, err)
	}
	defer blob.Close()

	// The layer is a gzipped tarball, so we create a gzip reader.
	gzr, err := gzip.NewReader(blob)
	if err != nil {
		return nil, fmt.Errorf("decompressing layer from artifact %s failed: %w", ociURL, err)
	}
	defer gzr.Close()

	// Create a tar reader to iterate through the files in the archive.
	tr := tar.NewReader(gzr)

	// Iterate through the files in the tar archive.
	for {
		header, err := tr.Next()
		if err == io.EOF {
			// End of tar archive reached.
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading tar archive from artifact %s failed: %w", ociURL, err)
		}

		// Check if the current file is the one we are looking for.
		if header.Name == filepath {
			// Read the file content into a byte slice.
			content, err := io.ReadAll(tr)
			if err != nil {
				return nil, fmt.Errorf("reading file '%s' from artifact %s failed: %w", filepath, ociURL, err)
			}
			return content, nil
		}
	}

	// If we get here, the file was not found in the archive.
	return nil, fmt.Errorf("file '%s' not found in artifact %s", filepath, ociURL)
}

// FetchManifestFromURL downloads a YAML file from the given URL and returns its content.
// It supports GitHub Gist, GitHub repository and GitLab project URLs, converting them to raw content URLs as needed.
// It also supports OCI URLs in the format: oci://registry/repository:tag@digest#filepath
func FetchManifestFromURL(ctx context.Context, rawURL string) ([]byte, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	// Handle OCI URLs.
	if parsedURL.Scheme == "oci" {
		// Extract the filepath from the fragment.
		filepath := parsedURL.Fragment
		if filepath == "" {
			return nil, fmt.Errorf("OCI URL must include a fragment with the file path, e.g., oci://registry/repository:tag@digest#filepath")
		}

		// Reconstruct the OCI URL without the fragment.
		ociURL := fmt.Sprintf("oci://%s%s", parsedURL.Host, parsedURL.Path)
		return ExtractFileFromArtifact(ctx, ociURL, filepath, authn.DefaultKeychain)
	}

	// Transform HTTP/S URL for specific providers to get raw content.
	switch {
	case parsedURL.Host == "gist.github.com":
		// Gist URLs: https://gist.github.com/username/gist-id
		// Raw URLs: https://gist.githubusercontent.com/username/gist-id/raw
		parsedURL.Host = "gist.githubusercontent.com"
		if !strings.HasSuffix(parsedURL.Path, "/raw") {
			parsedURL.Path = parsedURL.Path + "/raw"
		}
		// If there's a fragment (e.g., #file-flux-instance-yaml), append it as a path component
		if parsedURL.Fragment != "" && strings.HasPrefix(parsedURL.Fragment, "file-") {
			// Convert fragment like "file-flux-instance-yaml" to "flux-instance.yaml"
			filename := strings.TrimPrefix(parsedURL.Fragment, "file-")
			filename = strings.ReplaceAll(filename, "-", ".")
			parts := strings.Split(filename, ".")
			if len(parts) > 1 {
				// Reconstruct: everything before last dot, then dot, then extension
				filename = strings.Join(parts[:len(parts)-1], "-") + "." + parts[len(parts)-1]
			}
			parsedURL.Path = parsedURL.Path + "/" + filename
			parsedURL.Fragment = ""
		}
	case strings.Contains(parsedURL.Host, "github") && parsedURL.Query().Get("raw") != "true":
		// For standard GitHub URLs, add 'raw=true' to get the raw file.
		query := parsedURL.Query()
		query.Set("raw", "true")
		parsedURL.RawQuery = query.Encode()
	case strings.Contains(parsedURL.Host, "gitlab") && strings.Contains(parsedURL.Path, "/-/blob/"):
		// For GitLab URLs, replace '/-/blob/' with '/-/raw/'.
		parsedURL.Path = strings.Replace(parsedURL.Path, "/-/blob/", "/-/raw/", 1)
	}

	// Create a new retryable HTTP request.
	req, err := retryablehttp.NewRequestWithContext(ctx, http.MethodGet, parsedURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for %s: %w", rawURL, err)
	}

	// Use a retryable client to handle transient errors automatically.
	client := retryablehttp.NewClient()
	client.RetryMax = 3
	client.RetryWaitMin = 2 * time.Second
	client.RetryWaitMax = 5 * time.Second
	client.Logger = nil

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download from %s: %w", rawURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download from %s: server returned status %s", rawURL, resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body from %s: %w", rawURL, err)
	}

	return data, nil
}
