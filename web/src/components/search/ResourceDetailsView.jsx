// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useState, useMemo, useEffect } from 'preact/hooks'
import { useLocation } from 'preact-iso'
import { fetchWithMock } from '../../utils/fetch'
import { usePrismTheme, YamlBlock } from '../dashboards/common/yaml'
import { isKindWithInventory, getKindAlias } from '../../utils/constants'
import { getStatusBadgeClass } from '../../utils/status'
import { FluxOperatorIcon } from '../layout/Icons'
import { GraphTabContent } from '../dashboards/resource/GraphTabContent'

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
 *   1. Graph: Visual dependency graph showing source → reconciler → inventory
 *   2. Source: Details about the resource's source (if present)
 *   3. Specification: Complete resource definition as syntax-highlighted YAML
 *   4. Status: YAML display of apiVersion, kind, metadata, and status (without inventory)
 * - Dynamically switches Prism theme (light/dark) based on app theme
 * - Caches data to avoid redundant fetches
 * - Handles loading and error states
 */
export function ResourceDetailsView({ kind, name, namespace, isExpanded }) {
  const location = useLocation()
  const [resourceData, setResourceData] = useState(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState(null)
  const [activeTab, setActiveTab] = useState('graph')

  // Load Prism theme based on current app theme
  usePrismTheme()

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

        // Set default active tab: graph > source > specification
        const hasInventory = data.status?.inventory && data.status.inventory.length > 0
        if (isKindWithInventory(data.kind) || hasInventory) {
          setActiveTab('graph')
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

    const cleanStatus = resourceData.status
      ? (() => {
        // eslint-disable-next-line no-unused-vars
        const { inventory, sourceRef, reconcilerRef, ...rest } = resourceData.status
        return rest
      })()
      : undefined

    return {
      apiVersion: resourceData.apiVersion,
      kind: resourceData.kind,
      metadata: {
        name: resourceData.metadata.name,
        namespace: resourceData.metadata.namespace
      },
      status: cleanStatus
    }
  }, [resourceData])

  // Check if graph tab should be shown
  const shouldShowGraphTab = useMemo(() => {
    if (!resourceData) return false
    const hasInventory = resourceData.status?.inventory && resourceData.status.inventory.length > 0
    return isKindWithInventory(resourceData.kind) || hasInventory
  }, [resourceData])

  // Handle navigation from graph
  const handleGraphNavigate = (item) => {
    const ns = item.namespace || ''
    location.route(`/resource/${encodeURIComponent(item.kind)}/${encodeURIComponent(ns)}/${encodeURIComponent(item.name)}`)
  }

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
              {shouldShowGraphTab && (
                <button
                  onClick={() => setActiveTab('graph')}
                  class={`py-2 px-1 text-sm font-medium border-b-2 transition-colors ${
                    activeTab === 'graph'
                      ? 'border-flux-blue text-flux-blue dark:text-blue-400'
                      : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300 dark:text-gray-400 dark:hover:text-gray-300'
                  }`}
                >
                  Graph
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

          {/* Graph Tab */}
          {activeTab === 'graph' && shouldShowGraphTab && (
            <GraphTabContent
              resourceData={resourceData}
              namespace={resourceData.metadata?.namespace}
              onNavigate={handleGraphNavigate}
            />
          )}

          {/* Source Tab */}
          {activeTab === 'source' && resourceData.status?.sourceRef && (
            <div class="space-y-4">
              {/* Resource Link */}
              <button
                onClick={() => location.route(`/resource/${encodeURIComponent(resourceData.status.sourceRef.kind)}/${encodeURIComponent(resourceData.status.sourceRef.namespace)}/${encodeURIComponent(resourceData.status.sourceRef.name)}`)}
                class="flex items-center gap-2 text-sm text-flux-blue dark:text-blue-400 hover:underline"
              >
                <svg class="w-4 h-4 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
                </svg>
                <span class="hidden md:inline break-all">{resourceData.status.sourceRef.kind}/{resourceData.status.sourceRef.namespace}/{resourceData.status.sourceRef.name}</span>
                <span class="md:hidden break-all">{getKindAlias(resourceData.status.sourceRef.kind)}/{resourceData.status.sourceRef.name}</span>
              </button>

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
