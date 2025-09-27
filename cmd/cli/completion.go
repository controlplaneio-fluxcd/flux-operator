// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
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

// fluxKinds contains all Flux resource kinds.
var fluxKinds = slices.Concat(fluxcdv1.FluxOperatorKinds, fluxcdv1.FluxKinds)

// getFluxKinds returns a list of Flux kind names, optionally filtered by reconcilable status.
func getFluxKinds(reconcilableOnly bool) []string {
	var kinds []string
	for _, kind := range fluxKinds {
		if !reconcilableOnly || kind.Reconcilable {
			kinds = append(kinds, kind.Name+"/")
		}
	}
	return kinds
}

// findFluxKind searches for a Flux kind in a case-insensitive way and returns the proper casing.
// Returns an error if the kind is not found in the fluxKinds list.
func findFluxKind(kind string) (string, error) {
	for _, fluxKind := range fluxKinds {
		if strings.EqualFold(fluxKind.Name, kind) {
			return fluxKind.Name, nil
		}
		if strings.EqualFold(fluxKind.ShortName, kind) {
			return fluxKind.Name, nil
		}
	}
	return "", fmt.Errorf("kind %s not found", kind)
}

// resourceKindCompletionFunc returns a function that provides completion for resource kinds.
func resourceKindCompletionFunc(reconcilableOnly bool) func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		var comps []string
		for _, kind := range fluxKinds {
			if !reconcilableOnly || kind.Reconcilable {
				if strings.HasPrefix(kind.Name, toComplete) {
					comps = append(comps, kind.Name)
				}
			}
		}
		return comps, cobra.ShellCompDirectiveNoFileComp
	}
}

// resourceKindNameCompletionFunc returns a function that provides completion for <kind>/<name> format.
// If the input doesn't contain a slash, it returns available kinds.
// If the input contains a slash with a kind, it returns resource names for that kind.
func resourceKindNameCompletionFunc(reconcilableOnly bool) func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		// If input doesn't contain a slash, return kinds
		if !strings.Contains(toComplete, "/") {
			kinds := getFluxKinds(reconcilableOnly)
			var comps []string
			for _, kind := range kinds {
				if strings.HasPrefix(kind, toComplete) {
					comps = append(comps, kind)
				}
			}
			return comps, cobra.ShellCompDirectiveNoFileComp
		}

		// If input contains a slash, extract the kind and return resource names
		parts := strings.Split(toComplete, "/")
		if len(parts) != 2 {
			return nil, cobra.ShellCompDirectiveError
		}

		kind := parts[0]
		namePrefix := parts[1]

		// Get the GVK for the kind
		gvk, err := preferredFluxGVK(kind, kubeconfigArgs)
		if err != nil {
			return completionError(err)
		}

		// Get resource names from the cluster
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
			fullName := kind + "/" + name

			if strings.HasPrefix(name, namePrefix) {
				comps = append(comps, fullName)
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
