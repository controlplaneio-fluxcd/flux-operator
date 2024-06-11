package e2eolm

import (
	"os/exec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	utils "github.com/controlplaneio-fluxcd/flux-operator/test/e2e"
)

var _ = Describe("Scorecard", Ordered, func() {
	Context("test", func() {
		It("should run successfully", func() {
			By("run scorecard tests")
			cmd := exec.Command(operatorsdkBin, "scorecard",
				img, "-c", "config/operatorhub/flux-operator/"+version+"/tests/scorecard/config.yaml",
				"-w", "5m", "-o", "json")
			_, err := utils.Run(cmd, "/test/olm")
			ExpectWithOffset(2, err).NotTo(HaveOccurred())
		})
	})
})
