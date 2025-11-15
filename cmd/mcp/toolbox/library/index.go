// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package library

import (
	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

// SearchDocument represents a searchable document.
type SearchDocument struct {
	ID      string // Unique identifier
	Content string // Full markdown content
	Length  int    // Word count (for BM25 normalization)

	Metadata DocumentMetadata
}

// SearchIndex holds the inverted index and document corpus.
type SearchIndex struct {
	Terms        map[string][]Posting // term -> list of postings
	Documents    []SearchDocument     // All searchable documents
	AvgDocLength float64              // Average document length (for BM25)
	TotalDocs    int                  // Total number of documents
}

// Posting represents a term occurrence in a document.
type Posting struct {
	DocID     int // Document index in Documents slice
	Frequency int // Term frequency in this document
}

// GetDocsMetadata returns the collection of document metadata for indexing.
func GetDocsMetadata() []DocumentMetadata {
	return docsMetadata
}

// docsMetadata is a collection of DocumentMetadata instances,
// each describing metadata like URL, group, kind, and related keywords.
var docsMetadata = []DocumentMetadata{
	{
		URL:   "https://raw.githubusercontent.com/fluxcd/source-controller/refs/heads/main/docs/spec/v1/gitrepositories.md",
		Group: fluxcdv1.FluxSourceGroup,
		Kind:  fluxcdv1.FluxGitRepositoryKind,
		Keywords: []string{
			"source-controller", "git", "branch", "commit", "sha", "ref", "tag", "semver",
			"verification", "pgp", "signature", "ssh", "private", "public", "tls", "auth",
			"include", "submodules", "sparse", "checkout", "proxy", "ignore",
			"github", "gitlab", "devops", "githubapp",
		},
	},
	{
		URL:   "https://raw.githubusercontent.com/fluxcd/source-controller/refs/heads/main/docs/spec/v1/ocirepositories.md",
		Group: fluxcdv1.FluxSourceGroup,
		Kind:  fluxcdv1.FluxOCIRepositoryKind,
		Keywords: []string{
			"source-controller", "oci", "registry", "artifact", "tag", "semver", "digest",
			"verification", "cosign", "notation", "signature", "keyless", "layer", "proxy",
			"media", "auth", "provider", "aws", "azure", "gcp", "identity", "iam", "tls",
		},
	},
	{
		URL:   "https://raw.githubusercontent.com/fluxcd/source-controller/refs/heads/main/docs/spec/v1/helmrepositories.md",
		Group: fluxcdv1.FluxSourceGroup,
		Kind:  fluxcdv1.FluxHelmRepositoryKind,
		Keywords: []string{
			"source-controller", "index.yaml", "authentication",
		},
	},
	{
		URL:   "https://raw.githubusercontent.com/fluxcd/source-controller/refs/heads/main/docs/spec/v1/helmcharts.md",
		Group: fluxcdv1.FluxSourceGroup,
		Kind:  fluxcdv1.FluxHelmChartKind,
		Keywords: []string{
			"source-controller", "chart", "valuesfiles", "strategy", "version",
		},
	},
	{
		URL:   "https://raw.githubusercontent.com/fluxcd/source-controller/refs/heads/main/docs/spec/v1/buckets.md",
		Group: fluxcdv1.FluxSourceGroup,
		Kind:  fluxcdv1.FluxBucketKind,
		Keywords: []string{
			"source-controller", "s3", "storage", "minio", "blob", "endpoint",
			"region", "insecure", "managed-identity", "sas", "token", "certificate",
			"proxy", "authentication", "provider", "aws", "azure", "gcp",
		},
	},
	{
		URL:   "https://raw.githubusercontent.com/fluxcd/source-watcher/refs/heads/main/docs/spec/v1beta1/artifactgenerators.md",
		Group: fluxcdv1.FluxSourceExtensionsGroup,
		Kind:  fluxcdv1.FluxArtifactGeneratorKind,
		Keywords: []string{
			"source-watcher", "artifact", "generator", "external", "composition", "decomposition", "multiple",
			"copy", "alias", "originrevision", "exclude", "extension",
		},
	},
	{
		URL:   "https://raw.githubusercontent.com/fluxcd/kustomize-controller/refs/heads/main/docs/spec/v1/kustomizations.md",
		Group: fluxcdv1.FluxKustomizeGroup,
		Kind:  fluxcdv1.FluxKustomizationKind,
		Keywords: []string{
			"kustomize-controller", "kustomize", "git", "oci", "retry", "wait", "timeout",
			"validation", "health", "cel", "drift", "patches", "substitution", "variables",
			"target", "sourceref", "path", "depends", "build", "inventory", "prune",
			"encryption", "decryption", "sops", "age", "kms", "pgp", "kubeconfig",
			"impersonation", "tenant", "deploy", "manifest", "yaml", "apply",
		},
	},
	{
		URL:   "https://raw.githubusercontent.com/fluxcd/helm-controller/refs/heads/main/docs/spec/v2/helmreleases.md",
		Group: fluxcdv1.FluxHelmGroup,
		Kind:  fluxcdv1.FluxHelmReleaseKind,
		Keywords: []string{
			"helm-controller", "helm", "chart", "release", "values", "upgrade", "install",
			"uninstall", "rollback", "test", "remediation", "drift", "detection", "kubeconfig",
			"target", "storage", "timeout", "renderer", "depends", "retry", "tenant",
		},
	},
	{
		URL:   "https://raw.githubusercontent.com/fluxcd/notification-controller/refs/heads/main/docs/spec/v1/receivers.md",
		Group: fluxcdv1.FluxNotificationGroup,
		Kind:  fluxcdv1.FluxReceiverKind,
		Keywords: []string{
			"notification-controller", "webhook", "receiver", "hmac", "trigger",
			"github", "gitlab", "bitbucket", "harbor", "cdevents", "payload",
		},
	},
	{
		URL:   "https://raw.githubusercontent.com/fluxcd/notification-controller/refs/heads/main/docs/spec/v1beta3/alerts.md",
		Group: fluxcdv1.FluxNotificationGroup,
		Kind:  fluxcdv1.FluxAlertKind,
		Keywords: []string{
			"notification-controller", "alerting", "event", "notification", "observability",
			"incident", "error", "info", "severity",
		},
	},
	{
		URL:   "https://raw.githubusercontent.com/fluxcd/notification-controller/refs/heads/main/docs/spec/v1beta3/providers.md",
		Group: fluxcdv1.FluxNotificationGroup,
		Kind:  fluxcdv1.FluxAlertProviderKind,
		Keywords: []string{
			"notification-controller", "alert", "notification", "slack", "teams",
			"pagerduty", "discord", "matrix", "lark", "rocket", "datadog", "grafana",
			"sentry", "telegram", "webex", "nats", "pubsub", "eventhub", "dispatch",
		},
	},
	{
		URL:   "https://raw.githubusercontent.com/fluxcd/image-reflector-controller/refs/heads/main/docs/spec/v1/imagerepositories.md",
		Group: fluxcdv1.FluxImageGroup,
		Kind:  fluxcdv1.FluxImageRepositoryKind,
		Keywords: []string{
			"image-reflector-controller", "container", "image", "tags",
			"docker", "ecr", "gar", "acr", "scan",
		},
	},
	{
		URL:   "https://raw.githubusercontent.com/fluxcd/image-reflector-controller/refs/heads/main/docs/spec/v1/imagepolicies.md",
		Group: fluxcdv1.FluxImageGroup,
		Kind:  fluxcdv1.FluxImagePolicyKind,
		Keywords: []string{
			"image-reflector-controller", "container", "image", "policy", "tag", "semver", "range",
			"numerical", "alphabetical", "order", "filter", "pattern", "regex", "latest",
		},
	},
	{
		URL:   "https://raw.githubusercontent.com/fluxcd/image-automation-controller/refs/heads/main/docs/spec/v1/imageupdateautomations.md",
		Group: fluxcdv1.FluxImageGroup,
		Kind:  fluxcdv1.FluxImageUpdateAutomationKind,
		Keywords: []string{
			"image-automation-controller", "docker", "container", "image", "tag",
			"policy", "update", "commit", "push", "git", "scan", "automation", "automate",
		},
	},
	{
		URL:   "https://raw.githubusercontent.com/controlplaneio-fluxcd/flux-operator/refs/heads/main/docs/api/v1/fluxinstance.md",
		Group: fluxcdv1.GroupVersion.Group,
		Kind:  fluxcdv1.FluxInstanceKind,
		Keywords: []string{
			"flux-operator", "distribution", "registry", "components", "sync",
			"cluster", "storage", "multitenant", "network", "controller",
			"domain", "sharding", "migrate", "bootstrap", "cve", "auto", "deploy",
		},
	},
	{
		URL:   "https://raw.githubusercontent.com/controlplaneio-fluxcd/flux-operator/refs/heads/main/docs/api/v1/fluxreport.md",
		Group: fluxcdv1.GroupVersion.Group,
		Kind:  fluxcdv1.FluxReportKind,
		Keywords: []string{
			"flux-operator", "report", "monitor", "stats", "readiness",
			"metric", "prometheus", "entitlement", "size", "info", "troubleshooting",
		},
	},
	{
		URL:   "https://raw.githubusercontent.com/controlplaneio-fluxcd/flux-operator/refs/heads/main/docs/api/v1/resourceset.md",
		Group: fluxcdv1.GroupVersion.Group,
		Kind:  fluxcdv1.ResourceSetKind,
		Keywords: []string{
			"flux-operator", "resource", "inputs", "template", "templating",
			"common", "depends", "wait", "timeout", "account",
			"health", "cel", "prune", "inventory", "app", "definition",
		},
	},
	{
		URL:   "https://raw.githubusercontent.com/controlplaneio-fluxcd/flux-operator/refs/heads/main/docs/api/v1/resourcesetinputprovider.md",
		Group: fluxcdv1.GroupVersion.Group,
		Kind:  fluxcdv1.ResourceSetInputProviderKind,
		Keywords: []string{
			"flux-operator", "input", "provider", "pull", "merge", "request",
			"author", "github", "gitlab", "filter", "labels", "branch",
			"exclude", "default", "exported", "preview", "ephemeral", "environment",
		},
	},
}
