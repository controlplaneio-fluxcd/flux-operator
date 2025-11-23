// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

// Mock data for resources endpoint (GET /api/v1/resources)
// Generated from real cluster API responses

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

// Generate timestamps relative to now (same pattern as events.js)
const now = new Date()
const getTimestamp = (minutesAgo) => {
  const time = new Date(now.getTime() - minutesAgo * 60000)
  return time.toISOString()
}

export const mockResources = {
  resources: [
    // Ordered by lastReconciled descending (most recent first)
    {
      name: "flux-system",
      kind: "Kustomization",
      namespace: "flux-system",
      status: "Ready",
      message: "Applied revision: refs/heads/main@sha1:d676e33990dc2865d67c022d26dea93d5e3236ff",
      lastReconciled: getTimestamp(0)
    },
    {
      name: "flux",
      kind: "FluxInstance",
      namespace: "flux-system",
      status: "Ready",
      message: "Reconciliation finished in 2s",
      lastReconciled: getTimestamp(1)
    },
    {
      name: "flux-operator",
      kind: "ResourceSet",
      namespace: "flux-system",
      status: "Ready",
      message: "Reconciliation finished in 35ms",
      lastReconciled: getTimestamp(2)
    },
    {
      name: "podinfo",
      kind: "GitRepository",
      namespace: "automation",
      status: "Ready",
      message: "stored artifact for revision 'main@sha1:c1b613a1e083a8918185b11b317f3c75e3c1b6d0'",
      lastReconciled: getTimestamp(3)
    },
    {
      name: "tailscale-config",
      kind: "ResourceSet",
      namespace: "flux-system",
      status: "Ready",
      message: "Reconciliation finished in 31ms",
      lastReconciled: getTimestamp(3)
    },
    {
      name: "cluster-infra",
      kind: "Kustomization",
      namespace: "flux-system",
      status: "Ready",
      message: "Applied revision: refs/heads/main@sha1:d676e33990dc2865d67c022d26dea93d5e3236ff",
      lastReconciled: getTimestamp(4)
    },
    {
      name: "cluster-infra",
      kind: "ResourceSet",
      namespace: "flux-system",
      status: "Ready",
      message: "Reconciliation finished in 27ms",
      lastReconciled: getTimestamp(5)
    },
    {
      name: "flux-status-server",
      kind: "ResourceSetInputProvider",
      namespace: "flux-system",
      status: "Ready",
      message: "Reconciliation finished in 207ms",
      lastReconciled: getTimestamp(6)
    },
    {
      name: "podinfo",
      kind: "ImageRepository",
      namespace: "automation",
      status: "Ready",
      message: "successful scan, found 211 tags",
      lastReconciled: getTimestamp(6)
    },
    {
      name: "cert-manager",
      kind: "ResourceSet",
      namespace: "flux-system",
      status: "Ready",
      message: "Reconciliation finished in 41ms",
      lastReconciled: getTimestamp(7)
    },
    {
      name: "tailscale-operator",
      kind: "ResourceSet",
      namespace: "flux-system",
      status: "Ready",
      message: "Reconciliation finished in 44ms",
      lastReconciled: getTimestamp(8)
    },
    {
      name: "podinfo",
      kind: "ImagePolicy",
      namespace: "automation",
      status: "Ready",
      message: "Latest image tag for 'ghcr.io/stefanprodan/podinfo' resolved to 6.2.0",
      lastReconciled: getTimestamp(9)
    },
    {
      name: "zot-registry",
      kind: "ResourceSet",
      namespace: "flux-system",
      status: "Ready",
      message: "Reconciliation finished in 41ms",
      lastReconciled: getTimestamp(10)
    },
    {
      name: "metrics-server",
      kind: "ResourceSet",
      namespace: "flux-system",
      status: "Ready",
      message: "Reconciliation finished in 36ms",
      lastReconciled: getTimestamp(11)
    },
    {
      name: "podinfo",
      kind: "ImageUpdateAutomation",
      namespace: "automation",
      status: "Ready",
      message: "pushed commit '3ebb95c' to branch 'main'",
      lastReconciled: getTimestamp(12)
    },
    {
      name: "dev-configs",
      kind: "Bucket",
      namespace: "flux-system",
      status: "Ready",
      message: "stored artifact for revision 'sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855'",
      lastReconciled: getTimestamp(15)
    },
    {
      name: "staging-configs",
      kind: "Bucket",
      namespace: "flux-system",
      status: "Progressing",
      message: "reconciliation in progress: fetching artifact",
      lastReconciled: getTimestamp(15)
    },
    {
      name: "flux-status-server",
      kind: "ResourceSet",
      namespace: "flux-system",
      status: "Ready",
      message: "Reconciliation finished in 55ms",
      lastReconciled: getTimestamp(15)
    },
    {
      name: "msteams",
      kind: "Alert",
      namespace: "automation",
      status: "Ready",
      message: "Valid configuration",
      lastReconciled: getTimestamp(15)
    },
    {
      name: "slack",
      kind: "Alert",
      namespace: "flux-system",
      status: "Ready",
      message: "Valid configuration",
      lastReconciled: getTimestamp(18)
    },
    {
      name: "msteams",
      kind: "Provider",
      namespace: "automation",
      status: "Ready",
      message: "Valid configuration",
      lastReconciled: getTimestamp(18)
    },
    {
      name: "slack",
      kind: "Provider",
      namespace: "flux-system",
      status: "Ready",
      message: "Valid configuration",
      lastReconciled: getTimestamp(21)
    },
    {
      name: "podinfo-webhook",
      kind: "Receiver",
      namespace: "automation",
      status: "Ready",
      message: "Receiver initialized for path: /hook/cbdee599b7977a520a36692e5b872c39d09ee53dd75b2e3ae117fea283958fbf",
      lastReconciled: getTimestamp(21)
    },
    {
      name: "github-webhook",
      kind: "Receiver",
      namespace: "flux-system",
      status: "Ready",
      message: "Receiver initialized for path: /hook/bed6d00b5555b1603e1f59b94d7fdbca58089cb5663633fb83f2815dc626d92b",
      lastReconciled: getTimestamp(24)
    },
    {
      name: "redis",
      kind: "ImageRepository",
      namespace: "automation",
      status: "Suspended",
      message: "successful scan, found 50 tags",
      lastReconciled: getTimestamp(24)
    },
    {
      name: "aws-configs",
      kind: "Bucket",
      namespace: "flux-system",
      status: "Failed",
      message: 'authentication failed:\nSTS: AssumeRoleWithWebIdentity, https response error\nPost "https://sts.arn.amazonaws.com/": dial tcp: lookupts.arn.amazonaws.com on 10.100.0.10:53: no such host',
      lastReconciled: getTimestamp(25)
    },
    {
      name: "redis",
      kind: "ImagePolicy",
      namespace: "automation",
      status: "Suspended",
      message: "Latest image tag for 'redis' resolved to 7.0.5",
      lastReconciled: getTimestamp(27)
    },
    {
      name: "flux-system",
      kind: "GitRepository",
      namespace: "flux-system",
      status: "Ready",
      message: "stored artifact for revision 'refs/heads/main@sha1:d676e33990dc2865d67c022d26dea93d5e3236ff'",
      lastReconciled: getTimestamp(45)
    },
    {
      name: "flux-operator",
      kind: "OCIRepository",
      namespace: "flux-system",
      status: "Ready",
      message: "stored artifact for digest 'latest@sha256:043536cc6ec06ff978777ca31cf0adc3d654575a2aa8050988aadf90b9f9877c'",
      lastReconciled: getTimestamp(51)
    },
    {
      name: "flux-operator",
      kind: "Kustomization",
      namespace: "flux-system",
      status: "Ready",
      message: "Applied revision: latest@sha256:043536cc6ec06ff978777ca31cf0adc3d654575a2aa8050988aadf90b9f9877c",
      lastReconciled: getTimestamp(54)
    },
    {
      name: "prod-configs",
      kind: "Bucket",
      namespace: "flux-system",
      status: "Ready",
      message: "stored artifact for revision 'sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855'",
      lastReconciled: getTimestamp(60)
    },
    {
      name: "preview-configs",
      kind: "Bucket",
      namespace: "flux-system",
      status: "Suspended",
      message: "stored artifact for revision 'sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855'",
      lastReconciled: getTimestamp(72)
    },
    {
      name: "cert-manager",
      kind: "OCIRepository",
      namespace: "cert-manager",
      status: "Ready",
      message: "stored artifact for digest 'v1.19.1@sha256:9578566b26b2258bcb9a0be27feeaa7c0adaed635cc0f85b6293e42a80c58cc9'",
      lastReconciled: getTimestamp(75)
    },
    {
      name: "metrics-server",
      kind: "OCIRepository",
      namespace: "monitoring",
      status: "Ready",
      message: "stored artifact for digest '3.13.0@sha256:457df0544ec2553176bbaaba70bf5b68af6c400eff510a401b8eba1b13f9a8ad'",
      lastReconciled: getTimestamp(78)
    },
    {
      name: "zot-registry",
      kind: "HelmRepository",
      namespace: "registry",
      status: "Ready",
      message: "stored artifact: revision 'sha256:2b1fdd97e969c82ee149a7ee8b00f55061760832f23c39a3235936f0912f2125'",
      lastReconciled: getTimestamp(82)
    },
    {
      name: "tailscale-operator",
      kind: "HelmRepository",
      namespace: "tailscale",
      status: "Ready",
      message: "stored artifact: revision 'sha256:578d082975ad264ba4d09368febb298c3beb7f18e_459bb9d323d3b7c2fc4d475'",
      lastReconciled: getTimestamp(85)
    },
    {
      name: "registry-zot-registry",
      kind: "HelmChart",
      namespace: "registry",
      status: "Ready",
      message: "pulled 'zot' chart with version '0.1.89'",
      lastReconciled: getTimestamp(88)
    },
    {
      name: "bitnami",
      kind: "HelmRepository",
      namespace: "flux-system",
      status: "Failed",
      message: 'failed to fetch index: unable to connect to the server\nGet "https://charts.bitnami.com/bitnami/index.yaml": timeout awaiting response headers',
      lastReconciled: getTimestamp(91)
    },
    {
      name: "tailscale-tailscale-operator",
      kind: "HelmChart",
      namespace: "tailscale",
      status: "Ready",
      message: "pulled 'tailscale-operator' chart with version '1.90.6'",
      lastReconciled: getTimestamp(94)
    },
    {
      name: "cert-manager",
      kind: "HelmRelease",
      namespace: "cert-manager",
      status: "Ready",
      message: "Helm install succeeded for release cert-manager/cert-manager.v1 with chart cert-manager@1.19.1+9578566b26b2",
      lastReconciled: getTimestamp(98)
    },
    {
      name: "metrics-server",
      kind: "HelmRelease",
      namespace: "monitoring",
      status: "Ready",
      message: "Helm install succeeded for release monitoring/metrics-server.v1 with chart metrics-server@3.13.0+457df0544ec2",
      lastReconciled: getTimestamp(102)
    },
    {
      name: "tailscale-operator",
      kind: "HelmRelease",
      namespace: "tailscale",
      status: "Ready",
      message: "Helm install succeeded for release tailscale/tailscale-operator.v1 with chart tailscale-operator@1.90.6+62f0e73f4f82",
      lastReconciled: getTimestamp(106)
    },
    {
      name: "zot-registry",
      kind: "HelmRelease",
      namespace: "registry",
      status: "Ready",
      message: "Helm install succeeded for release registry/zot-registry.v1 with chart zot@0.1.89+aa4f1c1aa5fe",
      lastReconciled: getTimestamp(110)
    },
    {
      name: "default-configs",
      kind: "Bucket",
      namespace: "default",
      status: "Unknown",
      message: "No status information available",
      lastReconciled: getTimestamp(115)
    }
  ]
}

// Export function that filters resources based on query parameters
export const getMockResources = (endpoint) => {
  // Parse query params from endpoint URL
  // eslint-disable-next-line no-undef
  const url = new URL(endpoint, 'http://localhost')
  const params = url.searchParams

  const kindFilter = params.get('kind')
  const nameFilter = params.get('name')
  const namespaceFilter = params.get('namespace')
  const statusFilter = params.get('status')

  // If no filters, return all resources
  if (!kindFilter && !nameFilter && !namespaceFilter && !statusFilter) {
    return mockResources
  }

  // Filter resources based on query parameters
  const filteredResources = mockResources.resources.filter(resource => {
    // Filter by kind
    if (kindFilter && resource.kind !== kindFilter) {
      return false
    }

    // Filter by name (exact or wildcard match, case-insensitive)
    if (nameFilter && !matchesWildcard(resource.name, nameFilter)) {
      return false
    }

    // Filter by namespace
    if (namespaceFilter && resource.namespace !== namespaceFilter) {
      return false
    }

    // Filter by status (Ready, Failed, Progressing, Suspended, Unknown)
    if (statusFilter && resource.status !== statusFilter) {
      return false
    }

    return true
  })

  return { resources: filteredResources }
}

// Export function that filters resources for quick search (GET /api/v1/search?name=<term>)
// Filters by name (contains, case-insensitive) and limits to applier kinds
export const getMockSearchResults = (endpoint) => {
  // Parse query params from endpoint URL
  // eslint-disable-next-line no-undef
  const url = new URL(endpoint, 'http://localhost')
  const params = url.searchParams

  const nameFilter = params.get('name')

  // If no name filter, return empty results
  if (!nameFilter) {
    return { resources: [] }
  }

  // Applier kinds that the search endpoint returns (matches search.go)
  const searchKinds = ['FluxInstance', 'ResourceSet', 'Kustomization', 'HelmRelease']

  const searchTerm = nameFilter.toLowerCase()

  // Filter resources: match name (contains) and limit to applier kinds
  const filteredResources = mockResources.resources.filter(resource => {
    // Only include applier kinds
    if (!searchKinds.includes(resource.kind)) {
      return false
    }

    // Filter by name with case-insensitive contains
    if (!resource.name.toLowerCase().includes(searchTerm)) {
      return false
    }

    return true
  })

  // Limit to 10 results per kind (matches backend limit)
  const limitedResources = filteredResources.slice(0, 40) // 10 per kind * 4 kinds max

  return { resources: limitedResources }
}
