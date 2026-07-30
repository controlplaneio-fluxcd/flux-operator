// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func TestMergedKubeconfig(t *testing.T) {
	g := NewWithT(t)

	// Load the test environment kubeconfig and split it into separate files
	// for clusters, users and contexts to simulate a merged KUBECONFIG
	// where the current context references entries defined in other files.
	conf, err := clientcmd.LoadFromFile(*kubeconfigArgs.KubeConfig)
	g.Expect(err).ToNot(HaveOccurred())

	tmpDir := t.TempDir()

	clustersConf := clientcmdapi.NewConfig()
	clustersConf.Clusters = conf.Clusters
	clustersPath := filepath.Join(tmpDir, "clusters")
	g.Expect(clientcmd.WriteToFile(*clustersConf, clustersPath)).To(Succeed())

	usersConf := clientcmdapi.NewConfig()
	usersConf.AuthInfos = conf.AuthInfos
	usersPath := filepath.Join(tmpDir, "users")
	g.Expect(clientcmd.WriteToFile(*usersConf, usersPath)).To(Succeed())

	contextsConf := clientcmdapi.NewConfig()
	contextsConf.Contexts = conf.Contexts
	contextsConf.CurrentContext = conf.CurrentContext
	contextsPath := filepath.Join(tmpDir, "contexts")
	g.Expect(clientcmd.WriteToFile(*contextsConf, contextsPath)).To(Succeed())

	// Unset the explicit kubeconfig path and point the KUBECONFIG
	// environment variable to the list of config files.
	kubeconfigPath := kubeconfigArgs.KubeConfig
	kubeconfigArgs.KubeConfig = new("")
	defer func() {
		kubeconfigArgs.KubeConfig = kubeconfigPath
	}()
	t.Setenv("KUBECONFIG", strings.Join(
		[]string{contextsPath, usersPath, clustersPath},
		string(os.PathListSeparator)))

	_, err = executeCommand([]string{"get", "instance"})
	g.Expect(err).ToNot(HaveOccurred())
}
