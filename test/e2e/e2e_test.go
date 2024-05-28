// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package e2e

import (
	"fmt"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/controlplaneio-fluxcd/fluxcd-operator/test/utils"
)

const (
	namespace = "flux-system"
)

// Build the fluxcd-operator image and deploy it to the Kind cluster.
var _ = BeforeSuite(func() {
	image := "test/fluxcd-operator:v0.0.0-dev.1"
	var controllerPodName string
	var err error

	By("building the fluxcd-operator image")
	cmd := exec.Command("make", "docker-build", fmt.Sprintf("IMG=%s", image))
	_, err = utils.Run(cmd)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	By("loading the fluxcd-operator image on Kind")
	err = utils.LoadImageToKindClusterWithName(image)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	By("deploying fluxcd-operator")
	cmd = exec.Command("make", "deploy", fmt.Sprintf("IMG=%s", image))
	_, err = utils.Run(cmd)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	By("validating that the fluxcd-operator pod is running as expected")
	verifyControllerUp := func() error {
		// Get pod name

		cmd = exec.Command("kubectl", "get",
			"pods", "-l", "app.kubernetes.io/name=fluxcd-operator",
			"-o", "go-template={{ range .items }}"+
				"{{ if not .metadata.deletionTimestamp }}"+
				"{{ .metadata.name }}"+
				"{{ \"\\n\" }}{{ end }}{{ end }}",
			"-n", namespace,
		)

		podOutput, err := utils.Run(cmd)
		ExpectWithOffset(2, err).NotTo(HaveOccurred())
		podNames := utils.GetNonEmptyLines(string(podOutput))
		if len(podNames) != 1 {
			return fmt.Errorf("expect 1 fluxcd-operator pods running, but got %d", len(podNames))
		}
		controllerPodName = podNames[0]
		ExpectWithOffset(2, controllerPodName).Should(ContainSubstring("fluxcd-operator"))

		// Validate pod status
		cmd = exec.Command("kubectl", "get",
			"pods", controllerPodName, "-o", "jsonpath={.status.phase}",
			"-n", namespace,
		)
		status, err := utils.Run(cmd)
		ExpectWithOffset(2, err).NotTo(HaveOccurred())
		if string(status) != "Running" {
			return fmt.Errorf("fluxcd-operator pod in %s status", status)
		}
		return nil
	}
	EventuallyWithOffset(1, verifyControllerUp, time.Minute, 5*time.Second).Should(Succeed())
})

// Delete the fluxcd-operator CRDs, deployment and namespace.
var _ = AfterSuite(func() {
	By("uninstalling fluxcd-operator")
	cmd := exec.Command("make", "undeploy")
	_, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred())
})
