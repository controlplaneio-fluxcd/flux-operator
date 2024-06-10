package e2eolm

import (
	"os/exec"
	"time"

	utils "github.com/controlplaneio-fluxcd/flux-operator/test/e2e"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("FluxInstance", Ordered, func() {
	Context("installation", func() {
		It("should run successfully", func() {
			By("reconcile FluxInstance")
			verifyFluxInstanceReconcile := func() error {
				cmd := exec.Command("kubectl", "apply",
					"-k", "config/samples", "-n", namespace,
				)
				_, err := utils.Run(cmd, "/test/olm")
				ExpectWithOffset(2, err).NotTo(HaveOccurred())

				cmd = exec.Command("kubectl", "wait", "FluxInstance/flux", "-n", namespace,
					"--for=condition=Ready", "--timeout=5m",
				)
				_, err = utils.Run(cmd, "/test/olm")
				ExpectWithOffset(2, err).NotTo(HaveOccurred())
				return nil
			}
			EventuallyWithOffset(1, verifyFluxInstanceReconcile, 5*time.Minute, 10*time.Second).Should(Succeed())
		})
	})

	Context("uninstallation", func() {
		It("should run successfully", func() {
			By("delete FluxInstance")
			cmd := exec.Command("kubectl", "delete", "-k", "config/samples",
				"--timeout=30s", "-n", namespace)
			_, err := utils.Run(cmd, "/test/olm")
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
