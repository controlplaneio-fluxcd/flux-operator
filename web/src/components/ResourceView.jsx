// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useState, useMemo, useEffect } from 'preact/hooks'
import yaml from 'js-yaml'
import Prism from 'prismjs'
import 'prismjs/components/prism-yaml'
import { fetchWithMock } from '../utils/fetch'
import { appliedTheme } from '../utils/theme'

// Import Prism themes as URLs for dynamic loading
import prismLight from 'prismjs/themes/prism.css?url'
import prismDark from 'prismjs/themes/prism-tomorrow.css?url'

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
 */
function InventoryItem({ item }) {
  return (
    <div class="py-1 px-2 text-xs break-all">
      <span class="font-mono text-gray-900 dark:text-gray-100">
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
        <span class="text-xs font-bold text-gray-800 dark:text-gray-200 break-all">
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
 * ResourceView - Displays detailed view of a Flux resource with tabbed interface
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
export function ResourceView({ kind, name, namespace, isExpanded }) {
  const [resourceData, setResourceData] = useState(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState(null)
  const [activeTab, setActiveTab] = useState('inventory')

  // Dynamically load Prism theme based on current theme
  useEffect(() => {
    const linkId = 'prism-theme-link'

    // Remove existing Prism theme link if present
    const existingLink = document.getElementById(linkId)
    if (existingLink) {
      existingLink.remove()
    }

    // Add new theme link based on current theme
    const link = document.createElement('link')
    link.id = linkId
    link.rel = 'stylesheet'
    link.href = appliedTheme.value === 'dark' ? prismDark : prismLight
    document.head.appendChild(link)

    // Cleanup on unmount
    return () => {
      const link = document.getElementById(linkId)
      if (link) {
        link.remove()
      }
    }
  }, [appliedTheme.value])

  // Fetch resource details when expanded
  useEffect(() => {
    if (!isExpanded || resourceData || loading || error) return

    const fetchResourceDetails = async () => {
      setLoading(true)
      setError(null)

      const params = new URLSearchParams({ kind, name, namespace })

      try {
        const data = await fetchWithMock({
          endpoint: `/api/v1/resource?${params.toString()}`,
          mockPath: '../mock/resource',
          mockExport: 'getMockResource'
        })
        setResourceData(data)

        // Set default active tab: inventory > source > specification
        if (data.status?.inventory && data.status.inventory.length > 0) {
          setActiveTab('inventory')
        } else if (data.status?.sourceRef) {
          setActiveTab('source')
        } else {
          setActiveTab('specification')
        }
      } catch (err) {
        console.error('Failed to fetch resource details:', err)
        setError(err.message)
      } finally {
        setLoading(false)
      }
    }

    fetchResourceDetails()
  }, [isExpanded, kind, name, namespace, resourceData, loading, error])

  // Convert resource definition to YAML with syntax highlighting (memoized)
  const definitionYamlHighlighted = useMemo(() => {
    if (!resourceData) return ''

    // Build complete resource definition with apiVersion, kind, metadata, and spec
    const definition = {
      apiVersion: resourceData.apiVersion,
      kind: resourceData.kind,
      metadata: resourceData.metadata,
      spec: resourceData.spec
    }

    const yamlStr = yaml.dump(definition, { indent: 2, lineWidth: -1 })
    return Prism.highlight(yamlStr, Prism.languages.yaml, 'yaml')
  }, [resourceData])

  // Convert status to YAML with syntax highlighting (memoized)
  const statusYamlHighlighted = useMemo(() => {
    if (!resourceData) return ''

    // Build status object without inventory and sourceRef
    const cleanStatus = resourceData.status
      ? (() => {
        // eslint-disable-next-line no-unused-vars
        const { inventory, sourceRef, ...rest } = resourceData.status
        return rest
      })()
      : undefined

    const statusObj = {
      apiVersion: resourceData.apiVersion,
      kind: resourceData.kind,
      metadata: {
        name: resourceData.metadata.name,
        namespace: resourceData.metadata.namespace
      },
      status: cleanStatus
    }

    const yamlStr = yaml.dump(statusObj, { indent: 2, lineWidth: -1 })
    return Prism.highlight(yamlStr, Prism.languages.yaml, 'yaml')
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

  if (!isExpanded) return null

  return (
    <div class="mt-3 space-y-4">
      {/* Loading State */}
      {loading && (
        <div class="flex items-center justify-center p-4">
          <div class="animate-spin rounded-full h-6 w-6 border-b-2 border-flux-blue"></div>
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
          <div class="border-b border-gray-200 dark:border-gray-700">
            <nav class="flex space-x-4" aria-label="Tabs">
              {resourceData.status?.inventory && resourceData.status.inventory.length > 0 && (
                <button
                  onClick={() => setActiveTab('inventory')}
                  class={`py-2 px-1 text-sm font-medium border-b-2 transition-colors ${
                    activeTab === 'inventory'
                      ? 'border-flux-blue text-flux-blue dark:text-blue-400'
                      : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300 dark:text-gray-400 dark:hover:text-gray-300'
                  }`}
                >
                  Inventory ({resourceData.status.inventory.length})
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

          {/* Tab Content */}
          <div class="py-2">
            {/* Inventory Tab */}
            {activeTab === 'inventory' && resourceData.status?.inventory && (
              <div class="space-y-3">
                {sortedApiVersions.map(apiVersion => (
                  <InventoryGroupByApiVersion
                    key={apiVersion}
                    apiVersion={apiVersion}
                    items={groupedInventory[apiVersion]}
                  />
                ))}
              </div>
            )}

            {/* Source Tab */}
            {activeTab === 'source' && resourceData.status?.sourceRef && (
              <div class="space-y-2">
                <div class="grid grid-cols-1 gap-1 text-xs">
                  {/* ID: kind/namespace/name */}
                  <div class="py-1 px-2">
                    <span class="text-gray-600 dark:text-gray-400">ID: </span>
                    <span class="text-gray-900 dark:text-gray-100 font-mono">
                      {resourceData.status.sourceRef.kind}/{resourceData.status.sourceRef.namespace}/{resourceData.status.sourceRef.name}
                    </span>
                  </div>

                  {/* URL */}
                  <div class="py-1 px-2">
                    <span class="text-gray-600 dark:text-gray-400">URL: </span>
                    <span class="text-gray-900 dark:text-gray-100 font-mono break-all">{resourceData.status.sourceRef.url}</span>
                  </div>

                  {/* Origin URL (if present) */}
                  {resourceData.status.sourceRef.originURL && (
                    <div class="py-1 px-2">
                      <span class="text-gray-600 dark:text-gray-400">Origin URL: </span>
                      <span class="text-gray-900 dark:text-gray-100 font-mono break-all">{resourceData.status.sourceRef.originURL}</span>
                    </div>
                  )}

                  {/* Origin Revision (if present) */}
                  {resourceData.status.sourceRef.originRevision && (
                    <div class="py-1 px-2">
                      <span class="text-gray-600 dark:text-gray-400">Origin Revision: </span>
                      <span class="text-gray-900 dark:text-gray-100 font-mono break-all">{resourceData.status.sourceRef.originRevision}</span>
                    </div>
                  )}

                  {/* Status */}
                  <div class="py-1 px-2">
                    <span class="text-gray-600 dark:text-gray-400">Status: </span>
                    <span class={`font-semibold ${
                      resourceData.status.sourceRef.status === 'Ready'
                        ? 'text-green-600 dark:text-green-400'
                        : 'text-red-600 dark:text-red-400'
                    }`}>
                      {resourceData.status.sourceRef.status}
                    </span>
                  </div>

                  {/* Message */}
                  <div class="py-1 px-2">
                    <span class="text-gray-600 dark:text-gray-400">Message: </span>
                    <span class="text-gray-900 dark:text-gray-100 break-all">{resourceData.status.sourceRef.message}</span>
                  </div>
                </div>
              </div>
            )}

            {/* Specification Tab */}
            {activeTab === 'specification' && (
              <div>
                <pre class="p-2 bg-gray-50 dark:bg-gray-900 rounded-md overflow-x-auto language-yaml" style="font-size: 12px; line-height: 1.5;">
                  <code
                    class="language-yaml"
                    style="font-size: 12px;"
                    dangerouslySetInnerHTML={{ __html: definitionYamlHighlighted }}
                  />
                </pre>
              </div>
            )}

            {/* Status Tab */}
            {activeTab === 'status' && (
              <div>
                <pre class="p-2 bg-gray-50 dark:bg-gray-900 rounded-md overflow-x-auto language-yaml" style="font-size: 12px; line-height: 1.5;">
                  <code
                    class="language-yaml"
                    style="font-size: 12px;"
                    dangerouslySetInnerHTML={{ __html: statusYamlHighlighted }}
                  />
                </pre>
              </div>
            )}
          </div>
        </>
      )}
    </div>
  )
}
