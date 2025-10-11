// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

func TestDeleteInputProviderCmd(t *testing.T) {
	tests := []struct {
		name         string
		nsName       string
		wait         bool
		timeout      time.Duration
		expectError  bool
		expectOutput string
	}{
		{
			name:         "delete inputprovider without wait",
			nsName:       "test-delete-rsip-nowait",
			wait:         false,
			timeout:      30 * time.Second,
			expectError:  false,
			expectOutput: "Deletion initiated",
		},
		{
			name:         "delete inputprovider with wait",
			nsName:       "test-delete-rsip-wait",
			wait:         true,
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

			// Create inputprovider
			inputProvider := &fluxcdv1.ResourceSetInputProvider{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-inputprovider",
					Namespace: ns.Name,
				},
				Spec: fluxcdv1.ResourceSetInputProviderSpec{
					Type: fluxcdv1.InputProviderStatic,
				},
			}
			g.Expect(testClient.Create(ctx, inputProvider)).To(Succeed())

			// Ensure cleanup
			defer func() {
				_ = testClient.Delete(ctx, inputProvider)
			}()

			// Execute delete command
			rootArgs.timeout = tt.timeout
			kubeconfigArgs.Namespace = &ns.Name
			deleteInputProviderArgs.wait = tt.wait

			args := []string{"delete", "inputprovider", "test-inputprovider"}
			if !tt.wait {
				args = append(args, "--wait=false")
			}

			output, err := executeCommand(args)

			// Check expectations
			if tt.expectError {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(output).To(ContainSubstring(tt.expectOutput))
			}

			// Verify inputprovider was deleted
			err = testClient.Get(ctx, client.ObjectKeyFromObject(inputProvider), inputProvider)
			g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
		})
	}
}

func TestDeleteInputProviderCmd_NotFound(t *testing.T) {
	g := NewWithT(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test-delete-rsip-notfound")
	g.Expect(err).ToNot(HaveOccurred())

	// Execute delete command for non-existent inputprovider
	kubeconfigArgs.Namespace = &ns.Name
	deleteInputProviderArgs.wait = false

	_, err = executeCommand([]string{"delete", "inputprovider", "nonexistent"})

	// Should return error
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("not found"))
}

func TestDeleteInputProviderCmd_Alias(t *testing.T) {
	g := NewWithT(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test-delete-rsip-alias")
	g.Expect(err).ToNot(HaveOccurred())

	// Create inputprovider
	inputProvider := &fluxcdv1.ResourceSetInputProvider{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-inputprovider",
			Namespace: ns.Name,
		},
		Spec: fluxcdv1.ResourceSetInputProviderSpec{
			Type: fluxcdv1.InputProviderStatic,
		},
	}
	g.Expect(testClient.Create(ctx, inputProvider)).To(Succeed())

	// Ensure cleanup
	defer func() {
		_ = testClient.Delete(ctx, inputProvider)
	}()

	// Execute delete command using alias
	kubeconfigArgs.Namespace = &ns.Name
	deleteInputProviderArgs.wait = false

	output, err := executeCommand([]string{"delete", "rsip", "test-inputprovider", "--wait=false"})

	// Should succeed
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(output).To(ContainSubstring("Deletion initiated"))

	// Verify inputprovider was deleted
	err = testClient.Get(ctx, client.ObjectKeyFromObject(inputProvider), inputProvider)
	g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
}
