// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package reporter

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// pagedListClient simulates an API server that returns one page at a time.
type pagedListClient struct {
	client.Client

	pages     [][]unstructured.Unstructured
	listCalls int
}

func (c *pagedListClient) List(_ context.Context, list client.ObjectList, opts ...client.ListOption) error {
	var listOpts client.ListOptions
	listOpts.ApplyOptions(opts)

	if listOpts.Limit != listPageSize {
		return fmt.Errorf("expected list limit %d, got %d", listPageSize, listOpts.Limit)
	}

	page := 0
	if listOpts.Continue != "" {
		n, err := strconv.Atoi(listOpts.Continue)
		if err != nil {
			return fmt.Errorf("invalid continue token %q: %w", listOpts.Continue, err)
		}
		page = n
	}
	if page >= len(c.pages) {
		return fmt.Errorf("page %d out of range", page)
	}

	ul, ok := list.(*unstructured.UnstructuredList)
	if !ok {
		return fmt.Errorf("expected UnstructuredList, got %T", list)
	}
	ul.Items = c.pages[page]
	if page+1 < len(c.pages) {
		ul.SetContinue(strconv.Itoa(page + 1))
	}
	c.listCalls++
	return nil
}

func TestListFluxResourcesPaginates(t *testing.T) {
	g := NewWithT(t)

	item := func(name string) unstructured.Unstructured {
		return unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "helm.toolkit.fluxcd.io/v2",
			"kind":       "HelmRelease",
			"metadata": map[string]any{
				"name":      name,
				"namespace": "apps",
			},
		}}
	}

	kc := &pagedListClient{
		pages: [][]unstructured.Unstructured{
			{item("hr-1"), item("hr-2")},
			{item("hr-3")},
		},
	}
	r := NewFluxStatusReporter(kc, "flux", "flux-operator", "flux-system")

	var names []string
	err := r.listFluxResources(context.Background(), "helm.toolkit.fluxcd.io/v2", "HelmRelease",
		func(items []unstructured.Unstructured) {
			for _, item := range items {
				names = append(names, item.GetName())
			}
		})

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(kc.listCalls).To(Equal(2))
	g.Expect(names).To(Equal([]string{"hr-1", "hr-2", "hr-3"}))
}
