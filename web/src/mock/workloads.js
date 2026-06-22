// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

// Mock data for the workloads index endpoints:
// - GET /api/v1/workloads        (list, getMockWorkloadsList)
// - GET /api/v1/workloads/search (quick search, getMockWorkloadsSearch)
//
// Each entry mirrors the backend WorkloadRef shape. The list/search endpoints
// carry the parent reconciler reference and its status (badge only), never the
// reconciler message. Workloads have no status of their own in the index.

// Helper to match name with wildcard pattern.
// Supports * (matches any characters). If no wildcards, does exact match.
const matchesWildcard = (name, pattern) => {
  name = name.toLowerCase()
  pattern = pattern.toLowerCase()

  // If no wildcards, do exact match
  if (!pattern.includes('*')) {
    return name === pattern
  }

  // Convert wildcard pattern to regex (escape regex specials except *)
  const regexPattern = pattern
    .replace(/[.+?^${}()|[\]\\]/g, '\\$&')
    .replace(/\*/g, '.*')

  const regex = new RegExp(`^${regexPattern}$`, 'i')
  return regex.test(name)
}

// Generate timestamps relative to now (same pattern as resources.js)
const now = new Date()
const getTimestamp = (minutesAgo) => {
  const time = new Date(now.getTime() - minutesAgo * 60000)
  return time.toISOString()
}

export const mockWorkloads = {
  workloads: [
    {
      kind: 'Deployment',
      name: 'podinfo',
      namespace: 'apps',
      apiVersion: 'apps/v1',
      reconcilerKind: 'Kustomization',
      reconcilerNamespace: 'flux-system',
      reconcilerName: 'apps',
      reconcilerStatus: 'Ready',
      lastReconciled: getTimestamp(0)
    },
    {
      kind: 'Deployment',
      name: 'source-controller',
      namespace: 'flux-system',
      apiVersion: 'apps/v1',
      reconcilerKind: 'FluxInstance',
      reconcilerNamespace: 'flux-system',
      reconcilerName: 'flux',
      reconcilerStatus: 'Ready',
      lastReconciled: getTimestamp(1)
    },
    {
      kind: 'Deployment',
      name: 'kustomize-controller',
      namespace: 'flux-system',
      apiVersion: 'apps/v1',
      reconcilerKind: 'FluxInstance',
      reconcilerNamespace: 'flux-system',
      reconcilerName: 'flux',
      reconcilerStatus: 'Ready',
      lastReconciled: getTimestamp(1)
    },
    {
      kind: 'StatefulSet',
      name: 'redis',
      namespace: 'apps',
      apiVersion: 'apps/v1',
      reconcilerKind: 'HelmRelease',
      reconcilerNamespace: 'apps',
      reconcilerName: 'redis',
      reconcilerStatus: 'Ready',
      lastReconciled: getTimestamp(5)
    },
    {
      kind: 'DaemonSet',
      name: 'node-exporter',
      namespace: 'monitoring',
      apiVersion: 'apps/v1',
      reconcilerKind: 'Kustomization',
      reconcilerNamespace: 'flux-system',
      reconcilerName: 'monitoring',
      reconcilerStatus: 'Failed',
      lastReconciled: getTimestamp(8)
    },
    {
      kind: 'CronJob',
      name: 'backup',
      namespace: 'apps',
      apiVersion: 'batch/v1',
      reconcilerKind: 'ResourceSet',
      reconcilerNamespace: 'flux-system',
      reconcilerName: 'apps-jobs',
      reconcilerStatus: 'Ready',
      lastReconciled: getTimestamp(12)
    }
  ]
}

// Filter workloads based on query parameters (kind, name wildcard, namespace).
const filterWorkloads = (params, { wildcard }) => {
  const kindFilter = params.get('kind')
  const nameFilter = params.get('name')
  const namespaceFilter = params.get('namespace')

  return mockWorkloads.workloads.filter(workload => {
    if (kindFilter && workload.kind !== kindFilter) {
      return false
    }
    if (namespaceFilter && workload.namespace !== namespaceFilter) {
      return false
    }
    if (nameFilter) {
      if (wildcard) {
        // Quick-search wraps the term, matching by contains (** matches all)
        const term = nameFilter.toLowerCase()
        if (term !== '**' && !workload.name.toLowerCase().includes(term)) {
          return false
        }
      } else if (!matchesWildcard(workload.name, nameFilter)) {
        return false
      }
    }
    return true
  })
}

// GET /api/v1/workloads - full list with optional filters.
export const getMockWorkloadsList = (endpoint) => {
  // eslint-disable-next-line no-undef
  const url = new URL(endpoint, 'http://localhost')
  const params = url.searchParams

  if (!params.get('kind') && !params.get('name') && !params.get('namespace')) {
    return mockWorkloads
  }

  return { workloads: filterWorkloads(params, { wildcard: false }) }
}

// GET /api/v1/workloads/search - quick-search variant (contains match, limited results).
export const getMockWorkloadsSearch = (endpoint) => {
  // eslint-disable-next-line no-undef
  const url = new URL(endpoint, 'http://localhost')
  const params = url.searchParams

  // No name filter → empty results (mirrors search endpoint behavior)
  if (!params.get('name')) {
    return { workloads: [] }
  }

  const filtered = filterWorkloads(params, { wildcard: true })
  return { workloads: filtered.slice(0, 10) }
}
