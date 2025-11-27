// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useLocation } from 'preact-iso'
import { reportUpdatedAt } from '../../../app'
import { formatTime } from '../../../utils/time'
import {
  selectedResourceStatus,
  selectedResourceKind,
  selectedResourceName,
  selectedResourceNamespace
} from '../../search/ResourceList'

/**
 * OverallStatusPanel component - Displays overall health status of the Flux cluster
 *
 * @param {Object} props
 * @param {Object} props.report - FluxReport spec containing cluster status information
 *
 * Shows aggregate status metrics:
 * - Controller components (ready/failed count)
 * - Flux reconcilers (total/failing count)
 * - Cluster sync status
 * - Last update timestamp
 */
export function OverallStatusPanel({ report }) {
  // Calculate counters
  const totalComponents = report.components?.length ?? 0
  const failedComponents = report.components?.filter(c => !c.ready).length ?? 0
  const totalReconcilers = report.reconcilers?.length ?? 0
  const failingReconcilers = report.reconcilers?.reduce((sum, r) => sum + (r.stats.failing || 0), 0) ?? 0
  const allReconcilersFailing = totalReconcilers > 0 &&
    report.reconcilers?.every(r => (r.stats.failing || 0) > 0 && (r.stats.running || 0) === 0)
  const failedClusterSync = report.sync && report.sync.ready !== true ? 1 : 0
  const maintenanceMode = report.sync && report.sync.status && report.sync.status.startsWith('Suspended')
  const totalFailures = failedComponents + failingReconcilers

  const getStatusInfo = () => {
    // System Initializing - distribution not yet available
    if (!report.distribution || !report.distribution.version) {
      return {
        status: 'initializing',
        color: 'text-gray-600 dark:text-gray-400',
        bgColor: 'bg-gray-50',
        borderColor: 'border-gray-400',
        title: 'System Initializing',
        message: 'Waiting for the Flux instance rollout to complete'
      }
    }

    // Maintenance mode takes priority
    if (maintenanceMode) {
      return {
        status: 'maintenance',
        color: 'text-blue-600 dark:text-blue-400',
        bgColor: 'bg-blue-50',
        borderColor: 'border-blue-500',
        title: 'Under Maintenance',
        message: 'Cluster reconciliation is currently suspended'
      }
    }

    // Major Outage
    if (failedComponents === totalComponents || allReconcilersFailing) {
      return {
        status: 'major-outage',
        color: 'text-danger',
        bgColor: 'bg-red-50',
        borderColor: 'border-danger',
        title: 'Major Outage',
        message: `Critical system failure detected`
      }
    }

    // Partial Outage
    if ((failedComponents > 0 && failedComponents < totalComponents) || failedClusterSync === 1) {
      return {
        status: 'partial-outage',
        color: 'text-orange-600 dark:text-orange-400',
        bgColor: 'bg-orange-50',
        borderColor: 'border-orange-500',
        title: 'Partial Outage',
        message: `${totalFailures} failure${totalFailures !== 1 ? 's' : ''} detected${failedClusterSync ? ', cluster sync failed' : ''}`
      }
    }

    // Degraded Performance
    if (failedComponents === 0 && failedClusterSync === 0 && failingReconcilers > 0) {
      return {
        status: 'degraded',
        color: 'text-warning',
        bgColor: 'bg-yellow-50',
        borderColor: 'border-warning',
        title: 'Degraded Performance',
        message: `${failingReconcilers} reconciler${failingReconcilers !== 1 ? 's' : ''} failing`
      }
    }

    // Operational
    return {
      status: 'operational',
      color: 'text-success',
      bgColor: 'bg-green-50',
      borderColor: 'border-success',
      title: 'All Systems Operational',
      message: 'Cluster in sync with desired state'
    }
  }

  const statusInfo = getStatusInfo()

  const location = useLocation()

  // Handler to navigate to ResourceList with Failed filter
  const handleViewFailures = () => {
    // Clear all filters first
    selectedResourceKind.value = ''
    selectedResourceName.value = ''
    selectedResourceNamespace.value = ''
    // Set status filter to Failed
    selectedResourceStatus.value = 'Failed'
    // Navigate to resources page with status filter
    location.route('/resources?status=Failed')
  }

  // Check if status has failures (clickable states)
  const hasFailures = ['degraded', 'partial-outage', 'major-outage'].includes(statusInfo.status)

  // Wrapper element - button if clickable, div otherwise
  const WrapperElement = hasFailures ? 'button' : 'div'
  const wrapperProps = hasFailures ? {
    onClick: handleViewFailures,
    class: `card ${statusInfo.bgColor} dark:bg-opacity-20 border-2 ${statusInfo.borderColor} w-full text-left cursor-pointer hover:shadow-lg transition-shadow focus:outline-none focus:ring-2 focus:ring-flux-blue focus:ring-offset-2`
  } : {
    class: `card ${statusInfo.bgColor} dark:bg-opacity-20 border-2 ${statusInfo.borderColor}`
  }

  return (
    <WrapperElement {...wrapperProps}>
      <div class="flex items-center space-x-4">
        <div class="flex-shrink-0">
          <div class={`w-16 h-16 rounded-full ${statusInfo.bgColor} dark:bg-opacity-30 flex items-center justify-center`}>
            {statusInfo.status === 'operational' && (
              <svg class={`w-10 h-10 ${statusInfo.color}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
              </svg>
            )}
            {statusInfo.status === 'degraded' && (
              <svg class={`w-10 h-10 ${statusInfo.color}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
              </svg>
            )}
            {statusInfo.status === 'partial-outage' && (
              <svg class={`w-10 h-10 ${statusInfo.color}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
            )}
            {statusInfo.status === 'major-outage' && (
              <svg class={`w-10 h-10 ${statusInfo.color}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
              </svg>
            )}
            {statusInfo.status === 'maintenance' && (
              <svg class={`w-10 h-10 ${statusInfo.color}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 9v6m4-6v6m7-3a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
            )}
            {statusInfo.status === 'initializing' && (
              <svg class={`w-10 h-10 ${statusInfo.color} animate-spin`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
              </svg>
            )}
          </div>
        </div>
        <div class="flex-grow">
          <h2 class={`text-2xl font-bold ${statusInfo.color}`}>{statusInfo.title}</h2>
          <div class="flex items-center gap-2 text-gray-700 dark:text-gray-300 mt-1">
            <p class="hidden md:block">{statusInfo.message}</p>
            {hasFailures && (
              <svg class="hidden md:block w-4 h-4 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
              </svg>
            )}
          </div>
        </div>
        <div class="hidden md:block text-right">
          <div class="text-sm text-gray-600 dark:text-gray-400">Last Updated</div>
          <div class="text-lg font-semibold text-gray-900 dark:text-white">{formatTime(reportUpdatedAt.value)}</div>
        </div>
      </div>
    </WrapperElement>
  )
}
