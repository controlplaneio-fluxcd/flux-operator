// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package e2e

import (
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/controlplaneio-fluxcd/fluxcd-operator/test/utils"
)

var _ = Describe("FluxInstance", Ordered, func() {
	Context("installation", func() {
		It("should run successfully", func() {
			By("reconcile FluxInstance")
			verifyFluxInstanceReconcile := func() error {
				cmd := exec.Command("kubectl", "apply",
					"-k", "config/samples", "-n", namespace,
				)
				_, err := utils.Run(cmd)
				ExpectWithOffset(2, err).NotTo(HaveOccurred())

				cmd = exec.Command("kubectl", "wait", "FluxInstance/flux", "-n", namespace,
					"--for=condition=Ready", "--timeout=10s",
				)
				_, err = utils.Run(cmd)
				ExpectWithOffset(2, err).NotTo(HaveOccurred())
				return nil
			}
			EventuallyWithOffset(1, verifyFluxInstanceReconcile, time.Minute, 3*time.Second).Should(Succeed())
		})
	})

	Context("uninstallation", func() {
		It("should run successfully", func() {
			By("delete FluxInstance")
			cmd := exec.Command("kubectl", "delete", "-k", "config/samples",
				"--timeout=30s", "-n", namespace)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
