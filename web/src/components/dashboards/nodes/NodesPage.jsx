// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useState, useEffect, useCallback } from 'preact/hooks'
import { fetchWithMock } from '../../../utils/fetch'
import { usePageMeta } from '../../../utils/meta'
import { POLL_INTERVAL_MS } from '../../../utils/constants'
import { useRegisterPageShortcuts } from '../../../utils/useRegisterPageShortcuts'
import { commonNodeSuffix, shortNamer, plural } from '../../../utils/nodes'
import { StatusHeroCard } from '../common/StatusHeroCard'
import { CapacityPanel } from './CapacityPanel'
import { FindingsList } from './FindingsList'
import { NodesList } from './NodesList'
import { BurstingWorkloadsPanel } from './BurstingWorkloadsPanel'

const HERO = {
  critical: {
    title: 'Nodes Critical', color: 'text-danger', bgColor: 'bg-red-50', borderColor: 'border-danger',
    path: 'M12 9v2m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z'
  },
  warn: {
    title: 'Nodes Degraded', color: 'text-warning', bgColor: 'bg-yellow-50', borderColor: 'border-warning',
    path: 'M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z'
  },
  ok: {
    title: 'Nodes Healthy', color: 'text-success', bgColor: 'bg-green-50', borderColor: 'border-success',
    path: 'M5 13l4 4L19 7'
  }
}

/**
 * AccessDenied - shown when the API answers 403 (the user cannot list nodes).
 */
function AccessDenied() {
  return (
    <div class="card border-warning" data-testid="nodes-access-denied">
      <h3 class="text-sm font-medium text-yellow-800 dark:text-yellow-200">Access restricted</h3>
      <p class="mt-1 text-sm text-yellow-700 dark:text-yellow-300">Requires permission to list nodes.</p>
    </div>
  )
}

/**
 * NodesHero - status header card driven by the worst finding line, with the
 * node count and the critical and warning line counts.
 */
function NodesHero({ data, lastUpdatedAt }) {
  const lines = data.findings || []
  const critical = lines.filter(l => l.severity === 'critical').length
  const warn = lines.filter(l => l.severity === 'warn').length
  const hero = HERO[critical > 0 ? 'critical' : warn > 0 ? 'warn' : 'ok']
  const subtitle = [
    plural((data.nodes || []).length, 'node'),
    critical > 0 && `${critical} critical`,
    warn > 0 && plural(warn, 'warning')
  ].filter(Boolean).join(' · ')
  return (
    <StatusHeroCard
      bgColor={hero.bgColor}
      borderColor={hero.borderColor}
      lastUpdatedAt={lastUpdatedAt}
      icon={(
        <svg class={`w-10 h-10 ${hero.color}`} fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d={hero.path} />
        </svg>
      )}
    >
      <h2 class={`text-lg sm:text-2xl font-semibold ${hero.color}`} data-testid="nodes-hero-title">{hero.title}</h2>
      <p class="text-gray-700 dark:text-gray-300 mt-1" data-testid="nodes-hero-subtitle">{subtitle}</p>
    </StatusHeroCard>
  )
}

/**
 * NodesPage - the Nodes dashboard at /nodes: status header, Capacity, Health
 * Checks, Nodes and Bursting Workloads panels. The snapshot and the
 * workloads ranking are polled independently on the dashboards' interval.
 * A 403 shows the access-restricted message; other errors show the error,
 * and keep the last data, with the error above it, when a poll fails after
 * a successful load. A failed ranking shows as failed in its own panel.
 */
export function NodesPage() {
  usePageMeta('Nodes', 'Cluster nodes dashboard')
  const [data, setData] = useState(null)
  const [workloads, setWorkloads] = useState(null)
  const [error, setError] = useState(null)
  const [forbidden, setForbidden] = useState(false)
  const [lastUpdatedAt, setLastUpdatedAt] = useState(null)

  // The snapshot is applied as soon as it arrives; the workloads ranking
  // is fetched and settles on its own, so it never holds up the nodes.
  const fetchNodes = useCallback(async () => {
    try {
      const resp = await fetchWithMock({ endpoint: '/api/v1/nodes', mockPath: '../mock/nodes', mockExport: 'mockNodes' })
      setData(resp)
      setForbidden(false)
      setError(null)
      setLastUpdatedAt(new Date())
    } catch (err) {
      if (err?.status === 403) {
        setForbidden(true)
        setData(null)
        setError(null)
      } else {
        setForbidden(false)
        setError(err?.message || String(err))
      }
    }
  }, [])

  const fetchWorkloads = useCallback(async () => {
    try {
      const resp = await fetchWithMock({ endpoint: '/api/v1/nodes/workloads', mockPath: '../mock/nodes', mockExport: 'mockNodesWorkloads' })
      setWorkloads({ data: resp, error: null })
    } catch (err) {
      setWorkloads({ data: null, error: err?.message || String(err) })
    }
  }, [])

  const fetchData = useCallback(() => {
    fetchNodes()
    fetchWorkloads()
  }, [fetchNodes, fetchWorkloads])

  useEffect(() => {
    fetchData()
    const interval = setInterval(fetchData, POLL_INTERVAL_MS)
    return () => clearInterval(interval)
  }, [fetchData])

  useRegisterPageShortcuts({ onRefresh: fetchData })

  let content
  if (forbidden) {
    content = <AccessDenied />
  } else if (error && !data) {
    content = (
      <div data-testid="error-message" class="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-md p-4">
        <p class="text-sm text-red-800 dark:text-red-200">Failed to load nodes: {error}</p>
      </div>
    )
  } else if (!data) {
    content = (
      <div data-testid="loading-message" class="bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-md p-4">
        <p class="text-sm text-blue-800 dark:text-blue-200">Loading nodes...</p>
      </div>
    )
  } else {
    const shortName = shortNamer(commonNodeSuffix((data.nodes || []).map(n => n.name)))
    content = (
      <>
        {/* A failed refresh keeps the last data and says so above it. */}
        {error && (
          <div data-testid="refresh-error" class="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-md p-4">
            <p class="text-sm text-red-800 dark:text-red-200">Failed to refresh nodes: {error}</p>
          </div>
        )}
        <NodesHero data={data} lastUpdatedAt={lastUpdatedAt} />
        <CapacityPanel data={data} />
        <FindingsList data={data} shortName={shortName} />
        <NodesList data={data} shortName={shortName} />
        {workloads && <BurstingWorkloadsPanel data={workloads.data} error={workloads.error} />}
      </>
    )
  }

  return (
    <main data-testid="nodes-dashboard-view" class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8 flex-grow w-full">
      <div class="space-y-6">
        {content}
      </div>
    </main>
  )
}
