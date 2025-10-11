// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

func TestDeleteResourceSetCmd(t *testing.T) {
	tests := []struct {
		name         string
		nsName       string
		wait         bool
		withSuspend  bool
		timeout      time.Duration
		expectError  bool
		expectOutput string
	}{
		{
			name:         "delete resourceset without wait",
			nsName:       "test-delete-rset-nowait",
			wait:         false,
			withSuspend:  false,
			timeout:      30 * time.Second,
			expectError:  false,
			expectOutput: "Deletion initiated",
		},
		{
			name:         "delete resourceset with wait",
			nsName:       "test-delete-rset-wait",
			wait:         true,
			withSuspend:  false,
			timeout:      30 * time.Second,
			expectError:  false,
			expectOutput: "Deletion completed",
		},
		{
			name:         "delete resourceset with suspend",
			nsName:       "test-delete-rset-suspend",
			wait:         false,
			withSuspend:  true,
			timeout:      30 * time.Second,
			expectError:  false,
			expectOutput: "Reconciliation suspended",
		},
		{
			name:         "delete resourceset with suspend and wait",
			nsName:       "test-delete-rset-suspend-wait",
			wait:         true,
			withSuspend:  true,
			timeout:      30 * time.Second,
			expectError:  false,
			expectOutput: "Deletion completed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			ns, err := testEnv.CreateNamespace(ctx, tt.nsName)
			g.Expect(err).ToNot(HaveOccurred())

			// Create resourceset
			resourceSet := &fluxcdv1.ResourceSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-resourceset",
					Namespace: ns.Name,
				},
				Spec: fluxcdv1.ResourceSetSpec{
					Resources: []*apiextensionsv1.JSON{
						{
							Raw: []byte(`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"test"}}`),
						},
					},
				},
			}
			g.Expect(testClient.Create(ctx, resourceSet)).To(Succeed())

			// Ensure cleanup
			defer func() {
				_ = testClient.Delete(ctx, resourceSet)
			}()

			// Execute delete command
			rootArgs.timeout = tt.timeout
			kubeconfigArgs.Namespace = &ns.Name
			deleteResourceSetArgs.wait = tt.wait
			deleteResourceSetArgs.withSuspend = tt.withSuspend

			args := []string{"delete", "resourceset", "test-resourceset"}
			if !tt.wait {
				args = append(args, "--wait=false")
			}
			if tt.withSuspend {
				args = append(args, "--with-suspend")
			}

			output, err := executeCommand(args)

			// Check expectations
			if tt.expectError {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(output).To(ContainSubstring(tt.expectOutput))
			}

			// Verify resourceset was deleted
			err = testClient.Get(ctx, client.ObjectKeyFromObject(resourceSet), resourceSet)
			g.Expect(apierrors.IsNotFound(err)).To(BeTrue())

			if tt.withSuspend {
				g.Expect(output).To(ContainSubstring("Reconciliation suspended"))
			}
		})
	}
}

func TestDeleteResourceSetCmd_NotFound(t *testing.T) {
	g := NewWithT(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test-delete-rset-notfound")
	g.Expect(err).ToNot(HaveOccurred())

	// Execute delete command for non-existent resourceset
	rootArgs.timeout = 30 * time.Second
	kubeconfigArgs.Namespace = &ns.Name
	deleteResourceSetArgs.wait = false
	deleteResourceSetArgs.withSuspend = false

	_, err = executeCommand([]string{"delete", "resourceset", "nonexistent"})

	// Should return error
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("not found"))
}

func TestDeleteResourceSetCmd_WithSuspendNotFound(t *testing.T) {
	g := NewWithT(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test-delete-rset-suspend-notfound")
	g.Expect(err).ToNot(HaveOccurred())

	// Execute delete command with --with-suspend for non-existent resourceset
	kubeconfigArgs.Namespace = &ns.Name
	deleteResourceSetArgs.wait = false
	deleteResourceSetArgs.withSuspend = true

	_, err = executeCommand([]string{"delete", "resourceset", "nonexistent", "--with-suspend"})

	// Should return error during suspend phase
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("unable to"))
}

func TestDeleteResourceSetCmd_Alias(t *testing.T) {
	g := NewWithT(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test-delete-rset-alias")
	g.Expect(err).ToNot(HaveOccurred())

	// Create resourceset
	resourceSet := &fluxcdv1.ResourceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-resourceset",
			Namespace: ns.Name,
		},
		Spec: fluxcdv1.ResourceSetSpec{
			Resources: []*apiextensionsv1.JSON{
				{
					Raw: []byte(`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"test"}}`),
				},
			},
		},
	}
	g.Expect(testClient.Create(ctx, resourceSet)).To(Succeed())

	// Ensure cleanup
	defer func() {
		_ = testClient.Delete(ctx, resourceSet)
	}()

	// Execute delete command using alias
	kubeconfigArgs.Namespace = &ns.Name
	deleteResourceSetArgs.wait = false
	deleteResourceSetArgs.withSuspend = false

	output, err := executeCommand([]string{"delete", "rset", "test-resourceset", "--wait=false"})

	// Should succeed
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(output).To(ContainSubstring("Deletion initiated"))

	// Verify resourceset was deleted
	err = testClient.Get(ctx, client.ObjectKeyFromObject(resourceSet), resourceSet)
	g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
}
