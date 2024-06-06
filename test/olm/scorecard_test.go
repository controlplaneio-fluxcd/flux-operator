package e2eolm

import (
	"os/exec"

	utils "github.com/controlplaneio-fluxcd/flux-operator/test/e2e"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Scorecard", Ordered, func() {
	Context("test", func() {
		It("should run successfully", func() {
			By("run scorecard tests")
			cmd := exec.Command("operator-sdk", "scorecard",
				img, "-c", "config/operatorhub/flux-operator/"+version+"/tests/scorecard/config.yaml",
				"-w", "60s", "-o", "json")
			_, err := utils.Run(cmd, "/test/olm")
			ExpectWithOffset(2, err).NotTo(HaveOccurred())
		})
	})
})
