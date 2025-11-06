import { signal } from '@preact/signals'
import { showSearchView } from '../app'
import { activeSearchTab } from './SearchView'
import { selectedResourceKind } from './ResourceList'

// Store collapsed state for the grid
const isExpanded = signal(true)

function ReconcilerCard({ reconciler }) {
  const stats = reconciler.stats
  const total = (stats.failing || 0) + (stats.running || 0) + (stats.suspended || 0)

  // Determine status color
  const getStatusColor = () => {
    if (stats.failing > 0) return 'border-danger'
    if (stats.suspended > 0) return 'border-warning'
    return 'border-success'
  }

  // Handle card click - navigate to search view with resources tab and kind prefilled
  const handleClick = () => {
    selectedResourceKind.value = reconciler.kind
    activeSearchTab.value = 'resources'
    showSearchView.value = true
  }

  return (
    <button
      onClick={handleClick}
      class={`card border-l-4 px-4 ${getStatusColor()} hover:shadow-lg transition-all cursor-pointer text-left w-full`}
    >
      <div class="mb-3">
        <h4 class="font-semibold text-gray-900 dark:text-white text-md">{reconciler.kind}</h4>
        <p class="text-xs text-gray-500 dark:text-gray-400">{reconciler.apiVersion}</p>
      </div>

      <div class="flex items-center gap-2 text-2xl font-bold text-gray-900 dark:text-white mb-2">
        <span>{total}</span>
        {stats.failing > 0 && (
          <svg class="w-6 h-6 text-danger" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
        )}
      </div>

      <div class="flex flex-wrap gap-2">
        {stats.running > 0 && (
          <span class="inline-flex items-center px-2 py-1 rounded text-xs font-medium bg-green-200 text-green-800">
            {stats.running} running
          </span>
        )}
        {stats.failing > 0 && (
          <span class="inline-flex items-center px-2 py-1 rounded text-xs font-medium bg-red-100 text-red-800">
            {stats.failing} failing
          </span>
        )}
        {stats.suspended > 0 && (
          <span class="inline-flex items-center px-2 py-1 rounded text-xs font-medium bg-yellow-100 text-yellow-800">
            {stats.suspended} suspended
          </span>
        )}
      </div>
    </button>
  )
}

function ReconcilerGroup({ title, reconcilers }) {
  if (reconcilers.length === 0) return null

  return (
    <div class="mb-6">
      <h4 class="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-3">{title}</h4>
      <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
        {reconcilers.map(reconciler => (
          <ReconcilerCard
            key={`${reconciler.apiVersion}/${reconciler.kind}`}
            reconciler={reconciler}
          />
        ))}
      </div>
    </div>
  )
}

export function ReconcilerList({ reconcilers }) {
  const totalResources = reconcilers.reduce((sum, r) => {
    return sum + (r.stats.failing || 0) + (r.stats.running || 0) + (r.stats.suspended || 0)
  }, 0)

  const totalFailing = reconcilers.reduce((sum, r) => sum + (r.stats.failing || 0), 0)

  // Group reconcilers by API type
  const appliers = reconcilers.filter(r =>
    r.apiVersion.startsWith('fluxcd.controlplane') ||
    r.apiVersion.startsWith('kustomize') ||
    r.apiVersion.startsWith('helm')
  )

  const sources = reconcilers.filter(r => r.apiVersion.startsWith('source'))

  const notifications = reconcilers.filter(r => r.apiVersion.startsWith('notification'))

  const imageAutomation = reconcilers.filter(r => r.apiVersion.startsWith('image'))

  return (
    <div class="card">
      <button
        onClick={() => isExpanded.value = !isExpanded.value}
        class={`w-full text-left hover:opacity-80 transition-opacity ${isExpanded.value ? 'mb-6' : ''}`}
      >
        <div class="flex items-center justify-between">
          <div>
            <h3 class="text-lg font-semibold text-gray-900 dark:text-white">Flux Reconcilers</h3>
            <div class="flex items-center space-x-4 mt-2">
              <p class="text-sm text-gray-600 dark:text-gray-400">
                {reconcilers.length} CRDs • {totalResources} resources
              </p>
              {totalFailing > 0 && (
                <span class="status-badge status-not-ready">
                  {totalFailing} failing
                </span>
              )}
            </div>
          </div>
          <svg
            class={`w-5 h-5 text-gray-400 dark:text-gray-500 transition-transform ${isExpanded.value ? 'rotate-180' : ''}`}
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
          </svg>
        </div>
      </button>
      {isExpanded.value && (
        <div>
          <ReconcilerGroup title="Appliers" reconcilers={appliers} />
          <ReconcilerGroup title="Sources" reconcilers={sources} />
          <ReconcilerGroup title="Notifications" reconcilers={notifications} />
          <ReconcilerGroup title="Image Automation" reconcilers={imageAutomation} />
        </div>
      )}
    </div>
  )
}
