// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { Fragment } from 'preact'
import { useState, useEffect, useCallback, useMemo, useRef } from 'preact/hooks'
import { fetchWithMock } from '../../../utils/fetch'
import { downloadBlob } from '../../../utils/download'

// Selectable limits for the number of log lines to fetch from the backend.
const LINE_LIMITS = [100, 500, 1000, 5000]

// Default number of log lines requested from the backend.
const DEFAULT_TAIL_LINES = 100

// Follow polling interval in milliseconds.
const FOLLOW_INTERVAL = 5000

// How long the most recent log line stays highlighted after new entries arrive.
const HIGHLIGHT_DURATION = 2500

// Captures the leading RFC3339 timestamp the API prepends to each log line.
// Anchored to a date so lines without a timestamp (e.g. stack-trace
// continuations) are not mangled by splitting off their first token.
const TIMESTAMP_PREFIX = /^(\d{4}-\d{2}-\d{2}T\S+)\s+/

// Shared styling for the toolbar controls. Selects deliberately omit a chevron
// and right padding: those come from the global `select` rule in index.css.
const FIELD_CLASS = 'text-xs py-1 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-flux-blue'
const SELECT_CLASS = `${FIELD_CLASS} pl-2`
const INPUT_CLASS = `${FIELD_CLASS} px-2 placeholder-gray-400 dark:placeholder-gray-500`
// p-1 keeps the icon buttons the same height as the text-xs py-1 selects.
const ICON_TOGGLE_CLASS = 'inline-flex items-center justify-center p-1 rounded-md border transition-colors focus:outline-none focus:ring-2 focus:ring-flux-blue'
const INACTIVE_CLASS = 'border-gray-300 text-gray-600 hover:bg-gray-100 dark:border-gray-600 dark:text-gray-300 dark:hover:bg-gray-700'
const ACTIVE_CLASS = 'border-flux-blue text-flux-blue bg-blue-50 dark:bg-blue-900/30'
const ACTION_CLASS = 'inline-flex items-center p-1 rounded-md text-gray-400 hover:text-flux-blue dark:text-gray-500 dark:hover:text-flux-blue disabled:cursor-not-allowed'

/**
 * ToggleButton - square icon button in the logs toolbar that reflects an
 * on/off state via aria-pressed and active styling.
 *
 * @param {Object} props
 * @param {boolean} props.active - Whether the toggle is on
 * @param {Function} props.onClick - Click handler
 * @param {string} props.label - Accessible label (also the default tooltip)
 * @param {string} [props.title] - Optional tooltip text, defaults to label
 * @param {string} props.testid - data-testid value
 * @param {any} props.children - The button icon
 */
function ToggleButton({ active, onClick, label, title, testid, children }) {
  return (
    <button
      onClick={onClick}
      class={`${ICON_TOGGLE_CLASS} ${active ? ACTIVE_CLASS : INACTIVE_CLASS}`}
      data-testid={testid}
      aria-pressed={active}
      aria-label={label}
      title={title || label}
    >
      {children}
    </button>
  )
}

/**
 * WorkloadLogsViewer - Modal that displays the logs of a pod container.
 *
 * Fetches logs from GET /api/v1/workload/logs for the selected container and
 * renders each log entry on its own row, with its timestamp shown as a pill on
 * the row separator. Supports following (live polling), filtering by substring,
 * choosing the number of lines, selecting a container (restarted containers
 * also expose a "(previous)" entry for the prior instance's logs), downloading
 * the logs as a <pod>.log file, and a fullscreen mode.
 *
 * @param {Object} props
 * @param {string} props.namespace - Pod namespace
 * @param {string} props.name - Pod name
 * @param {Array<{name: string, isInit: boolean, restartCount?: number}>} props.containers - Pod containers
 * @param {Function} props.onClose - Callback to close the viewer
 */
export function WorkloadLogsViewer({ namespace, name, containers = [], onClose }) {
  // Default to the first regular container, falling back to the first entry.
  const defaultContainer = (containers.find(c => !c.isInit) || containers[0])?.name || ''

  const [container, setContainer] = useState(defaultContainer)
  const [previous, setPrevious] = useState(false)
  const [tailLines, setTailLines] = useState(DEFAULT_TAIL_LINES)
  const [follow, setFollow] = useState(true)
  const [filter, setFilter] = useState('')
  const [fullScreen, setFullScreen] = useState(false)
  const [logs, setLogs] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState(null)
  const [flashLatest, setFlashLatest] = useState(false)

  const bodyRef = useRef(null)
  const prevLogsRef = useRef('')

  // One option per container, plus a "(previous)" option for containers that
  // have restarted (a previous instance only has logs after a restart). The
  // value encodes both the container and whether to read the previous instance.
  const containerOptions = useMemo(() => {
    const opts = []
    for (const c of containers) {
      const base = c.isInit ? `init:${c.name}` : c.name
      opts.push({ key: `${c.name}::false`, label: base, container: c.name, previous: false })
      if ((c.restartCount || 0) > 0) {
        opts.push({ key: `${c.name}::true`, label: `${base} (previous)`, container: c.name, previous: true })
      }
    }
    return opts
  }, [containers])

  // Fetch the logs. Background follow-polls pass { silent: true } so they don't
  // toggle the loading spinner, which would flicker on every poll; only the
  // initial load and user-driven changes (container, line count) show it.
  const fetchLogs = useCallback(async ({ silent = false } = {}) => {
    if (!container) return
    if (!silent) setLoading(true)
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
      setError(null)
    } catch (err) {
      setError(err.message)
      setLogs('')
    } finally {
      if (!silent) setLoading(false)
    }
  }, [namespace, name, container, tailLines, previous])

  // Fetch logs whenever the container, line count or previous toggle changes.
  useEffect(() => {
    fetchLogs()
  }, [fetchLogs])

  // Poll for new logs while following, silently so the spinner doesn't flicker.
  useEffect(() => {
    if (!follow) return
    const id = setInterval(() => { fetchLogs({ silent: true }) }, FOLLOW_INTERVAL)
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

  // Lock the background page scroll while the viewer is open so scrolling the
  // logs does not bleed through to the dashboard behind the modal.
  useEffect(() => {
    const previousOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => { document.body.style.overflow = previousOverflow }
  }, [])

  // Briefly highlight the most recent line whenever a new batch of logs arrives
  // (e.g. while following), skipping the very first load so the highlight only
  // signals freshly appended entries.
  useEffect(() => {
    const prev = prevLogsRef.current
    prevLogsRef.current = logs
    if (prev === '' || logs === '' || prev === logs) return
    setFlashLatest(true)
    const id = setTimeout(() => setFlashLatest(false), HIGHLIGHT_DURATION)
    return () => clearTimeout(id)
  }, [logs])

  // Split the raw payload into entries once, separating the leading timestamp
  // from the message so the timestamp can be shown as a separator pill while
  // the message occupies the row. Only re-runs when the payload changes, not on
  // every filter keystroke.
  const baseLines = useMemo(() => {
    return logs.split('\n').filter(line => line.length > 0).map(line => {
      const m = line.match(TIMESTAMP_PREFIX)
      return m ? { ts: m[1], text: line.slice(m[0].length) } : { ts: '', text: line }
    })
  }, [logs])

  // Apply the substring filter on the message text; cheap pass over the
  // already-split entries. A leading "!" negates the match, keeping only lines
  // that do NOT contain the text (e.g. "!debug" hides every line mentioning debug).
  const logLines = useMemo(() => {
    const raw = filter.trim()
    if (!raw) return baseLines
    const negate = raw.startsWith('!')
    const needle = (negate ? raw.slice(1) : raw).trim().toLowerCase()
    if (!needle) return baseLines
    return baseLines.filter(entry => entry.text.toLowerCase().includes(needle) !== negate)
  }, [baseLines, filter])

  // Keep the most recent entry in view after each update.
  useEffect(() => {
    if (!bodyRef.current) return
    bodyRef.current.scrollTop = bodyRef.current.scrollHeight
  }, [logLines])

  // Download the current logs as a <pod>.log text file.
  const handleDownload = useCallback(() => {
    downloadBlob(new window.Blob([logs], { type: 'text/plain' }), `${name}.log`)
  }, [logs, name])

  return (
    <div
      class={`fixed inset-0 z-50 flex justify-center bg-black/50 ${fullScreen ? 'items-center p-0' : 'items-start pt-16 px-4 pb-4'}`}
      onClick={onClose}
      data-testid="logs-viewer-overlay"
    >
      <div
        class={`bg-white dark:bg-gray-900 shadow-xl flex flex-col overflow-hidden border border-gray-200 dark:border-gray-700 ${
          fullScreen ? 'w-full h-full max-w-full max-h-full rounded-none' : 'w-full max-w-7xl h-[calc(100vh-5rem)] rounded-lg'
        }`}
        onClick={(e) => e.stopPropagation()}
        role="dialog"
        aria-modal="true"
        aria-label={`Logs for pod ${name}`}
        data-testid="logs-viewer"
      >
        {/* Header */}
        <div class="flex items-center justify-between gap-2 px-4 py-3 border-b border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800">
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
        <div class="flex items-center flex-wrap gap-2 px-4 py-2 border-b border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800">
          {/* Follow logs */}
          <ToggleButton
            active={follow}
            onClick={() => setFollow(v => !v)}
            label="Follow logs"
            testid="logs-follow-toggle"
          >
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 13l-7 7-7-7m14-6l-7 7-7-7" />
            </svg>
          </ToggleButton>

          {/* Container select (always shown). Containers that have restarted
              also expose a "(previous)" entry for the prior instance's logs.
              Fixed width so a long container name is trimmed instead of pushing
              the rest of the toolbar. */}
          <select
            value={`${container}::${previous}`}
            onChange={(e) => {
              const opt = containerOptions.find(o => o.key === e.target.value)
              if (opt) {
                setContainer(opt.container)
                setPrevious(opt.previous)
              }
            }}
            class={`${SELECT_CLASS} w-28 sm:w-40 truncate`}
            data-testid="logs-container-select"
            aria-label="Container"
            title="Select container (a previous entry reads the prior instance's logs)"
          >
            {containerOptions.map((o) => (
              <option key={o.key} value={o.key}>{o.label}</option>
            ))}
          </select>

          {/* Contains filter */}
          <input
            type="text"
            value={filter}
            onInput={(e) => setFilter(e.target.value)}
            placeholder="contains…"
            class={`${INPUT_CLASS} w-28 sm:w-40`}
            data-testid="logs-filter-input"
            aria-label="Filter log lines containing text"
            title="Keep lines containing this text; prefix with ! to exclude (e.g. !debug)"
          />

          {/* Lines select */}
          <select
            value={tailLines}
            onChange={(e) => setTailLines(Number(e.target.value))}
            class={SELECT_CLASS}
            data-testid="logs-lines-select"
            aria-label="Number of lines"
            title="Number of log lines to fetch"
          >
            {LINE_LIMITS.map((n) => (
              <option key={n} value={n}>{n} ln</option>
            ))}
          </select>

          {/* Actions */}
          <div class="flex items-center gap-1 ml-auto">
            <button
              onClick={handleDownload}
              disabled={!logs}
              class={ACTION_CLASS}
              title="Download logs"
              aria-label="Download logs"
              data-testid="logs-download-button"
            >
              <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 16.5v2.25A2.25 2.25 0 005.25 21h13.5A2.25 2.25 0 0021 18.75V16.5M16.5 12L12 16.5m0 0L7.5 12m4.5 4.5V3" />
              </svg>
            </button>
            <button
              onClick={() => setFullScreen(v => !v)}
              class={ACTION_CLASS}
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
        <div ref={bodyRef} class="flex-1 overflow-auto overscroll-contain bg-white dark:bg-gray-950" data-testid="logs-body">
          {error ? (
            <div class="m-3 p-2 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded text-xs text-red-800 dark:text-red-200" data-testid="logs-error">
              {error}
            </div>
          ) : loading && !logs ? (
            <p class="p-3 text-xs text-gray-500 dark:text-gray-400" data-testid="logs-loading">Loading logs...</p>
          ) : logLines.length > 0 ? (
            <div class="pb-2" data-testid="logs-content">
              {logLines.map((entry, i) => {
                const isLatest = flashLatest && i === logLines.length - 1
                return (
                  <Fragment key={i}>
                    {entry.ts && (
                      <div class="flex items-center gap-2 px-3 pt-2 select-none" data-testid="logs-timestamp" data-latest={isLatest ? 'true' : undefined}>
                        <span
                          class={`text-[10px] font-mono px-1.5 py-0.5 rounded-full transition-colors duration-1000 ${
                            isLatest
                              ? 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300'
                              : 'bg-gray-200 text-gray-600 dark:bg-gray-800 dark:text-gray-400'
                          }`}
                        >
                          {entry.ts}
                        </span>
                        <span class="flex-1 border-t border-gray-200 dark:border-gray-800" />
                      </div>
                    )}
                    <div
                      class="px-3 py-1 text-sm font-mono whitespace-pre-wrap break-all text-gray-800 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-900"
                      data-testid="logs-line"
                    >
                      {entry.text}
                    </div>
                  </Fragment>
                )
              })}
            </div>
          ) : (
            <p class="p-3 text-xs text-gray-500 dark:text-gray-400" data-testid="logs-empty">
              {filter.trim() ? 'No matching log entries' : 'No logs available'}
            </p>
          )}
        </div>

        {/* Footer showing the current line count, with a loader to its left
            while a fetch is in flight. */}
        <div
          class="flex items-center justify-center gap-2 px-4 py-2 border-t border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800"
          data-testid="logs-footer"
        >
          {loading && (
            <div
              class="animate-spin rounded-full h-4 w-4 border-b-2 border-gray-400"
              data-testid="logs-loader"
              aria-label="Loading logs"
            />
          )}
          <span class="text-xs text-gray-500 dark:text-gray-400">{logLines.length} lines</span>
        </div>
      </div>
    </div>
  )
}
