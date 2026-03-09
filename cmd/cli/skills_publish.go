// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/agentops"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/cosign"
)

var skillsPublishCmd = &cobra.Command{
	Use:     "publish [repository]",
	Aliases: []string{"push"},
	Short:   "Publish skills as an OCI artifact",
	Long:    `The publish command packages a local skills directory and pushes it to a container registry.`,
	Example: `  # Publish skills to a ghcr.io repository with keyless signing
  flux-operator skills publish ghcr.io/my-org/agent-skills \
    --tag v1.0.0 \
    --tag latest \
	--sign

  # Publish latest tag with custom annotations only if contents differ
  flux-operator skills publish ghcr.io/my-org/agent-skills \
    --path ./my-skills \
    --diff-tag latest \
    -a 'org.opencontainers.image.source=https://github.com/my-org/agent-skills' \
    -a 'org.opencontainers.image.description=A collection of skills for my agent'
`,
	Args: cobra.ExactArgs(1),
	RunE: skillsPublishCmdRun,
}

type skillsPublishFlags struct {
	path        string
	tags        []string
	annotations []string
	sign        bool
	diffTag     string
	output      string
}

// skillsPublishResult holds the structured output for JSON format.
type skillsPublishResult struct {
	Repository  string            `json:"repository"`
	Digest      string            `json:"digest,omitempty"`
	Tags        []string          `json:"tags"`
	Skills      []string          `json:"skills"`
	Signed      bool              `json:"signed"`
	Skipped     bool              `json:"skipped"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

var skillsPublishArgs skillsPublishFlags

func init() {
	skillsPublishCmd.Flags().StringVar(&skillsPublishArgs.path, "path", "skills",
		"path to the skills directory")
	skillsPublishCmd.Flags().StringSliceVar(&skillsPublishArgs.tags, "tag", []string{"latest"},
		"OCI artifact tag (can be specified multiple times)")
	skillsPublishCmd.Flags().StringSliceVarP(&skillsPublishArgs.annotations, "annotation", "a", nil,
		"OCI manifest annotation in key=value format (can be specified multiple times)")
	skillsPublishCmd.Flags().BoolVar(&skillsPublishArgs.sign, "sign", false,
		"sign the artifact with cosign keyless")
	skillsPublishCmd.Flags().StringVar(&skillsPublishArgs.diffTag, "diff-tag", "",
		"only push if the contents differ from the specified tag")
	skillsPublishCmd.Flags().StringVarP(&skillsPublishArgs.output, "output", "o", "",
		"output format (json)")

	skillsCmd.AddCommand(skillsPublishCmd)
}

func skillsPublishCmdRun(cmd *cobra.Command, args []string) error {
	repo := agentops.NormalizeRepository(args[0])
	path := skillsPublishArgs.path
	jsonOutput := skillsPublishArgs.output == "json"

	// Parse annotations.
	annotations, err := agentops.ParseAnnotations(skillsPublishArgs.annotations)
	if err != nil {
		return err
	}

	// Auto-populate git metadata (user annotations take precedence).
	agentops.AppendGitMetadata(path, annotations)

	// Validate skills in path.
	skillNames, err := agentops.DiscoverSkills(path)
	if err != nil {
		return fmt.Errorf("discovering skills: %w", err)
	}
	if len(skillNames) == 0 {
		return fmt.Errorf("no skills found in %s", path)
	}

	if !jsonOutput {
		rootCmd.Println(`◎`, fmt.Sprintf("Found %d skill(s) in %s", len(skillNames), path))
		for _, name := range skillNames {
			rootCmd.Println(`  •`, name)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	// Build the artifact tarball from the skills directory.
	if !jsonOutput {
		rootCmd.Println(`◎`, "Building artifact...")
	}
	data, err := agentops.BuildArtifact(path, skillNames)
	if err != nil {
		return fmt.Errorf("building artifact: %w", err)
	}

	// Diff against the remote tag if requested.
	if skillsPublishArgs.diffTag != "" {
		if !jsonOutput {
			rootCmd.Println(`◎`, fmt.Sprintf("Comparing with %s:%s...", repo, skillsPublishArgs.diffTag))
		}
		if err := agentops.DiffArtifact(ctx, repo, skillsPublishArgs.diffTag, data); err != nil {
			if errors.Is(err, agentops.ErrDiffIdentical) {
				if jsonOutput {
					return printPublishResult(skillsPublishResult{
						Repository: repo,
						Tags:       skillsPublishArgs.tags,
						Skills:     skillNames,
						Skipped:    true,
					})
				}
				rootCmd.Println(`✔`, "Contents are identical, skipping push")
				return nil
			}
			return fmt.Errorf("comparing artifact: %w", err)
		}
	}

	// Push the artifact to the registry.
	if !jsonOutput {
		rootCmd.Println(`◎`, "Pushing artifact...")
	}
	digest, err := agentops.PushArtifact(ctx, repo, data, agentops.PushArtifactOptions{
		Tags:        skillsPublishArgs.tags,
		Annotations: annotations,
	})
	if err != nil {
		return err
	}

	// Sign the artifact if requested.
	signed := false
	if skillsPublishArgs.sign {
		pinnedRef := fmt.Sprintf("%s@%s", repo, digest)
		if !jsonOutput {
			rootCmd.Println(`◎`, "Signing artifact...")
		}
		if err := cosign.SignArtifact(ctx, pinnedRef); err != nil {
			return fmt.Errorf("signing artifact: %w", err)
		}
		signed = true
	}

	if jsonOutput {
		return printPublishResult(skillsPublishResult{
			Repository:  repo,
			Digest:      digest,
			Tags:        skillsPublishArgs.tags,
			Skills:      skillNames,
			Signed:      signed,
			Annotations: annotations,
		})
	}

	rootCmd.Println(`✔`, fmt.Sprintf("Pushed artifact with digest %s", digest))
	for _, tag := range skillsPublishArgs.tags {
		rootCmd.Println(`  •`, fmt.Sprintf("%s:%s", repo, tag))
	}
	if signed {
		rootCmd.Println(`✔`, "Artifact signed")
	}
	rootCmd.Println(`✔`, fmt.Sprintf("Artifact published to %s", repo))

	return nil
}

// printPublishResult marshals the result as indented JSON and prints it.
func printPublishResult(result skillsPublishResult) error {
	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling result: %w", err)
	}
	rootCmd.Println(string(output))
	return nil
}
