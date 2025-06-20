// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

var completionCmd = &cobra.Command{
	Use:   "completion",
	Short: "Generates completion scripts for various shells",
	Long:  "The completion sub-command generates completion scripts for various shells",
}

func init() {
	rootCmd.AddCommand(completionCmd)
}

// resourceNamesCompletionFunc returns a function that can be used as a completion function for resource names.
func resourceNamesCompletionFunc(gvk schema.GroupVersionKind) func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
		defer cancel()

		cfg, err := kubeconfigArgs.ToRESTConfig()
		if err != nil {
			return completionError(err)
		}

		mapper, err := kubeconfigArgs.ToRESTMapper()
		if err != nil {
			return completionError(err)
		}

		mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			return completionError(err)
		}

		kubeClient, err := dynamic.NewForConfig(cfg)
		if err != nil {
			return completionError(err)
		}

		var dr dynamic.ResourceInterface
		if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
			dr = kubeClient.Resource(mapping.Resource).Namespace(*kubeconfigArgs.Namespace)
		} else {
			dr = kubeClient.Resource(mapping.Resource)
		}

		list, err := dr.List(ctx, metav1.ListOptions{})
		if err != nil {
			return completionError(err)
		}

		var comps []string

		for _, item := range list.Items {
			name := item.GetName()

			if strings.HasPrefix(name, toComplete) {
				comps = append(comps, name)
			}
		}

		return comps, cobra.ShellCompDirectiveNoFileComp
	}
}

// resourceKindCompletionFunc returns a function that can be used as a completion function for Flux resource kinds.
func resourceKindCompletionFunc() func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		// These are the Flux kinds that react to the reconcileAt annotation.
		kinds := []string{
			"GitRepository/",
			"OCIRepository/",
			"Bucket/",
			"HelmRepository/",
			"HelmChart/",
			"HelmRelease/",
			"Kustomization/",
			"ImageRepository/",
			"Receiver/",
		}

		var comps []string
		for _, kind := range kinds {
			if strings.HasPrefix(kind, toComplete) {
				comps = append(comps, kind)
			}
		}

		return comps, cobra.ShellCompDirectiveNoFileComp
	}
}

// completionError is a helper function to handle errors in completion functions.
func completionError(err error) ([]string, cobra.ShellCompDirective) {
	cobra.CompError(err.Error())
	return nil, cobra.ShellCompDirectiveError
}
