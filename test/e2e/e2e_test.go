// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package e2e

import (
	"fmt"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	namespace = "flux-system"
	image     = "test/flux-operator:v0.0.0-dev.1"
	cli       = "./bin/flux-operator-cli"
)

// Build the flux-operator image and deploy it to the Kind cluster.
var _ = BeforeSuite(func() {
	var controllerPodName string
	var err error

	By("building the flux-operator CLI")
	cmd := exec.Command("make", "cli-build")
	_, err = Run(cmd, "/test/e2e")
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	By("building the flux-operator image")
	cmd = exec.Command("make", "docker-build", fmt.Sprintf("IMG=%s", image))
	_, err = Run(cmd, "/test/e2e")
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	By("loading the flux-operator image on Kind")
	err = LoadImageToKindClusterWithName(image, "/test/e2e")
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	By("deploying flux-operator")
	cmd = exec.Command("make", "deploy", fmt.Sprintf("IMG=%s", image))
	_, err = Run(cmd, "/test/e2e")
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	By("validating that the flux-operator pod is running as expected")
	verifyControllerUp := func() error {
		// Get pod name
		cmd = exec.Command("kubectl", "get",
			"pods", "-l", "app.kubernetes.io/name=flux-operator",
			"-o", "go-template={{ range .items }}"+
				"{{ if not .metadata.deletionTimestamp }}"+
				"{{ .metadata.name }}"+
				"{{ \"\\n\" }}{{ end }}{{ end }}",
			"-n", namespace,
		)

		podOutput, err := Run(cmd, "/test/e2e")
		ExpectWithOffset(2, err).NotTo(HaveOccurred())
		podNames := GetNonEmptyLines(string(podOutput))
		if len(podNames) != 1 {
			return fmt.Errorf("expect 1 flux-operator pods running, but got %d", len(podNames))
		}
		controllerPodName = podNames[0]
		ExpectWithOffset(2, controllerPodName).Should(ContainSubstring("flux-operator"))

		// Validate pod status
		cmd = exec.Command("kubectl", "get",
			"pods", controllerPodName, "-o", "jsonpath={.status.phase}",
			"-n", namespace,
		)
		status, err := Run(cmd, "/test/e2e")
		ExpectWithOffset(2, err).NotTo(HaveOccurred())
		if string(status) != "Running" {
			return fmt.Errorf("flux-operator pod in %s status", status)
		}
		return nil
	}
	EventuallyWithOffset(1, verifyControllerUp, time.Minute, 5*time.Second).Should(Succeed())
})

// Delete the flux-operator CRDs, deployment and namespace.
var _ = AfterSuite(func() {
	By("uninstalling flux-operator")
	cmd := exec.Command("make", "undeploy")
	_, err := Run(cmd, "/test/e2e")
	Expect(err).NotTo(HaveOccurred())
})
