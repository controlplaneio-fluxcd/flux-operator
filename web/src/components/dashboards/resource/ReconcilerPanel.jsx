// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useState, useMemo, useEffect, useRef } from 'preact/hooks'
import { useLocation } from 'preact-iso'
import { fetchWithMock } from '../../../utils/fetch'
import { formatTimestamp } from '../../../utils/time'
import { getControllerName, getKindAlias } from '../../../utils/constants'
import { DashboardPanel, TabButton } from '../common/panel'
import { YamlBlock } from '../common/yaml'
import { getStatusBadgeClass, getEventBadgeClass } from '../../../utils/status'
import { HistoryTimeline } from './HistoryTimeline'

export function ReconcilerPanel({ kind, name, namespace, resourceData }) {
  const location = useLocation()

  // State
  const [reconcilerTab, setReconcilerTab] = useState('overview')
  const [eventsData, setEventsData] = useState([])
  const [eventsLoading, setEventsLoading] = useState(false)
  const [eventsLoaded, setEventsLoaded] = useState(false)

  // Track initial mount to avoid refetching on first render
  const isInitialMount = useRef(true)

  // Derived data
  const reconcilerRef = resourceData?.status?.reconcilerRef
  const status = reconcilerRef?.status || 'Unknown'
  const message = reconcilerRef?.message || resourceData?.status?.conditions?.[0]?.message || ''
  const lastReconciled = reconcilerRef?.lastReconciled || resourceData?.status?.conditions?.[0]?.lastTransitionTime

  const reconcileInterval = useMemo(() => {
    if (!resourceData) return null

    // Check spec.interval first
    if (resourceData.spec?.interval) {
      return resourceData.spec.interval
    }

    // Check annotation
    const annotation = resourceData.metadata?.annotations?.['fluxcd.controlplane.io/reconcileEvery']
    if (annotation) {
      return annotation
    }

    // Apply defaults based on kind
    const k = resourceData.kind
    if (k === 'FluxInstance' || k === 'ResourceSet') {
      return '60m'
    }
    if (k === 'ResourceSetInputProvider') {
      return '10m'
    }

    return null
  }, [resourceData])

  const reconcileTimeout = useMemo(() => {
    if (!resourceData) return null

    const k = resourceData.kind
    const spec = resourceData.spec || {}
    const annotations = resourceData.metadata?.annotations || {}

    // Any resource with spec.timeout field, use that value if set
    if (spec.timeout) {
      return spec.timeout
    }

    // Source types
    if (resourceData.apiVersion && resourceData.apiVersion.startsWith('source.toolkit.fluxcd.io')) {
      return '1m'
    }

    // Kustomization
    if (k === 'Kustomization') {
      return spec.interval || null
    }

    // HelmRelease
    if (k === 'HelmRelease') {
      return '5m'
    }

    // FluxInstance, ResourceSet, ResourceSetInputProvider
    if (k === 'FluxInstance' || k === 'ResourceSet' || k === 'ResourceSetInputProvider') {
      return annotations['fluxcd.controlplane.io/reconcileTimeout'] || '5m'
    }

    return null
  }, [resourceData])

  // Memoized YAML data
  const specYaml = useMemo(() => {
    if (!resourceData) return null
    return {
      apiVersion: resourceData.apiVersion,
      kind: resourceData.kind,
      metadata: resourceData.metadata,
      spec: resourceData.spec
    }
  }, [resourceData])

  const statusYaml = useMemo(() => {
    if (!resourceData?.status) return null
    // eslint-disable-next-line no-unused-vars
    const { inventory, sourceRef, reconcilerRef, exportedInputs, ...rest } = resourceData.status
    return {
      apiVersion: resourceData.apiVersion,
      kind: resourceData.kind,
      metadata: { name: resourceData.metadata.name, namespace: resourceData.metadata.namespace },
      status: rest
    }
  }, [resourceData])

  // Fetch events on demand when Events tab is clicked
  useEffect(() => {
    if (reconcilerTab === 'events' && !eventsLoaded && !eventsLoading) {
      const fetchEvents = async () => {
        setEventsLoading(true)
        const params = new URLSearchParams({ kind, name, namespace })

        try {
          const eventsResp = await fetchWithMock({
            endpoint: `/api/v1/events?${params.toString()}`,
            mockPath: '../mock/events',
            mockExport: 'getMockEvents'
          })
          setEventsData(eventsResp?.events || [])
          setEventsLoaded(true)
        } catch (err) {
          console.error('Failed to fetch events:', err)
        } finally {
          setEventsLoading(false)
        }
      }

      fetchEvents()
    }
  }, [reconcilerTab, eventsLoaded, eventsLoading, kind, namespace, name])

  // Refetch events when resource data changes (auto-refresh) if Events tab is open
  useEffect(() => {
    // Skip if resource data hasn't been loaded yet or on initial mount
    if (isInitialMount.current || !resourceData) {
      if (isInitialMount.current && resourceData) {
        isInitialMount.current = false
      }
      return
    }

    // Only refetch if Events tab is open and events were previously loaded
    if (reconcilerTab === 'events' && eventsLoaded && !eventsLoading) {
      const refetchEvents = async () => {
        // Don't set loading state during auto-refresh to avoid showing spinner
        const params = new URLSearchParams({ kind, name, namespace })

        try {
          const eventsResp = await fetchWithMock({
            endpoint: `/api/v1/events?${params.toString()}`,
            mockPath: '../mock/events',
            mockExport: 'getMockEvents'
          })
          setEventsData(eventsResp?.events || [])
        } catch (err) {
          console.error('Failed to refetch events:', err)
        }
      }

      refetchEvents()
    }
  }, [resourceData])

  return (
    <DashboardPanel title="Reconciler" id="reconciler-panel">
      {/* Tab Navigation */}
      <div class="border-b border-gray-200 dark:border-gray-700 mb-4">
        <nav class="flex space-x-4">
          <TabButton active={reconcilerTab === 'overview'} onClick={() => setReconcilerTab('overview')}>
            <span class="sm:hidden">Info</span>
            <span class="hidden sm:inline">Overview</span>
          </TabButton>
          {resourceData?.status?.history && resourceData.status.history.length > 0 && (
            <TabButton active={reconcilerTab === 'history'} onClick={() => setReconcilerTab('history')}>
              History
            </TabButton>
          )}
          <TabButton active={reconcilerTab === 'events'} onClick={() => setReconcilerTab('events')}>
            Events
          </TabButton>
          <TabButton active={reconcilerTab === 'spec'} onClick={() => setReconcilerTab('spec')}>
            <span class="sm:hidden">Spec</span>
            <span class="hidden sm:inline">Specification</span>
          </TabButton>
          <TabButton active={reconcilerTab === 'status'} onClick={() => setReconcilerTab('status')}>
            Status
          </TabButton>
        </nav>
      </div>

      {/* Tab Content */}
      {reconcilerTab === 'overview' && (
        <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
          {/* Left column: Status and metadata */}
          <div class="space-y-4">
            {/* Status Badge */}
            <div class="text-sm">
              <span class="text-gray-500 dark:text-gray-400">Status</span>
              <span class={`ml-1 inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${getStatusBadgeClass(status)}`}>
                {status}
              </span>
            </div>

            {/* Reconciled by */}
            <div class="text-sm">
              <span class="text-gray-500 dark:text-gray-400">Reconciled by</span>
              <span class="ml-1 text-gray-900 dark:text-white">{getControllerName(kind)}</span>
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

            {/* Managed by */}
            {reconcilerRef?.managedBy && (
              <div class="text-sm">
                <span class="text-gray-500 dark:text-gray-400">Managed by</span>
                {(() => {
                  const [refKind, refNamespace, refName] = reconcilerRef.managedBy.split('/')
                  return (
                    <button
                      onClick={() => location.route(`/resource/${encodeURIComponent(refKind)}/${encodeURIComponent(refNamespace)}/${encodeURIComponent(refName)}`)}
                      class="ml-1 text-flux-blue dark:text-blue-400 hover:underline"
                    >
                      <span class="hidden md:inline break-all">{reconcilerRef.managedBy}</span>
                      <span class="md:hidden break-all">{getKindAlias(refKind)}/{refName}</span>
                    </button>
                  )
                })()}
              </div>
            )}
          </div>

          {/* Right column: Last action message */}
          {message && (
            <div class="space-y-2 border-gray-200 dark:border-gray-700 border-t pt-4 md:border-t-0 md:border-l md:pt-0 md:pl-6">
              <div class="text-sm text-gray-500 dark:text-gray-400">
                Last action <span class="text-gray-900 dark:text-white">{lastReconciled ? new Date(lastReconciled).toLocaleString().replace(',', '') : '-'}</span>
              </div>
              <div class="text-sm text-gray-700 dark:text-gray-300">
                <pre class="whitespace-pre-wrap break-all font-sans">{message}</pre>
              </div>
            </div>
          )}
        </div>
      )}

      {/* History Tab */}
      {reconcilerTab === 'history' && (
        <HistoryTimeline
          history={resourceData?.status?.history}
          kind={kind}
        />
      )}

      {reconcilerTab === 'spec' && <YamlBlock data={specYaml} />}
      {reconcilerTab === 'status' && <YamlBlock data={statusYaml} />}

      {/* Events Tab */}
      {reconcilerTab === 'events' && (
        <div>
          {eventsLoading ? (
            <div class="flex items-center justify-center p-8">
              <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-flux-blue"></div>
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
                    {/* Header row */}
                    <div class="flex items-center justify-between mb-3">
                      <span class={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${getEventBadgeClass(event.type)}`}>
                        {displayStatus}
                      </span>
                      <span class="text-xs text-gray-500 dark:text-gray-400">{formatTimestamp(event.lastTimestamp)}</span>
                    </div>

                    {/* Message */}
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
