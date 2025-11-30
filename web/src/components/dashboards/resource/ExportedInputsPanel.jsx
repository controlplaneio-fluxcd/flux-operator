// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useState } from 'preact/hooks'
import { DashboardPanel, TabButton } from '../common/panel'
import { YamlBlock } from '../common/yaml'

/**
 * Format camelCase to sentence case with spaces
 * e.g., "includeBranch" -> "Include branch"
 */
function formatFilterName(name) {
  return name
    .replace(/([a-z])([A-Z])/g, '$1 $2')
    .toLowerCase()
    .replace(/^./, str => str.toUpperCase())
}

/**
 * Get filter fields that have values
 * Returns array of { name, value } objects
 */
function getFilterFields(filter) {
  if (!filter) return []

  const filterTypes = ['semver', 'includeBranch', 'includeTag', 'excludeBranch', 'excludeTag']
  const fields = []

  for (const filterType of filterTypes) {
    if (filter[filterType]) {
      fields.push({
        name: formatFilterName(filterType),
        value: filter[filterType]
      })
    }
  }

  return fields
}

/**
 * ExportedInputsPanel - Displays exported inputs information for ResourceSetInputProvider resources
 */
export function ExportedInputsPanel({ resourceData }) {
  // Tab state
  const [activeTab, setActiveTab] = useState('overview')

  // Extract data
  const spec = resourceData?.spec
  const exportedInputs = resourceData?.status?.exportedInputs
  const lastReconciled = resourceData?.status?.reconcilerRef?.lastReconciled

  // Source info
  const sourceType = spec?.type
  const sourceUrl = spec?.url
  const filterFields = getFilterFields(spec?.filter)
  const sourceLabels = spec?.filter?.labels
  const skipLabels = spec?.skip?.labels

  // Check if there are exported inputs
  const hasExportedInputs = exportedInputs && exportedInputs.length > 0
  const totalInputs = exportedInputs?.length || 0
  const maxInputs = spec?.filter?.limit || 100

  // Format fetched at timestamp
  const fetchedAt = lastReconciled
    ? new Date(lastReconciled).toLocaleString().replace(',', '')
    : '-'

  return (
    <DashboardPanel title="Exported Inputs" id="exported-inputs-panel">
      {/* Tab Navigation */}
      <div class="border-b border-gray-200 dark:border-gray-700 mb-4">
        <nav class="flex space-x-4">
          <TabButton active={activeTab === 'overview'} onClick={() => setActiveTab('overview')}>
            <span class="sm:hidden">Info</span>
            <span class="hidden sm:inline">Overview</span>
          </TabButton>
          <TabButton active={activeTab === 'values'} onClick={() => setActiveTab('values')}>
            Values
          </TabButton>
        </nav>
      </div>

      {/* Tab Content */}
      {activeTab === 'overview' && (
        <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
          {/* Left column: Source information */}
          <div class="space-y-4">
            {/* Type */}
            {sourceType && (
              <div class="text-sm">
                <span class="text-gray-500 dark:text-gray-400">Type</span>
                <span class={`ml-1 inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                  sourceType === 'Static'
                    ? 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400'
                    : 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400'
                }`}>
                  {sourceType}
                </span>
              </div>
            )}

            {/* Source */}
            {sourceUrl && (
              <div class="text-sm">
                <span class="text-gray-500 dark:text-gray-400">Source</span>
                <span class="ml-1 text-gray-900 dark:text-white break-all">{sourceUrl}</span>
              </div>
            )}

            {/* Filter fields */}
            {filterFields.map((field) => (
              <div key={field.name} class="text-sm">
                <span class="text-gray-500 dark:text-gray-400">{field.name}</span>
                <span class="ml-1 text-gray-900 dark:text-white">{field.value}</span>
              </div>
            ))}

            {/* Labels */}
            {sourceLabels && sourceLabels.length > 0 && (
              <div class="text-sm">
                <span class="text-gray-500 dark:text-gray-400">Labels</span>
                <span class="ml-1 text-gray-900 dark:text-white">{sourceLabels.join(', ')}</span>
              </div>
            )}

            {/* Skip */}
            {skipLabels && skipLabels.length > 0 && (
              <div class="text-sm">
                <span class="text-gray-500 dark:text-gray-400">Skip</span>
                <span class="ml-1 text-gray-900 dark:text-white">{skipLabels.join(', ')}</span>
              </div>
            )}
          </div>

          {/* Right column: Stats */}
          <div class="space-y-4 border-gray-200 dark:border-gray-700 border-t pt-4 md:border-t-0 md:border-l md:pt-0 md:pl-6">
            {/* Fetched at */}
            <div class="text-sm">
              <span class="text-gray-500 dark:text-gray-400">Fetched at</span>
              <span class="ml-1 text-gray-900 dark:text-white">{fetchedAt}</span>
            </div>

            {/* Total exported */}
            <div class="text-sm">
              <span class="text-gray-500 dark:text-gray-400">Total exported</span>
              <span class="ml-1 text-gray-900 dark:text-white">{totalInputs} (max {maxInputs})</span>
            </div>
          </div>
        </div>
      )}

      {/* Values Tab */}
      {activeTab === 'values' && (
        <div class="space-y-4">
          {hasExportedInputs ? (
            exportedInputs.map((input, index) => (
              <div key={input.id || index} class="card p-0 overflow-hidden">
                <div class="px-4 py-2 bg-gray-50 dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700">
                  <span class="text-sm font-medium text-gray-700 dark:text-gray-300">#{input.id || index + 1}</span>
                </div>
                <div class="p-4">
                  <YamlBlock data={input} />
                </div>
              </div>
            ))
          ) : (
            <div class="text-sm text-gray-500 dark:text-gray-400 text-center py-4">
              No exported inputs available
            </div>
          )}
        </div>
      )}
    </DashboardPanel>
  )
}
