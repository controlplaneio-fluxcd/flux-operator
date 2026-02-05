// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useMemo } from 'preact/compat'
import { fluxKinds, workloadKinds, isKindWithInventory } from '../../../utils/constants'
import { DashboardPanel, TabButton } from '../common/panel'
import { WorkloadsTabContent } from './WorkloadsTabContent'
import { GraphTabContent } from './GraphTabContent'
import { useHashTab } from '../../../utils/hash'

// Valid tabs for the InventoryPanel
const INVENTORY_TABS = ['overview', 'graph', 'inventory', 'workloads']

/**
 * InventoryPanel - Displays managed objects inventory for a Flux resource
 * Handles its own state management and statistics calculations
 */
export function InventoryPanel({ resourceData, onNavigate }) {
  // Tab state synced with URL hash (e.g., #inventory-graph)
  const [activeTab, setActiveTab] = useHashTab('inventory', 'overview', INVENTORY_TABS, 'inventory-panel')

  // Check if inventory exists
  const hasInventory = resourceData?.status?.inventory && resourceData.status.inventory.length > 0

  // Check for inventory error
  const inventoryError = resourceData?.status?.inventoryError

  // Check if this kind should have inventory panel
  const shouldShowPanel = isKindWithInventory(resourceData?.kind)

  // Don't render if this kind doesn't support inventory
  if (!shouldShowPanel) {
    return null
  }

  // Calculate managed objects statistics
  const totalResourcesCount = useMemo(() => {
    return hasInventory ? resourceData.status.inventory.length : 0
  }, [resourceData, hasInventory])

  const fluxResourcesCount = useMemo(() => {
    return hasInventory ? resourceData.status.inventory.filter(item => fluxKinds.includes(item.kind)).length : 0
  }, [resourceData, hasInventory])

  const workloadsCount = useMemo(() => {
    return hasInventory ? resourceData.status.inventory.filter(item => workloadKinds.includes(item.kind)).length : 0
  }, [resourceData, hasInventory])

  const secretsCount = useMemo(() => {
    return hasInventory ? resourceData.status.inventory.filter(item => item.kind === 'Secret').length : 0
  }, [resourceData, hasInventory])

  // Calculate feature flags
  const pruningEnabled = useMemo(() => {
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
    if (resourceData.kind === 'Kustomization') {
      return !!resourceData.spec?.decryption
    }
    return false
  }, [resourceData])

  // Sort inventory items
  const sortedInventory = useMemo(() => {
    if (!hasInventory) return []
    return [...resourceData.status.inventory].sort((a, b) => {
      // Non-namespaced items first
      const aHasNamespace = !!a.namespace
      const bHasNamespace = !!b.namespace

      if (!aHasNamespace && bHasNamespace) return -1
      if (aHasNamespace && !bHasNamespace) return 1

      // Both non-namespaced: sort by kind, then name
      if (!aHasNamespace && !bHasNamespace) {
        if (a.kind !== b.kind) {
          return a.kind.localeCompare(b.kind)
        }
        return a.name.localeCompare(b.name)
      }

      // Both namespaced: sort by namespace, then kind, then name
      if (a.namespace !== b.namespace) {
        return a.namespace.localeCompare(b.namespace)
      }
      if (a.kind !== b.kind) {
        return a.kind.localeCompare(b.kind)
      }
      return a.name.localeCompare(b.name)
    })
  }, [resourceData, hasInventory])

  // Filter workload items
  const workloadItems = useMemo(() => {
    if (!hasInventory) return []
    return resourceData.status.inventory.filter(item => workloadKinds.includes(item.kind))
  }, [resourceData, hasInventory])

  // Build URL for a resource
  const getResourceUrl = (item) => {
    const ns = item.namespace || resourceData.metadata.namespace
    return `/resource/${encodeURIComponent(item.kind)}/${encodeURIComponent(ns)}/${encodeURIComponent(item.name)}`
  }

  return (
    <DashboardPanel title="Managed Objects" id="inventory-panel">
      {/* Tab Navigation */}
      <div class="border-b border-gray-200 dark:border-gray-700 mb-4">
        <nav class="flex space-x-4">
          <TabButton active={activeTab === 'overview'} onClick={() => setActiveTab('overview')}>
            <span class="sm:hidden">Info</span>
            <span class="hidden sm:inline">Overview</span>
          </TabButton>
          <TabButton active={activeTab === 'graph'} onClick={() => setActiveTab('graph')}>
            Graph
          </TabButton>
          {hasInventory && (
            <TabButton active={activeTab === 'inventory'} onClick={() => setActiveTab('inventory')}>
              Inventory
            </TabButton>
          )}
          {workloadsCount > 0 && (
            <TabButton active={activeTab === 'workloads'} onClick={() => setActiveTab('workloads')}>
              Workloads
            </TabButton>
          )}
        </nav>
      </div>

      {/* Inventory Error */}
      {inventoryError && (
        <div data-testid="inventory-error" class="mb-4 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-md p-3">
          <div class="flex items-start gap-2">
            <svg class="w-5 h-5 text-red-500 dark:text-red-400 flex-shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
            </svg>
            <p class="text-sm text-red-800 dark:text-red-200">{inventoryError}</p>
          </div>
        </div>
      )}

      {/* Tab Content */}
      {activeTab === 'overview' && !inventoryError && (
        <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
          {/* Left column: Feature toggles */}
          <div class="space-y-4">
            {/* Garbage collection */}
            <div class="text-sm">
              <span class="text-gray-500 dark:text-gray-400">Garbage collection</span>
              <span class={`ml-1 inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                pruningEnabled
                  ? 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400'
                  : 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400'
              }`}>
                {pruningEnabled ? 'Enabled' : 'Disabled'}
              </span>
            </div>

            {/* Health checking */}
            <div class="text-sm">
              <span class="text-gray-500 dark:text-gray-400">Health checking</span>
              <span class={`ml-1 inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                healthCheckEnabled
                  ? 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400'
                  : 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400'
              }`}>
                {healthCheckEnabled ? 'Enabled' : 'Disabled'}
              </span>
            </div>

            {/* Secret decryption */}
            <div class="text-sm">
              <span class="text-gray-500 dark:text-gray-400">Secret decryption</span>
              <span class={`ml-1 inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                secretDecryptionEnabled
                  ? 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400'
                  : 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400'
              }`}>
                {secretDecryptionEnabled ? 'Enabled' : 'Disabled'}
              </span>
            </div>
          </div>

          {/* Right column: Resource counts */}
          <div class="space-y-4 border-gray-200 dark:border-gray-700 border-t pt-4 md:border-t-0 md:border-l md:pt-0 md:pl-6">
            {/* Total resources */}
            <div class="text-sm">
              <span class="text-gray-500 dark:text-gray-400">Total resources</span>
              <span class="ml-1 text-gray-900 dark:text-white">{totalResourcesCount}</span>
            </div>

            {/* Flux resources */}
            <div class="text-sm">
              <span class="text-gray-500 dark:text-gray-400">Flux resources</span>
              <span class="ml-1 text-gray-900 dark:text-white">{fluxResourcesCount}</span>
            </div>

            {/* Kubernetes workloads */}
            <div class="text-sm">
              <span class="text-gray-500 dark:text-gray-400">Kubernetes workloads</span>
              <span class="ml-1 text-gray-900 dark:text-white">{workloadsCount}</span>
            </div>

            {/* Kubernetes secrets */}
            <div class="text-sm">
              <span class="text-gray-500 dark:text-gray-400">Kubernetes secrets</span>
              <span class="ml-1 text-gray-900 dark:text-white">{secretsCount}</span>
            </div>
          </div>
        </div>
      )}

      {activeTab === 'graph' && (
        <GraphTabContent
          resourceData={resourceData}
          namespace={resourceData.metadata.namespace}
          onNavigate={onNavigate}
          setActiveTab={setActiveTab}
        />
      )}

      {activeTab === 'inventory' && (
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
              {sortedInventory.map((item, idx) => {
                const isFluxResource = fluxKinds.includes(item.kind)
                return (
                  <tr key={idx} class="hover:bg-gray-50 dark:hover:bg-gray-800">
                    <td class="px-3 py-2 text-sm">
                      {isFluxResource ? (
                        <a
                          href={getResourceUrl(item)}
                          class="text-flux-blue dark:text-blue-400 hover:underline"
                        >
                          {item.name}
                        </a>
                      ) : (
                        <span class="text-gray-900 dark:text-gray-100">{item.name}</span>
                      )}
                    </td>
                    <td class="px-3 py-2 text-sm text-gray-900 dark:text-gray-100">{item.namespace || '-'}</td>
                    <td class="px-3 py-2 text-sm text-gray-900 dark:text-gray-100">{item.kind}</td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      )}

      {activeTab === 'workloads' && (
        <WorkloadsTabContent
          workloadItems={workloadItems}
          namespace={resourceData.metadata.namespace}
          userActions={resourceData?.status?.userActions || []}
        />
      )}
    </DashboardPanel>
  )
}
