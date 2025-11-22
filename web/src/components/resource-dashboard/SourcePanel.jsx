// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useState, useMemo, useEffect } from 'preact/hooks'
import { useSignal } from '@preact/signals'
import { useLocation } from 'preact-iso'
import { fetchWithMock } from '../../utils/fetch'
import { getControllerName } from '../../utils/constants'
import { formatTimestamp } from '../../utils/time'
import { TabButton, YamlBlock, getEventBadgeClass } from './PanelComponents'

/**
 * SourcePanel - Displays source information for a Flux resource
 * Handles its own data fetching and state management
 */
export function SourcePanel({ sourceRef, namespace }) {
  const location = useLocation()

  // State
  const [sourceData, setSourceData] = useState(null)
  const [sourceEventsData, setSourceEventsData] = useState([])
  const [sourceEventsLoading, setSourceEventsLoading] = useState(false)
  const [sourceEventsLoaded, setSourceEventsLoaded] = useState(false)
  const [loading, setLoading] = useState(true)

  // Tab state
  const [sourceTab, setSourceTab] = useState('overview')

  // Collapsible state
  const isExpanded = useSignal(true)

  // Fetch source data when component mounts
  useEffect(() => {
    const fetchSourceData = async () => {
      setLoading(true)

      const sourceParams = new URLSearchParams({
        kind: sourceRef.kind,
        name: sourceRef.name,
        namespace: sourceRef.namespace || namespace
      })

      try {
        const sourceResp = await fetchWithMock({
          endpoint: `/api/v1/resource?${sourceParams.toString()}`,
          mockPath: '../mock/resource',
          mockExport: 'getMockResource'
        })
        setSourceData(sourceResp)
      } catch (err) {
        console.error('Failed to fetch source data:', err)
      } finally {
        setLoading(false)
      }
    }

    fetchSourceData()
  }, [sourceRef, namespace])

  // Fetch source events on demand when Events tab is clicked
  useEffect(() => {
    if (sourceTab === 'events' && !sourceEventsLoaded && !sourceEventsLoading) {
      const fetchSourceEvents = async () => {
        setSourceEventsLoading(true)
        const params = new URLSearchParams({
          kind: sourceRef.kind,
          name: sourceRef.name,
          namespace: sourceRef.namespace || namespace
        })

        try {
          const eventsResp = await fetchWithMock({
            endpoint: `/api/v1/events?${params.toString()}`,
            mockPath: '../mock/events',
            mockExport: 'getMockEvents'
          })
          setSourceEventsData(eventsResp.events || [])
          setSourceEventsLoaded(true)
        } catch (err) {
          console.error('Failed to fetch source events:', err)
        } finally {
          setSourceEventsLoading(false)
        }
      }

      fetchSourceEvents()
    }
  }, [sourceTab, sourceEventsLoaded, sourceEventsLoading, sourceRef, namespace])

  // Memoized YAML data
  const sourceSpecYaml = useMemo(() => {
    if (!sourceData) return null
    return {
      apiVersion: sourceData.apiVersion,
      kind: sourceData.kind,
      metadata: sourceData.metadata,
      spec: sourceData.spec
    }
  }, [sourceData])

  const sourceStatusYaml = useMemo(() => {
    if (!sourceData?.status) return null
    return {
      apiVersion: sourceData.apiVersion,
      kind: sourceData.kind,
      metadata: { name: sourceData.metadata.name, namespace: sourceData.metadata.namespace },
      status: sourceData.status
    }
  }, [sourceData])

  return (
    <div class="card p-0" data-testid="source-view">
      <button
        onClick={() => isExpanded.value = !isExpanded.value}
        class="w-full px-6 py-4 text-left hover:bg-gray-50 dark:hover:bg-gray-700/30 transition-colors"
      >
        <div class="flex items-center justify-between">
          <div>
            <h3 class="text-lg font-semibold text-gray-900 dark:text-white">Source</h3>
          </div>
          <svg
            class={`w-5 h-5 text-gray-400 dark:text-gray-500 transition-transform ${isExpanded.value ? 'rotate-180' : ''}`}
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7"/>
          </svg>
        </div>
      </button>
      {isExpanded.value && (
        <div class="px-6 py-4">
          {loading ? (
            <div class="flex items-center justify-center p-8">
              <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-flux-blue"></div>
              <span class="ml-3 text-gray-600 dark:text-gray-400">Loading source...</span>
            </div>
          ) : (
            <>
              {/* Tab Navigation */}
              <div class="border-b border-gray-200 dark:border-gray-700 mb-4">
                <nav class="flex space-x-4">
                  <TabButton active={sourceTab === 'overview'} onClick={() => setSourceTab('overview')}>
                    Overview
                  </TabButton>
                  <TabButton active={sourceTab === 'events'} onClick={() => setSourceTab('events')}>
                    Events
                  </TabButton>
                  {sourceData && (
                    <>
                      <TabButton active={sourceTab === 'spec'} onClick={() => setSourceTab('spec')}>
                        Specification
                      </TabButton>
                      <TabButton active={sourceTab === 'status'} onClick={() => setSourceTab('status')}>
                        Status
                      </TabButton>
                    </>
                  )}
                </nav>
              </div>

              {/* Tab Content */}
              {sourceTab === 'overview' && (
                <div class="space-y-4">
                  {/* Status Badge */}
                  {sourceRef.status && (
                    <div class="flex items-baseline space-x-2">
                      <dt class="text-sm text-gray-500 dark:text-gray-400">Status:</dt>
                      <dd>
                        <span class={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                          sourceRef.status === 'Ready' ? 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400' :
                            sourceRef.status === 'Failed' ? 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400' :
                              sourceRef.status === 'Progressing' ? 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400' :
                                sourceRef.status === 'Suspended' ? 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-400' :
                                  'bg-gray-100 text-gray-800 dark:bg-gray-900/30 dark:text-gray-400'
                        }`}>
                          {sourceRef.status}
                        </span>
                      </dd>
                    </div>
                  )}

                  {/* Managed by */}
                  <div class="flex items-baseline space-x-2">
                    <dt class="text-sm text-gray-500 dark:text-gray-400">Managed by:</dt>
                    <dd class="text-sm text-gray-900 dark:text-white">{getControllerName(sourceRef.kind)}</dd>
                  </div>

                  {/* ID */}
                  <div class="flex items-baseline space-x-2">
                    <dt class="text-sm text-gray-500 dark:text-gray-400">ID:</dt>
                    <dd class="text-sm text-gray-900 dark:text-white">
                      <button
                        onClick={() => location.route(`/resource/${encodeURIComponent(sourceRef.kind)}/${encodeURIComponent(sourceRef.namespace || namespace)}/${encodeURIComponent(sourceRef.name)}`)}
                        class="text-gray-900 dark:text-gray-100 hover:text-flux-blue dark:hover:text-blue-400 inline-flex items-center gap-1"
                      >
                        {sourceRef.kind}/{sourceRef.namespace || namespace}/{sourceRef.name}
                        <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
                        </svg>
                      </button>
                    </dd>
                  </div>

                  {/* URL */}
                  {sourceRef.url && (
                    <div class="flex items-baseline space-x-2">
                      <dt class="text-sm text-gray-500 dark:text-gray-400">URL:</dt>
                      <dd class="text-sm text-gray-900 dark:text-white break-all">{sourceRef.url}</dd>
                    </div>
                  )}

                  {/* Origin URL */}
                  {sourceRef.originURL && (
                    <div class="flex items-baseline space-x-2">
                      <dt class="text-sm text-gray-500 dark:text-gray-400">Origin URL:</dt>
                      <dd class="text-sm text-gray-900 dark:text-white break-all">{sourceRef.originURL}</dd>
                    </div>
                  )}

                  {/* Origin Revision */}
                  {sourceRef.originRevision && (
                    <div class="flex items-baseline space-x-2">
                      <dt class="text-sm text-gray-500 dark:text-gray-400">Origin Revision:</dt>
                      <dd class="text-sm text-gray-900 dark:text-white break-all">{sourceRef.originRevision}</dd>
                    </div>
                  )}

                  {/* Fetch every */}
                  {sourceData?.spec?.interval && (
                    <div class="flex items-baseline space-x-2">
                      <dt class="text-sm text-gray-500 dark:text-gray-400">Fetch every:</dt>
                      <dd class="text-sm text-gray-900 dark:text-white">{sourceData.spec.interval}</dd>
                    </div>
                  )}

                  {/* Fetched at */}
                  {sourceData?.status?.conditions?.[0]?.lastTransitionTime && (
                    <div class="flex items-baseline space-x-2">
                      <dt class="text-sm text-gray-500 dark:text-gray-400">Fetched at:</dt>
                      <dd class="text-sm text-gray-900 dark:text-white">{new Date(sourceData.status.conditions[0].lastTransitionTime).toLocaleString().replace(',', '')}</dd>
                    </div>
                  )}

                  {/* Fetch result */}
                  {sourceRef.message && (
                    <div class="flex items-baseline space-x-2">
                      <dt class="text-sm text-gray-500 dark:text-gray-400">Fetch result:</dt>
                      <dd class="text-sm text-gray-700 dark:text-gray-300">
                        <pre class="whitespace-pre-wrap break-words font-sans">{sourceRef.message}</pre>
                      </dd>
                    </div>
                  )}
                </div>
              )}

              {/* Events Tab */}
              {sourceTab === 'events' && (
                <div>
                  {sourceEventsLoading ? (
                    <div class="flex items-center justify-center p-8">
                      <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-flux-blue"></div>
                      <span class="ml-3 text-gray-600 dark:text-gray-400">Loading events...</span>
                    </div>
                  ) : sourceEventsData.length === 0 ? (
                    <p class="text-sm text-gray-500 dark:text-gray-400">No events found</p>
                  ) : (
                    <div class="space-y-4">
                      {sourceEventsData.map((event, idx) => {
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

              {sourceTab === 'spec' && sourceData && <YamlBlock data={sourceSpecYaml} />}
              {sourceTab === 'status' && sourceData && <YamlBlock data={sourceStatusYaml} />}
            </>
          )}
        </div>
      )}
    </div>
  )
}
