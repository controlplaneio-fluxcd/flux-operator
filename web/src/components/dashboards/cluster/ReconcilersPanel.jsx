// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useSignal } from '@preact/signals'
import { useLocation } from 'preact-iso'
import { selectedResourceKind, selectedResourceName, selectedResourceNamespace, selectedResourceStatus } from '../../search/ResourceList'

/**
 * ReconcilerCard - Individual card displaying a Flux CRD with resource statistics
 *
 * @param {Object} props
 * @param {Object} props.reconciler - Reconciler object with kind, apiVersion, and stats
 *
 * Features:
 * - Shows CRD kind and API version
 * - Displays total resource count
 * - Shows status badges (running, failing, suspended) with counts
 * - Entire card is clickable to filter by kind in search view
 * - Individual status badges are clickable to filter by kind + status
 * - Color-coded border based on resource health
 */
function ReconcilerCard({ reconciler }) {
  const location = useLocation()
  const stats = reconciler.stats
  const total = (stats.failing || 0) + (stats.running || 0) + (stats.suspended || 0)

  // Determine status color
  const getStatusColor = () => {
    if (stats.failing > 0) return 'border-danger'
    if (stats.suspended > 0) return 'border-warning'
    return 'border-success'
  }

  // Handle card click - navigate to resources page with kind filter
  const handleClick = () => {
    selectedResourceKind.value = reconciler.kind
    selectedResourceName.value = ''
    selectedResourceNamespace.value = ''
    selectedResourceStatus.value = ''
    location.route(`/resources?kind=${encodeURIComponent(reconciler.kind)}`)
  }

  // Handle status badge click - navigate to resources page with kind and status filters
  const handleStatusClick = (e, status) => {
    e.stopPropagation()
    selectedResourceKind.value = reconciler.kind
    selectedResourceName.value = ''
    selectedResourceNamespace.value = ''
    selectedResourceStatus.value = status
    location.route(`/resources?kind=${encodeURIComponent(reconciler.kind)}&status=${encodeURIComponent(status)}`)
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
          <span
            onClick={(e) => handleStatusClick(e, 'Ready')}
            class="inline-flex items-center px-2 py-1 rounded text-xs font-medium bg-green-200 text-green-800 hover:bg-green-300 cursor-pointer transition-colors"
          >
            {stats.running} running
          </span>
        )}
        {stats.failing > 0 && (
          <span
            onClick={(e) => handleStatusClick(e, 'Failed')}
            class="inline-flex items-center px-2 py-1 rounded text-xs font-medium bg-red-100 text-red-800 hover:bg-red-200 cursor-pointer transition-colors"
          >
            {stats.failing} failing
          </span>
        )}
        {stats.suspended > 0 && (
          <span
            onClick={(e) => handleStatusClick(e, 'Suspended')}
            class="inline-flex items-center px-2 py-1 rounded text-xs font-medium bg-yellow-100 text-yellow-800 hover:bg-yellow-200 cursor-pointer transition-colors"
          >
            {stats.suspended} suspended
          </span>
        )}
      </div>
    </button>
  )
}

/**
 * ReconcilerGroup - Groups reconciler cards under a category heading
 *
 * @param {Object} props
 * @param {string} props.title - Group title (e.g., "Appliers", "Sources")
 * @param {Array} props.reconcilers - Array of reconcilers in this group
 */
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

/**
 * ReconcilersPanel component - Displays Flux Custom Resource Definitions (CRDs) grouped by type
 *
 * @param {Object} props
 * @param {Array} props.reconcilers - Array of Flux reconciler CRDs with statistics
 *
 * Features:
 * - Groups reconcilers by API type (Appliers, Sources, Notifications, Image Automation)
 * - Displays resource counts (running, failing, suspended) for each CRD
 * - Clickable cards navigate to search view with kind filter
 * - Clickable status badges navigate to search view with kind + status filters
 * - Shows total resource count and failing count
 * - Collapsible grid view
 */
export function ReconcilersPanel({ reconcilers }) {
  const isExpanded = useSignal(true)

  const totalResources = reconcilers.reduce((sum, r) => {
    return sum + (r.stats.failing || 0) + (r.stats.running || 0) + (r.stats.suspended || 0)
  }, 0)

  const totalFailing = reconcilers.reduce((sum, r) => sum + (r.stats.failing || 0), 0)

  // Group reconcilers by API type and sort by kind
  const appliers = reconcilers
    .filter(r =>
      (r.apiVersion.startsWith('fluxcd.controlplane') ||
      r.apiVersion.startsWith('kustomize') ||
      r.apiVersion.startsWith('helm')) &&
      r.kind !== 'ResourceSetInputProvider'
    )
    .sort((a, b) => {
      // FluxInstance always comes first
      if (a.kind === 'FluxInstance') return -1
      if (b.kind === 'FluxInstance') return 1
      // For other kinds, sort in reverse alphabetical order
      return b.kind.localeCompare(a.kind)
    })

  const sources = reconcilers
    .filter(r => r.apiVersion.startsWith('source') || r.kind === 'ResourceSetInputProvider')
    .sort((a, b) => a.kind.localeCompare(b.kind))

  const notifications = reconcilers
    .filter(r => r.apiVersion.startsWith('notification'))
    .sort((a, b) => a.kind.localeCompare(b.kind))

  const imageAutomation = reconcilers
    .filter(r => r.apiVersion.startsWith('image'))
    .sort((a, b) => a.kind.localeCompare(b.kind))

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
