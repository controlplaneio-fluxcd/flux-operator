// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useMemo, useEffect, useRef, useState } from 'preact/hooks'
import { fetchWithMock } from '../../../utils/fetch'
import { formatTimestamp } from '../../../utils/time'
import { getControllerName, getKindAlias } from '../../../utils/constants'
import { DashboardPanel, TabButton } from '../common/panel'
import { getStatusBadgeClass, getEventBadgeClass } from '../../../utils/status'
import { FluxOperatorIcon } from '../../layout/Icons'
import { useHashTab } from '../../../utils/hash'

// Valid tabs for the WorkloadReconcilerPanel
const RECONCILER_TABS = ['overview', 'source', 'events']

/**
 * WorkloadReconcilerPanel - Displays reconciler info for a workload's parent Flux resource.
 * Owns tab state, events lazy-loading, and auto-refresh on workloadData changes.
 */
export function WorkloadReconcilerPanel({ reconciler, workloadData }) {
  // Tab state synced with URL hash
  const [reconcilerTab, setReconcilerTab] = useHashTab('reconciler', 'overview', RECONCILER_TABS, 'reconciler-panel')

  // Events data state (lazy-loaded)
  const [eventsData, setEventsData] = useState([])
  const [eventsLoading, setEventsLoading] = useState(false)
  const [eventsLoaded, setEventsLoaded] = useState(false)

  // Track initial mount to avoid refetching on first render
  const isInitialMount = useRef(true)

  // Derived data
  const reconcilerRef = reconciler?.status?.reconcilerRef
  const sourceRef = reconciler?.status?.sourceRef
  const reconcilerStatus = reconcilerRef?.status || 'Unknown'
  const reconcilerMessage = reconcilerRef?.message || reconciler?.status?.conditions?.[0]?.message || ''
  const reconcilerLastReconciled = reconcilerRef?.lastReconciled || reconciler?.status?.conditions?.[0]?.lastTransitionTime

  // Compute reconcile interval from reconciler
  const reconcileInterval = useMemo(() => {
    if (!reconciler) return null
    if (reconciler.spec?.interval) return reconciler.spec.interval
    const annotation = reconciler.metadata?.annotations?.['fluxcd.controlplane.io/reconcileEvery']
    if (annotation) return annotation
    const k = reconciler.kind
    if (k === 'FluxInstance' || k === 'ResourceSet') return '60m'
    if (k === 'ResourceSetInputProvider') return '10m'
    return null
  }, [reconciler])

  // Compute reconcile timeout from reconciler
  const reconcileTimeout = useMemo(() => {
    if (!reconciler) return null
    const spec = reconciler.spec || {}
    const annotations = reconciler.metadata?.annotations || {}
    if (spec.timeout) return spec.timeout
    if (reconciler.apiVersion?.startsWith('source.toolkit.fluxcd.io')) return '1m'
    if (reconciler.kind === 'Kustomization') return spec.interval || null
    if (reconciler.kind === 'HelmRelease') return '5m'
    if (reconciler.kind === 'FluxInstance' || reconciler.kind === 'ResourceSet' || reconciler.kind === 'ResourceSetInputProvider') {
      return annotations['fluxcd.controlplane.io/reconcileTimeout'] || '5m'
    }
    return null
  }, [reconciler])

  // Fetch events on demand when Events tab is clicked
  useEffect(() => {
    if (reconcilerTab === 'events' && !eventsLoaded && !eventsLoading && reconciler) {
      const fetchEvents = async () => {
        setEventsLoading(true)
        const params = new URLSearchParams({
          kind: reconciler.kind,
          name: reconciler.metadata.name,
          namespace: reconciler.metadata.namespace
        })

        try {
          const eventsResp = await fetchWithMock({
            endpoint: `/api/v1/events?${params.toString()}`,
            mockPath: '../mock/events',
            mockExport: 'getMockEvents'
          })
          setEventsData(eventsResp?.events || [])
          setEventsLoaded(true)
        } catch (err) {
          console.error('Failed to fetch reconciler events:', err)
        } finally {
          setEventsLoading(false)
        }
      }

      fetchEvents()
    }
  }, [reconcilerTab, eventsLoaded, eventsLoading, reconciler])

  // Refetch events when workloadData changes (auto-refresh)
  useEffect(() => {
    if (isInitialMount.current) {
      if (workloadData) isInitialMount.current = false
      return
    }

    if (reconcilerTab === 'events' && eventsLoaded && !eventsLoading && reconciler) {
      const refetchEvents = async () => {
        const params = new URLSearchParams({
          kind: reconciler.kind,
          name: reconciler.metadata.name,
          namespace: reconciler.metadata.namespace
        })

        try {
          const eventsResp = await fetchWithMock({
            endpoint: `/api/v1/events?${params.toString()}`,
            mockPath: '../mock/events',
            mockExport: 'getMockEvents'
          })
          setEventsData(eventsResp?.events || [])
        } catch (err) {
          console.error('Failed to refetch reconciler events:', err)
        }
      }

      refetchEvents()
    }
  }, [workloadData])

  return (
    <DashboardPanel title="Reconciler" id="reconciler-panel">
      {/* Tab Navigation */}
      <div class="border-b border-gray-200 dark:border-gray-700 mb-4">
        <nav class="flex space-x-4">
          <TabButton active={reconcilerTab === 'overview'} onClick={() => setReconcilerTab('overview')}>
            <span class="sm:hidden">Info</span>
            <span class="hidden sm:inline">Overview</span>
          </TabButton>
          {sourceRef && (
            <TabButton active={reconcilerTab === 'source'} onClick={() => setReconcilerTab('source')}>
              Source
            </TabButton>
          )}
          <TabButton active={reconcilerTab === 'events'} onClick={() => setReconcilerTab('events')}>
            Events
          </TabButton>
        </nav>
      </div>

      {/* Reconciler Overview Tab */}
      {reconcilerTab === 'overview' && (
        <div class="space-y-4">
          <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
            {/* Left column: Status and metadata */}
            <div class="space-y-4">
              {/* Name */}
              <div class="text-sm">
                <span class="text-gray-500 dark:text-gray-400">Name</span>
                <a
                  href={`/resource/${encodeURIComponent(reconciler.kind)}/${encodeURIComponent(reconciler.metadata.namespace)}/${encodeURIComponent(reconciler.metadata.name)}`}
                  class="ml-1 text-flux-blue dark:text-blue-400 hover:underline"
                  data-testid="reconciler-link"
                >
                  <span class="hidden md:inline break-all">{reconciler.kind}/{reconciler.metadata.namespace}/{reconciler.metadata.name}</span>
                  <span class="md:hidden break-all">{getKindAlias(reconciler.kind)}/{reconciler.metadata.name}</span>
                </a>
              </div>

              {/* Status Badge */}
              <div class="text-sm">
                <span class="text-gray-500 dark:text-gray-400">Status</span>
                <span class={`ml-1 inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${getStatusBadgeClass(reconcilerStatus)}`}>
                  {reconcilerStatus}
                </span>
              </div>

              {/* Reconciled by */}
              <div class="text-sm">
                <span class="text-gray-500 dark:text-gray-400">Reconciled by</span>
                <span class="ml-1 text-gray-900 dark:text-white">{getControllerName(reconciler.kind)}</span>
              </div>

              {/* Reconcile every */}
              {reconcileInterval && (
                <div class="text-sm">
                  <span class="text-gray-500 dark:text-gray-400">Reconcile every</span>
                  <span class="ml-1 text-gray-900 dark:text-white">
                    {reconcileInterval}
                  </span>
                  {reconcileTimeout && (
                    <span class="ml-1 text-gray-900 dark:text-white">
                      (timeout {reconcileTimeout})
                    </span>
                  )}
                </div>
              )}
            </div>

            {/* Right column: Suspended by and Last action message */}
            {(reconcilerMessage || (reconcilerStatus === 'Suspended' && reconciler.metadata?.annotations?.['fluxcd.controlplane.io/suspendedBy'])) && (
              <div class="space-y-2 border-gray-200 dark:border-gray-700 border-t pt-4 md:border-t-0 md:border-l md:pt-0 md:pl-6">
                {reconcilerStatus === 'Suspended' && reconciler.metadata?.annotations?.['fluxcd.controlplane.io/suspendedBy'] && (
                  <div class="text-sm text-gray-500 dark:text-gray-400">
                    Suspended by <span class="text-gray-900 dark:text-white">{reconciler.metadata.annotations['fluxcd.controlplane.io/suspendedBy']}</span>
                  </div>
                )}
                {reconcilerMessage && (
                  <>
                    <div class="text-sm text-gray-500 dark:text-gray-400">
                      Last action <span class="text-gray-900 dark:text-white">{reconcilerLastReconciled ? new Date(reconcilerLastReconciled).toLocaleString().replace(',', '') : '-'}</span>
                    </div>
                    <div class="text-sm text-gray-700 dark:text-gray-300">
                      <pre class="whitespace-pre-wrap break-all font-sans">{reconcilerMessage}</pre>
                    </div>
                  </>
                )}
              </div>
            )}
          </div>
        </div>
      )}

      {/* Reconciler Source Tab */}
      {reconcilerTab === 'source' && sourceRef && (
        <div class="space-y-4">
          {/* Name */}
          <div class="text-sm">
            <span class="text-gray-500 dark:text-gray-400">Name</span>
            <a
              href={`/resource/${encodeURIComponent(sourceRef.kind)}/${encodeURIComponent(sourceRef.namespace || reconciler.metadata.namespace)}/${encodeURIComponent(sourceRef.name)}`}
              class="ml-1 text-flux-blue dark:text-blue-400 hover:underline"
            >
              <span class="hidden md:inline break-all">{sourceRef.kind}/{sourceRef.namespace || reconciler.metadata.namespace}/{sourceRef.name}</span>
              <span class="md:hidden break-all">{getKindAlias(sourceRef.kind)}/{sourceRef.name}</span>
            </a>
          </div>

          {/* Status Badge */}
          {sourceRef.status && (
            <div class="text-sm">
              <span class="text-gray-500 dark:text-gray-400">Status</span>
              <span class={`ml-1 inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${getStatusBadgeClass(sourceRef.status)}`}>
                {sourceRef.status}
              </span>
            </div>
          )}

          {/* Reconciled by */}
          <div class="text-sm">
            <span class="text-gray-500 dark:text-gray-400">Reconciled by</span>
            <span class="ml-1 text-gray-900 dark:text-white">{getControllerName(sourceRef.kind)}</span>
          </div>

          {/* URL */}
          {sourceRef.url && (
            <div class="text-sm">
              <span class="text-gray-500 dark:text-gray-400">URL</span>
              <span class="ml-1 text-gray-900 dark:text-white break-all">{sourceRef.url}</span>
            </div>
          )}

          {/* Origin URL */}
          {sourceRef.originURL && (
            <div class="text-sm">
              <span class="text-gray-500 dark:text-gray-400">Origin URL</span>
              <span class="ml-1 text-gray-900 dark:text-white break-all">{sourceRef.originURL}</span>
            </div>
          )}

          {/* Origin Revision */}
          {sourceRef.originRevision && (
            <div class="text-sm">
              <span class="text-gray-500 dark:text-gray-400">Origin Revision</span>
              <span class="ml-1 text-gray-900 dark:text-white break-all">{sourceRef.originRevision}</span>
            </div>
          )}

          {/* Fetch result */}
          {sourceRef.message && (
            <div class="text-sm">
              <span class="text-gray-500 dark:text-gray-400">Fetch result</span>
              <span class="ml-1 text-gray-900 dark:text-white break-all">{sourceRef.message}</span>
            </div>
          )}
        </div>
      )}

      {/* Reconciler Events Tab */}
      {reconcilerTab === 'events' && (
        <div>
          {eventsLoading ? (
            <div class="flex items-center justify-center p-8">
              <FluxOperatorIcon className="animate-spin h-8 w-8 text-flux-blue" />
              <span class="ml-3 text-gray-600 dark:text-gray-400">Loading events...</span>
            </div>
          ) : eventsData.length === 0 ? (
            <p class="text-sm text-gray-500 dark:text-gray-400">No events found</p>
          ) : (
            <div class="space-y-4">
              {eventsData.map((event, idx) => {
                const displayStatus = event.type === 'Normal' ? 'Info' : 'Warning'
                return (
                  <div key={idx} class="card p-4 hover:shadow-md transition-shadow">
                    <div class="flex items-center justify-between mb-3">
                      <span class={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${getEventBadgeClass(event.type)}`}>
                        {displayStatus}
                      </span>
                      <span class="text-xs text-gray-500 dark:text-gray-400">{formatTimestamp(event.lastTimestamp)}</span>
                    </div>
                    <div class="text-sm text-gray-700 dark:text-gray-300">
                      <pre class="whitespace-pre-wrap break-all font-sans">{event.message}</pre>
                    </div>
                  </div>
                )
              })}
            </div>
          )}
        </div>
      )}
    </DashboardPanel>
  )
}
