// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package e2e

import (
	"bytes"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("FluxInstance", Ordered, func() {
	Context("build", func() {
		It("should run successfully", func() {
			By("build FluxInstance")
			build := func() error {
				cmd := exec.Command(cli, "build", "instance",
					"-f", "config/samples/fluxcd_v1_fluxinstance.yaml")
				output, err := Run(cmd, "/test/e2e")
				ExpectWithOffset(2, err).NotTo(HaveOccurred())
				ExpectWithOffset(2, output).To(ContainSubstring("--requeue-dependency=10s"))

				return nil
			}
			EventuallyWithOffset(1, build, time.Minute, 10*time.Second).Should(Succeed())
		})
	})

	Context("installation", func() {
		It("should run successfully", func() {
			By("reconcile FluxInstance")
			verifyFluxInstanceReconcile := func() error {
				cmd := exec.Command("kubectl", "apply",
					"-f", "config/samples/fluxcd_v1_fluxinstance.yaml", "-n", namespace,
				)
				_, err := Run(cmd, "/test/e2e")
				ExpectWithOffset(2, err).NotTo(HaveOccurred())

				cmd = exec.Command(cli, "wait", "instance", "flux", "-n", namespace)
				_, err = Run(cmd, "/test/e2e")
				ExpectWithOffset(2, err).NotTo(HaveOccurred())

				cmd = exec.Command(cli, "suspend", "instance", "flux", "-n", namespace)
				output, err := Run(cmd, "/test/e2e")
				ExpectWithOffset(2, err).NotTo(HaveOccurred())
				ExpectWithOffset(2, output).To(ContainSubstring("Reconciliation suspended"))

				cmd = exec.Command(cli, "reconcile", "instance", "flux", "-n", namespace)
				output, err = Run(cmd, "/test/e2e")
				ExpectWithOffset(2, err).To(HaveOccurred())
				ExpectWithOffset(2, output).To(ContainSubstring("Reconciliation is disabled"))

				cmd = exec.Command(cli, "resume", "instance", "flux", "-n", namespace)
				_, err = Run(cmd, "/test/e2e")
				ExpectWithOffset(2, err).NotTo(HaveOccurred())

				cmd = exec.Command(cli, "get", "resources", "-n", namespace)
				output, err = Run(cmd, "/test/e2e")
				ExpectWithOffset(2, err).NotTo(HaveOccurred())
				ExpectWithOffset(2, output).To(ContainSubstring("GitRepository"))
				ExpectWithOffset(2, output).To(ContainSubstring("Kustomization"))

				cmd = exec.Command(cli, "version")
				output, err = Run(cmd, "/test/e2e")
				ExpectWithOffset(2, err).NotTo(HaveOccurred())
				ExpectWithOffset(2, output).To(ContainSubstring("client:"))
				ExpectWithOffset(2, output).To(ContainSubstring("server:"))
				ExpectWithOffset(2, output).To(ContainSubstring("distribution:"))

				return nil
			}
			EventuallyWithOffset(1, verifyFluxInstanceReconcile, 5*time.Minute, 10*time.Second).Should(Succeed())
		})
	})

	Context("ResourceSet lifecycle", func() {
		It("should run successfully", func() {
			By("reconcile ResourceSet")
			reconcile := func() error {
				cmd := exec.Command(cli, "build", "resourceset",
					"-f", "config/samples/fluxcd_v1_resourceset.yaml")
				output, err := Run(cmd, "/test/e2e")
				ExpectWithOffset(2, err).NotTo(HaveOccurred())
				ExpectWithOffset(2, output).To(ContainSubstring("resourceset.fluxcd.controlplane.io/name: podinfo"))

				cmd = exec.Command("kubectl", "apply",
					"-f", "config/samples/fluxcd_v1_resourceset.yaml",
				)
				_, err = Run(cmd, "/test/e2e")
				ExpectWithOffset(2, err).NotTo(HaveOccurred())

				cmd = exec.Command(cli, "wait", "rset", "podinfo")
				_, err = Run(cmd, "/test/e2e")
				ExpectWithOffset(2, err).NotTo(HaveOccurred())

				cmd = exec.Command(cli, "get", "rset", "-A")
				output, err = Run(cmd, "/test/e2e")
				ExpectWithOffset(2, err).NotTo(HaveOccurred())
				ExpectWithOffset(2, output).To(ContainSubstring("podinfo"))

				cmd = exec.Command(cli, "reconcile", "rset", "podinfo")
				output, err = Run(cmd, "/test/e2e")
				ExpectWithOffset(2, err).ToNot(HaveOccurred())
				ExpectWithOffset(2, output).To(ContainSubstring("Reconciliation finished"))

				cmd = exec.Command(cli, "stats")
				output, err = Run(cmd, "/test/e2e")
				ExpectWithOffset(2, err).NotTo(HaveOccurred())
				ExpectWithOffset(2, output).To(ContainSubstring("ResourceSet"))

				cmd = exec.Command(cli, "delete", "rset", "podinfo")
				output, err = Run(cmd, "/test/e2e")
				ExpectWithOffset(2, err).NotTo(HaveOccurred())
				ExpectWithOffset(2, output).To(ContainSubstring("completed"))
				return nil
			}
			EventuallyWithOffset(1, reconcile, 5*time.Minute, 10*time.Second).Should(Succeed())
		})
	})

	Context("ResourceSetInputProvider lifecycle", func() {
		It("should run successfully", func() {
			By("reconcile ResourceSetInputProvider")
			reconcile := func() error {
				cmd := exec.Command("kubectl", "apply",
					"-f", "config/samples/fluxcd_v1_resourcesetinputprovider.yaml",
				)
				_, err := Run(cmd, "/test/e2e")
				ExpectWithOffset(2, err).NotTo(HaveOccurred())

				cmd = exec.Command(cli, "wait", "rsip", "demo")
				_, err = Run(cmd, "/test/e2e")
				ExpectWithOffset(2, err).NotTo(HaveOccurred())

				cmd = exec.Command(cli, "get", "rsip", "-A")
				output, err := Run(cmd, "/test/e2e")
				ExpectWithOffset(2, err).NotTo(HaveOccurred())
				ExpectWithOffset(2, output).To(ContainSubstring("demo"))

				cmd = exec.Command(cli, "delete", "rsip", "demo")
				output, err = Run(cmd, "/test/e2e")
				ExpectWithOffset(2, err).NotTo(HaveOccurred())
				ExpectWithOffset(2, output).To(ContainSubstring("completed"))
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

				cmd = exec.Command(cli, "wait", "instance", "flux", "-n", namespace)
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

	Context("reporting", func() {
		It("should run successfully", func() {
			By("generates FluxReport")
			verifyFluxReport := func() error {
				cmd := exec.Command(cli, "export", "report")
				output, err := Run(cmd, "/test/e2e")
				ExpectWithOffset(2, err).NotTo(HaveOccurred())
				ExpectWithOffset(2, output).To(ContainSubstring("nodes: 1"))
				ExpectWithOffset(2, output).To(ContainSubstring("managedBy: flux-operator"))
				ExpectWithOffset(2, output).To(ContainSubstring("id: kustomization/flux-system"))

				return nil
			}
			EventuallyWithOffset(1, verifyFluxReport, time.Minute, 10*time.Second).Should(Succeed())
		})
	})

	Context("web API", func() {
		It("should return report data", func() {
			By("patching service to NodePort")
			cmd := exec.Command("kubectl", "-n", namespace, "patch", "svc/flux-operator",
				"--type=json", `-p=[{"op":"replace","path":"/spec/type","value":"NodePort"},{"op":"add","path":"/spec/ports/0/nodePort","value":30080}]`,
			)
			_, err := Run(cmd, "/test/e2e")
			ExpectWithOffset(1, err).NotTo(HaveOccurred())

			By("fetching report from web API")
			verifyWebAPI := func() error {
				cmd := exec.Command("docker", "exec", "kind-control-plane",
					"curl", "-s", "-f", "--max-time", "5", "http://localhost:30080/api/v1/report")
				output, err := Run(cmd, "/test/e2e")
				if err != nil {
					GinkgoWriter.Printf("curl failed: %v\n", err)
					return err
				}

				GinkgoWriter.Println("running: jq assertions on response")
				cmd = exec.Command("jq", "-e",
					`.kind == "FluxReport" and .spec.distribution.managedBy == "flux-operator" and (.spec.userInfo.username | startswith("flux-operator-"))`)
				cmd.Stdin = bytes.NewReader(output)
				if err := cmd.Run(); err != nil {
					GinkgoWriter.Printf("jq assertion failed, response: %s\n", output)
					return err
				}
				return nil
			}
			EventuallyWithOffset(1, verifyWebAPI, time.Minute, 10*time.Second).Should(Succeed())
		})
	})

	Context("uninstallation", func() {
		It("should run successfully", func() {
			By("delete FluxInstance")
			cmd := exec.Command(cli, "delete", "instance", "flux", "-n", namespace)
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
