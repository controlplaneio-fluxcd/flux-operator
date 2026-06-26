// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useEffect, useMemo, useState } from 'preact/hooks'
import { fetchWithMock } from '../../../utils/fetch'
import { usePrismTheme, EditableYamlBlock, YamlBlock } from '../common/yaml'
import { DashboardPanel, TabButton } from '../common/panel'

/**
 * ObjectPage displays and edits an arbitrary Kubernetes object by apiVersion, kind, namespace and name.
 */
export function ObjectPage({ apiVersion, kind, namespace, name }) {
  const resolvedNamespace = namespace === '_' ? '' : namespace
  const decodedApiVersion = decodeURIComponent(apiVersion)
  const decodedKind = decodeURIComponent(kind)
  const decodedNamespace = decodeURIComponent(resolvedNamespace)
  const decodedName = decodeURIComponent(name)
  const [objectData, setObjectData] = useState(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)
  const [tab, setTab] = useState('spec')

  usePrismTheme()

  const loadObject = async () => {
    setLoading(true)
    setError(null)
    const params = new URLSearchParams({
      apiVersion: decodedApiVersion,
      kind: decodedKind,
      name: decodedName
    })
    if (decodedNamespace) {
      params.set('namespace', decodedNamespace)
    }
    try {
      const data = await fetchWithMock({
        endpoint: `/api/v1/object?${params.toString()}`,
        mockPath: '../mock/resource',
        mockExport: 'getMockResource'
      })
      setObjectData(data)
    } catch (err) {
      setError(err.message)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadObject()
  }, [decodedApiVersion, decodedKind, decodedNamespace, decodedName])

  const editableYaml = useMemo(() => {
    if (!objectData) return null
    const editable = { ...objectData }
    delete editable.status
    return editable
  }, [objectData])

  const statusYaml = useMemo(() => {
    if (!objectData?.status) return null
    return {
      apiVersion: objectData.apiVersion,
      kind: objectData.kind,
      metadata: {
        name: objectData.metadata?.name,
        namespace: objectData.metadata?.namespace
      },
      status: objectData.status
    }
  }, [objectData])

  return (
    <main class="flex-1 max-w-7xl mx-auto w-full px-4 sm:px-6 lg:px-8 py-6">
      <DashboardPanel
        title={`${decodedKind}/${decodedName}`}
        subtitle={decodedNamespace ? `${decodedApiVersion} · ${decodedNamespace}` : decodedApiVersion}
      >
        {loading && (
          <p class="text-sm text-gray-600 dark:text-gray-400">Loading object...</p>
        )}
        {error && (
          <div class="p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-md">
            <p class="text-sm text-red-800 dark:text-red-200">Failed to load object: {error}</p>
          </div>
        )}
        {!loading && !error && objectData && (
          <>
            <div class="border-b border-gray-200 dark:border-gray-700 mb-4">
              <nav class="flex space-x-4" aria-label="Tabs">
                <TabButton active={tab === 'spec'} onClick={() => setTab('spec')}>Specification</TabButton>
                {statusYaml && <TabButton active={tab === 'status'} onClick={() => setTab('status')}>Status</TabButton>}
              </nav>
            </div>
            {tab === 'spec' && <EditableYamlBlock data={editableYaml} onSaved={loadObject} />}
            {tab === 'status' && <YamlBlock data={statusYaml} />}
          </>
        )}
      </DashboardPanel>
    </main>
  )
}
