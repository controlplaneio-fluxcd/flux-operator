// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

// Helper to generate timestamps within the last 2 hours (events expire after 2h)
const now = new Date()
const getTimestamp = (minutesAgo) => {
  const time = new Date(now.getTime() - minutesAgo * 60000)
  return time.toISOString()
}

// Helper to parse involvedObject field (format: "Kind/name")
const parseInvolvedObject = (involvedObject) => {
  const parts = involvedObject.split('/')
  return {
    kind: parts[0] || '',
    name: parts[1] || ''
  }
}

// Helper to match name with wildcard pattern
// Supports * (matches any characters). If no wildcards, does exact match.
const matchesWildcard = (name, pattern) => {
  name = name.toLowerCase()
  pattern = pattern.toLowerCase()

  // If no wildcards, do exact match
  if (!pattern.includes('*')) {
    return name === pattern
  }

  // Convert wildcard pattern to regex
  // Escape special regex characters except *
  const regexPattern = pattern
    .replace(/[.+?^${}()|[\]\\]/g, '\\$&')
    .replace(/\*/g, '.*')

  const regex = new RegExp(`^${regexPattern}$`, 'i')
  return regex.test(name)
}

const mockEvents = {
  events: [
    // FluxInstance (3 events)
    {
      lastTimestamp: getTimestamp(5),
      type: "Normal",
      message: "Reconciliation finished in 3s",
      involvedObject: "FluxInstance/flux",
      namespace: "flux-system"
    },
    {
      lastTimestamp: getTimestamp(45),
      type: "Normal",
      message: "FluxInstance health check passed",
      involvedObject: "FluxInstance/flux",
      namespace: "flux-system"
    },
    {
      lastTimestamp: getTimestamp(90),
      type: "Normal",
      message: "Upgrade completed to version v2.7.3",
      involvedObject: "FluxInstance/flux",
      namespace: "flux-system"
    },
    // ResourceSet (2 events)
    {
      lastTimestamp: getTimestamp(8),
      type: "Normal",
      message: "Applied 15 resources, Health: Healthy",
      involvedObject: "ResourceSet/flux-controllers",
      namespace: "flux-system"
    },
    {
      lastTimestamp: getTimestamp(75),
      type: "Normal",
      message: "All resources reconciled successfully",
      involvedObject: "ResourceSet/monitoring",
      namespace: "flux-system"
    },
    // ResourceSetInputProvider (2 events)
    {
      lastTimestamp: getTimestamp(10),
      type: "Normal",
      message: "Input synchronized from GitRepository/flux-system",
      involvedObject: "ResourceSetInputProvider/flux-config",
      namespace: "flux-system"
    },
    {
      lastTimestamp: getTimestamp(55),
      type: "Warning",
      message: "Input source temporarily unavailable, using cached version",
      involvedObject: "ResourceSetInputProvider/apps-config",
      namespace: "flux-system"
    },
    // Kustomization (4 events)
    {
      lastTimestamp: getTimestamp(15),
      type: "Normal",
      message: "Reconciliation finished in 145ms, next run in 10m0s",
      involvedObject: "Kustomization/flux-system",
      namespace: "flux-system"
    },
    {
      lastTimestamp: getTimestamp(30),
      type: "Warning",
      message: "Reconciliation failed\nError: failed to apply manifests\n  Caused by:\n    - manifest validation failed for Deployment/podinfo\n    - field spec.replicas: Invalid value: \"invalid\": must be an integer\n  Retrying in 5m0s",
      involvedObject: "Kustomization/apps",
      namespace: "flux-system"
    },
    {
      lastTimestamp: getTimestamp(65),
      type: "Normal",
      message: "Pruned 3 resources that are no longer in the source",
      involvedObject: "Kustomization/infrastructure",
      namespace: "flux-system"
    },
    {
      lastTimestamp: getTimestamp(105),
      type: "Normal",
      message: "Health assessment passed: all resources ready",
      involvedObject: "Kustomization/apps",
      namespace: "flux-system"
    },
    // HelmRelease (4 events)
    {
      lastTimestamp: getTimestamp(95),
      type: "Warning",
      message: "Helm install failed: failed to download chart from https://stefanprodan.github.io/podinfo\nError details:\n  Status: 404 Not Found\n  URL: https://stefanprodan.github.io/podinfo/podinfo-6.5.0.tgz\n  Response: The requested chart version does not exist in the repository\n\nPlease verify:\n  1. Chart name is correct\n  2. Chart version exists in the repository\n  3. Repository URL is accessible\n\nThis is a very long error message that will definitely need truncation and the show more/less functionality to work properly. It contains multiple lines and detailed debugging information that should be preserved when expanded.",
      involvedObject: "HelmRelease/podinfo",
      namespace: "default"
    },
    {
      lastTimestamp: getTimestamp(12),
      type: "Normal",
      message: "Helm upgrade succeeded, chart version: 6.5.4",
      involvedObject: "HelmRelease/ingress-nginx",
      namespace: "kube-system"
    },
    {
      lastTimestamp: getTimestamp(40),
      type: "Normal",
      message: "Helm test completed successfully",
      involvedObject: "HelmRelease/cert-manager",
      namespace: "cert-manager"
    },
    {
      lastTimestamp: getTimestamp(80),
      type: "Warning",
      message: "Helm rollback initiated due to failed upgrade\nRolling back to revision 5",
      involvedObject: "HelmRelease/prometheus",
      namespace: "monitoring"
    },
    // GitRepository (3 events)
    {
      lastTimestamp: getTimestamp(35),
      type: "Warning",
      message: "Failed to fetch artifact: connection timeout",
      involvedObject: "GitRepository/flux-system",
      namespace: "flux-system"
    },
    {
      lastTimestamp: getTimestamp(18),
      type: "Normal",
      message: "Stored artifact for revision: main@sha1:a7b3c2d",
      involvedObject: "GitRepository/podinfo",
      namespace: "default"
    },
    {
      lastTimestamp: getTimestamp(60),
      type: "Normal",
      message: "Repository cloned successfully",
      involvedObject: "GitRepository/apps",
      namespace: "flux-system"
    },
    // OCIRepository (2 events)
    {
      lastTimestamp: getTimestamp(20),
      type: "Normal",
      message: "artifact up-to-date with remote revision: '0.33.0@sha256:5a303365aa7479964476cb151926616b6cb5980002e3e28f9007772a12673c42'",
      involvedObject: "OCIRepository/prometheus-config",
      namespace: "monitoring"
    },
    {
      lastTimestamp: getTimestamp(70),
      type: "Normal",
      message: "Pulled OCI artifact from ghcr.io/stefanprodan/manifests",
      involvedObject: "OCIRepository/manifests",
      namespace: "flux-system"
    },
    // HelmRepository (2 events)
    {
      lastTimestamp: getTimestamp(22),
      type: "Normal",
      message: "Fetched index: 247 charts",
      involvedObject: "HelmRepository/bitnami",
      namespace: "flux-system"
    },
    {
      lastTimestamp: getTimestamp(85),
      type: "Warning",
      message: "Repository index download failed: TLS handshake timeout",
      involvedObject: "HelmRepository/stable",
      namespace: "flux-system"
    },
    // HelmChart (2 events)
    {
      lastTimestamp: getTimestamp(25),
      type: "Normal",
      message: "Pulled chart version 15.2.3",
      involvedObject: "HelmChart/nginx",
      namespace: "flux-system"
    },
    {
      lastTimestamp: getTimestamp(50),
      type: "Normal",
      message: "Chart package verified with digest sha256:xyz789",
      involvedObject: "HelmChart/prometheus",
      namespace: "monitoring"
    },
    // Bucket (2 events)
    {
      lastTimestamp: getTimestamp(42),
      type: "Warning",
      message: "Failed to download bucket: access denied\nError: The AWS Access Key Id you provided does not exist in our records\nBucket: s3://my-flux-bucket\nRegion: us-west-2",
      involvedObject: "Bucket/terraform-state",
      namespace: "flux-system"
    },
    {
      lastTimestamp: getTimestamp(100),
      type: "Normal",
      message: "Bucket contents synchronized successfully",
      involvedObject: "Bucket/configs",
      namespace: "flux-system"
    },
    // ImageRepository (2 events)
    {
      lastTimestamp: getTimestamp(28),
      type: "Normal",
      message: "Found 12 tags for image ghcr.io/stefanprodan/podinfo",
      involvedObject: "ImageRepository/podinfo",
      namespace: "flux-system"
    },
    {
      lastTimestamp: getTimestamp(110),
      type: "Normal",
      message: "Scanning image repository for new tags",
      involvedObject: "ImageRepository/app",
      namespace: "default"
    },
    // ImagePolicy (2 events)
    {
      lastTimestamp: getTimestamp(32),
      type: "Normal",
      message: "Latest image tag for policy semver:^6.x is: 6.5.4",
      involvedObject: "ImagePolicy/podinfo",
      namespace: "flux-system"
    },
    {
      lastTimestamp: getTimestamp(88),
      type: "Normal",
      message: "Policy evaluation successful, selected tag: v1.2.3",
      involvedObject: "ImagePolicy/backend",
      namespace: "default"
    },
    // ImageUpdateAutomation (2 events)
    {
      lastTimestamp: getTimestamp(48),
      type: "Normal",
      message: "Committed and pushed change to branch main",
      involvedObject: "ImageUpdateAutomation/flux-system",
      namespace: "flux-system"
    },
    {
      lastTimestamp: getTimestamp(115),
      type: "Warning",
      message: "Failed to push commits: authentication failed\nVerify git credentials and permissions",
      involvedObject: "ImageUpdateAutomation/apps",
      namespace: "default"
    },
    // Alert (1 event)
    {
      lastTimestamp: getTimestamp(38),
      type: "Normal",
      message: "Notification sent to slack channel #flux-alerts",
      involvedObject: "Alert/on-call-alerts",
      namespace: "flux-system"
    },
    // Provider (1 event)
    {
      lastTimestamp: getTimestamp(52),
      type: "Normal",
      message: "Provider initialized: Slack webhook configured",
      involvedObject: "Provider/slack",
      namespace: "flux-system"
    },
    // Receiver (1 event)
    {
      lastTimestamp: getTimestamp(78),
      type: "Normal",
      message: "Webhook received, triggered reconciliation for GitRepository/flux-system",
      involvedObject: "Receiver/github-webhook",
      namespace: "flux-system"
    },
    // ArtifactGenerator (1 event)
    {
      lastTimestamp: getTimestamp(58),
      type: "Normal",
      message: "Generated artifact from ConfigMap/app-config",
      involvedObject: "ArtifactGenerator/config-bundle",
      namespace: "flux-system"
    },
    // ExternalArtifact (1 event)
    {
      lastTimestamp: getTimestamp(68),
      type: "Normal",
      message: "Artifact synchronized from external source",
      involvedObject: "ExternalArtifact/vendor-manifests",
      namespace: "flux-system"
    }
  ]
}

// Export function that filters events based on query parameters
export const getMockEvents = (endpoint) => {
  // Parse query params from endpoint URL
  // eslint-disable-next-line no-undef
  const url = new URL(endpoint, 'http://localhost')
  const params = url.searchParams

  const kindFilter = params.get('kind')
  const nameFilter = params.get('name')
  const namespaceFilter = params.get('namespace')
  const typeFilter = params.get('type')

  // If no filters, return all events
  if (!kindFilter && !nameFilter && !namespaceFilter && !typeFilter) {
    return mockEvents
  }

  // Filter events based on query parameters
  const filteredEvents = mockEvents.events.filter(event => {
    const { kind, name } = parseInvolvedObject(event.involvedObject)

    // Filter by kind
    if (kindFilter && kind !== kindFilter) {
      return false
    }

    // Filter by name (exact or wildcard match, case-insensitive)
    if (nameFilter && !matchesWildcard(name, nameFilter)) {
      return false
    }

    // Filter by namespace
    if (namespaceFilter && event.namespace !== namespaceFilter) {
      return false
    }

    // Filter by type (severity: Normal, Warning)
    if (typeFilter && event.type !== typeFilter) {
      return false
    }

    return true
  })

  return { events: filteredEvents }
}
