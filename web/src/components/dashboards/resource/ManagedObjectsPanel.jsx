// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useMemo } from 'preact/compat'
import { useState, useEffect } from 'preact/hooks'
import { isKindWithInventory, isFluxInventoryItem, isWorkloadInventoryItem } from '../../../utils/constants'
import { fetchWithMock } from '../../../utils/fetch'
import { DashboardPanel, TabButton } from '../common/panel'
import { InventoryTabContent } from './InventoryTabContent'
import { GraphTabContent } from './GraphTabContent'
import { useHashTab } from '../../../utils/hash'

// Valid tabs for the ManagedObjectsPanel
const INVENTORY_TABS = ['overview', 'graph', 'inventory']

/**
 * ManagedObjectsPanel - Displays the managed objects for a Flux resource
 * Handles its own state management and statistics calculations
 */
export function ManagedObjectsPanel({ resourceData, onNavigate }) {
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
    return hasInventory ? resourceData.status.inventory.filter(item => isFluxInventoryItem(item)).length : 0
  }, [resourceData, hasInventory])

  const workloadsCount = useMemo(() => {
    return hasInventory ? resourceData.status.inventory.filter(item => isWorkloadInventoryItem(item)).length : 0
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

  // Inventory items whose live status the Graph tab renders: the Flux resources
  // and the Kubernetes workloads. The "Resources" bucket has no per-item status,
  // so it is excluded to keep the batch small.
  const statusItems = useMemo(() => {
    if (!hasInventory) return []
    return resourceData.status.inventory.filter(
      item => isFluxInventoryItem(item) || isWorkloadInventoryItem(item)
    )
  }, [resourceData, hasInventory])

  // Live status (status + statusMessage) for the Graph tab's Flux and Workloads
  // groups, keyed by `${kind}/${namespace}/${name}` to match the Graph item keys.
  // Owned here, at the panel level — not inside the tab component — so it survives
  // tab switches: re-entering the Graph tab shows the last-known status immediately
  // instead of refetching and flickering the badge in. It refreshes when the
  // inventory changes, which the parent resource poll drives, so no dedicated
  // polling loop is needed. Uses the inventory/objects endpoint in statusOnly mode
  // so the response carries status without the sanitized manifest.
  const [itemStatuses, setItemStatuses] = useState({})
  const tracksStatus = activeTab === 'graph'
  useEffect(() => {
    if (!tracksStatus || statusItems.length === 0) {
      return
    }

    let cancelled = false

    const fetchItemStatuses = async () => {
      try {
        const objects = statusItems.map(item => ({
          apiVersion: item.apiVersion,
          kind: item.kind,
          namespace: item.namespace || resourceData.metadata.namespace,
          name: item.name
        }))

        const response = await fetchWithMock({
          endpoint: '/api/v1/inventory/objects',
          mockPath: '../mock/inventory',
          mockExport: 'getMockInventoryObjects',
          method: 'POST',
          body: { objects, statusOnly: true }
        })

        const newStatuses = {}
        for (const obj of (response.objects || [])) {
          const key = `${obj.kind}/${obj.namespace}/${obj.name}`
          // Error items (Forbidden/NotFound) surface the error as both status and
          // message: the Graph row is message-driven, so without a message it would
          // render blank. Mirroring the error into the message keeps the dot + label
          // visible, consistent with the Inventory tab's status pill.
          newStatuses[key] = obj.error
            ? { status: obj.error, statusMessage: obj.error }
            : { status: obj.status, statusMessage: obj.statusMessage }
        }
        if (!cancelled) setItemStatuses(newStatuses)
      } catch (err) {
        console.error('Failed to fetch inventory statuses:', err)
      }
    }

    fetchItemStatuses()
    return () => { cancelled = true }
  }, [tracksStatus, statusItems, resourceData])

  return (
    <DashboardPanel title="Managed Objects" id="inventory-panel">
      {/* Tab Navigation */}
      <div class="border-b border-gray-200 dark:border-gray-700 mb-4">
        <nav class="flex space-x-4 overflow-x-auto">
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
          itemStatuses={itemStatuses}
        />
      )}

      {activeTab === 'inventory' && (
        <InventoryTabContent
          inventory={resourceData.status?.inventory}
        />
      )}
    </DashboardPanel>
  )
}
