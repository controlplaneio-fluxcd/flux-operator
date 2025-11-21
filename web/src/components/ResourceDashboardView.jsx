// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useState, useMemo, useEffect } from 'preact/hooks'
import { useSignal } from '@preact/signals'
import { useLocation } from 'preact-iso'
import yaml from 'js-yaml'
import Prism from 'prismjs'
import 'prismjs/components/prism-yaml'
import { fetchWithMock } from '../utils/fetch'
import { appliedTheme } from '../utils/theme'
import { formatTimestamp } from '../utils/time'
import { fluxKinds, getControllerName, workloadKinds } from '../utils/constants'

// Import Prism themes as URLs for dynamic loading
import prismLight from 'prismjs/themes/prism.css?url'
import prismDark from 'prismjs/themes/prism-tomorrow.css?url'

/**
 * Get status styling info
 */
function getStatusInfo(status) {
  switch (status) {
  case 'Ready':
    return {
      color: 'text-success',
      bgColor: 'bg-green-50',
      borderColor: 'border-success',
      icon: (
        <svg class="w-10 h-10 text-success" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
        </svg>
      )
    }
  case 'Failed':
    return {
      color: 'text-danger',
      bgColor: 'bg-red-50',
      borderColor: 'border-danger',
      icon: (
        <svg class="w-10 h-10 text-danger" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
        </svg>
      )
    }
  case 'Progressing':
    return {
      color: 'text-blue-600 dark:text-blue-400',
      bgColor: 'bg-blue-50',
      borderColor: 'border-blue-500',
      icon: (
        <svg class="w-10 h-10 text-blue-600 dark:text-blue-400 animate-spin" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
        </svg>
      )
    }
  case 'Suspended':
    return {
      color: 'text-yellow-600 dark:text-yellow-400',
      bgColor: 'bg-yellow-50',
      borderColor: 'border-yellow-500',
      icon: (
        <svg class="w-10 h-10 text-yellow-600 dark:text-yellow-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 9v6m4-6v6m7-3a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
      )
    }
  default:
    return {
      color: 'text-gray-600 dark:text-gray-400',
      bgColor: 'bg-gray-50',
      borderColor: 'border-gray-400',
      icon: (
        <svg class="w-10 h-10 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8.228 9c.549-1.165 2.03-2 3.772-2 2.21 0 4 1.343 4 3 0 1.4-1.278 2.575-3.006 2.907-.542.104-.994.54-.994 1.093m0 3h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
      )
    }
  }
}

/**
 * Get event status badge class
 */
function getEventBadgeClass(type) {
  return type === 'Normal'
    ? 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400'
    : 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400'
}

/**
 * YAML code block with syntax highlighting
 */
function YamlBlock({ data }) {
  const highlighted = useMemo(() => {
    if (!data) return ''
    const yamlStr = yaml.dump(data, { indent: 2, lineWidth: -1 })
    return Prism.highlight(yamlStr, Prism.languages.yaml, 'yaml')
  }, [data])

  return (
    <pre class="p-3 bg-gray-50 dark:bg-gray-900 rounded-md overflow-x-auto language-yaml" style="font-size: 12px; line-height: 1.5;">
      <code class="language-yaml" style="font-size: 12px;" dangerouslySetInnerHTML={{ __html: highlighted }} />
    </pre>
  )
}

/**
 * Tab button for section tabs
 */
function TabButton({ active, onClick, children }) {
  return (
    <button
      onClick={onClick}
      class={`py-2 px-1 text-sm font-medium border-b-2 transition-colors ${
        active
          ? 'border-flux-blue text-flux-blue dark:text-blue-400'
          : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300 dark:text-gray-400 dark:hover:text-gray-300'
      }`}
    >
      {children}
    </button>
  )
}

/**
 * ResourceDashboardView - Full page dashboard for a single Flux resource
 */
export function ResourceDashboardView({ kind, namespace, name }) {
  const location = useLocation()

  // State
  const [resourceData, setResourceData] = useState(null)
  const [overviewData, setOverviewData] = useState(null)
  const [eventsData, setEventsData] = useState([])
  const [eventsLoading, setEventsLoading] = useState(false)
  const [eventsLoaded, setEventsLoaded] = useState(false)
  const [sourceData, setSourceData] = useState(null)
  const [sourceEventsData, setSourceEventsData] = useState([])
  const [sourceEventsLoading, setSourceEventsLoading] = useState(false)
  const [sourceEventsLoaded, setSourceEventsLoaded] = useState(false)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)

  // Tab states
  const [infoTab, setInfoTab] = useState('overview')
  const [sourceTab, setSourceTab] = useState('overview')
  const [managedObjectsTab, setManagedObjectsTab] = useState('overview')

  // Collapsible state
  const isInfoExpanded = useSignal(true)
  const isSourceExpanded = useSignal(true)
  const isManagedObjectsExpanded = useSignal(true)

  // Dynamically load Prism theme
  useEffect(() => {
    const linkId = 'prism-theme-link'
    const existingLink = document.getElementById(linkId)
    if (existingLink) existingLink.remove()

    const link = document.createElement('link')
    link.id = linkId
    link.rel = 'stylesheet'
    link.href = appliedTheme.value === 'dark' ? prismDark : prismLight
    document.head.appendChild(link)

    return () => {
      const link = document.getElementById(linkId)
      if (link) link.remove()
    }
  }, [appliedTheme.value])

  // Fetch data
  useEffect(() => {
    const fetchData = async () => {
      setLoading(true)
      setError(null)

      const params = new URLSearchParams({ kind, name, namespace })

      try {
        const [resourceResp, overviewResp] = await Promise.all([
          fetchWithMock({
            endpoint: `/api/v1/resource?${params.toString()}`,
            mockPath: '../mock/resource',
            mockExport: 'getMockResource'
          }),
          fetchWithMock({
            endpoint: `/api/v1/resources?${params.toString()}`,
            mockPath: '../mock/resources',
            mockExport: 'getMockResources'
          }).then(data => data.resources?.[0] || null)
        ])

        setResourceData(resourceResp)
        setOverviewData(overviewResp)

        // Fetch source if available
        if (resourceResp?.status?.sourceRef) {
          const sourceRef = resourceResp.status.sourceRef
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
          } catch {
            // Source fetch is optional
          }
        }
      } catch (err) {
        console.error('Failed to fetch resource data:', err)
        setError(err.message)
      } finally {
        setLoading(false)
      }
    }

    fetchData()
  }, [kind, namespace, name])

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

  // Fetch source events on demand when Source Events tab is clicked
  useEffect(() => {
    if (sourceTab === 'events' && !sourceEventsLoaded && !sourceEventsLoading && resourceData?.status?.sourceRef) {
      const fetchSourceEvents = async () => {
        setSourceEventsLoading(true)
        const sourceRef = resourceData.status.sourceRef
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
  }, [sourceTab, sourceEventsLoaded, sourceEventsLoading, resourceData, namespace])

  // Derived data
  const status = overviewData?.status || 'Unknown'
  const statusInfo = getStatusInfo(status)
  const message = overviewData?.message || resourceData?.status?.conditions?.[0]?.message || ''
  const lastReconciled = overviewData?.lastReconciled || resourceData?.status?.conditions?.[0]?.lastTransitionTime
  const hasSource = resourceData?.status?.sourceRef
  const hasInventory = resourceData?.status?.inventory && resourceData.status.inventory.length > 0

  // Calculate managed objects statistics
  const totalResourcesCount = useMemo(() => {
    if (!hasInventory) return 0
    return resourceData.status.inventory.length
  }, [hasInventory, resourceData])

  const fluxResourcesCount = useMemo(() => {
    if (!hasInventory) return 0
    return resourceData.status.inventory.filter(item => fluxKinds.includes(item.kind)).length
  }, [hasInventory, resourceData])

  const workloadsCount = useMemo(() => {
    if (!hasInventory) return 0
    return resourceData.status.inventory.filter(item => workloadKinds.includes(item.kind)).length
  }, [hasInventory, resourceData])

  const secretsCount = useMemo(() => {
    if (!hasInventory) return 0
    return resourceData.status.inventory.filter(item => item.kind === 'Secret').length
  }, [hasInventory, resourceData])

  const pruningEnabled = useMemo(() => {
    if (!resourceData) return false
    const k = resourceData.kind
    if (k === 'Kustomization') {
      return resourceData.spec?.prune === true
    }
    if (k === 'HelmRelease' || k === 'FluxInstance' || k === 'ResourceSet' || k === 'ArtifactGenerator') {
      return true
    }
    return false
  }, [resourceData])

  const healthCheckEnabled = useMemo(() => {
    if (!resourceData) return false
    const k = resourceData.kind
    if (k === 'Kustomization' || k === 'FluxInstance' || k === 'ResourceSet') {
      return resourceData.spec?.wait === true
    }
    if (k === 'HelmRelease') {
      return !resourceData.spec?.upgrade?.disableWait
    }
    return false
  }, [resourceData])

  const secretDecryptionEnabled = useMemo(() => {
    if (!resourceData) return false
    if (resourceData.kind === 'Kustomization') {
      return !!resourceData.spec?.decryption
    }
    return false
  }, [resourceData])

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

  // Navigate to another resource
  const handleNavigate = (item) => {
    const ns = item.namespace || namespace
    location.route(`/resource/${encodeURIComponent(item.kind)}/${encodeURIComponent(ns)}/${encodeURIComponent(item.name)}`)
  }

  // Loading state
  if (loading) {
    return (
      <main data-testid="resource-dashboard-view" class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8 flex-grow w-full">
        <div class="flex items-center justify-center p-8">
          <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-flux-blue"></div>
          <span class="ml-3 text-gray-600 dark:text-gray-400">Loading resource...</span>
        </div>
      </main>
    )
  }

  // Error state
  if (error) {
    return (
      <main data-testid="resource-dashboard-view" class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8 flex-grow w-full">
        <div class="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-md p-4">
          <p class="text-sm text-red-800 dark:text-red-200">Failed to load resource: {error}</p>
        </div>
      </main>
    )
  }

  // Not found state
  if (!resourceData) {
    return (
      <main data-testid="resource-dashboard-view" class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8 flex-grow w-full">
        <div class="card text-center py-8">
          <p class="text-gray-600 dark:text-gray-400">Resource not found: {kind}/{namespace}/{name}</p>
        </div>
      </main>
    )
  }

  return (
    <main data-testid="resource-dashboard-view" class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8 flex-grow w-full">
      <div class="space-y-6">

        {/* Header */}
        <div class={`card ${statusInfo.bgColor} dark:bg-opacity-20 border-2 ${statusInfo.borderColor}`}>
          <div class="flex items-center space-x-4">
            <div class="flex-shrink-0">
              <div class={`w-16 h-16 rounded-full ${statusInfo.bgColor} dark:bg-opacity-30 flex items-center justify-center`}>
                {statusInfo.icon}
              </div>
            </div>
            <div class="flex-grow min-w-0">
              <span class="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase">{kind}</span>
              <h1 class="text-2xl font-bold text-gray-900 dark:text-white font-mono break-all">
                {name}
              </h1>
              <span class="text-sm text-gray-500 dark:text-gray-400">{namespace} namespace</span>
            </div>
            <div class="hidden md:block text-right flex-shrink-0">
              <div class="text-sm text-gray-600 dark:text-gray-400">Last Reconciled</div>
              <div class="text-lg font-semibold text-gray-900 dark:text-white">{lastReconciled ? formatTimestamp(lastReconciled) : '-'}</div>
            </div>
          </div>
        </div>

        {/* Info Section - Collapsible */}
        <div class="card p-0">
          <button
            onClick={() => isInfoExpanded.value = !isInfoExpanded.value}
            class="w-full px-6 py-4 text-left hover:bg-gray-50 dark:hover:bg-gray-700/30 transition-colors"
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

        {/* Managed objects Section - Collapsible */}
        {hasInventory && (
          <div class="card p-0">
            <button
              onClick={() => isManagedObjectsExpanded.value = !isManagedObjectsExpanded.value}
              class="w-full px-6 py-4 text-left hover:bg-gray-50 dark:hover:bg-gray-700/30 transition-colors"
            >
              <div class="flex items-center justify-between">
                <div>
                  <h3 class="text-lg font-semibold text-gray-900 dark:text-white">Managed Objects</h3>
                </div>
                <svg
                  class={`w-5 h-5 text-gray-400 dark:text-gray-500 transition-transform ${isManagedObjectsExpanded.value ? 'rotate-180' : ''}`}
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7"/>
                </svg>
              </div>
            </button>
            {isManagedObjectsExpanded.value && (
              <div class="px-6 py-4">
                {/* Tab Navigation */}
                <div class="border-b border-gray-200 dark:border-gray-700 mb-4">
                  <nav class="flex space-x-4">
                    <TabButton active={managedObjectsTab === 'overview'} onClick={() => setManagedObjectsTab('overview')}>
                      Overview
                    </TabButton>
                    <TabButton active={managedObjectsTab === 'inventory'} onClick={() => setManagedObjectsTab('inventory')}>
                      Inventory
                    </TabButton>
                  </nav>
                </div>

                {/* Tab Content */}
                {managedObjectsTab === 'overview' && (
                  <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
                    {/* Left column: Feature toggles */}
                    <div class="space-y-4">
                      {/* Garbage collection */}
                      <div class="flex items-baseline space-x-2">
                        <dt class="text-sm text-gray-500 dark:text-gray-400">Garbage collection:</dt>
                        <dd>
                          <span class={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                            pruningEnabled
                              ? 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400'
                              : 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400'
                          }`}>
                            {pruningEnabled ? 'Enabled' : 'Disabled'}
                          </span>
                        </dd>
                      </div>

                      {/* Health checking */}
                      <div class="flex items-baseline space-x-2">
                        <dt class="text-sm text-gray-500 dark:text-gray-400">Health checking:</dt>
                        <dd>
                          <span class={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                            healthCheckEnabled
                              ? 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400'
                              : 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400'
                          }`}>
                            {healthCheckEnabled ? 'Enabled' : 'Disabled'}
                          </span>
                        </dd>
                      </div>

                      {/* Secret Decryption */}
                      <div class="flex items-baseline space-x-2">
                        <dt class="text-sm text-gray-500 dark:text-gray-400">Secret Decryption:</dt>
                        <dd>
                          <span class={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                            secretDecryptionEnabled
                              ? 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400'
                              : 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400'
                          }`}>
                            {secretDecryptionEnabled ? 'Enabled' : 'Disabled'}
                          </span>
                        </dd>
                      </div>
                    </div>

                    {/* Right column: Resource counts */}
                    <div class="space-y-4 border-gray-200 dark:border-gray-700 md:border-l md:pl-6">
                      {/* Total resources */}
                      <div class="flex items-baseline space-x-2">
                        <dt class="text-sm text-gray-500 dark:text-gray-400">Total resources:</dt>
                        <dd class="text-sm text-gray-900 dark:text-white">{totalResourcesCount}</dd>
                      </div>

                      {/* Flux resources */}
                      <div class="flex items-baseline space-x-2">
                        <dt class="text-sm text-gray-500 dark:text-gray-400">Flux resources:</dt>
                        <dd class="text-sm text-gray-900 dark:text-white">{fluxResourcesCount}</dd>
                      </div>

                      {/* Kubernetes workloads */}
                      <div class="flex items-baseline space-x-2">
                        <dt class="text-sm text-gray-500 dark:text-gray-400">Kubernetes workloads:</dt>
                        <dd class="text-sm text-gray-900 dark:text-white">{workloadsCount}</dd>
                      </div>

                      {/* Kubernetes secrets */}
                      <div class="flex items-baseline space-x-2">
                        <dt class="text-sm text-gray-500 dark:text-gray-400">Kubernetes secrets:</dt>
                        <dd class="text-sm text-gray-900 dark:text-white">{secretsCount}</dd>
                      </div>
                    </div>
                  </div>
                )}

                {managedObjectsTab === 'inventory' && (
                  <div class="overflow-x-auto">
                    <table class="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
                      <thead>
                        <tr>
                          <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Name</th>
                          <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Namespace</th>
                          <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Kind</th>
                        </tr>
                      </thead>
                      <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
                        {resourceData.status.inventory.map((item, idx) => {
                          const isFluxResource = fluxKinds.includes(item.kind)
                          return (
                            <tr key={idx} class="hover:bg-gray-50 dark:hover:bg-gray-800">
                              <td class="px-3 py-2 text-sm font-mono">
                                {isFluxResource ? (
                                  <button
                                    onClick={() => handleNavigate(item)}
                                    class="text-gray-900 dark:text-gray-100 hover:text-flux-blue dark:hover:text-blue-400 inline-flex items-center gap-1"
                                  >
                                    {item.name}
                                    <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
                                    </svg>
                                  </button>
                                ) : (
                                  <span class="text-gray-900 dark:text-gray-100">{item.name}</span>
                                )}
                              </td>
                              <td class="px-3 py-2 text-sm text-gray-900 dark:text-gray-100 font-mono">{item.namespace || '-'}</td>
                              <td class="px-3 py-2 text-sm text-gray-900 dark:text-gray-100">{item.kind}</td>
                            </tr>
                          )
                        })}
                      </tbody>
                    </table>
                  </div>
                )}
              </div>
            )}
          </div>
        )}

        {/* Source Section - Collapsible */}
        {hasSource && (
          <div class="card p-0">
            <button
              onClick={() => isSourceExpanded.value = !isSourceExpanded.value}
              class="w-full px-6 py-4 text-left hover:bg-gray-50 dark:hover:bg-gray-700/30 transition-colors"
            >
              <div class="flex items-center justify-between">
                <div>
                  <h3 class="text-lg font-semibold text-gray-900 dark:text-white">Source</h3>
                </div>
                <svg
                  class={`w-5 h-5 text-gray-400 dark:text-gray-500 transition-transform ${isSourceExpanded.value ? 'rotate-180' : ''}`}
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7"/>
                </svg>
              </div>
            </button>
            {isSourceExpanded.value && (
              <div class="px-6 py-4">
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
                    {resourceData.status.sourceRef.status && (
                      <div class="flex items-baseline space-x-2">
                        <dt class="text-sm text-gray-500 dark:text-gray-400">Status:</dt>
                        <dd>
                          <span class={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                            resourceData.status.sourceRef.status === 'Ready' ? 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400' :
                              resourceData.status.sourceRef.status === 'Failed' ? 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400' :
                                resourceData.status.sourceRef.status === 'Progressing' ? 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400' :
                                  resourceData.status.sourceRef.status === 'Suspended' ? 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-400' :
                                    'bg-gray-100 text-gray-800 dark:bg-gray-900/30 dark:text-gray-400'
                          }`}>
                            {resourceData.status.sourceRef.status}
                          </span>
                        </dd>
                      </div>
                    )}

                    {/* Managed by */}
                    <div class="flex items-baseline space-x-2">
                      <dt class="text-sm text-gray-500 dark:text-gray-400">Managed by:</dt>
                      <dd class="text-sm text-gray-900 dark:text-white">{getControllerName(resourceData.status.sourceRef.kind)}</dd>
                    </div>

                    {/* ID */}
                    <div class="flex items-baseline space-x-2">
                      <dt class="text-sm text-gray-500 dark:text-gray-400">ID:</dt>
                      <dd class="text-sm text-gray-900 dark:text-white">
                        {resourceData.status.sourceRef.kind}/{resourceData.status.sourceRef.namespace || namespace}/{resourceData.status.sourceRef.name}
                      </dd>
                    </div>

                    {/* URL */}
                    {resourceData.status.sourceRef.url && (
                      <div class="flex items-baseline space-x-2">
                        <dt class="text-sm text-gray-500 dark:text-gray-400">URL:</dt>
                        <dd class="text-sm text-gray-900 dark:text-white break-all">{resourceData.status.sourceRef.url}</dd>
                      </div>
                    )}

                    {/* Origin URL */}
                    {resourceData.status.sourceRef.originURL && (
                      <div class="flex items-baseline space-x-2">
                        <dt class="text-sm text-gray-500 dark:text-gray-400">Origin URL:</dt>
                        <dd class="text-sm text-gray-900 dark:text-white break-all">{resourceData.status.sourceRef.originURL}</dd>
                      </div>
                    )}

                    {/* Origin Revision */}
                    {resourceData.status.sourceRef.originRevision && (
                      <div class="flex items-baseline space-x-2">
                        <dt class="text-sm text-gray-500 dark:text-gray-400">Origin Revision:</dt>
                        <dd class="text-sm text-gray-900 dark:text-white break-all">{resourceData.status.sourceRef.originRevision}</dd>
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
                    {resourceData.status.sourceRef.message && (
                      <div class="flex items-baseline space-x-2">
                        <dt class="text-sm text-gray-500 dark:text-gray-400">Fetch result:</dt>
                        <dd class="text-sm text-gray-700 dark:text-gray-300">
                          <pre class="whitespace-pre-wrap break-words font-sans">{resourceData.status.sourceRef.message}</pre>
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
              </div>
            )}
          </div>
        )}

      </div>
    </main>
  )
}
