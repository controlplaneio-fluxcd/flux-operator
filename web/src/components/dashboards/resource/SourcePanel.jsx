// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useState, useMemo, useEffect, useRef } from 'preact/hooks'
import { useLocation } from 'preact-iso'
import { fetchWithMock } from '../../../utils/fetch'
import { getControllerName, getKindAlias } from '../../../utils/constants'
import { formatTimestamp } from '../../../utils/time'
import { DashboardPanel, TabButton } from '../common/panel'
import { YamlBlock } from '../common/yaml'
import { getStatusBadgeClass, getEventBadgeClass } from '../../../utils/status'
import { FluxIcon } from '../../common/icons'

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

  // Track initial mount to avoid refetching on first render
  const isInitialMount = useRef(true)

  // Tab state
  const [sourceTab, setSourceTab] = useState('overview')

  // Fetch source data when component mounts
  useEffect(() => {
    const fetchSourceData = async () => {
      // Only show loading spinner on initial load (when no data exists)
      if (!sourceData) {
        setLoading(true)
      }

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
          setSourceEventsData(eventsResp?.events || [])
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

  // Refetch events when sourceRef changes (auto-refresh from parent) if Events tab is open
  useEffect(() => {
    // Skip on initial mount
    if (isInitialMount.current) {
      isInitialMount.current = false
      return
    }

    // Only refetch if Events tab is open and events were previously loaded
    if (sourceTab === 'events' && sourceEventsLoaded && !sourceEventsLoading) {
      const refetchSourceEvents = async () => {
        // Don't set loading state during auto-refresh to avoid showing spinner
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
          setSourceEventsData(eventsResp?.events || [])
        } catch (err) {
          console.error('Failed to refetch source events:', err)
        }
      }

      refetchSourceEvents()
    }
  }, [sourceRef, namespace])

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
    <DashboardPanel title="Source" id="source-panel">
      {loading ? (
        <div class="flex items-center justify-center p-8">
          <FluxIcon className="animate-spin h-8 w-8 text-flux-blue" />
          <span class="ml-3 text-gray-600 dark:text-gray-400">Loading source...</span>
        </div>
      ) : (
        <>
          {/* Tab Navigation */}
          <div class="border-b border-gray-200 dark:border-gray-700 mb-4">
            <nav class="flex space-x-4">
              <TabButton active={sourceTab === 'overview'} onClick={() => setSourceTab('overview')}>
                <span class="sm:hidden">Info</span>
                <span class="hidden sm:inline">Overview</span>
              </TabButton>
              <TabButton active={sourceTab === 'events'} onClick={() => setSourceTab('events')}>
                Events
              </TabButton>
              {sourceData && (
                <>
                  <TabButton active={sourceTab === 'spec'} onClick={() => setSourceTab('spec')}>
                    <span class="sm:hidden">Spec</span>
                    <span class="hidden sm:inline">Specification</span>
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
              {/* Resource Link */}
              <button
                onClick={() => location.route(`/resource/${encodeURIComponent(sourceRef.kind)}/${encodeURIComponent(sourceRef.namespace || namespace)}/${encodeURIComponent(sourceRef.name)}`)}
                class="flex items-center gap-2 text-sm text-flux-blue dark:text-blue-400 hover:underline"
              >
                <svg class="w-4 h-4 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
                </svg>
                <span class="hidden md:inline break-all">{sourceRef.kind}/{sourceRef.namespace || namespace}/{sourceRef.name}</span>
                <span class="md:hidden break-all">{getKindAlias(sourceRef.kind)}/{sourceRef.name}</span>
              </button>

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

              {/* Fetch every */}
              {sourceData?.spec?.interval && (
                <div class="text-sm">
                  <span class="text-gray-500 dark:text-gray-400">Fetch every</span>
                  <span class="ml-1 text-gray-900 dark:text-white">{sourceData.spec.interval}</span>
                </div>
              )}

              {/* Fetched at */}
              {sourceData?.status?.conditions?.[0]?.lastTransitionTime && (
                <div class="text-sm">
                  <span class="text-gray-500 dark:text-gray-400">Fetched at</span>
                  <span class="ml-1 text-gray-900 dark:text-white">{new Date(sourceData.status.conditions[0].lastTransitionTime).toLocaleString().replace(',', '')}</span>
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

          {/* Events Tab */}
          {sourceTab === 'events' && (
            <div>
              {sourceEventsLoading ? (
                <div class="flex items-center justify-center p-8">
                  <FluxIcon className="animate-spin h-8 w-8 text-flux-blue" />
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
                          <pre class="whitespace-pre-wrap break-all font-sans">{event.message}</pre>
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
    </DashboardPanel>
  )
}
