// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useState, useMemo, useEffect } from 'preact/hooks'
import { useSignal } from '@preact/signals'
import { fetchWithMock } from '../../utils/fetch'
import { formatTimestamp } from '../../utils/time'
import { getControllerName } from '../../utils/constants'
import { TabButton, YamlBlock, getEventBadgeClass } from './PanelComponents'

export function ReconcilerPanel({ kind, name, namespace, resourceData, overviewData }) {
  // State
  const [infoTab, setInfoTab] = useState('overview')
  const [eventsData, setEventsData] = useState([])
  const [eventsLoading, setEventsLoading] = useState(false)
  const [eventsLoaded, setEventsLoaded] = useState(false)

  // Collapsible state
  const isInfoExpanded = useSignal(true)

  // Derived data
  const status = overviewData?.status || 'Unknown'
  const message = overviewData?.message || resourceData?.status?.conditions?.[0]?.message || ''
  const lastReconciled = overviewData?.lastReconciled || resourceData?.status?.conditions?.[0]?.lastTransitionTime

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
    const { inventory, sourceRef, ...rest } = resourceData.status
    return {
      apiVersion: resourceData.apiVersion,
      kind: resourceData.kind,
      metadata: { name: resourceData.metadata.name, namespace: resourceData.metadata.namespace },
      status: rest
    }
  }, [resourceData])

  // Fetch events on demand when Events tab is clicked
  useEffect(() => {
    if (infoTab === 'events' && !eventsLoaded && !eventsLoading) {
      const fetchEvents = async () => {
        setEventsLoading(true)
        const params = new URLSearchParams({ kind, name, namespace })

        try {
          const eventsResp = await fetchWithMock({
            endpoint: `/api/v1/events?${params.toString()}`,
            mockPath: '../mock/events',
            mockExport: 'getMockEvents'
          })
          setEventsData(eventsResp.events || [])
          setEventsLoaded(true)
        } catch (err) {
          console.error('Failed to fetch events:', err)
        } finally {
          setEventsLoading(false)
        }
      }

      fetchEvents()
    }
  }, [infoTab, eventsLoaded, eventsLoading, kind, namespace, name])

  return (
    <div class="card p-0" data-testid="reconciler-panel">
      <button
        onClick={() => isInfoExpanded.value = !isInfoExpanded.value}
        class="w-full px-6 py-4 text-left hover:bg-gray-50 dark:hover:bg-gray-700/30 transition-colors"
        aria-expanded={isInfoExpanded.value}
      >
        <div class="flex items-center justify-between">
          <div>
            <h3 class="text-lg font-semibold text-gray-900 dark:text-white">Reconciler</h3>
          </div>
          <svg
            class={`w-5 h-5 text-gray-400 dark:text-gray-500 transition-transform ${isInfoExpanded.value ? 'rotate-180' : ''}`}
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7"/>
          </svg>
        </div>
      </button>
      {isInfoExpanded.value && (
        <div class="px-6 py-4">
          {/* Tab Navigation */}
          <div class="border-b border-gray-200 dark:border-gray-700 mb-4">
            <nav class="flex space-x-4">
              <TabButton active={infoTab === 'overview'} onClick={() => setInfoTab('overview')}>
                    Overview
              </TabButton>
              <TabButton active={infoTab === 'events'} onClick={() => setInfoTab('events')}>
                    Events
              </TabButton>
              <TabButton active={infoTab === 'spec'} onClick={() => setInfoTab('spec')}>
                    Specification
              </TabButton>
              <TabButton active={infoTab === 'status'} onClick={() => setInfoTab('status')}>
                    Status
              </TabButton>
            </nav>
          </div>

          {/* Tab Content */}
          {infoTab === 'overview' && (
            <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
              {/* Left column: Status and metadata */}
              <div class="space-y-4">
                {/* Status Badge */}
                <div class="flex items-baseline space-x-2">
                  <dt class="text-sm text-gray-500 dark:text-gray-400">Status:</dt>
                  <dd>
                    <span class={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                      status === 'Ready' ? 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400' :
                        status === 'Failed' ? 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400' :
                          status === 'Progressing' ? 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400' :
                            status === 'Suspended' ? 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-400' :
                              'bg-gray-100 text-gray-800 dark:bg-gray-900/30 dark:text-gray-400'
                    }`}>
                      {status}
                    </span>
                  </dd>
                </div>

                {/* Managed by */}
                <div class="flex items-baseline space-x-2">
                  <dt class="text-sm text-gray-500 dark:text-gray-400">Managed by:</dt>
                  <dd class="text-sm text-gray-900 dark:text-white">{getControllerName(kind)}</dd>
                </div>

                {/* Reconcile every */}
                {reconcileInterval && (
                  <div class="flex items-baseline space-x-2">
                    <dt class="text-sm text-gray-500 dark:text-gray-400">Reconcile every:</dt>
                    <dd class="text-sm text-gray-900 dark:text-white">{reconcileInterval}</dd>
                  </div>
                )}

                {/* ID */}
                <div class="flex items-baseline space-x-2">
                  <dt class="text-sm text-gray-500 dark:text-gray-400">ID:</dt>
                  <dd class="text-sm text-gray-900 dark:text-white">{kind}/{namespace}/{name}</dd>
                </div>
              </div>

              {/* Right column: Last action message */}
              {message && (
                <div class="space-y-2 border-gray-200 dark:border-gray-700 md:border-l md:pl-6">
                  <div class="text-sm text-gray-500 dark:text-gray-400">
                        Last action: <span class="text-gray-900 dark:text-white">{lastReconciled ? new Date(lastReconciled).toLocaleString().replace(',', '') : '-'}</span>
                  </div>
                  <div class="text-sm text-gray-700 dark:text-gray-300">
                    <pre class="whitespace-pre-wrap break-words font-sans">{message}</pre>
                  </div>
                </div>
              )}
            </div>
          )}

          {infoTab === 'spec' && <YamlBlock data={specYaml} />}
          {infoTab === 'status' && <YamlBlock data={statusYaml} />}

          {/* Events Tab */}
          {infoTab === 'events' && (
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
                          <pre class="whitespace-pre-wrap break-words font-sans">{event.message}</pre>
                        </div>
                      </div>
                    )
                  })}
                </div>
              )}
            </div>
          )}
        </div>
      )}
    </div>
  )
}
