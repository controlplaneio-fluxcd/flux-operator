// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useSignal } from '@preact/signals'
import { useLocation } from 'preact-iso'
import { selectedResourceKind, selectedResourceName, selectedResourceNamespace, selectedResourceStatus } from '../../search/ResourceList'
import { fluxCRDs } from '../../../utils/constants'

/**
 * ReconcilerCard - Individual card displaying a Flux CRD with resource statistics
 *
 * @param {Object} props
 * @param {Object} props.crd - CRD metadata from constants (kind, apiVersion, docUrl)
 * @param {Object} props.stats - Resource statistics (running, failing, suspended)
 * @param {boolean} props.isInstalled - Whether the CRD is installed in the cluster
 *
 * Features:
 * - Shows CRD kind and API version
 * - Displays total resource count
 * - Shows status badges (running, failing, suspended) with counts
 * - Entire card is clickable to filter by kind in search view
 * - Individual status badges are clickable to filter by kind + status
 * - Color-coded border based on resource health (gray if not installed)
 * - Docs icon link opens documentation in new tab
 */
function ReconcilerCard({ crd, stats, isInstalled }) {
  const location = useLocation()
  const total = (stats.failing || 0) + (stats.running || 0) + (stats.suspended || 0)

  // Determine status color - gray for not installed CRDs
  const getStatusColor = () => {
    if (!isInstalled) return 'border-gray-300 dark:border-gray-600'
    if (stats.failing > 0) return 'border-danger'
    if (stats.suspended > 0) return 'border-warning'
    return 'border-success'
  }

  // Handle card click - navigate to resources page with kind filter
  const handleClick = () => {
    selectedResourceKind.value = crd.kind
    selectedResourceName.value = ''
    selectedResourceNamespace.value = ''
    selectedResourceStatus.value = ''
    location.route(`/resources?kind=${encodeURIComponent(crd.kind)}`)
  }

  // Handle status badge click - navigate to resources page with kind and status filters
  const handleStatusClick = (e, status) => {
    e.stopPropagation()
    selectedResourceKind.value = crd.kind
    selectedResourceName.value = ''
    selectedResourceNamespace.value = ''
    selectedResourceStatus.value = status
    location.route(`/resources?kind=${encodeURIComponent(crd.kind)}&status=${encodeURIComponent(status)}`)
  }

  const cardClass = `card border-l-4 px-4 ${getStatusColor()} hover:shadow-lg transition-all cursor-pointer text-left w-full`

  const cardContent = (
    <>
      <div class="mb-3 flex items-start justify-between">
        <div>
          <h4 class="font-semibold text-gray-900 dark:text-white text-md">{crd.kind}</h4>
          <p class="text-xs text-gray-500 dark:text-gray-400">{crd.apiVersion}</p>
        </div>
        {crd.docUrl && (
          (isInstalled && total > 0) ? (
            <a
              href={crd.docUrl}
              target="_blank"
              rel="noopener noreferrer"
              onClick={(e) => e.stopPropagation()}
              class="p-1 text-blue-500 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300 transition-colors"
              title={`${crd.kind} documentation`}
            >
              <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
              </svg>
            </a>
          ) : (
            <span class="p-1 text-blue-500">
              <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
              </svg>
            </span>
          )
        )}
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
        {!isInstalled && (
          <span class="inline-flex items-center px-2 py-1 rounded text-xs font-medium bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-400">
            not installed
          </span>
        )}
        {isInstalled && total === 0 && (
          <span class="inline-flex items-center px-2 py-1 rounded text-xs font-medium bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-400">
            no resources
          </span>
        )}
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
    </>
  )

  // Not installed or no resources: entire card links to documentation
  if ((!isInstalled || total === 0) && crd.docUrl) {
    return (
      <a
        href={crd.docUrl}
        target="_blank"
        rel="noopener noreferrer"
        class={cardClass}
        title={`${crd.kind} documentation`}
      >
        {cardContent}
      </a>
    )
  }

  // Installed with resources: card navigates to resources page
  return (
    <button onClick={handleClick} class={cardClass}>
      {cardContent}
    </button>
  )
}

/**
 * ReconcilerGroup - Groups reconciler cards under a category heading
 *
 * @param {Object} props
 * @param {string} props.title - Group title (e.g., "Appliers", "Sources")
 * @param {Array} props.groupCrds - Array of CRDs in this group (from constants)
 * @param {Object} props.statsMap - Map of kind to stats object
 * @param {Set} props.installedKinds - Set of kinds that are installed in the cluster
 */
function ReconcilerGroup({ title, groupCrds, statsMap, installedKinds }) {
  if (groupCrds.length === 0) return null

  return (
    <div class="mb-6">
      <h4 class="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-3">{title}</h4>
      <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
        {groupCrds.map(crd => (
          <ReconcilerCard
            key={`${crd.apiVersion}/${crd.kind}`}
            crd={crd}
            stats={statsMap[crd.kind] || { running: 0, failing: 0, suspended: 0 }}
            isInstalled={installedKinds.has(crd.kind)}
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
 * @param {Array} props.reconcilers - Array of Flux reconciler CRDs with statistics from the API
 *
 * Features:
 * - Shows all CRDs from constants in their defined order
 * - Groups reconcilers by API type (Appliers, Sources, Notifications, Image Automation)
 * - Displays resource counts (running, failing, suspended) for each CRD
 * - Clickable cards navigate to search view with kind filter
 * - Clickable status badges navigate to search view with kind + status filters
 * - Shows total resource count and failing count
 * - Collapsible grid view
 */
export function ReconcilersPanel({ reconcilers }) {
  const isExpanded = useSignal(true)

  // Build a map of kind -> stats from the API response
  const statsMap = reconcilers.reduce((map, r) => {
    map[r.kind] = r.stats
    return map
  }, {})

  // Track which CRDs are installed (have data from the API)
  const installedKinds = new Set(reconcilers.map(r => r.kind))

  const totalResources = reconcilers.reduce((sum, r) => {
    return sum + (r.stats.failing || 0) + (r.stats.running || 0) + (r.stats.suspended || 0)
  }, 0)

  const totalFailing = reconcilers.reduce((sum, r) => sum + (r.stats.failing || 0), 0)

  // Group CRDs by their group property (preserves array order within each group)
  const appliers = fluxCRDs.filter(crd => crd.group === 'Appliers')
  const sources = fluxCRDs.filter(crd => crd.group === 'Sources')
  const notifications = fluxCRDs.filter(crd => crd.group === 'Notifications')
  const imageAutomation = fluxCRDs.filter(crd => crd.group === 'Image Automation')

  return (
    <div class="card">
      <button
        onClick={() => isExpanded.value = !isExpanded.value}
        class={`w-full text-left hover:opacity-80 transition-opacity ${isExpanded.value ? 'mb-6' : ''}`}
      >
        <div class="flex items-center justify-between">
          <div>
            <h3 class="text-base sm:text-lg font-semibold text-gray-900 dark:text-white">Flux Reconcilers</h3>
            <div class="flex items-center space-x-4 mt-2">
              <p class="text-sm text-gray-600 dark:text-gray-400">
                {installedKinds.size} CRDs • {totalResources} resources
              </p>
              {totalFailing > 0 && (
                <span class="status-badge status-not-ready text-xs sm:text-sm">
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
          <ReconcilerGroup title="Appliers" groupCrds={appliers} statsMap={statsMap} installedKinds={installedKinds} />
          <ReconcilerGroup title="Sources" groupCrds={sources} statsMap={statsMap} installedKinds={installedKinds} />
          <ReconcilerGroup title="Notifications" groupCrds={notifications} statsMap={statsMap} installedKinds={installedKinds} />
          <ReconcilerGroup title="Image Automation" groupCrds={imageAutomation} statsMap={statsMap} installedKinds={installedKinds} />
        </div>
      )}
    </div>
  )
}
