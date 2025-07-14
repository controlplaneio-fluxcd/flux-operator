// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/yaml"
)

var createSecretCmd = &cobra.Command{
	Use:   "secret",
	Short: "Create Kubernetes Secret resources",
}

func init() {
	createCmd.AddCommand(createSecretCmd)
}

func printSecret(secret *corev1.Secret) error {
	secret.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Secret"))
	output, err := yaml.Marshal(secret)
	if err != nil {
		return fmt.Errorf("unable to marshal Secret to YAML: %w", err)
	}

	output = bytes.Replace(output, []byte("  creationTimestamp: null\n"), []byte(""), 1)
	output = []byte("---\n" + string(output))

	_, err = rootCmd.OutOrStdout().Write(output)
	return err
}

func setSecretMetadata(secret *corev1.Secret, annotations, labels []string) error {
	// parse annotations
	if len(annotations) > 0 {
		parsedAnnotations, err := parseMetadata(annotations)
		if err != nil {
			return fmt.Errorf("invalid annotations: %w", err)
		}
		if secret.ObjectMeta.Annotations == nil {
			secret.ObjectMeta.Annotations = make(map[string]string)
		}
		for k, v := range parsedAnnotations {
			secret.ObjectMeta.Annotations[k] = v
		}
	}

	// parse labels
	if len(labels) > 0 {
		parsedLabels, err := parseMetadata(labels)
		if err != nil {
			return fmt.Errorf("invalid labels: %w", err)
		}
		if secret.ObjectMeta.Labels == nil {
			secret.ObjectMeta.Labels = make(map[string]string)
		}
		for k, v := range parsedLabels {
			secret.ObjectMeta.Labels[k] = v
		}
	}

	return nil
}

// parseMetadata parses a slice of strings in the format "key=value" into a map.
func parseMetadata(meta []string) (map[string]string, error) {
	result := make(map[string]string)
	for _, label := range meta {
		parts := strings.Split(label, "=")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid format '%s', must be key=value", label)
		}

		// validate name
		if errors := validation.IsQualifiedName(parts[0]); len(errors) > 0 {
			return nil, fmt.Errorf("invalid '%s': %v", parts[0], errors)
		}

		// validate value
		if errors := validation.IsValidLabelValue(parts[1]); len(errors) > 0 {
			return nil, fmt.Errorf("invalid value '%s': %v", parts[1], errors)
		}

		result[parts[0]] = parts[1]
	}

	return result, nil
}
