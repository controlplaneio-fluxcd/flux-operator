// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
)

func TestReadOperatorManifests(t *testing.T) {
	g := NewWithT(t)

	manifest := `apiVersion: v1
kind: Namespace
metadata:
  name: flux-system
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: flux-operator
  namespace: flux-system
`
	path := filepath.Join(t.TempDir(), "install.yaml")
	g.Expect(os.WriteFile(path, []byte(manifest), 0644)).To(Succeed())

	objects, err := readOperatorManifests(path)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(objects).To(HaveLen(2))
	g.Expect(objects[0].GetKind()).To(Equal("Namespace"))
	g.Expect(objects[1].GetKind()).To(Equal("ServiceAccount"))
}

func TestReadOperatorManifestsEmpty(t *testing.T) {
	g := NewWithT(t)

	path := filepath.Join(t.TempDir(), "install.yaml")
	g.Expect(os.WriteFile(path, nil, 0644)).To(Succeed())

	_, err := readOperatorManifests(path)
	g.Expect(err).To(MatchError(ContainSubstring("no Kubernetes objects found in install file")))
}

func TestValidateInstallFlagsInstallFileWithVerify(t *testing.T) {
	g := NewWithT(t)

	original := installArgs
	t.Cleanup(func() {
		installArgs = original
	})

	installArgs.installFile = "/install-file.yaml"
	installArgs.verify = true

	g.Expect(validateInstallFlags()).To(MatchError("--verify cannot be used with --install-file"))
}

func TestValidateInstallFlagsAutoUpdateOCIRepositoryFileWithAutoUpdateDisabled(t *testing.T) {
	g := NewWithT(t)

	original := installArgs
	t.Cleanup(func() {
		installArgs = original
	})

	installArgs.autoUpdate = false
	installArgs.autoUpdateOCIRepositoryFile = "/etc/flux-config/auto-update-oci-repository.yaml"

	g.Expect(validateInstallFlags()).To(MatchError("--auto-update-oci-repository-file cannot be set when --auto-update=false"))
}

func TestValidateInstallFlagsAutoUpdateOCIRepositoryFileWithAutoUpdateEnabled(t *testing.T) {
	g := NewWithT(t)

	original := installArgs
	t.Cleanup(func() {
		installArgs = original
	})

	installArgs.autoUpdate = true
	installArgs.autoUpdateOCIRepositoryFile = "/etc/flux-config/auto-update-oci-repository.yaml"

	g.Expect(validateInstallFlags()).To(Succeed())
}

func TestReadAutoUpdateOCIRepository(t *testing.T) {
	g := NewWithT(t)

	manifest := `apiVersion: source.toolkit.fluxcd.io/v1
kind: OCIRepository
metadata:
  name: flux-operator
  namespace: flux-system
spec:
  interval: 1h
  url: oci://registry.example.com/flux-operator-manifests
  ref:
    tag: latest
  insecure: true
`
	path := filepath.Join(t.TempDir(), "auto-update-oci-repository.yaml")
	g.Expect(os.WriteFile(path, []byte(manifest), 0644)).To(Succeed())

	result, err := readAutoUpdateOCIRepository(path)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result).To(ContainSubstring("kind: OCIRepository"))
	g.Expect(result).To(ContainSubstring("insecure: true"))
}

func TestReadAutoUpdateOCIRepositoryRejectsMultipleObjects(t *testing.T) {
	g := NewWithT(t)

	manifest := `apiVersion: source.toolkit.fluxcd.io/v1
kind: OCIRepository
metadata:
  name: flux-operator
---
apiVersion: v1
kind: Secret
metadata:
  name: flux-operator-auth
`
	path := filepath.Join(t.TempDir(), "auto-update-oci-repository.yaml")
	g.Expect(os.WriteFile(path, []byte(manifest), 0644)).To(Succeed())

	_, err := readAutoUpdateOCIRepository(path)
	g.Expect(err).To(MatchError(ContainSubstring("expected exactly one Kubernetes object in auto-update OCIRepository file")))
}

func TestReadAutoUpdateOCIRepositoryRejectsOtherKind(t *testing.T) {
	g := NewWithT(t)

	manifest := `apiVersion: v1
kind: ConfigMap
metadata:
  name: not-an-oci-repository
`
	path := filepath.Join(t.TempDir(), "auto-update-oci-repository.yaml")
	g.Expect(os.WriteFile(path, []byte(manifest), 0644)).To(Succeed())

	_, err := readAutoUpdateOCIRepository(path)
	g.Expect(err).To(MatchError(ContainSubstring("to contain kind OCIRepository, got ConfigMap")))
}
