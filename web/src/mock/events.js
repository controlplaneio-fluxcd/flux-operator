// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

// Mock data for resources endpoint (GET /api/v1/resources)
// Generated from real cluster API responses

// Helper to generate timestamps
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

// Mock events data ordered by lastTimestamp descending
const mockEvents = {
  events: [
    {
      lastTimestamp: getTimestamp(0),
      type: "Normal",
      message: "Applied revision: refs/heads/main@sha1:d676e33990dc2865d67c022d26dea93d5e3236ff",
      involvedObject: "Kustomization/flux-system",
      namespace: "flux-system"
    },
    {
      lastTimestamp: getTimestamp(0),
      type: "Normal",
      message: "Server-side apply completed\nFluxInstance/flux-system/flux updated\nResourceSet/flux-system/cluster-infra updated",
      involvedObject: "Kustomization/flux-system",
      namespace: "flux-system"
    },
    {
      lastTimestamp: getTimestamp(1),
      type: "Normal",
      message: "Reconciliation finished in 2s",
      involvedObject: "FluxInstance/flux",
      namespace: "flux-system"
    },
    {
      lastTimestamp: getTimestamp(2),
      type: "Normal",
      message: "Reconciliation finished in 35ms",
      involvedObject: "ResourceSet/flux-operator",
      namespace: "flux-system"
    },
    {
      lastTimestamp: getTimestamp(3),
      type: "Normal",
      message: "Reconciliation finished in 31ms",
      involvedObject: "ResourceSet/tailscale-config",
      namespace: "flux-system"
    },
    {
      lastTimestamp: getTimestamp(3),
      type: "Normal",
      message: "stored artifact for revision 'main@sha1:c1b613a1e083a8918185b11b317f3c75e3c1b6d0'",
      involvedObject: "GitRepository/podinfo",
      namespace: "automation"
    },
    {
      lastTimestamp: getTimestamp(4),
      type: "Normal",
      message: "Applied revision: refs/heads/main@sha1:d676e33990dc2865d67c022d26dea93d5e3236ff",
      involvedObject: "Kustomization/cluster-infra",
      namespace: "flux-system"
    },
    {
      lastTimestamp: getTimestamp(5),
      type: "Normal",
      message: "Reconciliation finished in 27ms",
      involvedObject: "ResourceSet/cluster-infra",
      namespace: "flux-system"
    },
    {
      lastTimestamp: getTimestamp(6),
      type: "Normal",
      message: "Reconciliation finished in 207ms",
      involvedObject: "ResourceSetInputProvider/flux-status-server",
      namespace: "flux-system"
    },
    {
      lastTimestamp: getTimestamp(6),
      type: "Normal",
      message: "successful scan, found 211 tags",
      involvedObject: "ImageRepository/podinfo",
      namespace: "automation"
    },
    {
      lastTimestamp: getTimestamp(7),
      type: "Normal",
      message: "Reconciliation finished in 41ms",
      involvedObject: "ResourceSet/cert-manager",
      namespace: "flux-system"
    },
    {
      lastTimestamp: getTimestamp(8),
      type: "Normal",
      message: "Reconciliation finished in 44ms",
      involvedObject: "ResourceSet/tailscale-operator",
      namespace: "flux-system"
    },
    {
      lastTimestamp: getTimestamp(9),
      type: "Normal",
      message: "Latest image tag for 'ghcr.io/stefanprodan/podinfo' resolved to 6.2.0",
      involvedObject: "ImagePolicy/podinfo",
      namespace: "automation"
    },
    {
      lastTimestamp: getTimestamp(10),
      type: "Normal",
      message: "Reconciliation finished in 41ms",
      involvedObject: "ResourceSet/zot-registry",
      namespace: "flux-system"
    },
    {
      lastTimestamp: getTimestamp(11),
      type: "Normal",
      message: "Reconciliation finished in 36ms",
      involvedObject: "ResourceSet/metrics-server",
      namespace: "flux-system"
    },
    {
      lastTimestamp: getTimestamp(12),
      type: "Normal",
      message: "pushed commit '3ebb95c' to branch 'main'",
      involvedObject: "ImageUpdateAutomation/podinfo",
      namespace: "automation"
    },
    {
      lastTimestamp: getTimestamp(15),
      type: "Normal",
      message: "stored artifact for revision 'sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855'",
      involvedObject: "Bucket/dev-configs",
      namespace: "flux-system"
    },
    {
      lastTimestamp: getTimestamp(15),
      type: "Normal",
      message: "reconciliation in progress: fetching artifact",
      involvedObject: "Bucket/staging-configs",
      namespace: "flux-system"
    },
    {
      lastTimestamp: getTimestamp(15),
      type: "Normal",
      message: "Reconciliation finished in 55ms",
      involvedObject: "ResourceSet/flux-status-server",
      namespace: "flux-system"
    },
    {
      lastTimestamp: getTimestamp(15),
      type: "Normal",
      message: "Valid configuration",
      involvedObject: "Alert/msteams",
      namespace: "automation"
    },
    {
      lastTimestamp: getTimestamp(18),
      type: "Normal",
      message: "Valid configuration",
      involvedObject: "Alert/slack",
      namespace: "flux-system"
    },
    {
      lastTimestamp: getTimestamp(18),
      type: "Normal",
      message: "Valid configuration",
      involvedObject: "Provider/msteams",
      namespace: "automation"
    },
    {
      lastTimestamp: getTimestamp(21),
      type: "Normal",
      message: "Valid configuration",
      involvedObject: "Provider/slack",
      namespace: "flux-system"
    },
    {
      lastTimestamp: getTimestamp(21),
      type: "Normal",
      message: "Receiver initialized for path: /hook/cbdee599b7977a520a36692e5b872c39d09ee53dd75b2e3ae117fea283958fbf",
      involvedObject: "Receiver/podinfo-webhook",
      namespace: "automation"
    },
    {
      lastTimestamp: getTimestamp(24),
      type: "Normal",
      message: "Receiver initialized for path: /hook/bed6d00b5555b1603e1f59b94d7fdbca58089cb5663633fb83f2815dc626d92b",
      involvedObject: "Receiver/github-webhook",
      namespace: "flux-system"
    },
    {
      lastTimestamp: getTimestamp(24),
      type: "Normal",
      message: "successful scan, found 50 tags",
      involvedObject: "ImageRepository/redis",
      namespace: "automation"
    },
    {
      lastTimestamp: getTimestamp(25),
      type: "Warning",
      message: 'authentication failed:\nSTS: AssumeRoleWithWebIdentity, https response error\nPost "https://sts.arn.amazonaws.com/": dial tcp: lookupts.arn.amazonaws.com on 10.100.0.10:53: no such host',
      involvedObject: "Bucket/aws-configs",
      namespace: "flux-system"
    },
    {
      lastTimestamp: getTimestamp(27),
      type: "Normal",
      message: "Latest image tag for 'redis' resolved to 7.0.5",
      involvedObject: "ImagePolicy/redis",
      namespace: "automation"
    },
    {
      lastTimestamp: getTimestamp(40),
      type: "Normal",
      message: "successful scan, found 49 tags",
      involvedObject: "ImageRepository/redis",
      namespace: "automation"
    },
    {
      lastTimestamp: getTimestamp(45),
      type: "Normal",
      message: "stored artifact for revision 'refs/heads/main@sha1:d676e33990dc2865d67c022d26dea93d5e3236ff'",
      involvedObject: "GitRepository/flux-system",
      namespace: "flux-system"
    },
    {
      lastTimestamp: getTimestamp(51),
      type: "Normal",
      message: "stored artifact for digest 'latest@sha256:043536cc6ec06ff978777ca31cf0adc3d654575a2aa8050988aadf90b9f9877c'",
      involvedObject: "OCIRepository/flux-operator",
      namespace: "flux-system"
    },
    {
      lastTimestamp: getTimestamp(54),
      type: "Normal",
      message: "Applied revision: latest@sha256:043536cc6ec06ff978777ca31cf0adc3d654575a2aa8050988aadf90b9f9877c",
      involvedObject: "Kustomization/flux-operator",
      namespace: "flux-system"
    },
    {
      lastTimestamp: getTimestamp(60),
      type: "Normal",
      message: "stored artifact for revision 'sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855'",
      involvedObject: "Bucket/prod-configs",
      namespace: "flux-system"
    },
    {
      lastTimestamp: getTimestamp(65),
      type: "Normal",
      message: "stored artifact for revision 'refs/heads/main@sha1:996fb92427ae41e4649b934ca495991b7852b855'",
      involvedObject: "GitRepository/flux-system",
      namespace: "flux-system"
    },
    {
      lastTimestamp: getTimestamp(72),
      type: "Normal",
      message: "stored artifact for revision 'sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855'",
      involvedObject: "Bucket/preview-configs",
      namespace: "flux-system"
    },
    {
      lastTimestamp: getTimestamp(75),
      type: "Normal",
      message: "stored artifact for digest 'v1.19.1@sha256:9578566b26b2258bcb9a0be27feeaa7c0adaed635cc0f85b6293e42a80c58cc9'",
      involvedObject: "OCIRepository/cert-manager",
      namespace: "cert-manager"
    },
    {
      lastTimestamp: getTimestamp(78),
      type: "Normal",
      message: "stored artifact for digest '3.13.0@sha256:457df0544ec2553176bbaaba70bf5b68af6c400eff510a401b8eba1b13f9a8ad'",
      involvedObject: "OCIRepository/metrics-server",
      namespace: "monitoring"
    },
    {
      lastTimestamp: getTimestamp(82),
      type: "Normal",
      message: "stored artifact: revision 'sha256:2b1fdd97e969c82ee149a7ee8b00f55061760832f23c39a3235936f0912f2125'",
      involvedObject: "HelmRepository/zot-registry",
      namespace: "registry"
    },
    {
      lastTimestamp: getTimestamp(85),
      type: "Normal",
      message: "stored artifact: revision 'sha256:578d082975ad264ba4d09368febb298c3beb7f18e_459bb9d323d3b7c2fc4d475'",
      involvedObject: "HelmRepository/tailscale-operator",
      namespace: "tailscale"
    },
    {
      lastTimestamp: getTimestamp(88),
      type: "Normal",
      message: "pulled 'zot' chart with version '0.1.89'",
      involvedObject: "HelmChart/registry-zot-registry",
      namespace: "registry"
    },
    {
      lastTimestamp: getTimestamp(91),
      type: "Warning",
      message: 'failed to fetch index: unable to connect to the server\nGet "https://charts.bitnami.com/bitnami/index.yaml": timeout awaiting response headers',
      involvedObject: "HelmRepository/bitnami",
      namespace: "flux-system"
    },
    {
      lastTimestamp: getTimestamp(94),
      type: "Normal",
      message: "pulled 'tailscale-operator' chart with version '1.90.6'",
      involvedObject: "HelmChart/tailscale-tailscale-operator",
      namespace: "tailscale"
    },
    {
      lastTimestamp: getTimestamp(98),
      type: "Normal",
      message: "Helm install succeeded for release cert-manager/cert-manager.v1 with chart cert-manager@1.19.1+9578566b26b2",
      involvedObject: "HelmRelease/cert-manager",
      namespace: "cert-manager"
    },
    {
      lastTimestamp: getTimestamp(102),
      type: "Normal",
      message: "Helm install succeeded for release monitoring/metrics-server.v1 with chart metrics-server@3.13.0+457df0544ec2",
      involvedObject: "HelmRelease/metrics-server",
      namespace: "monitoring"
    },
    {
      lastTimestamp: getTimestamp(106),
      type: "Normal",
      message: "Helm install succeeded for release tailscale/tailscale-operator.v1 with chart tailscale-operator@1.90.6+62f0e73f4f82",
      involvedObject: "HelmRelease/tailscale-operator",
      namespace: "tailscale"
    },
    {
      lastTimestamp: getTimestamp(110),
      type: "Normal",
      message: "Helm install succeeded for release registry/zot-registry.v1 with chart zot@0.1.89+aa4f1c1aa5fe",
      involvedObject: "HelmRelease/zot-registry",
      namespace: "registry"
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
