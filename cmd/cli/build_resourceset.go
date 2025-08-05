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
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/json"
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
	buildResourceSetCmd.Flags().StringVar(&buildResourceSetArgs.inputsFromProvider, "inputs-from-provider", "", "Path to the ResourceSetInputProvider static type YAML manifest.")

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

	if buildResourceSetArgs.inputsFromProvider != "" {
		providers, err := loadProvidersFromFile(buildResourceSetArgs.inputsFromProvider)
		if err != nil {
			return fmt.Errorf("error loading providers from file: %w", err)
		}

		filteredProviders, err := filterProviders(rset.Spec.InputsFrom, providers)
		if err != nil {
			return fmt.Errorf("error filtering providers: %w", err)
		}

		if err := appendInputs(&rset, filteredProviders); err != nil {
			return fmt.Errorf("failed to append inputs: %w", err)
		}
	}

	providerInputs, err := rset.GetInputs()
	if err != nil {
		return fmt.Errorf("error reading '.spec.inputs': %w", err)
	}

	for _, input := range providerInputs {
		inputs.AddProviderReference(input, &rset)
	}

	objects, err := builder.BuildResourceSet(rset.Spec.ResourcesTemplate, rset.Spec.Resources, providerInputs)
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

func loadProvidersFromFile(path string) ([]*fluxcdv1.ResourceSetInputProvider, error) {
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

		if provider.Spec.Type != fluxcdv1.InputProviderStatic {
			return nil, fmt.Errorf("unsupported provider type '%s', only '%s' is supported",
				provider.Spec.Type,
				fluxcdv1.InputProviderStatic)
		}

		providers = append(providers, &provider)
	}

	return providers, nil
}

func filterProviders(inputSources []fluxcdv1.InputProviderReference, allProviders []*fluxcdv1.ResourceSetInputProvider) ([]*fluxcdv1.ResourceSetInputProvider, error) {
	filtered := make([]*fluxcdv1.ResourceSetInputProvider, 0, len(inputSources))
	providerByName := make(map[string]*fluxcdv1.ResourceSetInputProvider, len(allProviders))

	for _, p := range allProviders {
		providerByName[p.Name] = p
	}

	for _, inputSource := range inputSources {
		if inputSource.Name != "" {
			if provider, ok := providerByName[inputSource.Name]; ok {
				filtered = append(filtered, provider)
			}
			continue
		}

		if inputSource.Selector != nil {
			selector, err := metav1.LabelSelectorAsSelector(inputSource.Selector)
			if err != nil {
				return nil, fmt.Errorf("failed to parse selector: %w", err)
			}

			for _, provider := range allProviders {
				if selector.Matches(labels.Set(provider.Labels)) {
					filtered = append(filtered, provider)
				}
			}
		}
	}

	return filtered, nil
}

func appendInputs(rset *fluxcdv1.ResourceSet, providers []*fluxcdv1.ResourceSetInputProvider) error {
	for _, provider := range providers {
		// copy defaultValues from the provider to the ResourceSet inputs
		if len(provider.Spec.DefaultValues) > 0 {
			if rset.Spec.Inputs == nil {
				rset.Spec.Inputs = []fluxcdv1.ResourceSetInput{}
			}
			vals := provider.Spec.DefaultValues

			// compute the provider ID and add it to the inputs
			b, err := json.Marshal(inputs.ID(provider.GetName()))
			if err != nil {
				return fmt.Errorf("failed to compute provider ID: %w", err)
			}
			vals["id"] = &apiextensionsv1.JSON{Raw: b}
			rset.Spec.Inputs = append(rset.Spec.Inputs, vals)
		}
	}

	return nil
}
