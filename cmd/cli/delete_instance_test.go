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

func TestDeleteInstanceCmd(t *testing.T) {
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
			name:         "delete instance without wait",
			nsName:       "test-delete-instance-nowait",
			wait:         false,
			withSuspend:  false,
			timeout:      30 * time.Second,
			expectError:  false,
			expectOutput: "Deletion initiated",
		},
		{
			name:         "delete instance with wait",
			nsName:       "test-delete-instance-wait",
			wait:         true,
			withSuspend:  false,
			timeout:      30 * time.Second,
			expectError:  false,
			expectOutput: "Deletion completed",
		},
		{
			name:         "delete instance with suspend",
			nsName:       "test-delete-instance-suspend",
			wait:         false,
			withSuspend:  true,
			timeout:      30 * time.Second,
			expectError:  false,
			expectOutput: "Reconciliation suspended",
		},
		{
			name:         "delete instance with suspend and wait",
			nsName:       "test-delete-instance-suspend-wait",
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

			// Create instance
			instance := &fluxcdv1.FluxInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "flux",
					Namespace: ns.Name,
				},
				Spec: fluxcdv1.FluxInstanceSpec{
					Distribution: fluxcdv1.Distribution{
						Version:  "2.x",
						Registry: "ghcr.io/fluxcd",
					},
				},
			}
			g.Expect(testClient.Create(ctx, instance)).To(Succeed())

			// Ensure cleanup
			defer func() {
				_ = testClient.Delete(ctx, instance)
			}()

			// Execute delete command
			rootArgs.timeout = tt.timeout
			kubeconfigArgs.Namespace = &ns.Name
			deleteInstanceArgs.wait = tt.wait
			deleteInstanceArgs.withSuspend = tt.withSuspend

			args := []string{"delete", "instance", "flux"}
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

			// Verify instance was deleted
			err = testClient.Get(ctx, client.ObjectKeyFromObject(instance), instance)
			g.Expect(apierrors.IsNotFound(err)).To(BeTrue())

			if tt.withSuspend {
				g.Expect(output).To(ContainSubstring("Reconciliation suspended"))
			}
		})
	}
}

func TestDeleteInstanceCmd_NotFound(t *testing.T) {
	g := NewWithT(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test-delete-instance-notfound")
	g.Expect(err).ToNot(HaveOccurred())

	// Execute delete command for non-existent instance
	kubeconfigArgs.Namespace = &ns.Name
	deleteInstanceArgs.wait = false
	deleteInstanceArgs.withSuspend = false

	_, err = executeCommand([]string{"delete", "instance", "nonexistent"})

	// Should return error
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("not found"))
}

func TestDeleteInstanceCmd_WithSuspendNotFound(t *testing.T) {
	g := NewWithT(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test-delete-instance-suspend-notfound")
	g.Expect(err).ToNot(HaveOccurred())

	// Execute delete command with --with-suspend for non-existent instance
	kubeconfigArgs.Namespace = &ns.Name
	deleteInstanceArgs.wait = false
	deleteInstanceArgs.withSuspend = true

	_, err = executeCommand([]string{"delete", "instance", "nonexistent", "--with-suspend"})

	// Should return error during suspend phase
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("unable to"))
}
