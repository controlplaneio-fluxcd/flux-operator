// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package config

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"sigs.k8s.io/yaml"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

// Load reads, validates, and applies default values to missing fields in the configuration
// for the Flux Status Page. If the filename is empty it returns the configuration object
// with default values applied.
func Load(filename string) (*fluxcdv1.WebConfigSpec, error) {
	if filename == "" {
		var confSpec fluxcdv1.WebConfigSpec
		ApplyWebConfigSpecDefaults(&confSpec)
		confSpec.Version = "no-config"
		return &confSpec, nil
	}
	b, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	conf, err := parse(b)
	if err != nil {
		return nil, fmt.Errorf("invalid configuration in web config file '%s': %w", filename, err)
	}
	conf.Version = "static-file"
	return conf, nil
}

// parse unmarshals, validates and applies default values to
// missing fields in the configuration.
func parse(b []byte) (*fluxcdv1.WebConfigSpec, error) {
	var conf fluxcdv1.WebConfig
	if err := yaml.Unmarshal(b, &conf); err != nil {
		return nil, err
	}
	if err := checkUnknownFields(b, &conf); err != nil {
		return nil, fmt.Errorf("unknown fields: %w", err)
	}
	if err := ValidateWebConfig(&conf); err != nil {
		return nil, err
	}
	ApplyWebConfigSpecDefaults(&conf.Spec)
	return &conf.Spec, nil
}

// checkUnknownFields checks for any fields in the raw YAML
// that are not defined in the WebConfig struct schema.
func checkUnknownFields(b []byte, conf *fluxcdv1.WebConfig) error {
	// Unmarshal the raw YAML into a generic map.
	var withoutSchema map[string]any
	if err := yaml.Unmarshal(b, &withoutSchema); err != nil {
		return err
	}

	// Recast the WebConfig struct back to YAML and then into a generic map.
	b, err := yaml.Marshal(conf)
	if err != nil {
		return err
	}
	var withSchema map[string]any
	if err := yaml.Unmarshal(b, &withSchema); err != nil {
		return err
	}

	// Find unknown fields.
	var unknownFields []string
	const rootPath = ""
	findUnknownFields(rootPath, withoutSchema, withSchema, &unknownFields)
	if len(unknownFields) == 0 {
		return nil
	}

	// Sort by levels and return as error.
	slices.SortFunc(unknownFields, func(a, b string) int {
		aLevels := strings.Count(a, ".") + strings.Count(a, "[")
		bLevels := strings.Count(b, ".") + strings.Count(b, "[")
		if aLevels != bLevels {
			return aLevels - bLevels
		}
		return strings.Compare(a, b)
	})
	return errors.New(strings.Join(unknownFields, ", "))
}

// findUnknownFields recursively compares two values and records any fields
// that are present in withoutSchema but missing in withSchema.
func findUnknownFields(path string, withoutSchema, withSchema any, unknownFields *[]string) {
	switch withoutSchemaTyped := withoutSchema.(type) {
	case map[string]any:
		withSchemaTyped := withSchema.(map[string]any)
		for k, withoutSchemaValue := range withoutSchemaTyped {
			keyPath := fmt.Sprintf("%s.%s", path, k)
			withSchemaValue, found := withSchemaTyped[k]
			if !found {
				*unknownFields = append(*unknownFields, keyPath)
				continue
			}
			findUnknownFields(keyPath, withoutSchemaValue, withSchemaValue, unknownFields)
		}
	case []any:
		withSchemaTyped := withSchema.([]any)
		for i := range withoutSchemaTyped {
			itemPath := fmt.Sprintf("%s[%d]", path, i)
			findUnknownFields(itemPath, withoutSchemaTyped[i], withSchemaTyped[i], unknownFields)
		}
	}
}
