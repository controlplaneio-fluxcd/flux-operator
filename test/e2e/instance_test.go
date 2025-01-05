// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package e2e

import (
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("FluxInstance", Ordered, func() {
	Context("installation", func() {
		It("should run successfully", func() {
			By("reconcile FluxInstance")
			verifyFluxInstanceReconcile := func() error {
				cmd := exec.Command("kubectl", "apply",
					"-f", "config/samples/fluxcd_v1_fluxinstance.yaml", "-n", namespace,
				)
				_, err := Run(cmd, "/test/e2e")
				ExpectWithOffset(2, err).NotTo(HaveOccurred())

				cmd = exec.Command("kubectl", "wait", "FluxInstance/flux", "-n", namespace,
					"--for=condition=Ready", "--timeout=5m",
				)
				_, err = Run(cmd, "/test/e2e")
				ExpectWithOffset(2, err).NotTo(HaveOccurred())
				return nil
			}
			EventuallyWithOffset(1, verifyFluxInstanceReconcile, 5*time.Minute, 10*time.Second).Should(Succeed())
		})
	})

	Context("resource group lifecycle", func() {
		It("should run successfully", func() {
			By("reconcile ResourceSet")
			reconcile := func() error {
				cmd := exec.Command("kubectl", "apply",
					"-f", "config/samples/fluxcd_v1_resourceset.yaml",
				)
				_, err := Run(cmd, "/test/e2e")
				ExpectWithOffset(2, err).NotTo(HaveOccurred())

				cmd = exec.Command("kubectl", "wait", "ResourceSet/podinfo",
					"--for=condition=Ready", "--timeout=5m",
				)
				_, err = Run(cmd, "/test/e2e")
				ExpectWithOffset(2, err).NotTo(HaveOccurred())

				cmd = exec.Command("kubectl", "delete", "ResourceSet/podinfo")
				_, err = Run(cmd, "/test/e2e")
				ExpectWithOffset(2, err).NotTo(HaveOccurred())
				return nil
			}
			EventuallyWithOffset(1, reconcile, 5*time.Minute, 10*time.Second).Should(Succeed())
		})
	})

	Context("upgrade", func() {
		It("should run successfully", func() {
			By("reconcile FluxInstance")
			verifyFluxInstanceReconcile := func() error {
				cmd := exec.Command("kubectl", "-n", namespace, "patch", "FluxInstance/flux",
					"--type=json", `-p=[{"op": "replace", "path": "/spec/cluster/multitenant", "value":true}]`,
				)
				_, err := Run(cmd, "/test/e2e")
				ExpectWithOffset(2, err).NotTo(HaveOccurred())

				cmd = exec.Command("kubectl", "wait", "FluxInstance/flux", "-n", namespace,
					"--for=condition=Ready", "--timeout=5m",
				)
				_, err = Run(cmd, "/test/e2e")
				ExpectWithOffset(2, err).NotTo(HaveOccurred())

				cmd = exec.Command("kubectl", "get", "deploy/kustomize-controller",
					"-n", namespace, "-o=yaml",
				)
				output, err := Run(cmd, "/test/e2e")
				ExpectWithOffset(2, err).NotTo(HaveOccurred())
				ExpectWithOffset(2, output).To(ContainSubstring("no-cross-namespace-refs=true"))

				return nil
			}
			EventuallyWithOffset(1, verifyFluxInstanceReconcile, 5*time.Minute, 10*time.Second).Should(Succeed())
		})
	})

	Context("uninstallation", func() {
		It("should run successfully", func() {
			By("delete FluxInstance")
			cmd := exec.Command("kubectl", "delete", "FluxInstance/flux",
				"--timeout=30s", "-n", namespace)
			_, err := Run(cmd, "/test/e2e")
			Expect(err).NotTo(HaveOccurred())
			By("source-controller deleted")
			cmd = exec.Command("kubectl", "get", "deploy/source-controller", "-n", namespace)
			_, err = Run(cmd, "/test/e2e")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))
			By("namespace exists")
			cmd = exec.Command("kubectl", "get", "ns", namespace)
			_, err = Run(cmd, "/test/e2e")
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
