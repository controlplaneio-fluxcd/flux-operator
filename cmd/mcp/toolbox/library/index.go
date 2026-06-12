// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package library

import (
	"fmt"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

// IndexFormat identifies a searchable documentation corpus.
type IndexFormat string

const (
	// IndexFormatConcise is the compact Flux knowledge corpus optimized for agent context.
	IndexFormatConcise IndexFormat = "concise"
	// IndexFormatComplete is the full upstream Flux API documentation corpus.
	IndexFormatComplete IndexFormat = "complete"
)

// ParseIndexFormat validates the given format string and returns the
// matching IndexFormat, defaulting to IndexFormatConcise when empty.
func ParseIndexFormat(s string) (IndexFormat, error) {
	switch format := IndexFormat(s); format {
	case "":
		return IndexFormatConcise, nil
	case IndexFormatConcise, IndexFormatComplete:
		return format, nil
	default:
		return "", fmt.Errorf("format must be one of: %s, %s", IndexFormatConcise, IndexFormatComplete)
	}
}

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

// SearchDatabase holds the search indexes for all documentation corpora,
// keyed by format. It is serialized to a single file embedded in the binary.
type SearchDatabase struct {
	Indexes map[IndexFormat]*SearchIndex
}

// Posting represents a term occurrence in a document.
type Posting struct {
	DocID     int // Document index in Documents slice
	Frequency int // Term frequency in this document
}

// GetCompleteDocsMetadata returns the full upstream API document metadata for indexing.
func GetCompleteDocsMetadata() []DocumentMetadata {
	return completeDocsMetadata
}

// GetConciseDocsMetadata returns the compact Flux knowledge document metadata for indexing.
func GetConciseDocsMetadata() []DocumentMetadata {
	return conciseDocsMetadata
}

// completeDocsMetadata is a collection of full upstream API documents,
// each describing metadata like URL, group, kind, and related keywords.
var completeDocsMetadata = []DocumentMetadata{
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
		URL:   "https://raw.githubusercontent.com/fluxcd/source-controller/refs/heads/main/docs/spec/v1/externalartifacts.md",
		Group: fluxcdv1.FluxSourceGroup,
		Kind:  fluxcdv1.FluxExternalArtifactKind,
		Keywords: []string{
			"source-controller", "external", "artifact", "digest", "checksum",
			"revision", "url", "metadata", "origin", "source-watcher",
		},
	},
	{
		URL:   "https://raw.githubusercontent.com/fluxcd/source-watcher/refs/heads/main/docs/spec/v1beta1/artifactgenerators.md",
		Group: fluxcdv1.FluxSourceExtensionsGroup,
		Kind:  fluxcdv1.FluxArtifactGeneratorKind,
		Keywords: []string{
			"source-watcher", "artifact", "generator", "composition", "decomposition", "multiple",
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
			"steps", "job", "migration",
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

const conciseDocsBaseURL = "https://raw.githubusercontent.com/fluxcd/agent-skills/refs/heads/main/skills/gitops-knowledge/references"

// conciseDocsMetadata is a collection of compact Flux knowledge references.
//
// Keep this list in sync with gitops-knowledge/references/*.md. The skill's
// SKILL.md and JSON schemas are intentionally excluded from the MCP docs index.
var conciseDocsMetadata = []DocumentMetadata{
	{
		Title: "Best Practices Reference",
		URL:   conciseDocsBaseURL + "/best-practices.md",
		Keywords: []string{
			"best", "practice", "production", "security", "multi-tenancy", "reliability",
			"multi tenancy", "interval", "retryInterval", "timeout", "prune", "wait",
			"suspend", "namespace", "serviceAccount", "RBAC",
		},
	},
	{
		Title: "Flux Operator Reference",
		URL:   conciseDocsBaseURL + "/flux-operator.md",
		Keywords: []string{
			"Flux", "operator", "FluxInstance", "FluxReport", "distribution", "components",
			"cluster", "storage", "network", "sharding", "bootstrap", "cve", "report",
		},
	},
	{
		Title: "Gitless Image Automation Reference",
		URL:   conciseDocsBaseURL + "/gitless-image-automation.md",
		Keywords: []string{
			"gitless", "image", "automation", "OCIArtifactTag", "OCIRepository", "artifact",
			"tag", "ResourceSet", "ResourceSetInputProvider", "HelmRelease", "Kustomization",
			"preview", "promotion", "registry",
		},
	},
	{
		Title: "Gitless GitOps Reference",
		URL:   conciseDocsBaseURL + "/gitless-gitops.md",
		Keywords: []string{
			"gitless", "GitOps", "OCI artifact", "flux push artifact", "flux tag artifact",
			"GitHub Actions", "registry", "provenance", "attestation", "Cosign",
			"OCIRepository", "Kustomization", "FluxInstance", "GHCR", "promotion",
			"air-gapped", "monorepo",
		},
	},
	{
		Title: "HelmRelease Reference",
		URL:   conciseDocsBaseURL + "/helmrelease.md",
		Keywords: []string{
			"HelmRelease", "HelmRepository", "HelmChart", "OCIRepository", "Helm", "chart",
			"chartRef", "values", "valuesFrom", "upgrade", "install", "rollback", "test",
			"remediation", "driftDetection", "postRenderers", "RetryOnFailure", "releaseName",
			"targetNamespace",
		},
	},
	{
		Title: "Image Automation Reference",
		URL:   conciseDocsBaseURL + "/image-automation.md",
		Keywords: []string{
			"ImageRepository", "ImagePolicy", "ImageUpdateAutomation", "image", "automation",
			"tag", "semver", "alphabetical", "numerical", "filterTags", "regex", "Git",
			"commit",
		},
	},
	{
		Title: "Kustomization Reference",
		URL:   conciseDocsBaseURL + "/kustomization.md",
		Keywords: []string{
			"Kustomization", "Kustomize", "sourceRef", "path", "prune", "wait", "healthChecks",
			"healthCheckExprs", "dependsOn", "readyExpr", "postBuild", "substitute",
			"substituteFrom", "decryption", "SOPS", "remote", "cluster", "kubeConfig",
			"serviceAccountName",
		},
	},
	{
		Title: "MCP Server Reference",
		URL:   conciseDocsBaseURL + "/mcp-server.md",
		Keywords: []string{
			"MCP", "server", "tool", "tools", "Kubernetes", "Flux", "cluster",
			"troubleshooting", "resources", "logs", "metrics", "reconcile",
			"get_flux_instance", "get_kubernetes_resources", "search_flux_docs",
		},
	},
	{
		Title: "Notifications Reference",
		URL:   conciseDocsBaseURL + "/notifications.md",
		Keywords: []string{
			"notification", "Provider", "Alert", "Receiver", "webhook", "HMAC", "Slack",
			"Microsoft Teams", "GitHub", "GitLab", "event", "eventSeverity", "severity",
			"secretRef",
		},
	},
	{
		Title: "Repository Patterns Reference",
		URL:   conciseDocsBaseURL + "/repo-patterns.md",
		Keywords: []string{
			"repository", "repo", "patterns", "structure", "monorepo", "environment",
			"cluster", "tenant", "base", "overlay", "Kustomization", "HelmRelease",
			"ResourceSet", "GitOps", "apps", "infrastructure",
		},
	},
	{
		Title: "ResourceSet Reference",
		URL:   conciseDocsBaseURL + "/resourcesets.md",
		Keywords: []string{
			"ResourceSet", "ResourceSetInputProvider", "inputs", "inputsFrom", "input matrix",
			"template", "templating", "commonMetadata", "dependsOn", "wait", "healthCheckExprs",
			"preview", "GitHub Pull Request", "GitLab Merge Request", "OCIArtifactTag",
		},
	},
	{
		Title: "Sources Reference",
		URL:   conciseDocsBaseURL + "/sources.md",
		Keywords: []string{
			"GitRepository", "OCIRepository", "HelmRepository", "HelmChart", "Bucket",
			"ExternalArtifact", "ArtifactGenerator", "sourceRef", "ref", "interval", "Git",
			"OCI", "Helm", "S3", "artifact", "verification", "Cosign", "semver", "secretRef",
			"provider", "proxy",
		},
	},
	{
		Title: "Terraform Bootstrap Reference",
		URL:   conciseDocsBaseURL + "/terraform-bootstrap.md",
		Keywords: []string{
			"Terraform", "bootstrap", "Flux", "operator", "FluxInstance", "Helm", "Kubernetes",
			"cluster", "install", "infrastructure", "GitOps", "provider",
		},
	},
	{
		Title: "Flux Web UI Reference",
		URL:   conciseDocsBaseURL + "/web-ui.md",
		Keywords: []string{
			"web", "UI", "dashboard", "status", "RBAC", "least privilege", "ServiceAccount",
			"ClusterRole", "FluxReport", "metrics", "tenant", "read-only",
		},
	},
}
