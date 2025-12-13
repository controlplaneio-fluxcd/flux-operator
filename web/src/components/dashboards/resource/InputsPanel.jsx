// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useState } from 'preact/hooks'
import { fetchWithMock } from '../../../utils/fetch'
import { formatTimestamp } from '../../../utils/time'
import { DashboardPanel, TabButton } from '../common/panel'
import { YamlBlock } from '../common/yaml'
import { FluxOperatorIcon } from '../../common/icons'

/**
 * Get badge class for provider type
 */
function getProviderBadgeClass(type) {
  return type === 'Static'
    ? 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400'
    : 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400'
}

/**
 * InputsPanel - Displays inputs information for ResourceSet resources
 */
export function InputsPanel({ resourceData, namespace }) {
  // Tab state
  const [activeTab, setActiveTab] = useState('overview')

  // Values tab state (on-demand loading)
  const [valuesLoaded, setValuesLoaded] = useState(false)
  const [valuesLoading, setValuesLoading] = useState(false)
  const [providerInputs, setProviderInputs] = useState({})

  // Extract data
  const spec = resourceData?.spec
  const status = resourceData?.status

  // Input strategy
  const inputStrategy = spec?.inputStrategy?.name || 'Flatten'

  // Inline inputs
  const inlineInputs = spec?.inputs || []
  const inlineInputsCount = inlineInputs.length

  // External input providers
  const inputProviderRefs = status?.inputProviderRefs || []
  const externalInputsCount = inputProviderRefs.length

  // Get unique provider types
  const providerTypes = [...new Set(inputProviderRefs.map(ref => ref.type).filter(Boolean))]

  // Handle tab change - load values on demand
  const handleTabChange = async (tab) => {
    setActiveTab(tab)

    if (tab === 'values' && !valuesLoaded && inputProviderRefs.length > 0) {
      setValuesLoading(true)

      try {
        const fetchPromises = inputProviderRefs.map(async (ref) => {
          const params = new URLSearchParams({
            kind: 'ResourceSetInputProvider',
            name: ref.name,
            namespace: ref.namespace || namespace
          })

          try {
            const providerData = await fetchWithMock({
              endpoint: `/api/v1/resource?${params.toString()}`,
              mockPath: '../mock/resource',
              mockExport: 'getMockResource'
            })
            return {
              key: `${ref.namespace || namespace}/${ref.name}`,
              name: ref.name,
              namespace: ref.namespace || namespace,
              type: ref.type,
              url: providerData?.spec?.url,
              exportedInputs: providerData?.status?.exportedInputs || [],
              lastReconciled: providerData?.status?.reconcilerRef?.lastReconciled
            }
          } catch (err) {
            console.error(`Failed to fetch provider ${ref.name}:`, err)
            return {
              key: `${ref.namespace || namespace}/${ref.name}`,
              name: ref.name,
              namespace: ref.namespace || namespace,
              type: ref.type,
              exportedInputs: [],
              error: err.message
            }
          }
        })

        const results = await Promise.all(fetchPromises)
        const newProviderInputs = {}
        results.forEach(result => {
          newProviderInputs[result.key] = result
        })
        setProviderInputs(newProviderInputs)
      } finally {
        setValuesLoading(false)
        setValuesLoaded(true)
      }
    }
  }

  // Check if there are any inputs
  const hasInputs = inlineInputsCount > 0 || externalInputsCount > 0

  return (
    <DashboardPanel title="Inputs" id="inputs-panel">
      {/* Tab Navigation */}
      <div class="border-b border-gray-200 dark:border-gray-700 mb-4">
        <nav class="flex space-x-4">
          <TabButton active={activeTab === 'overview'} onClick={() => handleTabChange('overview')}>
            <span class="sm:hidden">Info</span>
            <span class="hidden sm:inline">Overview</span>
          </TabButton>
          <TabButton active={activeTab === 'values'} onClick={() => handleTabChange('values')}>
            Values
          </TabButton>
        </nav>
      </div>

      {/* Tab Content */}
      {activeTab === 'overview' && (
        <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
          {/* Left column: Strategy and Providers */}
          <div class="space-y-4">
            {/* Strategy */}
            <div class="text-sm">
              <span class="text-gray-500 dark:text-gray-400">Strategy</span>
              <span class="ml-1 inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400">
                {inputStrategy}
              </span>
            </div>

            {/* Providers */}
            <div class="text-sm">
              <span class="text-gray-500 dark:text-gray-400">Providers</span>
              {providerTypes.length > 0 ? (
                <span class="ml-1">
                  {providerTypes.map((type, index) => (
                    <span
                      key={type}
                      class={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${getProviderBadgeClass(type)} ${index > 0 ? 'ml-1' : ''}`}
                    >
                      {type}
                    </span>
                  ))}
                </span>
              ) : (
                <span class="ml-1 inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-gray-100 text-gray-800 dark:bg-gray-900/30 dark:text-gray-400">
                  None
                </span>
              )}
            </div>
          </div>

          {/* Right column: Counts */}
          <div class="space-y-4 border-gray-200 dark:border-gray-700 border-t pt-4 md:border-t-0 md:border-l md:pt-0 md:pl-6">
            {/* Inline inputs */}
            <div class="text-sm">
              <span class="text-gray-500 dark:text-gray-400">Inline inputs</span>
              <span class="ml-1 text-gray-900 dark:text-white">{inlineInputsCount}</span>
            </div>

            {/* External inputs */}
            <div class="text-sm">
              <span class="text-gray-500 dark:text-gray-400">External inputs</span>
              <span class="ml-1 text-gray-900 dark:text-white">{externalInputsCount}</span>
            </div>
          </div>
        </div>
      )}

      {/* Values Tab */}
      {activeTab === 'values' && (
        <div class="space-y-4">
          {valuesLoading ? (
            <div class="flex items-center justify-center p-8">
              <FluxOperatorIcon className="animate-spin h-8 w-8 text-flux-blue" />
              <span class="ml-3 text-gray-600 dark:text-gray-400">Loading inputs...</span>
            </div>
          ) : hasInputs ? (
            <>
              {/* Inline Inputs */}
              {inlineInputsCount > 0 && (
                <div class="card p-0 overflow-hidden">
                  <div class="px-4 py-2 bg-gray-50 dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700">
                    <span class="text-sm font-medium text-gray-700 dark:text-gray-300 inline-flex items-center gap-1">
                      <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4" />
                      </svg>
                      Inline inputs
                    </span>
                  </div>
                  <div class="p-4">
                    <YamlBlock data={inlineInputs} />
                  </div>
                </div>
              )}

              {/* External Inputs from Providers */}
              {inputProviderRefs.map((ref) => {
                const key = `${ref.namespace || namespace}/${ref.name}`
                const provider = providerInputs[key]

                if (!provider) return null

                return (
                  <div key={key} class="card p-0 overflow-hidden">
                    <div class="px-4 py-2 bg-gray-50 dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700">
                      {/* Header: Name, badge, timestamp (stacked on mobile, inline on sm+) */}
                      <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-1 mb-1">
                        <div class="flex flex-col sm:flex-row sm:items-center gap-1 sm:gap-2">
                          <a
                            href={`/resource/ResourceSetInputProvider/${provider.namespace}/${provider.name}`}
                            class="text-sm font-medium text-flux-blue hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300 inline-flex items-center gap-1"
                          >
                            <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
                            </svg>
                            {provider.name}
                          </a>
                          {provider.type && (
                            <span class={`self-start sm:self-auto inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${getProviderBadgeClass(provider.type)}`}>
                              {provider.type}
                            </span>
                          )}
                        </div>
                        {provider.lastReconciled && (
                          <span class="text-xs text-gray-500 dark:text-gray-400">
                            {formatTimestamp(provider.lastReconciled)}
                          </span>
                        )}
                      </div>
                      {/* Line 2: URL if available */}
                      {provider.url && (
                        <div class="text-xs text-gray-500 dark:text-gray-400 break-all">
                          {provider.url}
                        </div>
                      )}
                    </div>
                    <div class="p-4">
                      {provider.error ? (
                        <div class="text-sm text-red-600 dark:text-red-400">
                          Failed to load: {provider.error}
                        </div>
                      ) : provider.exportedInputs.length > 0 ? (
                        <YamlBlock data={provider.exportedInputs} />
                      ) : (
                        <div class="text-sm text-gray-500 dark:text-gray-400">
                          No exported inputs
                        </div>
                      )}
                    </div>
                  </div>
                )
              })}
            </>
          ) : (
            <div class="text-sm text-gray-500 dark:text-gray-400 text-center py-4">
              No inputs available
            </div>
          )}
        </div>
      )}
    </DashboardPanel>
  )
}
