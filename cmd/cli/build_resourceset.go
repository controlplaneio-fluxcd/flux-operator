// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	ssautil "github.com/fluxcd/pkg/ssa/utils"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/yaml"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/builder"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/inputs"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var buildResourceSetCmd = &cobra.Command{
	Use:     "resourceset",
	Aliases: []string{"rset"},
	Short:   "Build ResourceSet templates to Kubernetes objects",
	Example: `  # Build the given ResourceSet and print the generated objects
  flux-operator build resourceset -f my-resourceset.yaml

  # Build a ResourceSet by providing the inputs from a static ResourceSetInputProvider
  flux-operator build resourceset -f my-resourceset.yaml \
    --inputs-from-provider my-resourcesetinputprovider.yaml

  # Build a ResourceSet by providing the inputs from a file
  flux-operator build resourceset -f my-resourceset.yaml \
    --inputs-from my-resourceset-inputs.yaml

  # Pipe the ResourceSet manifest to the build command
  cat my-resourceset.yaml | flux-operator build rset -f -

  # Build a ResourceSet and print a diff of the generated objects
  flux-operator build resourceset -f my-resourceset.yaml | \
    kubectl diff --server-side --field-manager=flux-operator -f -
`,
	Args: cobra.NoArgs,
	RunE: buildResourceSetCmdRun,
}

type buildResourceSetFlags struct {
	filename           string
	inputsFrom         string
	inputsFromProvider string
}

var buildResourceSetArgs buildResourceSetFlags

func init() {
	buildResourceSetCmd.Flags().StringVarP(&buildResourceSetArgs.filename, "filename", "f", "", "Path to the ResourceSet YAML manifest.")
	buildResourceSetCmd.Flags().StringVarP(&buildResourceSetArgs.inputsFrom, "inputs-from", "i", "", "Path to the ResourceSet inputs YAML manifest.")
	buildResourceSetCmd.Flags().StringVar(&buildResourceSetArgs.inputsFromProvider, "inputs-from-provider", "", "Path to a file containing YAML manifests of ResourceSetInputProvider objects of type Static.")

	buildCmd.AddCommand(buildResourceSetCmd)
}

func buildResourceSetCmdRun(cmd *cobra.Command, args []string) error {
	if buildResourceSetArgs.filename == "" {
		return errors.New("--filename is required")
	}

	path := buildResourceSetArgs.filename
	var err error
	if buildResourceSetArgs.filename == "-" {
		path, err = saveReaderToFile(os.Stdin)
		if err != nil {
			return err
		}

		defer os.Remove(path)
	}

	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("invalid filename '%s', must point to an existing file", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("error reading file: %w", err)
	}

	var rset fluxcdv1.ResourceSet
	err = yaml.Unmarshal(data, &rset)
	if err != nil {
		return fmt.Errorf("error parsing ResourceSet: %w", err)
	}

	// Ensure correct GVK.
	rset.SetGroupVersionKind(fluxcdv1.GroupVersion.WithKind(fluxcdv1.ResourceSetKind))

	// Ensure non-empty namespace.
	if rset.GetNamespace() == "" {
		rset.SetNamespace(*kubeconfigArgs.Namespace)
	}
	if rset.GetNamespace() == "" {
		return fmt.Errorf("ResourceSet namespace must be set either in the manifest or with --namespace")
	}

	// Ensure non-empty name.
	if rset.GetName() == "" {
		return fmt.Errorf("ResourceSet name must be set in the manifest")
	}

	if len(rset.Spec.InputsFrom) > 0 &&
		buildResourceSetArgs.inputsFrom == "" &&
		buildResourceSetArgs.inputsFromProvider == "" {
		return fmt.Errorf("ResourceSet has '.spec.inputsFrom', please provide the inputs with --inputs-from or --inputs-from-provider")
	}

	if buildResourceSetArgs.inputsFrom != "" {
		inputsData, err := os.ReadFile(buildResourceSetArgs.inputsFrom)
		if err != nil {
			return fmt.Errorf("error reading inputs file: %w", err)
		}

		if err := yaml.Unmarshal(inputsData, &rset.Spec.Inputs); err != nil {
			return fmt.Errorf("error parsing inputs file: %w", err)
		}
	}

	var providerMap map[inputs.ProviderKey]fluxcdv1.InputProvider

	if buildResourceSetArgs.inputsFromProvider != "" {
		providers, err := loadProvidersFromFile(&rset, buildResourceSetArgs.inputsFromProvider)
		if err != nil {
			return fmt.Errorf("error loading providers from file: %w", err)
		}

		providerMap, err = filterProviders(&rset, providers)
		if err != nil {
			return fmt.Errorf("error filtering providers: %w", err)
		}
	}

	combinedInputs, err := inputs.Combine(&rset, providerMap)
	if err != nil {
		return fmt.Errorf("error combining inputs: %w", err)
	}

	objects, err := builder.BuildResourceSet(rset.Spec.ResourcesTemplate, rset.Spec.Resources, combinedInputs)
	if err != nil {
		return err
	}

	if len(objects) == 0 {
		return fmt.Errorf("no objects were generated")
	}

	if rset.Spec.CommonMetadata != nil {
		ssautil.SetCommonMetadata(objects, rset.Spec.CommonMetadata.Labels, rset.Spec.CommonMetadata.Annotations)
	}

	ownerGroup := fmt.Sprintf("resourceset.%s", fluxcdv1.GroupVersion.Group)
	ssautil.SetCommonMetadata(objects, map[string]string{
		fmt.Sprintf("%s/name", ownerGroup):      rset.GetName(),
		fmt.Sprintf("%s/namespace", ownerGroup): rset.GetNamespace(),
	}, nil)

	for _, obj := range objects {
		var strBuilder strings.Builder
		strBuilder.WriteString("---\n")
		yml, ymlErr := yaml.Marshal(obj)
		if ymlErr != nil {
			return fmt.Errorf("error marshalling object: %w", ymlErr)
		}
		strBuilder.Write(yml)
		rootCmd.Print(strBuilder.String())
	}

	return nil
}

func saveReaderToFile(reader io.Reader) (string, error) {
	b, err := io.ReadAll(bufio.NewReader(reader))
	if err != nil {
		return "", err
	}
	b = bytes.TrimRight(b, "\r\n")
	f, err := os.CreateTemp("", "*.yaml")
	if err != nil {
		return "", fmt.Errorf("unable to create temp dir for stdin")
	}

	defer f.Close()

	if _, err := f.Write(b); err != nil {
		return "", fmt.Errorf("error writing stdin to file: %w", err)
	}

	return f.Name(), nil
}

func loadProvidersFromFile(rset *fluxcdv1.ResourceSet, path string) ([]*fluxcdv1.ResourceSetInputProvider, error) {
	rsipGVK := fluxcdv1.GroupVersion.WithKind(fluxcdv1.ResourceSetInputProviderKind)
	rsetNamespace := rset.GetNamespace()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	documents := bytes.Split(data, []byte("---"))
	providers := make([]*fluxcdv1.ResourceSetInputProvider, 0)

	for _, doc := range documents {
		if len(bytes.TrimSpace(doc)) == 0 {
			continue
		}

		var provider fluxcdv1.ResourceSetInputProvider
		if err := yaml.Unmarshal(doc, &provider); err != nil {
			return nil, fmt.Errorf("error parsing inputs provider file: %w", err)
		}

		// Ensure correct GVK.
		provider.SetGroupVersionKind(rsipGVK)

		// Ensure non-empty namespace.
		if provider.GetNamespace() == "" {
			provider.SetNamespace(rsetNamespace)
		}

		// Ignore providers from different namespaces.
		if provider.GetNamespace() != rsetNamespace {
			continue
		}

		// Ensure non-empty name.
		if provider.GetName() == "" {
			return nil, fmt.Errorf("ResourceSetInputProvider name must be set in the manifest")
		}

		if provider.Spec.Type != fluxcdv1.InputProviderStatic {
			return nil, fmt.Errorf("unsupported provider type '%s', only '%s' is supported",
				provider.Spec.Type,
				fluxcdv1.InputProviderStatic)
		}

		vals := provider.Spec.DefaultValues
		if len(vals) == 0 {
			vals = make(fluxcdv1.ResourceSetInput)
		}

		// Note: In runtime the operator uses the RSIP UID provided by the Kubernetes API server.
		// Since here we are loading from a file, we cannot use the UID to compute the export ID.
		jsonID, err := inputs.JSON(inputs.ID(provider.GetName()))
		if err != nil {
			return nil, err
		}
		vals["id"] = jsonID

		provider.Status.ExportedInputs = []fluxcdv1.ResourceSetInput{vals}

		providers = append(providers, &provider)
	}

	return providers, nil
}

func filterProviders(rset *fluxcdv1.ResourceSet,
	allProviders []*fluxcdv1.ResourceSetInputProvider,
) (map[inputs.ProviderKey]fluxcdv1.InputProvider, error) {

	rsipGVK := fluxcdv1.GroupVersion.WithKind(fluxcdv1.ResourceSetInputProviderKind)

	providerMap := make(map[inputs.ProviderKey]fluxcdv1.InputProvider)
	for _, p := range allProviders {
		providerMap[inputs.NewProviderKey(p)] = p
	}

	filtered := make(map[inputs.ProviderKey]fluxcdv1.InputProvider)
	for _, inputSource := range rset.Spec.InputsFrom {
		switch {
		case inputSource.Name != "":
			key := inputs.ProviderKey{
				GVK:       rsipGVK,
				Name:      inputSource.Name,
				Namespace: rset.GetNamespace(),
			}
			if _, ok := filtered[key]; ok {
				continue
			}
			if p, ok := providerMap[key]; ok {
				filtered[key] = p
			}
		case inputSource.Selector != nil:
			selector, err := metav1.LabelSelectorAsSelector(inputSource.Selector)
			if err != nil {
				return nil, fmt.Errorf("failed to parse selector: %w", err)
			}
			for _, p := range allProviders {
				key := inputs.NewProviderKey(p)
				if _, ok := filtered[key]; ok {
					continue
				}
				if selector.Matches(labels.Set(p.GetLabels())) {
					filtered[key] = p
				}
			}
		default:
			return nil, errors.New("input provider reference must have either name or selector set")
		}
	}

	return filtered, nil
}
