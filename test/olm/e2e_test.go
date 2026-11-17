// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package e2eolm

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	utils "github.com/controlplaneio-fluxcd/flux-operator/test/e2e"
)

const (
	namespace             = "flux-system"
	bundlePath            = "bin/olm"
	olmArtifactPrefix     = "ghcr.io/controlplaneio-fluxcd/openshift-flux-operator"
	defaultVersion        = "v0.3.0"
	defaultOLMVersion     = "v0.28.0"
	defaultOperatorsdkBin = "bin/operator-sdk"
)

var (
	version        string
	img            string
	operatorsdkBin string
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
	operatorsdkBin = os.Getenv("OPERATOR_SDK_BIN")
	if operatorsdkBin == "" {
		operatorsdkBin = defaultOperatorsdkBin
	}

	img = fmt.Sprintf("%s-catalog:bundle-%s", olmArtifactPrefix, version)
	opm := fmt.Sprintf("%s-index:v%s", olmArtifactPrefix, version)

	By("loading the flux-operator bundle image on Kind")
	err := utils.LoadImageToKindClusterWithName(img, "/test/olm")
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	err = utils.LoadImageToKindClusterWithName(opm, "/test/olm")
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	By("installing OLM")
	cmd := exec.Command(operatorsdkBin, "olm", "install", "--version", olmVersion)
	_, err = utils.Run(cmd, "/test/olm")
	ExpectWithOffset(3, err).NotTo(HaveOccurred())

	By("deploying flux-operator olm kubernetes resources")
	cmd = exec.Command("kubectl", "apply", "-f", bundlePath+"/test/olm.yaml")
	_, err = utils.Run(cmd, "/test/olm")
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	By("waiting for the flux-operator install plan")
	var installPlanName string
	EventuallyWithOffset(1, func() error {
		cmd = exec.Command("kubectl", "get",
			"installplan", "-o", "jsonpath={.items[].metadata.name}",
			"-n", namespace,
		)
		output, err := utils.Run(cmd, "/test/olm")
		if err != nil {
			return fmt.Errorf("failed to get installplan, please check the subscription, %w", err)
		}
		name := strings.TrimSpace(string(output))
		if !strings.HasPrefix(name, "install") {
			return fmt.Errorf("expect installplan to be created")
		}
		installPlanName = name
		return nil
	}, 5*time.Minute, 10*time.Second).Should(Succeed())

	By("validating that the flux-operator is installed")
	// OLM marks the install plan as installed only after the bundle unpack
	// job completes, which pulls images from the network. Relying on the
	// kubectl wait default timeout of 30s makes this step flaky on CI.
	cmd = exec.Command("kubectl", "wait", "--for=condition=Installed=true",
		"--timeout=5m", "installplan/"+installPlanName, "-n", namespace)
	if _, err = utils.Run(cmd, "/test/olm"); err != nil {
		dumpInstallDiagnostics()
	}
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	By("validating that the flux-operator is running")
	cmd = exec.Command("kubectl", "wait", "--for=condition=Ready=true", "pod",
		"-l", "app.kubernetes.io/name=flux-operator", "-n", namespace, "--timeout=5m")
	if _, err = utils.Run(cmd, "/test/olm"); err != nil {
		dumpInstallDiagnostics()
	}
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
})

// dumpInstallDiagnostics prints the state of the OLM resources and the
// flux-operator pod to the Ginkgo writer, to ease debugging failed installs.
func dumpInstallDiagnostics() {
	commands := [][]string{
		{"get", "installplan,csv,pods", "-n", namespace, "-o", "wide"},
		{"describe", "installplan", "-n", namespace},
		{"describe", "csv", "-n", namespace},
		{"get", "events", "-n", namespace, "--sort-by=.lastTimestamp"},
		{"logs", "-n", namespace, "-l", "app.kubernetes.io/name=flux-operator", "--tail=100"},
	}
	for _, args := range commands {
		output, _ := utils.Run(exec.Command("kubectl", args...), "/test/olm")
		fmt.Fprintf(GinkgoWriter, "$ kubectl %s\n%s\n", strings.Join(args, " "), output) //nolint:errcheck
	}
}

// Delete the flux-operator CRDs, deployment and namespace.
var _ = AfterSuite(func() {
	By("uninstalling flux-operator olm kubernetes resources")
	cmd := exec.Command("kubectl", "delete", "-f", bundlePath+"/test/olm.yaml")
	_, err := utils.Run(cmd, "/test/olm")
	Expect(err).NotTo(HaveOccurred())

	operatorsdkBin = os.Getenv("OPERATOR_SDK_BIN")
	if operatorsdkBin == "" {
		operatorsdkBin = defaultOperatorsdkBin
	}
	cmd = exec.Command(operatorsdkBin, "olm", "uninstall")
	_, err = utils.Run(cmd, "/test/olm")
	Expect(err).NotTo(HaveOccurred())
})
