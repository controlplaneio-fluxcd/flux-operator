package e2eolm

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	utils "github.com/controlplaneio-fluxcd/flux-operator/test/e2e"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	namespace         = "flux-system"
	defaultVersion    = "v0.3.0"
	defaultOLMVersion = "v0.28.0"
)

var (
	version string
	img     string
)

// Build the flux-operator image and deploy it to the Kind cluster.
var _ = BeforeSuite(func() {
	version = os.Getenv("FLUX_OPERATOR_VERSION")
	if version == "" {
		version = defaultVersion
	}
	olmVersion := os.Getenv("OLM_VERSION")
	if olmVersion == "" {
		olmVersion = defaultOLMVersion
	}

	img = fmt.Sprintf("ghcr.io/controlplaneio-fluxcd/openshift-flux-operator-catalog:bundle-%s", version)
	opm := fmt.Sprintf("ghcr.io/controlplaneio-fluxcd/openshift-flux-operator-index:%s", version)

	By("loading the flux-operator bundle image on Kind")
	err := utils.LoadImageToKindClusterWithName(img, "/test/olm")
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	err = utils.LoadImageToKindClusterWithName(opm, "/test/olm")
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	By("installing OLM")
	cmd := exec.Command("operator-sdk", "olm", "install", "--version", olmVersion)
	_, err = utils.Run(cmd, "/test/olm")
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	By("deploying flux-operator olm kubernetes resources")
	cmd = exec.Command("make", "deploy-olm-data")
	_, err = utils.Run(cmd, "/test/olm")
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	By("validating that the flux-operator is installed")
	verifyInstallPlan := func() error {

		cmd = exec.Command("kubectl", "get",
			"installplan", "-o", "jsonpath={.items[].metadata.name}",
			"-n", namespace,
		)
		installPlanName, err := utils.Run(cmd, "/test/olm")
		if err != nil {
			return err
		}
		if !strings.Contains(string(installPlanName), "install") {
			return fmt.Errorf("expect installplan to be in installed state")
		}

		cmd = exec.Command("kubectl", "wait", "--for=condition=Installed=true",
			"installplan/"+string(installPlanName), "-n", namespace)
		_, err = utils.Run(cmd, "/test/olm")
		ExpectWithOffset(1, err).NotTo(HaveOccurred())

		return nil
	}
	EventuallyWithOffset(1, verifyInstallPlan, 3*time.Minute, 10*time.Second).Should(Succeed())

	By("validating that the flux-operator is running")
	exec.Command("kubectl", "wait", "--for=condition=Ready=true", "pod",
		"-lapp.kubernetes.io/name=flux-operator ", "-n", namespace)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

})

// Delete the flux-operator CRDs, deployment and namespace.
var _ = AfterSuite(func() {
	By("uninstalling flux-operator olm kubernetes resources")
	cmd := exec.Command("make", "undeploy-olm-data")
	_, err := utils.Run(cmd, "/test/olm")
	Expect(err).NotTo(HaveOccurred())
})
