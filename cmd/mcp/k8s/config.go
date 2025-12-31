// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package k8s

import (
	"fmt"
	"slices"
	"strings"

	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// KubeConfig represents a configuration for managing Kubernetes
// contexts and clusters. This must be kept thread-safe.
type KubeConfig struct {
	CurrentContextName string
	flags              *genericclioptions.ConfigFlags
}

// KubeConfigContext represents a Kubernetes context with
// its associated cluster and current selection status.
type KubeConfigContext struct {
	ClusterName    string `json:"cluster"`
	ContextName    string `json:"context"`
	CurrentContext bool   `json:"current"`
}

// NewKubeConfig initializes a new instance of KubeConfig
// set to the default context.
func NewKubeConfig(flags *genericclioptions.ConfigFlags) *KubeConfig {
	return &KubeConfig{
		CurrentContextName: "",
		flags:              flags,
	}
}

// Contexts returns a slice of all Kubernetes contexts
// currently loaded in the kubeconfig files.
func (c *KubeConfig) Contexts() ([]KubeConfigContext, error) {
	rawConfig, err := c.flags.ToRawKubeConfigLoader().RawConfig()
	if err != nil {
		return nil, fmt.Errorf("loading kubeconfig failed: %w", err)
	}

	var currentContext string
	if c.CurrentContextName != "" {
		currentContext = c.CurrentContextName
	} else {
		currentContext = rawConfig.CurrentContext
	}

	contexts := make([]KubeConfigContext, 0)
	for name, ct := range rawConfig.Contexts {
		kubeCtx := KubeConfigContext{
			ContextName: name,
			ClusterName: ct.Cluster,
		}
		if name == currentContext {
			kubeCtx.CurrentContext = true
		}
		contexts = append(contexts, kubeCtx)
	}

	slices.SortFunc(contexts, func(a, b KubeConfigContext) int {
		return strings.Compare(a.ContextName, b.ContextName)
	})
	return contexts, nil
}

// SetCurrentContext sets the specified context as the current context in the KubeConfig.
// It returns an error if the context with the given name does not exist.
// This function does not change the kubeconfig file.
func (c *KubeConfig) SetCurrentContext(name string) error {
	rawConfig, err := c.flags.ToRawKubeConfigLoader().RawConfig()
	if err != nil {
		return fmt.Errorf("loading kubeconfig failed: %w", err)
	}

	// Validate that the context actually exists before switching
	if _, exists := rawConfig.Contexts[name]; !exists {
		return fmt.Errorf("context %s not found", name)
	}
	c.CurrentContextName = name
	return nil
}
