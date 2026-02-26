// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package inputs_test

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/inputs"
)

func TestCombine(t *testing.T) {
	rset := &fluxcdv1.ResourceSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: fluxcdv1.GroupVersion.String(),
			Kind:       fluxcdv1.ResourceSetKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rset",
			Namespace: "default",
		},
		Spec: fluxcdv1.ResourceSetSpec{
			Inputs: []fluxcdv1.ResourceSetInput{
				{"id": mustJSON(t, inputs.ID("foo"))},
				{"id": mustJSON(t, inputs.ID("bar"))},
			},
		},
	}

	rsip := map[inputs.ProviderKey]fluxcdv1.InputProvider{
		{
			GVK: schema.GroupVersionKind{
				Group:   fluxcdv1.GroupVersion.Group,
				Version: fluxcdv1.GroupVersion.Version,
				Kind:    fluxcdv1.ResourceSetInputProviderKind,
			},
			Name:      "rsip",
			Namespace: "default",
		}: &fluxcdv1.ResourceSetInputProvider{
			TypeMeta: metav1.TypeMeta{
				APIVersion: fluxcdv1.GroupVersion.String(),
				Kind:       fluxcdv1.ResourceSetInputProviderKind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "rsip",
				Namespace: "default",
			},
			Status: fluxcdv1.ResourceSetInputProviderStatus{
				ExportedInputs: []fluxcdv1.ResourceSetInput{{
					"id": mustJSON(t, inputs.ID("baz")),
				}},
			},
		},
	}

	flattened := `
- id: "42074437"
  provider:
    apiVersion: fluxcd.controlplane.io/v1
    kind: ResourceSet
    name: rset
    namespace: default
- id: "39649590"
  provider:
    apiVersion: fluxcd.controlplane.io/v1
    kind: ResourceSet
    name: rset
    namespace: default
- id: "40173886"
  provider:
    apiVersion: fluxcd.controlplane.io/v1
    kind: ResourceSetInputProvider
    name: rsip
    namespace: default`

	permuted := `
- id: "563152006"
  rset:
    id: "42074437"
    provider:
      apiVersion: fluxcd.controlplane.io/v1
      kind: ResourceSet
      name: rset
      namespace: default
  rsip:
    id: "40173886"
    provider:
      apiVersion: fluxcd.controlplane.io/v1
      kind: ResourceSetInputProvider
      name: rsip
      namespace: default
- id: "563676295"
  rset:
    id: "39649590"
    provider:
      apiVersion: fluxcd.controlplane.io/v1
      kind: ResourceSet
      name: rset
      namespace: default
  rsip:
    id: "40173886"
    provider:
      apiVersion: fluxcd.controlplane.io/v1
      kind: ResourceSetInputProvider
      name: rsip
      namespace: default`

	t.Run("defaults to flattening", func(t *testing.T) {
		g := NewWithT(t)

		rset := rset.DeepCopy()
		rset.Spec.InputStrategy = nil

		c, err := inputs.Combine(rset, rsip)

		g.Expect(err).NotTo(HaveOccurred())

		b, err := yaml.Marshal(c)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(string(b)).To(MatchYAML(flattened))
	})

	t.Run("flattens if configured", func(t *testing.T) {
		g := NewWithT(t)

		rset := rset.DeepCopy()
		rset.Spec.InputStrategy = &fluxcdv1.InputStrategySpec{
			Name: fluxcdv1.InputStrategyFlatten,
		}

		c, err := inputs.Combine(rset, rsip)

		g.Expect(err).NotTo(HaveOccurred())

		b, err := yaml.Marshal(c)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(string(b)).To(MatchYAML(flattened))
	})

	t.Run("permutes if configured", func(t *testing.T) {
		g := NewWithT(t)

		rset := rset.DeepCopy()
		rset.Spec.InputStrategy = &fluxcdv1.InputStrategySpec{
			Name: fluxcdv1.InputStrategyPermute,
		}

		c, err := inputs.Combine(rset, rsip)

		g.Expect(err).NotTo(HaveOccurred())

		b, err := yaml.Marshal(c)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(string(b)).To(MatchYAML(permuted))
	})

	t.Run("errors on unknown input strategy", func(t *testing.T) {
		g := NewWithT(t)

		rset := rset.DeepCopy()
		rset.Spec.InputStrategy = &fluxcdv1.InputStrategySpec{
			Name: "invalid-strategy",
		}

		c, err := inputs.Combine(rset, rsip)
		g.Expect(err).To(MatchError("unknown input strategy: 'invalid-strategy'"))
		g.Expect(c).To(BeNil())
	})

	t.Run("errors if the combiner errors", func(t *testing.T) {
		g := NewWithT(t)

		rset := rset.DeepCopy()
		rset.Spec.InputStrategy = &fluxcdv1.InputStrategySpec{
			Name: fluxcdv1.InputStrategyPermute,
		}
		rset.Spec.Inputs = nil
		for range 100000 {
			rset.Spec.Inputs = append(rset.Spec.Inputs, fluxcdv1.ResourceSetInput{})
		}

		c, err := inputs.Combine(rset, rsip)
		g.Expect(err).To(MatchError("failed to get inputs from ResourceSet/rset: adding provider 'rset' with 100000 inputs would exceed the maximum allowed permutations. max: 10000, got: 100000"))
		g.Expect(c).To(BeNil())
	})

	t.Run("sorts multiple providers deterministically", func(t *testing.T) {
		g := NewWithT(t)

		rset := rset.DeepCopy()
		rset.Spec.InputStrategy = &fluxcdv1.InputStrategySpec{
			Name: fluxcdv1.InputStrategyFlatten,
		}
		rset.Spec.Inputs = nil

		gvk := schema.GroupVersionKind{
			Group:   fluxcdv1.GroupVersion.Group,
			Version: fluxcdv1.GroupVersion.Version,
			Kind:    fluxcdv1.ResourceSetInputProviderKind,
		}

		providerMap := map[inputs.ProviderKey]fluxcdv1.InputProvider{
			{GVK: gvk, Name: "z-provider", Namespace: "default"}: &fluxcdv1.ResourceSetInputProvider{
				TypeMeta: metav1.TypeMeta{
					APIVersion: fluxcdv1.GroupVersion.String(),
					Kind:       fluxcdv1.ResourceSetInputProviderKind,
				},
				ObjectMeta: metav1.ObjectMeta{Name: "z-provider", Namespace: "default"},
				Status: fluxcdv1.ResourceSetInputProviderStatus{
					ExportedInputs: []fluxcdv1.ResourceSetInput{{"id": mustJSON(t, "z")}},
				},
			},
			{GVK: gvk, Name: "a-provider", Namespace: "default"}: &fluxcdv1.ResourceSetInputProvider{
				TypeMeta: metav1.TypeMeta{
					APIVersion: fluxcdv1.GroupVersion.String(),
					Kind:       fluxcdv1.ResourceSetInputProviderKind,
				},
				ObjectMeta: metav1.ObjectMeta{Name: "a-provider", Namespace: "default"},
				Status: fluxcdv1.ResourceSetInputProviderStatus{
					ExportedInputs: []fluxcdv1.ResourceSetInput{{"id": mustJSON(t, "a")}},
				},
			},
			{GVK: gvk, Name: "a-provider", Namespace: "other"}: &fluxcdv1.ResourceSetInputProvider{
				TypeMeta: metav1.TypeMeta{
					APIVersion: fluxcdv1.GroupVersion.String(),
					Kind:       fluxcdv1.ResourceSetInputProviderKind,
				},
				ObjectMeta: metav1.ObjectMeta{Name: "a-provider", Namespace: "other"},
				Status: fluxcdv1.ResourceSetInputProviderStatus{
					ExportedInputs: []fluxcdv1.ResourceSetInput{{"id": mustJSON(t, "a-other")}},
				},
			},
		}

		c1, err := inputs.Combine(rset, providerMap)
		g.Expect(err).NotTo(HaveOccurred())

		c2, err := inputs.Combine(rset, providerMap)
		g.Expect(err).NotTo(HaveOccurred())

		b1, err := yaml.Marshal(c1)
		g.Expect(err).NotTo(HaveOccurred())
		b2, err := yaml.Marshal(c2)
		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(string(b1)).To(Equal(string(b2)))
		g.Expect(c1).To(HaveLen(3))
	})
}
