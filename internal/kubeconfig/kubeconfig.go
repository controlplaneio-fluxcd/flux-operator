// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package kubeconfig

import (
	"fmt"

	"sigs.k8s.io/yaml"
)

type kubeConfig struct {
	Clusters []cluster `json:"clusters"`
}

type cluster struct {
	Name    string        `json:"name"`
	Cluster clusterConfig `json:"cluster"`
}

type clusterConfig struct {
	Server                   string `json:"server"`
	CertificateAuthorityData []byte `json:"certificate-authority-data"`
}

type clusterData struct {
	Name   string
	Server string
	CACert string
}

// extractAllFluxFields parses a kubeconfig YAML and extracts the API server
// endpoint and CA certificate data for each cluster.
func extractAllFluxFields(kubeconfigYAML string) ([]clusterData, error) {
	var config kubeConfig
	if err := yaml.Unmarshal([]byte(kubeconfigYAML), &config); err != nil {
		return nil, fmt.Errorf("failed to parse kubeconfig YAML: %w", err)
	}

	if len(config.Clusters) == 0 {
		return nil, fmt.Errorf("no clusters found in kubeconfig")
	}

	clusters := make([]clusterData, 0, len(config.Clusters))

	for _, c := range config.Clusters {
		cluster := c.Cluster

		if cluster.Server == "" {
			return nil, fmt.Errorf("server field is empty in kubeconfig cluster \"%s\"", c.Name)
		}

		if len(cluster.CertificateAuthorityData) == 0 {
			return nil, fmt.Errorf("certificate-authority-data field is empty in kubeconfig cluster \"%s\"", c.Name)
		}

		clusters = append(clusters, clusterData{
			Name:   c.Name,
			Server: cluster.Server,
			CACert: string(cluster.CertificateAuthorityData),
		})
	}

	return clusters, nil
}

// ExtractFluxFields returns the API server address and CA certificate
// from the first cluster defined in a kubeconfig.
func ExtractFluxFields(kubeconfigYAML string) (server, caCert string, err error) {
	clusters, err := extractAllFluxFields(kubeconfigYAML)
	if err != nil {
		return "", "", err
	}

	return clusters[0].Server, clusters[0].CACert, nil
}
