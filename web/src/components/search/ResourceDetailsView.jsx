// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useState, useMemo, useEffect, useRef } from 'preact/hooks'
import { fetchWithMock } from '../../utils/fetch'
import { usePrismTheme, YamlBlock } from '../dashboards/common/yaml'
import { isKindWithInventory, getKindAlias, isFluxInventoryItem } from '../../utils/constants'
import { getStatusBadgeClass, cleanStatus } from '../../utils/status'
import { FluxOperatorIcon } from '../layout/Icons'

/**
 * Helper to group inventory items by apiVersion
 * @param {Array} inventory - Array of inventory items
 * @returns {Object} Structure: { apiVersion: [items] }
 */
function groupInventoryByApiVersion(inventory) {
  // Ensure inventory is an array
  if (!inventory || !Array.isArray(inventory) || inventory.length === 0) return {}

  const grouped = {}

  inventory.forEach(item => {
    const apiVersion = item.apiVersion || 'unknown'
    if (!grouped[apiVersion]) {
      grouped[apiVersion] = []
    }
    grouped[apiVersion].push(item)
  })

  // Sort items within each apiVersion by kind, namespace, then name
  Object.keys(grouped).forEach(apiVersion => {
    grouped[apiVersion].sort((a, b) => {
      const kindCompare = a.kind.localeCompare(b.kind)
      if (kindCompare !== 0) return kindCompare

      const nsA = a.namespace || ''
      const nsB = b.namespace || ''
      if (nsA !== nsB) return nsA.localeCompare(nsB)

      return a.name.localeCompare(b.name)
    })
  })

  return grouped
}

/**
 * InventoryItem - Displays a single inventory item
 *
 * Features:
 * - Displays kind, namespace (if present), and name
 * - If kind matches a Flux resource kind, renders as clickable link to resources page
 * - Includes navigation icon for clickable items
 */
function InventoryItem({ item }) {
  const isFluxResource = isFluxInventoryItem(item)

  // Build resource URL
  const ns = item.namespace || ''
  const resourceUrl = `/resource/${encodeURIComponent(item.kind)}/${encodeURIComponent(ns)}/${encodeURIComponent(item.name)}`

  if (isFluxResource) {
    return (
      <div class="py-1 px-2 text-xs break-all">
        <a
          href={resourceUrl}
          class="text-left hover:opacity-80 transition-opacity focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-flux-blue rounded inline-block group"
        >
          <span class="text-gray-600 dark:text-gray-400">{item.kind}/</span>{item.namespace && <span class="text-gray-500 dark:text-gray-400">{item.namespace}/</span>}<span class="text-gray-900 dark:text-gray-100 group-hover:text-flux-blue dark:group-hover:text-blue-400">{item.name}</span><svg class="w-3 h-3 text-gray-400 group-hover:text-flux-blue dark:group-hover:text-blue-400 transition-colors ml-1 inline-block align-middle" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" /></svg>
        </a>
      </div>
    )
  }

  // Non-Flux resource - render as plain text
  return (
    <div class="py-1 px-2 text-xs break-all">
      <span class="text-gray-900 dark:text-gray-100">
        <span class="text-gray-600 dark:text-gray-400">{item.kind}/</span>
        {item.namespace && (
          <span class="text-gray-500 dark:text-gray-400">{item.namespace}/</span>
        )}
        {item.name}
      </span>
    </div>
  )
}

/**
 * InventoryGroupByApiVersion - Groups inventory items under an API version heading
 */
function InventoryGroupByApiVersion({ apiVersion, items }) {
  return (
    <div class="mb-3">
      <div class="flex items-center gap-2 py-1 flex-wrap">
        <span class="text-xs font-semibold text-gray-800 dark:text-gray-200 break-all">
          {apiVersion}
        </span>
        <span class="inline-flex items-center px-2 py-0.5 rounded text-xs bg-gray-200 dark:bg-gray-700 text-gray-700 dark:text-gray-300">
          {items.length}
        </span>
      </div>
      <div class="ml-0 sm:ml-2">
        {items.map((item, idx) => (
          <InventoryItem key={idx} item={item} />
        ))}
      </div>
    </div>
  )
}

/**
 * ResourceDetailsView - Displays detailed view of a Flux resource with tabbed interface
 *
 * @param {Object} props
 * @param {string} props.kind - Resource kind
 * @param {string} props.name - Resource name
 * @param {string} props.namespace - Resource namespace
 * @param {boolean} props.isExpanded - Whether the view is expanded
 *
 * Features:
 * - Lazy loads complete resource data on expand
 * - Tabbed interface with up to four sections (in order):
 *   1. Inventory: Grouped list of managed resources (if present, shown first)
 *   2. Source: Details about the resource's source (if present)
 *   3. Specification: Complete resource definition as syntax-highlighted YAML
 *   4. Status: YAML display of apiVersion, kind, metadata, and status (without inventory)
 * - Dynamically switches Prism theme (light/dark) based on app theme
 * - Caches data to avoid redundant fetches
 * - Handles loading and error states
 */
export function ResourceDetailsView({ kind, name, namespace, isExpanded }) {
  const [resourceData, setResourceData] = useState(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState(null)
  const [activeTab, setActiveTab] = useState('inventory')
  const fetchingRef = useRef(false)

  // Load Prism theme based on current app theme
  usePrismTheme()

  // Reset state when resource identity changes
  useEffect(() => {
    setResourceData(null)
    setError(null)
    setActiveTab('inventory')
  }, [kind, name, namespace])

  // Fetch resource details when expanded
  useEffect(() => {
    if (!isExpanded || resourceData || fetchingRef.current) return

    let cancelled = false
    fetchingRef.current = true

    const fetchResourceDetails = async () => {
      if (!cancelled) {
        setLoading(true)
        setError(null)
      }

      const params = new URLSearchParams({ kind, name, namespace })

      try {
        const data = await fetchWithMock({
          endpoint: `/api/v1/resource?${params.toString()}`,
          mockPath: '../mock/resource',
          mockExport: 'getMockResource'
        })
        if (!cancelled) {
          setResourceData(data)

          // Set default active tab: inventory > source > specification
          const hasInventory = data.status?.inventory && data.status.inventory.length > 0
          if (isKindWithInventory(data.kind) || hasInventory) {
            setActiveTab('inventory')
          } else if (data.status?.sourceRef) {
            setActiveTab('source')
          } else {
            setActiveTab('specification')
          }
        }
      } catch (err) {
        console.error('Failed to fetch resource details:', err)
        if (!cancelled) setError(err.message)
      } finally {
        fetchingRef.current = false
        if (!cancelled) setLoading(false)
      }
    }

    fetchResourceDetails()
    return () => { cancelled = true }
  }, [isExpanded, kind, name, namespace, resourceData])

  // Build resource definition object (memoized)
  const definitionData = useMemo(() => {
    if (!resourceData) return null
    return {
      apiVersion: resourceData.apiVersion,
      kind: resourceData.kind,
      metadata: resourceData.metadata,
      spec: resourceData.spec
    }
  }, [resourceData])

  // Build status object without inventory and sourceRef (memoized)
  const statusData = useMemo(() => {
    if (!resourceData) return null

    return {
      apiVersion: resourceData.apiVersion,
      kind: resourceData.kind,
      metadata: {
        name: resourceData.metadata.name,
        namespace: resourceData.metadata.namespace
      },
      status: cleanStatus(resourceData.status)
    }
  }, [resourceData])

  // Group inventory by apiVersion (memoized)
  const groupedInventory = useMemo(
    () => groupInventoryByApiVersion(resourceData?.status?.inventory || []),
    [resourceData]
  )

  // Sort apiVersions
  const sortedApiVersions = useMemo(() => {
    const versions = Object.keys(groupedInventory)
    return versions.sort((a, b) => {
      if (a === 'apiextensions.k8s.io/v1' && b !== 'apiextensions.k8s.io/v1') return -1
      if (b === 'apiextensions.k8s.io/v1' && a !== 'apiextensions.k8s.io/v1') return 1
      if (a === 'v1' && b !== 'v1') return -1
      if (b === 'v1' && a !== 'v1') return 1
      return a.localeCompare(b)
    })
  }, [groupedInventory])

  // Check if inventory tab should be shown
  const shouldShowInventoryTab = useMemo(() => {
    if (!resourceData) return false
    const hasInventory = resourceData.status?.inventory && resourceData.status.inventory.length > 0
    return isKindWithInventory(resourceData.kind) || hasInventory
  }, [resourceData])

  // Get inventory count
  const inventoryCount = resourceData?.status?.inventory?.length || 0

  if (!isExpanded) return null

  return (
    <div class="mt-3 space-y-4">
      {/* Loading State */}
      {loading && (
        <div class="flex items-center justify-center p-4">
          <FluxOperatorIcon className="animate-spin h-6 w-6 text-flux-blue" />
          <span class="ml-2 text-sm text-gray-600 dark:text-gray-400">
            Loading details...
          </span>
        </div>
      )}

      {/* Error State */}
      {error && (
        <div class="p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-md">
          <p class="text-sm text-red-800 dark:text-red-200">
            Failed to load details: {error}
          </p>
        </div>
      )}

      {/* Tabs + Content */}
      {!loading && !error && resourceData && (
        <>
          {/* Tab Navigation */}
          <div class="border-b border-gray-200 dark:border-gray-700 mb-4">
            <nav class="flex space-x-4" aria-label="Tabs">
              {shouldShowInventoryTab && (
                <button
                  onClick={() => setActiveTab('inventory')}
                  class={`py-2 px-1 text-sm font-medium border-b-2 transition-colors ${
                    activeTab === 'inventory'
                      ? 'border-flux-blue text-flux-blue dark:text-blue-400'
                      : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300 dark:text-gray-400 dark:hover:text-gray-300'
                  }`}
                >
                  Inventory ({inventoryCount})
                </button>
              )}
              {resourceData.status?.sourceRef && (
                <button
                  onClick={() => setActiveTab('source')}
                  class={`py-2 px-1 text-sm font-medium border-b-2 transition-colors ${
                    activeTab === 'source'
                      ? 'border-flux-blue text-flux-blue dark:text-blue-400'
                      : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300 dark:text-gray-400 dark:hover:text-gray-300'
                  }`}
                >
                  Source
                </button>
              )}
              <button
                onClick={() => setActiveTab('specification')}
                class={`py-2 px-1 text-sm font-medium border-b-2 transition-colors ${
                  activeTab === 'specification'
                    ? 'border-flux-blue text-flux-blue dark:text-blue-400'
                    : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300 dark:text-gray-400 dark:hover:text-gray-300'
                }`}
              >
                <span class="inline sm:hidden">Spec</span>
                <span class="hidden sm:inline">Specification</span>
              </button>
              <button
                onClick={() => setActiveTab('status')}
                class={`py-2 px-1 text-sm font-medium border-b-2 transition-colors ${
                  activeTab === 'status'
                    ? 'border-flux-blue text-flux-blue dark:text-blue-400'
                    : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300 dark:text-gray-400 dark:hover:text-gray-300'
                }`}
              >
                Status
              </button>
            </nav>
          </div>

          {/* Inventory Tab */}
          {activeTab === 'inventory' && shouldShowInventoryTab && (
            <div class="space-y-3">
              {inventoryCount > 0 ? (
                sortedApiVersions.map(apiVersion => (
                  <InventoryGroupByApiVersion
                    key={apiVersion}
                    apiVersion={apiVersion}
                    items={groupedInventory[apiVersion]}
                  />
                ))
              ) : (
                <div class="py-4 px-2 text-sm text-gray-600 dark:text-gray-400">
                  Empty inventory, no managed objects
                </div>
              )}
            </div>
          )}

          {/* Source Tab */}
          {activeTab === 'source' && resourceData.status?.sourceRef && (
            <div class="space-y-4">
              {/* Resource Link */}
              <a
                href={`/resource/${encodeURIComponent(resourceData.status.sourceRef.kind)}/${encodeURIComponent(resourceData.status.sourceRef.namespace)}/${encodeURIComponent(resourceData.status.sourceRef.name)}`}
                class="flex items-center gap-2 text-sm text-flux-blue dark:text-blue-400 hover:underline"
              >
                <svg class="w-4 h-4 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
                </svg>
                <span class="hidden md:inline break-all">{resourceData.status.sourceRef.kind}/{resourceData.status.sourceRef.namespace}/{resourceData.status.sourceRef.name}</span>
                <span class="md:hidden break-all">{getKindAlias(resourceData.status.sourceRef.kind)}/{resourceData.status.sourceRef.name}</span>
              </a>

              {/* Status Badge */}
              {resourceData.status.sourceRef.status && (
                <div class="text-sm">
                  <span class="text-gray-500 dark:text-gray-400">Status</span>
                  <span class={`ml-1 inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${getStatusBadgeClass(resourceData.status.sourceRef.status)}`}>
                    {resourceData.status.sourceRef.status}
                  </span>
                </div>
              )}

              {/* URL */}
              <div class="text-sm">
                <span class="text-gray-500 dark:text-gray-400">URL</span>
                <span class="ml-1 text-gray-900 dark:text-white break-words">{resourceData.status.sourceRef.url}</span>
              </div>

              {/* Origin URL (if present) */}
              {resourceData.status.sourceRef.originURL && (
                <div class="text-sm">
                  <span class="text-gray-500 dark:text-gray-400">Origin URL</span>
                  <span class="ml-1 text-gray-900 dark:text-white break-words">{resourceData.status.sourceRef.originURL}</span>
                </div>
              )}

              {/* Origin Revision (if present) */}
              {resourceData.status.sourceRef.originRevision && (
                <div class="text-sm">
                  <span class="text-gray-500 dark:text-gray-400">Origin Revision</span>
                  <span class="ml-1 text-gray-900 dark:text-white break-words">{resourceData.status.sourceRef.originRevision}</span>
                </div>
              )}

              {/* Fetch result */}
              {resourceData.status.sourceRef.message && (
                <div class="text-sm">
                  <span class="text-gray-500 dark:text-gray-400">Fetch result</span>
                  <span class="ml-1 text-gray-900 dark:text-white break-words">{resourceData.status.sourceRef.message}</span>
                </div>
              )}
            </div>
          )}

          {/* Specification Tab */}
          {activeTab === 'specification' && (
            <YamlBlock data={definitionData} />
          )}

          {/* Status Tab */}
          {activeTab === 'status' && (
            <YamlBlock data={statusData} />
          )}
        </>
      )}
    </div>
  )
}
