// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

// docsMetadata is a collection of DocumentMetadata instances,
// each describing metadata like URL, group, kind, and related keywords.
var docsMetadata = []DocumentMetadata{
	{
		URL:   "https://raw.githubusercontent.com/fluxcd/source-controller/refs/heads/main/docs/spec/v1/gitrepositories.md",
		Group: "source.toolkit.fluxcd.io",
		Kind:  "GitRepository",
		Keywords: []string{
			"source-controller", "git", "branch", "commit", "sha", "ref", "tag", "semver",
			"verification", "pgp", "signature", "ssh", "private", "public", "tls", "auth",
			"include", "submodules", "sparse", "checkout", "proxy", "ignore",
			"github", "gitlab", "devops", "githubapp",
		},
	},
	{
		URL:   "https://raw.githubusercontent.com/fluxcd/source-controller/refs/heads/main/docs/spec/v1beta2/ocirepositories.md",
		Group: "source.toolkit.fluxcd.io",
		Kind:  "OCIRepository",
		Keywords: []string{
			"source-controller", "oci", "registry", "artifact", "tag", "semver", "digest",
			"verification", "cosign", "notation", "signature", "keyless", "layer", "proxy",
			"media", "auth", "provider", "aws", "azure", "gcp", "identity", "iam", "tls",
		},
	},
	{
		URL:   "https://raw.githubusercontent.com/fluxcd/source-controller/refs/heads/main/docs/spec/v1/helmrepositories.md",
		Group: "source.toolkit.fluxcd.io",
		Kind:  "HelmRepository",
		Keywords: []string{
			"source-controller", "index.yaml", "authentication",
		},
	},
	{
		URL:   "https://raw.githubusercontent.com/fluxcd/source-controller/refs/heads/main/docs/spec/v1/helmcharts.md",
		Group: "source.toolkit.fluxcd.io",
		Kind:  "HelmChart",
		Keywords: []string{
			"source-controller", "chart", "valuesfiles", "strategy", "version",
		},
	},
	{
		URL:   "https://raw.githubusercontent.com/fluxcd/source-controller/refs/heads/main/docs/spec/v1/buckets.md",
		Group: "source.toolkit.fluxcd.io",
		Kind:  "Bucket",
		Keywords: []string{
			"source-controller", "s3", "storage", "minio", "blob", "endpoint",
			"region", "insecure", "managed-identity", "sas", "token", "certificate",
			"proxy", "authentication", "provider", "aws", "azure", "gcp",
		},
	},
	{
		URL:   "https://raw.githubusercontent.com/fluxcd/kustomize-controller/refs/heads/main/docs/spec/v1/kustomizations.md",
		Group: "kustomize.toolkit.fluxcd.io",
		Kind:  "Kustomization",
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
		Group: "helm.toolkit.fluxcd.io",
		Kind:  "HelmRelease",
		Keywords: []string{
			"helm-controller", "helm", "chart", "release", "values", "upgrade", "install",
			"uninstall", "rollback", "test", "remediation", "drift", "detection", "kubeconfig",
			"target", "storage", "timeout", "renderer", "depends", "retry", "tenant",
		},
	},
	{
		URL:   "https://raw.githubusercontent.com/fluxcd/notification-controller/refs/heads/main/docs/spec/v1/receivers.md",
		Group: "notification.toolkit.fluxcd.io",
		Kind:  "Receiver",
		Keywords: []string{
			"notification-controller", "webhook", "receiver", "hmac", "trigger",
			"github", "gitlab", "bitbucket", "harbor", "cdevents", "payload",
		},
	},
	{
		URL:   "https://raw.githubusercontent.com/fluxcd/notification-controller/refs/heads/main/docs/spec/v1beta3/alerts.md",
		Group: "notification.toolkit.fluxcd.io",
		Kind:  "Alert",
		Keywords: []string{
			"notification-controller", "alerting", "event", "notification", "observability",
			"incident", "error", "info", "severity",
		},
	},
	{
		URL:   "https://raw.githubusercontent.com/fluxcd/notification-controller/refs/heads/main/docs/spec/v1beta3/providers.md",
		Group: "notification.toolkit.fluxcd.io",
		Kind:  "Provider",
		Keywords: []string{
			"notification-controller", "alert", "notification", "slack", "teams",
			"pagerduty", "discord", "matrix", "lark", "rocket", "datadog", "grafana",
			"sentry", "telegram", "webex", "nats", "pubsub", "eventhub", "dispatch",
		},
	},
	{
		URL:   "https://raw.githubusercontent.com/fluxcd/image-reflector-controller/refs/heads/main/docs/spec/v1beta2/imagerepositories.md",
		Group: "image.toolkit.fluxcd.io",
		Kind:  "ImageRepository",
		Keywords: []string{
			"image-reflector-controller", "container", "image", "tags",
			"docker", "ecr", "gar", "acr", "scan",
		},
	},
	{
		URL:   "https://raw.githubusercontent.com/fluxcd/image-reflector-controller/refs/heads/main/docs/spec/v1beta2/imagepolicies.md",
		Group: "image.toolkit.fluxcd.io",
		Kind:  "ImagePolicy",
		Keywords: []string{
			"image-reflector-controller", "container", "image", "policy", "tag", "semver", "range",
			"numerical", "alphabetical", "order", "filter", "pattern", "regex", "latest",
		},
	},
	{
		URL:   "https://raw.githubusercontent.com/fluxcd/image-automation-controller/refs/heads/main/docs/spec/v1beta2/imageupdateautomations.md",
		Group: "image.toolkit.fluxcd.io",
		Kind:  "ImageUpdateAutomation",
		Keywords: []string{
			"image-automation-controller", "docker", "container", "image", "tag",
			"policy", "update", "commit", "push", "git", "scan", "automation", "automate",
		},
	},
	{
		URL:   "https://raw.githubusercontent.com/controlplaneio-fluxcd/flux-operator/refs/heads/main/docs/api/v1/fluxinstance.md",
		Group: "fluxcd.controlplane.io",
		Kind:  "FluxInstance",
		Keywords: []string{
			"flux-operator", "distribution", "registry", "components", "sync",
			"cluster", "storage", "multitenant", "network", "controller",
			"domain", "sharding", "migrate", "bootstrap", "cve", "auto", "deploy",
		},
	},
	{
		URL:   "https://raw.githubusercontent.com/controlplaneio-fluxcd/flux-operator/refs/heads/main/docs/api/v1/fluxreport.md",
		Group: "fluxcd.controlplane.io",
		Kind:  "FluxReport",
		Keywords: []string{
			"flux-operator", "report", "monitor", "stats", "readiness",
			"metric", "prometheus", "entitlement", "size", "info", "troubleshooting",
		},
	},
	{
		URL:   "https://raw.githubusercontent.com/controlplaneio-fluxcd/flux-operator/refs/heads/main/docs/api/v1/resourceset.md",
		Group: "fluxcd.controlplane.io",
		Kind:  "ResourceSet",
		Keywords: []string{
			"flux-operator", "resource", "inputs", "template", "templating",
			"common", "depends", "wait", "timeout", "account",
			"health", "cel", "prune", "inventory", "app", "definition",
		},
	},
	{
		URL:   "https://raw.githubusercontent.com/controlplaneio-fluxcd/flux-operator/refs/heads/main/docs/api/v1/resourcesetinputprovider.md",
		Group: "fluxcd.controlplane.io",
		Kind:  "ResourceSetInputProvider",
		Keywords: []string{
			"flux-operator", "input", "provider", "pull", "merge", "request",
			"author", "github", "gitlab", "filter", "labels", "branch",
			"exclude", "default", "exported", "preview", "ephemeral", "environment",
		},
	},
}
