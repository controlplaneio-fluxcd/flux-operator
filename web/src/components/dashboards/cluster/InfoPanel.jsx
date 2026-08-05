// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useSignal } from '@preact/signals'
import { updateInfo } from '../../../utils/version'
import { formatCores, formatBytes, percentOf, percentText } from '../../../utils/metrics'
import { ResourceMetric, sumResourceUsage } from '../common/ResourceMetric'

/**
 * InfoPanel component - Displays cluster and operator version information
 *
 * @param {Object} props
 * @param {Object} props.cluster - Cluster information (id, name)
 * @param {Object} props.distribution - Distribution information (version, type)
 * @param {Object} props.operator - Operator information (version, type)
 * @param {Array} props.components - Array of controller components (for status calculation)
 * @param {Array} props.metrics - Array of controller metrics (for resource usage)
 */
export function InfoPanel({ cluster, distribution, operator, components, metrics }) {
  const isExpanded = useSignal(true)

  const k8sVersion = cluster?.serverVersion === '' ? 'Unknown' : (cluster?.serverVersion ?? 'Unknown')
  const platform = cluster?.platform === '' ? 'Unknown' : (cluster?.platform ?? 'Unknown')
  const nodes = cluster?.nodes ?? 0
  const nodesText = nodes === 1 ? '1 node' : `${nodes} nodes`

  const resources = sumResourceUsage(metrics)

  return (
    <div class="card p-0">
      <button
        onClick={() => isExpanded.value = !isExpanded.value}
        class="w-full px-6 py-4 border-b border-gray-200 dark:border-gray-700 text-left hover:bg-gray-50 dark:hover:bg-gray-700/30 transition-colors"
      >
        <div class="flex items-center justify-between">
          <div>
            <h3 class="text-base sm:text-lg font-semibold text-gray-900 dark:text-white">Cluster Info</h3>
            <p class="text-sm text-gray-600 dark:text-gray-400 mt-1">Kubernetes {k8sVersion} · {nodesText}</p>
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
          <div class="flex flex-col lg:flex-row lg:gap-8">
            {/* Left side: Version info grid */}
            <dl class="grid grid-cols-2 gap-x-6 gap-y-2 lg:flex-1">
              <div class="flex items-baseline space-x-2">
                <dt class="text-xs sm:text-sm text-gray-500 dark:text-gray-400">
                  <span class="sm:hidden">Operator:</span>
                  <span class="hidden sm:inline">Flux Operator:</span>
                </dt>
                <dd class="text-xs sm:text-sm font-semibold text-gray-900 dark:text-white flex items-center gap-1.5">
                  {operator?.version === '' ? 'Unknown' : (operator?.version ?? 'Unknown')}
                  {updateInfo.value?.isOutdated && (
                    <a
                      href="https://github.com/controlplaneio-fluxcd/flux-operator/releases"
                      target="_blank"
                      rel="noopener noreferrer"
                      title={`Update available: ${updateInfo.value.latest}`}
                      class="text-amber-500 hover:text-amber-600 dark:text-amber-400 dark:hover:text-amber-300"
                      onClick={(e) => e.stopPropagation()}
                    >
                      <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 10l7-7m0 0l7 7m-7-7v18"/>
                      </svg>
                    </a>
                  )}
                </dd>
              </div>
              <div class="flex items-baseline space-x-2">
                <dt class="text-xs sm:text-sm text-gray-500 dark:text-gray-400">
                  <span class="sm:hidden">Flux Distro:</span>
                  <span class="hidden sm:inline">Flux Distribution:</span>
                </dt>
                <dd class="text-xs sm:text-sm font-semibold text-gray-900 dark:text-white">{distribution?.version === '' ? 'Unknown' : (distribution?.version ?? 'Unknown')}</dd>
              </div>
              <div class="flex items-baseline space-x-2">
                <dt class="text-xs sm:text-sm text-gray-500 dark:text-gray-400">Platform:</dt>
                <dd class="text-xs sm:text-sm font-semibold text-gray-900 dark:text-white">{platform}</dd>
              </div>
              <div class="flex items-baseline space-x-2">
                <dt class="text-xs sm:text-sm text-gray-500 dark:text-gray-400">Controller Pods:</dt>
                <dd class="text-xs sm:text-sm font-semibold text-gray-900 dark:text-white">{components?.length ?? 0}</dd>
              </div>
            </dl>

            {/* Right side: Metrics */}
            {resources && (
              <div class="space-y-3 mt-4 pt-4 border-t border-gray-200 dark:border-gray-700 lg:flex-1 lg:mt-0 lg:pt-0 lg:border-t-0 lg:border-l lg:pl-8">
                <ResourceMetric
                  label="Flux CPU Usage"
                  value={formatCores(resources.cpu)}
                  percentLabel={percentText(resources.cpu, resources.cpuRequests, resources.cpuLimits)}
                  barPercent={percentOf(resources.cpu, resources.cpuLimits)}
                />
                <ResourceMetric
                  label="Flux Memory Usage"
                  value={formatBytes(resources.memory)}
                  percentLabel={percentText(resources.memory, resources.memoryRequests, resources.memoryLimits)}
                  barPercent={percentOf(resources.memory, resources.memoryLimits)}
                />
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  )
}
