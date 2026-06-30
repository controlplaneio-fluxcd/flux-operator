// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useState, useMemo, useEffect, useRef } from 'preact/hooks'
import { fetchWithMock } from '../../../utils/fetch'
import { usePrismTheme, YamlBlock } from '../common/yaml'

/**
 * ObjectDetailsView - Read-only inline detail panel for any inventory object.
 *
 * Lazily fetches the object's sanitized manifest from POST /api/v1/inventory/objects
 * (a single-item list) on first expand, scoped to the caller's RBAC, and renders the
 * whole manifest as a single syntax-highlighted YAML block (no tabs). The manifest
 * is backend-sanitized (managed fields stripped, Secret values masked). Tall
 * manifests are capped and scroll inside the box so one expanded row never takes
 * over the page. Handles loading, error, and not-found (pruned/forbidden) states.
 *
 * @param {Object} props
 * @param {string} props.apiVersion - Object apiVersion (group/version or "v1")
 * @param {string} props.kind - Object kind
 * @param {string} [props.namespace] - Object namespace (empty for cluster-scoped)
 * @param {string} props.name - Object name
 * @param {boolean} props.isExpanded - Whether the view is expanded
 * @param {Function} [props.onReady] - Called once the fetch settles (success or
 *   error), so the parent row can swap its spinner for the revealed panel
 * @param {*} [props.refreshKey] - Changes once per parent poll; while the panel is
 *   open it triggers a background refetch that keeps the last-good content visible
 *   (no spinner) and swaps on success, or shows the not-found message if the object
 *   was deleted meanwhile
 */
export function ObjectDetailsView({ apiVersion, kind, namespace, name, isExpanded, onReady, refreshKey }) {
  const [result, setResult] = useState(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState(null)
  const fetchingRef = useRef(false)

  // Load Prism theme based on current app theme.
  usePrismTheme()

  // Reset state when the object identity changes.
  useEffect(() => {
    setResult(null)
    setError(null)
  }, [apiVersion, kind, name, namespace])

  // Fetch this object's sanitized manifest (single-item list) and return the result
  // item, or null. Shared by the initial expand fetch and the poll refetch.
  const loadObject = async () => {
    const data = await fetchWithMock({
      endpoint: '/api/v1/inventory/objects',
      mockPath: '../mock/inventory',
      mockExport: 'getMockInventoryObjects',
      method: 'POST',
      body: { objects: [{ apiVersion, kind, namespace, name }] }
    })
    return (data?.objects || [])[0] || null
  }

  // Fetch the object on expand, flipping the loading spinner and signaling onReady.
  useEffect(() => {
    if (!isExpanded || result || fetchingRef.current) return

    let cancelled = false
    fetchingRef.current = true

    const fetchObject = async () => {
      if (!cancelled) {
        setLoading(true)
        setError(null)
      }
      try {
        const item = await loadObject()
        if (!cancelled) {
          setResult(item)
        }
      } catch (err) {
        console.error('Failed to fetch object details:', err)
        if (!cancelled) setError(err.message)
      } finally {
        fetchingRef.current = false
        if (!cancelled) {
          setLoading(false)
          // Signal the parent row once the fetch settles so it can reveal the panel.
          // Skipped when cancelled (row collapsed mid-fetch and unmounted us).
          if (onReady) onReady()
        }
      }
    }

    fetchObject()
    return () => { cancelled = true }
  }, [isExpanded, apiVersion, kind, name, namespace, result])

  // Background refetch on parent poll: when refreshKey changes while the panel is
  // open and already loaded (the `!result` guard also skips the mount run), refetch
  // without flipping the loading spinner — keep the current content until the new
  // manifest arrives, then swap; a deleted object (NotFound) lets the not-found
  // message take over, and a transient refresh error keeps the last-good content.
  useEffect(() => {
    if (!isExpanded || !result || fetchingRef.current) return

    let cancelled = false
    fetchingRef.current = true

    const refetch = async () => {
      try {
        const item = await loadObject()
        if (!cancelled) {
          setResult(item)
        }
      } catch (err) {
        console.error('Failed to refresh object details:', err)
      } finally {
        fetchingRef.current = false
      }
    }

    refetch()
    return () => { cancelled = true }
  }, [refreshKey])

  const obj = result?.object || null

  // Render keys in the conventional manifest order (apiVersion, kind, metadata,
  // then the rest). The backend marshals via a Go map, so keys arrive sorted
  // alphabetically — without this, kind would sit after data/spec.
  const manifest = useMemo(() => {
    if (!obj) return null
    const ordered = {}
    for (const key of ['apiVersion', 'kind', 'metadata']) {
      if (key in obj) ordered[key] = obj[key]
    }
    for (const key of Object.keys(obj)) {
      if (!(key in ordered)) ordered[key] = obj[key]
    }
    return ordered
  }, [obj])

  if (!isExpanded) return null

  // The object was pruned or is not visible to the user (NotFound), or simply
  // carries no manifest (e.g. Forbidden) — show the not-found/forbidden message.
  let notFoundMessage = null
  if (result) {
    if (result.error === 'Forbidden') notFoundMessage = 'You do not have permission to view this object.'
    else if (result.error === 'NotFound') notFoundMessage = 'Object no longer exists in the cluster.'
    else if (!obj && result.error) notFoundMessage = `Unable to load object: ${result.error}`
    else if (!obj) notFoundMessage = 'Object no longer exists in the cluster.'
  }

  return (
    <div class="mt-3">
      {/* Loading State */}
      {loading && (
        <div class="py-3 text-xs text-gray-500 dark:text-gray-400">
          Loading details…
        </div>
      )}

      {/* Error State */}
      {error && (
        <div class="py-3 text-xs text-red-600 dark:text-red-400">
          Failed to load details: {error}
        </div>
      )}

      {/* Not Found / Forbidden State */}
      {!loading && !error && notFoundMessage && (
        <div class="py-3 text-xs text-gray-500 dark:text-gray-400">
          {notFoundMessage}
        </div>
      )}

      {/* Full sanitized manifest as YAML. */}
      {!loading && !error && obj && !notFoundMessage && (
        <div data-testid="object-yaml" class="max-h-[60vh] overflow-y-auto rounded-md border border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-900 px-3 py-2">
          <YamlBlock data={manifest} nested />
        </div>
      )}
    </div>
  )
}
