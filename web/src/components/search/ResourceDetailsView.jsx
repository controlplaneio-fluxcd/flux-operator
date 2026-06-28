// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useState, useMemo, useEffect, useRef } from 'preact/hooks'
import { fetchWithMock } from '../../utils/fetch'
import { formatTimestamp } from '../../utils/time'
import { usePrismTheme, YamlBlock } from '../dashboards/common/yaml'
import { isKindWithInventory, getControllerName, isFluxInventoryItem, isWorkloadInventoryItem } from '../../utils/constants'
import { getDashboardUrl } from '../../utils/routing'
import { cleanStatus } from '../../utils/status'
import { getReconcileInterval, getReconcileTimeout, getReconcilerSummary } from '../../utils/reconciler'
import { FluxOperatorIcon } from '../layout/Icons'
import { TabbedPanel, Field, ResourceLink, StatusBadge } from './detailPanel'

/**
 * Helper to group inventory items by apiVersion
 * @param {Array} inventory - Array of inventory items
 * @returns {Object} Structure: { apiVersion: [items] }
 */
function groupInventoryByApiVersion(inventory) {
  // Ensure inventory is an array
  if (!inventory || !Array.isArray(inventory) || inventory.length === 0) return {}

  const grouped = {}

  inventory.forEach(item => {
    const apiVersion = item.apiVersion || 'unknown'
    if (!grouped[apiVersion]) {
      grouped[apiVersion] = []
    }
    grouped[apiVersion].push(item)
  })

  // Sort items within each apiVersion by kind, namespace, then name
  Object.keys(grouped).forEach(apiVersion => {
    grouped[apiVersion].sort((a, b) => {
      const kindCompare = a.kind.localeCompare(b.kind)
      if (kindCompare !== 0) return kindCompare

      const nsA = a.namespace || ''
      const nsB = b.namespace || ''
      if (nsA !== nsB) return nsA.localeCompare(nsB)

      return a.name.localeCompare(b.name)
    })
  })

  return grouped
}

/**
 * InventoryItem - Displays a single inventory item
 *
 * Features:
 * - Displays kind, namespace (if present), and name
 * - If kind matches a Flux resource kind, renders as clickable link to resources page
 * - Includes navigation icon for clickable items
 */
function InventoryItem({ item }) {
  const isFluxResource = isFluxInventoryItem(item)
  const isWorkload = !isFluxResource && isWorkloadInventoryItem(item)

  // Build the dashboard URL, routing workloads and Flux resources accordingly
  const ns = item.namespace || ''
  const dashboardUrl = getDashboardUrl(item.kind, ns, item.name)

  if (isFluxResource || isWorkload) {
    return (
      <div class="py-1 px-2 text-xs break-all">
        <a
          href={dashboardUrl}
          class="text-left hover:opacity-80 transition-opacity focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-flux-blue rounded inline-block group"
        >
          <span class="text-gray-600 dark:text-gray-400">{item.kind}/</span>{item.namespace && <span class="text-gray-500 dark:text-gray-400">{item.namespace}/</span>}<span class="text-gray-900 dark:text-gray-100 group-hover:text-flux-blue dark:group-hover:text-blue-400">{item.name}</span><svg class="w-3 h-3 text-gray-400 group-hover:text-flux-blue dark:group-hover:text-blue-400 transition-colors ml-1 inline-block align-middle" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" /></svg>
        </a>
      </div>
    )
  }

  // Non-Flux, non-workload resource - render as plain text
  return (
    <div class="py-1 px-2 text-xs break-all">
      <span class="text-gray-900 dark:text-gray-100">
        <span class="text-gray-600 dark:text-gray-400">{item.kind}/</span>
        {item.namespace && (
          <span class="text-gray-500 dark:text-gray-400">{item.namespace}/</span>
        )}
        {item.name}
      </span>
    </div>
  )
}

/**
 * InventoryGroupByApiVersion - Groups inventory items under an API version heading
 */
function InventoryGroupByApiVersion({ apiVersion, items }) {
  return (
    <div class="mb-3">
      <div class="flex items-center gap-2 py-1 flex-wrap">
        <span class="text-xs font-semibold text-gray-800 dark:text-gray-200 break-all">
          {apiVersion}
        </span>
        <span class="inline-flex items-center px-2 py-0.5 rounded text-xs bg-gray-200 dark:bg-gray-700 text-gray-700 dark:text-gray-300">
          {items.length}
        </span>
      </div>
      <div class="ml-0 sm:ml-2">
        {items.map((item, idx) => (
          <InventoryItem key={idx} item={item} />
        ))}
      </div>
    </div>
  )
}

/**
 * ResourceDetailsView - Displays detailed view of a Flux resource with tabbed interface
 *
 * @param {Object} props
 * @param {string} props.kind - Resource kind
 * @param {string} props.name - Resource name
 * @param {string} props.namespace - Resource namespace
 * @param {boolean} props.isExpanded - Whether the view is expanded
 * @param {string} [props.status] - Display status for the Overview badge, used as a
 *   fallback when the resource has no reconcilerRef status yet
 * @param {Function} [props.onReady] - Called once the data fetch settles (success or
 *   error), so the parent row can swap its spinner for the revealed panel
 * @param {Function} [props.onData] - Called with the fetched resource on success, so
 *   the parent row can refresh its collapsed summary (status/message/lastReconciled)
 *   from the server-computed reconcilerRef and not contradict the open panel
 *
 * Features:
 * - Lazy loads complete resource data on expand
 * - Tabbed interface with up to five sections (in order):
 *   1. Overview: Reconciler status, controller, interval, owner and last action (default)
 *   2. Inventory: Grouped list of managed resources (if present)
 *   3. Source: Details about the resource's source (if present)
 *   4. Specification: Complete resource definition as syntax-highlighted YAML
 *   5. Status: YAML display of apiVersion, kind, metadata, and status (without inventory)
 * - Renders the tab chrome through the shared compact TabbedPanel (mobile segmented
 *   control, desktop vertical right-rail merging into a cohesive panel)
 * - Dynamically switches Prism theme (light/dark) based on app theme
 * - Caches data to avoid redundant fetches
 * - Handles loading and error states
 */
export function ResourceDetailsView({ kind, name, namespace, isExpanded, status, onReady, onData }) {
  const [resourceData, setResourceData] = useState(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState(null)
  const [activeTab, setActiveTab] = useState('overview')
  const fetchingRef = useRef(false)

  // Load Prism theme based on current app theme
  usePrismTheme()

  // Reset state when resource identity changes
  useEffect(() => {
    setResourceData(null)
    setError(null)
    setActiveTab('overview')
  }, [kind, name, namespace])

  // Fetch resource details when expanded
  useEffect(() => {
    if (!isExpanded || resourceData || fetchingRef.current) return

    let cancelled = false
    fetchingRef.current = true

    const fetchResourceDetails = async () => {
      if (!cancelled) {
        setLoading(true)
        setError(null)
      }

      const params = new URLSearchParams({ kind, name, namespace })

      try {
        const data = await fetchWithMock({
          endpoint: `/api/v1/resource?${params.toString()}`,
          mockPath: '../mock/resource',
          mockExport: 'getMockResource'
        })
        // Overview is the default tab and is always available (set on mount and on
        // identity change), so the active tab needs no adjustment once data lands.
        if (!cancelled) {
          setResourceData(data)
          // Hand the fresh, server-computed summary back to the row so the collapsed
          // list entry can't contradict the panel that just opened.
          onData && onData(data)
        }
      } catch (err) {
        console.error('Failed to fetch resource details:', err)
        if (!cancelled) setError(err.message)
      } finally {
        fetchingRef.current = false
        if (!cancelled) {
          setLoading(false)
          // Signal the parent row that the fetch settled (success or error) so it
          // can swap its spinner for the revealed panel.
          onReady && onReady()
        }
      }
    }

    fetchResourceDetails()
    return () => { cancelled = true }
  }, [isExpanded, kind, name, namespace, resourceData])

  // Build resource definition object (memoized)
  const definitionData = useMemo(() => {
    if (!resourceData) return null
    return {
      apiVersion: resourceData.apiVersion,
      kind: resourceData.kind,
      metadata: resourceData.metadata,
      spec: resourceData.spec
    }
  }, [resourceData])

  // Build status object without inventory and sourceRef (memoized)
  const statusData = useMemo(() => {
    if (!resourceData) return null

    return {
      apiVersion: resourceData.apiVersion,
      kind: resourceData.kind,
      metadata: {
        name: resourceData.metadata.name,
        namespace: resourceData.metadata.namespace
      },
      status: cleanStatus(resourceData.status)
    }
  }, [resourceData])

  // Group inventory by apiVersion (memoized)
  const groupedInventory = useMemo(
    () => groupInventoryByApiVersion(resourceData?.status?.inventory || []),
    [resourceData]
  )

  // Sort apiVersions
  const sortedApiVersions = useMemo(() => {
    const versions = Object.keys(groupedInventory)
    return versions.sort((a, b) => {
      if (a === 'apiextensions.k8s.io/v1' && b !== 'apiextensions.k8s.io/v1') return -1
      if (b === 'apiextensions.k8s.io/v1' && a !== 'apiextensions.k8s.io/v1') return 1
      if (a === 'v1' && b !== 'v1') return -1
      if (b === 'v1' && a !== 'v1') return 1
      return a.localeCompare(b)
    })
  }, [groupedInventory])

  // Check if inventory tab should be shown
  const shouldShowInventoryTab = useMemo(() => {
    if (!resourceData) return false
    const hasInventory = resourceData.status?.inventory && resourceData.status.inventory.length > 0
    return isKindWithInventory(resourceData.kind) || hasInventory
  }, [resourceData])

  // Get inventory count
  const inventoryCount = resourceData?.status?.inventory?.length || 0

  // Overview derivations (mirror the Resource dashboard Reconciler panel). The
  // shared summary falls back to the first status condition; rStatus additionally
  // folds in the row's listed `status` prop before the detail data lands.
  const { ref: reconcilerRef, message: rMessage, lastReconciled: rLast } = getReconcilerSummary(resourceData)
  const rStatus = reconcilerRef?.status || status || 'Unknown'
  const interval = getReconcileInterval(resourceData)
  const timeout = getReconcileTimeout(resourceData)
  const managedBy = reconcilerRef?.managedBy

  // Build the ordered tab list, including Inventory/Source only when they apply.
  const tabs = useMemo(() => {
    const list = [{ id: 'overview', label: 'Overview' }]
    if (shouldShowInventoryTab) list.push({ id: 'inventory', label: 'Inventory' })
    if (resourceData?.status?.sourceRef) list.push({ id: 'source', label: 'Source' })
    list.push({ id: 'specification', label: 'Spec' })
    list.push({ id: 'status', label: 'Status' })
    return list
  }, [shouldShowInventoryTab, resourceData])

  if (!isExpanded) return null

  return (
    <div class="mt-3 space-y-4">
      {/* Loading State */}
      {loading && (
        <div class="flex items-center justify-center p-4">
          <FluxOperatorIcon className="animate-spin h-6 w-6 text-flux-blue" />
          <span class="ml-2 text-sm text-gray-600 dark:text-gray-400">
            Loading details...
          </span>
        </div>
      )}

      {/* Error State */}
      {error && (
        <div class="p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-md">
          <p class="text-sm text-red-800 dark:text-red-200">
            Failed to load details: {error}
          </p>
        </div>
      )}

      {/* Tabs + Content */}
      {!loading && !error && resourceData && (
        <TabbedPanel tabs={tabs} active={activeTab} onSelect={setActiveTab}>
          {/* Overview Tab — metadata fields left, last action + message right */}
          {activeTab === 'overview' && (
            <div class="grid grid-cols-1 md:grid-cols-2 gap-x-8 gap-y-4 text-xs">
              {/* Left: reconciler metadata, evenly stacked */}
              <div class="space-y-2.5 self-start">
                <Field label="Resource"><ResourceLink kind={kind} namespace={namespace} name={name} /></Field>
                <Field label={kind}><StatusBadge status={rStatus} /></Field>
                <Field label="Reconciled by">{getControllerName(kind)}</Field>
                <Field label="Reconcile every">{interval ? (timeout ? `${interval} (timeout ${timeout})` : interval) : null}</Field>
                <Field label="Managed by">
                  {managedBy ? (() => {
                    const [rk, rn, rnm] = managedBy.split('/')
                    return <ResourceLink kind={rk} namespace={rn} name={rnm} />
                  })() : null}
                </Field>
              </div>

              {/* Right: last action + reconciler message */}
              {rMessage && (
                <div class="space-y-1.5 self-start md:border-l md:border-gray-200 md:dark:border-gray-700 md:pl-8">
                  {rLast && (
                    <div class="text-gray-500 dark:text-gray-400">Last action <span class="text-gray-900 dark:text-gray-100">{formatTimestamp(rLast)}</span></div>
                  )}
                  <pre class="whitespace-pre-wrap break-words font-sans leading-relaxed text-gray-700 dark:text-gray-300">{rMessage}</pre>
                </div>
              )}
            </div>
          )}

          {/* Inventory Tab */}
          {activeTab === 'inventory' && shouldShowInventoryTab && (
            <div class="space-y-3">
              {inventoryCount > 0 ? (
                sortedApiVersions.map(apiVersion => (
                  <InventoryGroupByApiVersion
                    key={apiVersion}
                    apiVersion={apiVersion}
                    items={groupedInventory[apiVersion]}
                  />
                ))
              ) : (
                <div class="py-4 px-2 text-sm text-gray-600 dark:text-gray-400">
                  Empty inventory, no managed objects
                </div>
              )}
            </div>
          )}

          {/* Source Tab — same "Resource: ns/name link" + "<kind>: status"
              layout and link style as the Overview tab. */}
          {activeTab === 'source' && resourceData.status?.sourceRef && (
            <div class="space-y-2.5 text-xs">
              <Field label="Resource">
                <ResourceLink
                  kind={resourceData.status.sourceRef.kind}
                  namespace={resourceData.status.sourceRef.namespace}
                  name={resourceData.status.sourceRef.name}
                />
              </Field>
              <Field label={resourceData.status.sourceRef.kind}><StatusBadge status={resourceData.status.sourceRef.status} /></Field>
              <Field label="URL">{resourceData.status.sourceRef.url}</Field>
              <Field label="Origin URL">{resourceData.status.sourceRef.originURL}</Field>
              <Field label="Origin Revision">{resourceData.status.sourceRef.originRevision}</Field>
              <Field label="Fetch result">{resourceData.status.sourceRef.message}</Field>
            </div>
          )}

          {/* Specification Tab */}
          {activeTab === 'specification' && (
            <YamlBlock data={definitionData} nested />
          )}

          {/* Status Tab */}
          {activeTab === 'status' && (
            <YamlBlock data={statusData} nested />
          )}
        </TabbedPanel>
      )}
    </div>
  )
}
