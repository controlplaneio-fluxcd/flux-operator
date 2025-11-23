// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useState, useMemo } from 'preact/hooks'
import { useSignal } from '@preact/signals'
import { fluxKinds, workloadKinds } from '../../utils/constants'
import { TabButton } from './PanelComponents'

/**
 * InventoryPanel - Displays managed objects inventory for a Flux resource
 * Handles its own state management and statistics calculations
 */
export function InventoryPanel({ resourceData, onNavigate }) {
  // Tab state
  const [activeTab, setActiveTab] = useState('overview')

  // Collapsible state
  const isExpanded = useSignal(true)

  // Check if inventory exists
  const hasInventory = resourceData?.status?.inventory && resourceData.status.inventory.length > 0

  // Don't render if no inventory
  if (!hasInventory) {
    return null
  }

  // Calculate managed objects statistics
  const totalResourcesCount = useMemo(() => {
    return resourceData.status.inventory.length
  }, [resourceData])

  const fluxResourcesCount = useMemo(() => {
    return resourceData.status.inventory.filter(item => fluxKinds.includes(item.kind)).length
  }, [resourceData])

  const workloadsCount = useMemo(() => {
    return resourceData.status.inventory.filter(item => workloadKinds.includes(item.kind)).length
  }, [resourceData])

  const secretsCount = useMemo(() => {
    return resourceData.status.inventory.filter(item => item.kind === 'Secret').length
  }, [resourceData])

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
  }, [resourceData])

  // Handle navigation to a resource
  const handleNavigate = (item) => {
    if (onNavigate) {
      onNavigate(item)
    }
  }

  return (
    <div class="card p-0" data-testid="inventory-view">
      <button
        onClick={() => isExpanded.value = !isExpanded.value}
        class="w-full px-6 py-4 text-left hover:bg-gray-50 dark:hover:bg-gray-700/30 transition-colors"
      >
        <div class="flex items-center justify-between">
          <div>
            <h3 class="text-lg font-semibold text-gray-900 dark:text-white">Managed Objects</h3>
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
          {/* Tab Navigation */}
          <div class="border-b border-gray-200 dark:border-gray-700 mb-4">
            <nav class="flex space-x-4">
              <TabButton active={activeTab === 'overview'} onClick={() => setActiveTab('overview')}>
                <span class="sm:hidden">Info</span>
                <span class="hidden sm:inline">Overview</span>
              </TabButton>
              <TabButton active={activeTab === 'inventory'} onClick={() => setActiveTab('inventory')}>
                Inventory
              </TabButton>
            </nav>
          </div>

          {/* Tab Content */}
          {activeTab === 'overview' && (
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
                            <button
                              onClick={() => handleNavigate(item)}
                              class="text-flux-blue dark:text-blue-400 hover:underline"
                            >
                              {item.name}
                            </button>
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
        </div>
      )}
    </div>
  )
}
