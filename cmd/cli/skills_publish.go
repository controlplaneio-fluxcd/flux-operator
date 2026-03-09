// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
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

  # Publish latest tag with custom annotations
  flux-operator skills publish ghcr.io/my-org/agent-skills \
    --path ./my-skills \
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

	skillsCmd.AddCommand(skillsPublishCmd)
}

func skillsPublishCmdRun(cmd *cobra.Command, args []string) error {
	repo := agentops.NormalizeRepository(args[0])
	path := skillsPublishArgs.path

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

	rootCmd.Println(`◎`, fmt.Sprintf("Found %d skill(s) in %s", len(skillNames), path))
	for _, name := range skillNames {
		rootCmd.Println(`  •`, name)
	}

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	// Push the artifact.
	rootCmd.Println(`◎`, "Pushing artifact...")
	digest, err := agentops.PushArtifact(ctx, repo, path, agentops.PushArtifactOptions{
		Tags:        skillsPublishArgs.tags,
		Annotations: annotations,
		SkillNames:  skillNames,
	})
	if err != nil {
		return err
	}

	rootCmd.Println(`✔`, fmt.Sprintf("Pushed artifact with digest %s", digest))
	for _, tag := range skillsPublishArgs.tags {
		rootCmd.Println(`  •`, fmt.Sprintf("%s:%s", repo, tag))
	}

	// Sign the artifact if requested.
	if skillsPublishArgs.sign {
		pinnedRef := fmt.Sprintf("%s@%s", repo, digest)
		rootCmd.Println(`◎`, "Signing artifact...")
		if err := cosign.SignArtifact(ctx, pinnedRef); err != nil {
			return fmt.Errorf("signing artifact: %w", err)
		}
		rootCmd.Println(`✔`, "Artifact signed")
	}

	rootCmd.Println(`✔`, fmt.Sprintf("Artifact published to %s", repo))

	return nil
}
