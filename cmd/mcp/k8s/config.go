// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package k8s

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"k8s.io/client-go/tools/clientcmd"
)

// KubeConfig represents a thread-safe configuration for
// managing Kubernetes contexts and clusters.
type KubeConfig struct {
	mx       sync.Mutex
	contexts []KubeConfigContext
}

// KubeConfigContext represents a Kubernetes context with
// its associated cluster and current selection status.
type KubeConfigContext struct {
	ClusterName    string `json:"cluster"`
	ContextName    string `json:"context"`
	CurrentContext bool   `json:"current"`
}

// NewKubeConfig initializes a new instance of KubeConfig
// with an empty list of contexts.
func NewKubeConfig() *KubeConfig {
	return &KubeConfig{
		contexts: []KubeConfigContext{},
	}
}

// Load loads and updates Kubernetes configuration contexts based on the KUBECONFIG environment variable.
// It ensures thread safety and preserves the current context if it exists in the new configuration.
// Returns an error if KUBECONFIG is not set or if there is an issue loading the configuration.
func (c *KubeConfig) Load() error {
	c.mx.Lock()
	defer c.mx.Unlock()

	configPaths := os.Getenv("KUBECONFIG")
	if configPaths == "" {
		return fmt.Errorf("KUBECONFIG environment variable not set")
	}

	paths := filepath.SplitList(configPaths)

	var newContexts []KubeConfigContext
	config, err := clientcmd.LoadFromFile(paths[0])
	if err != nil {
		return err
	}

	for name, ct := range config.Contexts {
		kubeCtx := KubeConfigContext{
			ContextName: name,
			ClusterName: ct.Cluster,
		}
		if name == config.CurrentContext {
			kubeCtx.CurrentContext = true
		}
		newContexts = append(newContexts, kubeCtx)
	}

	if len(c.contexts) > 0 {
		currentContextName := ""
		for i := range c.contexts {
			if c.contexts[i].CurrentContext {
				currentContextName = c.contexts[i].ContextName
				break
			}
		}

		currentContextExists := false
		for i := range newContexts {
			if newContexts[i].ContextName == currentContextName {
				currentContextExists = true
				break
			}
		}

		if currentContextExists {
			for i := range newContexts {
				newContexts[i].CurrentContext = false
			}
			for i := range newContexts {
				if newContexts[i].ContextName == currentContextName {
					newContexts[i].CurrentContext = true
					break
				}
			}
		}
	}

	c.contexts = newContexts
	return nil
}

// Contexts returns a slice of all Kubernetes contexts
// currently loaded in the KubeConfig instance.
func (c *KubeConfig) Contexts() []KubeConfigContext {
	c.mx.Lock()
	defer c.mx.Unlock()

	return c.contexts
}

// SetCurrentContext sets the specified context as the current context in the KubeConfig.
// It returns an error if the context with the given name does not exist.
// This function does not change the kubeconfig file.
func (c *KubeConfig) SetCurrentContext(name string) error {
	c.mx.Lock()
	defer c.mx.Unlock()

	found := false
	for i := range c.contexts {
		if c.contexts[i].ContextName == name {
			found = true
			c.contexts[i].CurrentContext = true
		} else {
			c.contexts[i].CurrentContext = false
		}
	}

	if !found {
		return fmt.Errorf("context %s not found", name)
	}
	return nil
}
