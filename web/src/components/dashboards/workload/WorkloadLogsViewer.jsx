// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useState, useEffect, useCallback, useMemo, useRef } from 'preact/hooks'
import { fetchWithMock } from '../../../utils/fetch'

// Selectable limits for the number of log lines to fetch from the backend.
const LINE_LIMITS = [100, 500, 1000, 5000]

// Default number of log lines requested from the backend.
const DEFAULT_TAIL_LINES = 100

// Follow polling interval in milliseconds.
const FOLLOW_INTERVAL = 5000

// Matches the leading RFC3339 timestamp the API prepends to each log line.
const TIMESTAMP_PREFIX = /^\S+\s+/

/**
 * WorkloadLogsViewer - Modal that displays the logs of a pod container.
 *
 * Fetches logs from GET /api/v1/workload/logs for the selected container and
 * renders each log entry on its own separated row. Supports following (live
 * polling), filtering by substring, choosing the number of lines, viewing the
 * previous container instance, toggling timestamps, and a fullscreen mode.
 *
 * @param {Object} props
 * @param {string} props.namespace - Pod namespace
 * @param {string} props.name - Pod name
 * @param {Array<{name: string, isInit: boolean}>} props.containers - Pod containers
 * @param {Function} props.onClose - Callback to close the viewer
 */
export function WorkloadLogsViewer({ namespace, name, containers = [], onClose }) {
  // Default to the first regular container, falling back to the first entry.
  const defaultContainer = (containers.find(c => !c.isInit) || containers[0])?.name || ''

  const [container, setContainer] = useState(defaultContainer)
  const [previous, setPrevious] = useState(false)
  const [tailLines, setTailLines] = useState(DEFAULT_TAIL_LINES)
  const [follow, setFollow] = useState(false)
  const [filter, setFilter] = useState('')
  const [showTimestamps, setShowTimestamps] = useState(false)
  const [fullScreen, setFullScreen] = useState(false)
  const [logs, setLogs] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState(null)
  const [copied, setCopied] = useState(false)

  const bodyRef = useRef(null)

  const fetchLogs = useCallback(async () => {
    if (!container) return
    setLoading(true)
    setError(null)
    try {
      const params = new URLSearchParams({
        namespace,
        name,
        container,
        tailLines: String(tailLines),
        previous: String(previous)
      })
      const resp = await fetchWithMock({
        endpoint: `/api/v1/workload/logs?${params.toString()}`,
        mockPath: '../mock/workload',
        mockExport: 'getMockWorkloadLogs'
      })
      setLogs(resp?.logs || '')
    } catch (err) {
      setError(err.message)
      setLogs('')
    } finally {
      setLoading(false)
    }
  }, [namespace, name, container, tailLines, previous])

  // Fetch logs whenever the container, line count or previous toggle changes.
  useEffect(() => {
    fetchLogs()
  }, [fetchLogs])

  // Poll for new logs while following.
  useEffect(() => {
    if (!follow) return
    const id = setInterval(() => { fetchLogs() }, FOLLOW_INTERVAL)
    return () => clearInterval(id)
  }, [follow, fetchLogs])

  // Close the viewer on Escape.
  useEffect(() => {
    const onKeyDown = (e) => {
      if (e.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [onClose])

  // Split the raw log payload into entries, apply the substring filter and
  // optionally strip the leading timestamp for display.
  const logLines = useMemo(() => {
    let lines = logs.split('\n').filter(line => line.length > 0)
    const needle = filter.trim().toLowerCase()
    if (needle) {
      lines = lines.filter(line => line.toLowerCase().includes(needle))
    }
    return showTimestamps ? lines : lines.map(line => line.replace(TIMESTAMP_PREFIX, ''))
  }, [logs, filter, showTimestamps])

  // Keep the most recent entry in view after each update.
  useEffect(() => {
    if (!bodyRef.current) return
    bodyRef.current.scrollTop = bodyRef.current.scrollHeight
  }, [logLines])

  const handleCopy = useCallback(async () => {
    try {
      await window.navigator.clipboard.writeText(logs)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } catch {
      // Clipboard access can fail in insecure contexts; ignore silently.
    }
  }, [logs])

  const fieldClass = 'text-xs px-2 py-1 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-flux-blue'
  // appearance-none removes the native caret; the @tailwindcss/forms plugin
  // also injects a chevron via background-image which is suppressed with an
  // inline style on each select (see noFormsChevron). pr-6 leaves room for the
  // single custom chevron rendered alongside.
  const selectClass = `${fieldClass} appearance-none pr-6`
  const noFormsChevron = { backgroundImage: 'none' }
  const inputClass = `${fieldClass} placeholder-gray-400 dark:placeholder-gray-500`
  const iconToggleClass = 'inline-flex items-center justify-center p-1.5 rounded-md border transition-colors focus:outline-none focus:ring-2 focus:ring-flux-blue'
  const inactiveClass = 'border-gray-300 text-gray-600 hover:bg-gray-100 dark:border-gray-600 dark:text-gray-300 dark:hover:bg-gray-700'
  const activeClass = 'border-flux-blue text-flux-blue bg-blue-50 dark:bg-blue-900/30'
  const actionClass = 'inline-flex items-center p-1.5 rounded-md text-gray-400 hover:text-flux-blue dark:text-gray-500 dark:hover:text-flux-blue disabled:cursor-not-allowed'

  return (
    <div
      class={`fixed inset-0 z-50 flex items-center justify-center bg-black/50 ${fullScreen ? 'p-0' : 'p-4'}`}
      onClick={onClose}
      data-testid="logs-viewer-overlay"
    >
      <div
        class={`bg-white dark:bg-gray-900 shadow-xl flex flex-col border border-gray-200 dark:border-gray-700 ${
          fullScreen ? 'w-full h-full max-w-full max-h-full rounded-none' : 'w-full max-w-4xl h-[85vh] rounded-lg'
        }`}
        onClick={(e) => e.stopPropagation()}
        role="dialog"
        aria-modal="true"
        aria-label={`Logs for pod ${name}`}
        data-testid="logs-viewer"
      >
        {/* Header */}
        <div class="flex items-center justify-between gap-2 px-4 py-3 border-b border-gray-200 dark:border-gray-700">
          <div class="min-w-0">
            <h2 class="text-sm font-semibold text-gray-900 dark:text-white truncate">Logs</h2>
            <p class="text-xs text-gray-500 dark:text-gray-400 truncate">{namespace}/{name}</p>
          </div>
          <button
            onClick={onClose}
            class="inline-flex items-center p-1 rounded text-gray-400 hover:text-gray-700 dark:text-gray-500 dark:hover:text-gray-200 flex-shrink-0"
            aria-label="Close logs viewer"
            data-testid="logs-close-button"
          >
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        {/* Toolbar */}
        <div class="flex items-center flex-wrap gap-2 px-4 py-2 border-b border-gray-200 dark:border-gray-700">
          {/* Follow logs */}
          <button
            onClick={() => setFollow(v => !v)}
            class={`${iconToggleClass} ${follow ? activeClass : inactiveClass}`}
            data-testid="logs-follow-toggle"
            aria-pressed={follow}
            aria-label="Follow logs"
            title="Follow logs"
          >
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 13l-7 7-7-7m14-6l-7 7-7-7" />
            </svg>
          </button>

          {/* Container select (always shown) */}
          <div class="relative inline-flex">
            <select
              value={container}
              onChange={(e) => setContainer(e.target.value)}
              class={selectClass}
              style={noFormsChevron}
              data-testid="logs-container-select"
              aria-label="Container"
            >
              {containers.map((c) => (
                <option key={c.name} value={c.name}>{c.isInit ? `init:${c.name}` : c.name}</option>
              ))}
            </select>
            <svg class="pointer-events-none absolute right-1.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-gray-500 dark:text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
            </svg>
          </div>

          {/* Contains filter */}
          <input
            type="text"
            value={filter}
            onInput={(e) => setFilter(e.target.value)}
            placeholder="contains…"
            class={`${inputClass} w-28 sm:w-44`}
            data-testid="logs-filter-input"
            aria-label="Filter log lines containing text"
          />

          {/* Previous container instance */}
          <button
            onClick={() => setPrevious(v => !v)}
            class={`${iconToggleClass} ${previous ? activeClass : inactiveClass}`}
            data-testid="logs-previous-toggle"
            aria-pressed={previous}
            aria-label="Previous container instance"
            title="Show logs from the previous container instance"
          >
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 19l-7-7 7-7m8 14l-7-7 7-7" />
            </svg>
          </button>

          {/* Lines select */}
          <div class="relative inline-flex">
            <select
              value={tailLines}
              onChange={(e) => setTailLines(Number(e.target.value))}
              class={selectClass}
              style={noFormsChevron}
              data-testid="logs-lines-select"
              aria-label="Number of lines"
            >
              {LINE_LIMITS.map((n) => (
                <option key={n} value={n}>{n} ln</option>
              ))}
            </select>
            <svg class="pointer-events-none absolute right-1.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-gray-500 dark:text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
            </svg>
          </div>

          {/* Timestamps show/hide */}
          <button
            onClick={() => setShowTimestamps(v => !v)}
            class={`${iconToggleClass} ${showTimestamps ? activeClass : inactiveClass}`}
            data-testid="logs-timestamps-toggle"
            aria-pressed={showTimestamps}
            aria-label="Show or hide timestamps"
            title="Show or hide timestamps"
          >
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
          </button>

          {/* Actions */}
          <div class="flex items-center gap-1 ml-auto">
            <button
              onClick={fetchLogs}
              disabled={loading}
              class={actionClass}
              title="Refresh logs"
              aria-label="Refresh logs"
              data-testid="logs-refresh-button"
            >
              <svg class={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
              </svg>
            </button>
            <button
              onClick={handleCopy}
              class={actionClass}
              title="Copy logs to clipboard"
              aria-label="Copy logs to clipboard"
              data-testid="logs-copy-button"
            >
              {copied ? (
                <svg class="w-4 h-4 text-green-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
                </svg>
              ) : (
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
                </svg>
              )}
            </button>
            <button
              onClick={() => setFullScreen(v => !v)}
              class={actionClass}
              aria-pressed={fullScreen}
              title={fullScreen ? 'Exit fullscreen' : 'Fullscreen'}
              aria-label={fullScreen ? 'Exit fullscreen' : 'Fullscreen'}
              data-testid="logs-fullscreen-toggle"
            >
              {fullScreen ? (
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 9V4.5M9 9H4.5M9 9 3.75 3.75M15 9h4.5M15 9V4.5M15 9l5.25-5.25M9 15v4.5M9 15H4.5M9 15l-5.25 5.25M15 15h4.5M15 15v4.5m0-4.5 5.25 5.25" />
                </svg>
              ) : (
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3.75 3.75v4.5m0-4.5h4.5m-4.5 0L9 9M3.75 20.25v-4.5m0 4.5h4.5m-4.5 0L9 15M20.25 3.75h-4.5m4.5 0v4.5m0-4.5L15 9m5.25 11.25h-4.5m4.5 0v-4.5m0 4.5L15 15" />
                </svg>
              )}
            </button>
          </div>
        </div>

        {/* Body */}
        <div ref={bodyRef} class="flex-1 overflow-auto bg-gray-50 dark:bg-gray-950" data-testid="logs-body">
          {error ? (
            <div class="m-3 p-2 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded text-xs text-red-800 dark:text-red-200" data-testid="logs-error">
              {error}
            </div>
          ) : loading && !logs ? (
            <p class="p-3 text-xs text-gray-500 dark:text-gray-400" data-testid="logs-loading">Loading logs...</p>
          ) : logLines.length > 0 ? (
            <div class="divide-y divide-gray-200 dark:divide-gray-800" data-testid="logs-content">
              {logLines.map((line, i) => (
                <div
                  key={i}
                  class="px-3 py-1 text-sm font-mono text-gray-800 dark:text-gray-200 whitespace-pre-wrap break-all hover:bg-gray-100 dark:hover:bg-gray-900"
                  data-testid="logs-line"
                >
                  {line}
                </div>
              ))}
            </div>
          ) : (
            <p class="p-3 text-xs text-gray-500 dark:text-gray-400" data-testid="logs-empty">
              {filter.trim() ? 'No matching log entries' : 'No logs available'}
            </p>
          )}
        </div>
      </div>
    </div>
  )
}
