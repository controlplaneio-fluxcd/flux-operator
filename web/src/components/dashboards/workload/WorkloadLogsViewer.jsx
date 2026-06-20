// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { Fragment } from 'preact'
import { useState, useEffect, useCallback, useMemo, useRef } from 'preact/hooks'
import { fetchWithMock } from '../../../utils/fetch'
import { downloadBlob } from '../../../utils/download'
import { LEVELS, LEVEL_META, DEFAULT_LEVEL, detectLevel, stripAnsi } from '../../../utils/logLevel'
import { decorateLine, highlightJson } from '../../../utils/logFormat'
import { useDismiss } from '../../../utils/useDismiss'

// Selectable limits for the number of log lines to fetch from the backend.
const LINE_LIMITS = [100, 500, 1000, 5000]

// Default number of log lines requested from the backend.
const DEFAULT_TAIL_LINES = 100

// Sentinel option key for the "All containers" view, which streams every
// container (init and regular) and interleaves the lines chronologically.
const ALL_CONTAINERS = 'all'

// Sentinel option key for the "All pods" view: every pod streamed and interleaved
// chronologically, each line tagged with its pod. Distinct from ALL_CONTAINERS so
// it can never collide with a pod name.
const ALL_PODS = '__all_pods__'

// Follow polling interval in milliseconds.
const FOLLOW_INTERVAL = 5000

// How long the most recent log line stays highlighted after new entries arrive.
const HIGHLIGHT_DURATION = 2500

// Distance in pixels from the bottom within which the view is still considered
// pinned, so auto-scroll keeps following while allowing for sub-pixel rounding.
const SCROLL_BOTTOM_THRESHOLD = 32

// Upper bound on the number of accumulated log lines kept in the buffer while
// following, so a long-running session does not grow without limit. Matches the
// backend's maxLogTailLines cap.
const MAX_BUFFER_LINES = 5000

// Shown inline when the workload has lost every pod while the viewer is open.
const NO_PODS_MESSAGE = 'The workload has no running pods to stream logs from.'

/**
 * mergeLogs - appends the entries of an incoming payload to the accumulated
 * buffer, dropping whole entries already present and capping to MAX_BUFFER_LINES.
 *
 * Follow polls re-send the last second of entries (the API filters at second
 * granularity). Dedup is by the timestamped HEAD line, unique per entry (nanosecond
 * ts, plus pod tag in the all-pods view): a re-sent head already buffered drops the
 * whole entry — head and its continuation lines. Continuation lines (stack frames,
 * no ts) are never deduped alone, so a new entry keeps its full stack even when a
 * replica emitted an identical frame earlier. Returns prev unchanged when nothing
 * is new, so the state update is a no-op.
 *
 * @param {string} prev - The accumulated log payload
 * @param {string} incoming - The newly fetched log payload
 * @returns {string} The merged payload, newline-terminated
 */
function mergeLogs(prev, incoming) {
  const add = incoming.split('\n').filter(Boolean)
  if (add.length === 0) return prev
  const prevLines = prev.split('\n').filter(Boolean)
  const seen = new Set(prevLines)
  const fresh = []
  let dropping = false
  for (const line of add) {
    if (entryTimestamp(line)) {
      // Entry head: a re-send already buffered drops the entry and its
      // continuation lines until the next head.
      dropping = seen.has(line)
      if (!dropping) { fresh.push(line); seen.add(line) }
    } else if (!dropping) {
      // Continuation line of a kept entry (or a leading orphan): appended verbatim.
      fresh.push(line)
    }
  }
  if (fresh.length === 0) return prev
  const merged = prevLines.concat(fresh)
  const capped = merged.length > MAX_BUFFER_LINES ? merged.slice(merged.length - MAX_BUFFER_LINES) : merged
  return capped.join('\n') + '\n'
}

// Captures the leading RFC3339 timestamp the API prepends to each log line.
// Anchored to a date so lines without a timestamp (e.g. stack-trace
// continuations) are not mangled by splitting off their first token.
const TIMESTAMP_PREFIX = /^(\d{4}-\d{2}-\d{2}T\S+)\s+/

// entryTimestamp returns the RFC3339 timestamp heading a log entry, or null for a
// continuation line (stack frame) that carries none. A head is "<ts> msg" (single
// pod) or "<pod> <ts> msg" (all-pods); the ts must be a real instant, not merely
// date-shaped, so a frame beginning with a word and date-like token isn't a head.
function entryTimestamp(line) {
  const m = line.match(TIMESTAMP_PREFIX)
  if (m && !Number.isNaN(Date.parse(m[1]))) return m[1]
  const sp = line.indexOf(' ')
  if (sp > 0) {
    const am = line.slice(sp + 1).match(TIMESTAMP_PREFIX)
    if (am && !Number.isNaN(Date.parse(am[1]))) return am[1]
  }
  return null
}

// Shared styling for the toolbar controls. Selects deliberately omit a chevron
// and right padding: those come from the global `select` rule in index.css.
const FIELD_CLASS = 'text-xs py-1 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:outline-none focus-visible:ring-2 focus-visible:ring-flux-blue'
const SELECT_CLASS = `${FIELD_CLASS} pl-2`
const INPUT_CLASS = `${FIELD_CLASS} px-2 placeholder-gray-400 dark:placeholder-gray-500`
// p-1 keeps the icon buttons the same height as the text-xs py-1 selects.
const ICON_TOGGLE_CLASS = 'inline-flex items-center justify-center p-1 rounded-md border transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-flux-blue'
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

// Button styled like the toolbar selects, but for a custom dropdown (a native
// <select> can't render the per-level color swatches).
const LEVEL_BUTTON_CLASS = `${FIELD_CLASS} px-2 inline-flex items-center gap-1.5`

// Level filter options: "All levels" plus one entry per level. Constant.
const LEVEL_OPTIONS = [
  { value: 'all', label: 'All levels', swatch: null },
  ...LEVELS.map((l) => ({ value: l, label: LEVEL_META[l].label, swatch: LEVEL_META[l].swatch }))
]

/**
 * LevelMenu - log-level filter. A custom dropdown (not a native select) so each
 * option can show its level color swatch. Selecting a level shows only entries
 * of that exact level; "All levels" disables the filter.
 *
 * @param {Object} props
 * @param {string} props.value - Current level, or 'all'
 * @param {Function} props.onChange - Called with the chosen value
 */
function LevelMenu({ value, onChange }) {
  const [open, setOpen] = useState(false)
  const ref = useRef(null)

  useDismiss(ref, () => setOpen(false), open)

  const current = LEVEL_OPTIONS.find(o => o.value === value) || LEVEL_OPTIONS[0]

  return (
    <div class="relative w-full sm:w-auto" ref={ref}>
      <button
        type="button"
        onClick={() => setOpen(v => !v)}
        class={`${LEVEL_BUTTON_CLASS} w-full justify-between sm:w-auto`}
        data-testid="logs-level-filter"
        aria-label="Log level"
        aria-expanded={open}
        title="Show only logs of this level"
      >
        {current.swatch && <span class={`w-2 h-2 rounded-full ${current.swatch}`} />}
        <span class="flex-1 text-left sm:flex-none">{current.label}</span>
        <svg class="w-3 h-3 ml-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
        </svg>
      </button>
      {open && (
        <div
          class="absolute left-0 mt-1 w-full sm:w-36 bg-white dark:bg-gray-800 rounded-md shadow-lg border border-gray-200 dark:border-gray-700 py-1 z-50"
          data-testid="logs-level-menu"
        >
          {LEVEL_OPTIONS.map((o) => (
            <button
              key={o.value}
              type="button"
              onClick={() => { onChange(o.value); setOpen(false) }}
              class={`w-full px-2 py-1 text-left text-xs inline-flex items-center gap-2 hover:bg-gray-100 dark:hover:bg-gray-700 ${
                o.value === value ? 'font-semibold text-gray-900 dark:text-gray-100' : 'text-gray-700 dark:text-gray-300'
              }`}
              data-testid={`logs-level-option-${o.value}`}
            >
              <span class={`w-2 h-2 rounded-full flex-shrink-0 ${o.swatch || 'border border-gray-400 dark:border-gray-500'}`} />
              {o.label}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}

/**
 * WorkloadLogsViewer - Modal that displays the logs of a workload's pods.
 *
 * Two toolbar dropdowns scope the stream: pods ("All pods" plus each pod) and
 * containers ("All containers" plus each). Switching pods resets the container
 * dropdown to "All containers".
 *
 * Fetches from GET /api/v1/workload/logs, one entry per row. Formatted mode (the
 * default) shows the timestamp as a level-colored pill on the row separator
 * (prefixed with the pod id in "All pods"), pretty-prints JSON, and summarizes
 * per-level counts in the footer; raw mode strips all styling. Supports following
 * (live polling, appending only new entries), formatted/raw toggle, substring and
 * minimum-level filters, line count, and the chronologically-interleaved "All
 * containers" and "All pods" views (the latter tags each line with its pod id).
 * Downloads as a <pod>.log file and has a fullscreen mode. A failed follow poll
 * shows inline at the tail; an initial/post-reset failure (no buffer) shows a
 * full-pane error. If all pods vanish while open (scaled to zero, deleted), the
 * viewer stays open showing buffered logs and an inline notice, rather than closing.
 *
 * Pod selection is internal: given every pod with its containers, the viewer keeps
 * the current pod in local state to build the "All pods" request and resolve the
 * selected pod's containers live.
 *
 * @param {Object} props
 * @param {string} props.kind - Workload kind, shown in the title
 * @param {string} props.namespace - Workload namespace
 * @param {string} props.workloadName - Workload name, shown in the title
 * @param {Array<{name: string, status?: string, containers: Array<{name: string, isInit: boolean, restartCount?: number}>}>} props.pods - Pods of the workload, with their containers
 * @param {string} [props.initialPodName] - Pod to pre-select; defaults to "All pods" when absent
 * @param {Function} props.onClose - Callback to close the viewer
 * @param {Function} [props.onPodChange] - Called with the selected pod name (or null
 *   for the "All pods" view) whenever the selection changes, so the parent can keep
 *   a shareable URL in sync with the pod currently shown.
 */
export function WorkloadLogsViewer({ kind, namespace, workloadName, pods = [], initialPodName, onClose, onPodChange }) {
  // Selected pod: a specific pod name or ALL_PODS. Initialised from initialPodName;
  // the parent remounts the viewer per open, so this re-inits on each open.
  const [podKey, setPodKey] = useState(initialPodName || ALL_PODS)

  // Resolve against the live pod list, falling back to "All pods" if the selected
  // pod has disappeared.
  const effectivePodKey = (podKey === ALL_PODS || pods.some(p => p.name === podKey)) ? podKey : ALL_PODS
  const allPods = effectivePodKey === ALL_PODS
  const selectedPod = allPods ? null : (pods.find(p => p.name === effectivePodKey) || null)

  // No pods left to stream (scaled to zero, deleted): show an inline notice
  // instead of closing.
  const podsGone = pods.length === 0

  // Commit the fallback when the selected pod disappears. effectivePodKey already
  // renders "All pods", but persisting it stops a later pod reappearing with the
  // same name from silently snapping the view back without the user choosing it.
  useEffect(() => {
    if (podKey !== ALL_PODS && !pods.some(p => p.name === podKey)) {
      setPodKey(ALL_PODS)
    }
  }, [podKey, pods])

  // Report the effective pod selection to the parent so it can keep the shareable
  // URL pointed at the pod shown. Guarded by a ref so an unstable onPodChange
  // identity doesn't trigger redundant writes.
  const reportedPodRef = useRef(undefined)
  useEffect(() => {
    const pod = allPods ? null : effectivePodKey
    if (onPodChange && reportedPodRef.current !== pod) {
      reportedPodRef.current = pod
      onPodChange(pod)
    }
  }, [allPods, effectivePodKey, onPodChange])

  // Sorted pod names: streamed in the request (first as name, rest as repeated pod
  // params) and used as the set of valid pod tags. Sorting keeps the request stable
  // when the backend reorders the same pods.
  const podNames = useMemo(() => pods.map(p => p.name).sort(), [pods])

  // Containers for the dropdown and request: the union across pods for "All pods"
  // (templates are uniform, so every container), else the selected pod's live ones.
  const containers = useMemo(() => {
    if (!allPods) return selectedPod?.containers || []
    const seen = new Set()
    const out = []
    for (const p of pods) {
      for (const c of (p.containers || [])) {
        if (!seen.has(c.name)) { seen.add(c.name); out.push(c) }
      }
    }
    return out
  }, [allPods, selectedPod, pods])

  // Every container name (init and regular) streamed by "All containers". Init
  // containers run to completion first, so their lines sort to the front.
  const streamNames = useMemo(
    () => containers.map(c => c.name),
    [containers]
  )

  // "All containers" is the default whenever there is at least one container,
  // falling back to the first available container otherwise.
  const defaultKey = useMemo(() => {
    if (streamNames.length >= 1) return ALL_CONTAINERS
    const c = containers[0]
    return c ? `${c.name}::false` : ''
  }, [containers, streamNames])

  const [containerKey, setContainerKey] = useState(defaultKey)
  const [tailLines, setTailLines] = useState(DEFAULT_TAIL_LINES)
  const [follow, setFollow] = useState(true)
  const [filter, setFilter] = useState('')
  const [levelFilter, setLevelFilter] = useState('all')
  const [formatted, setFormatted] = useState(true)
  const [fullScreen, setFullScreen] = useState(false)
  const [logs, setLogs] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState(null)
  // A failed follow poll, shown inline at the tail of the feed so the buffer
  // stays visible. The full-pane `error` is used only when there is no buffer.
  const [followError, setFollowError] = useState(null)
  const [flashLatest, setFlashLatest] = useState(false)
  // Whether the current buffer's lines are pod-tagged (set from the response, so
  // the parser never has to guess the wire format). Reset with the buffer.
  const [tagged, setTagged] = useState(false)
  // Coverage of the "All pods" view: { streamed, total, forbidden } when the
  // response did not cover every requested pod, else null.
  const [partial, setPartial] = useState(null)

  const bodyRef = useRef(null)
  const prevLogsRef = useRef('')
  // Identity (timestamp + text) of the most recent visible line, so the
  // new-entry highlight fires only when that line actually changes.
  const prevLatestKeyRef = useRef(null)
  // Whether the log view is scrolled to the bottom; gates auto-follow scrolling.
  const atBottomRef = useRef(true)
  // Timestamp of the last buffered line, sent as sinceTime on single-pod follow
  // polls so the server returns only newer entries to append.
  const lastTsRef = useRef('')
  // Per-pod follow cursors (pod -> newest ts) for "All pods", sent as repeated
  // `since` params so each pod advances independently of node clock skew. In a ref
  // so updating it doesn't re-create fetchLogs.
  const cursorsRef = useRef(new Map())
  // Monotonic id bumped by every reset (non-append) fetch. A fetch applies its
  // result only if its id is still current, so an in-flight append poll from
  // previous params can't merge stale lines into a reset buffer.
  const fetchGenRef = useRef(0)
  // True while a reset fetch is in flight; follow polls skip so a poll started
  // mid-reset can't append the old buffer's cursor into the resetting buffer (a
  // poll fired after the reset shares its generation, so the guard alone misses it).
  const resettingRef = useRef(false)
  // Memoizes formatted output per raw line text, so appends format only the new
  // lines instead of the whole buffer each poll. See displayLines for pruning.
  const formatCacheRef = useRef(new Map())

  // Pod dropdown options: "All pods" leads, then each pod.
  const podOptions = useMemo(() => {
    const opts = [{ key: ALL_PODS, label: 'All pods' }]
    for (const p of pods) opts.push({ key: p.name, label: p.name })
    return opts
  }, [pods])

  // Container dropdown options. "All containers" leads whenever there is at least
  // one container, then every container (init ones prefixed "init:"). A
  // "(previous)" entry for restarted containers is offered only for a single pod,
  // since the previous instance is per-pod and meaningless aggregated over pods.
  // The value encodes the container name and the previous flag.
  const containerOptions = useMemo(() => {
    const opts = []
    if (streamNames.length >= 1) {
      opts.push({ key: ALL_CONTAINERS, label: 'All containers' })
    }
    for (const c of containers) {
      const base = c.isInit ? `init:${c.name}` : c.name
      opts.push({ key: `${c.name}::false`, label: base })
      if (!allPods && (c.restartCount || 0) > 0) {
        opts.push({ key: `${c.name}::true`, label: `${base} (previous)` })
      }
    }
    return opts
  }, [allPods, containers, streamNames])

  // Pods to stream as a comma-joined string: every pod for "All pods", else the
  // selected one. Pod names hold no commas, so this is a value-stable fetch
  // dependency (a pod-set change resets the buffer; a sorted-stable reorder doesn't).
  const reqPodsStr = useMemo(
    () => (allPods ? podNames : (selectedPod ? [selectedPod.name] : [])).join(','),
    [allPods, podNames, selectedPod]
  )

  // Resolve the container option into request params: names to stream and whether
  // to read the previous instance (only for a specific container of a single pod).
  const { reqContainersStr, reqPrevious } = useMemo(() => {
    if (containerKey === ALL_CONTAINERS) {
      // Sorted (like reqPodsStr) so a backend reorder of the same set keeps the
      // fetch dependency stable and doesn't reset the follow buffer.
      return { reqContainersStr: [...streamNames].sort().join(','), reqPrevious: false }
    }
    const sep = containerKey.lastIndexOf('::')
    const cname = sep >= 0 ? containerKey.slice(0, sep) : containerKey
    const prev = !allPods && sep >= 0 && containerKey.slice(sep + 2) === 'true'
    // Brief window after a pod switch where containerKey still holds the previous
    // pod's container (before the reset effect runs): if absent from the current
    // set, fall back to all containers. Sorted to match the ALL_CONTAINERS branch.
    if (!containers.some(c => c.name === cname)) {
      return { reqContainersStr: [...streamNames].sort().join(','), reqPrevious: false }
    }
    return { reqContainersStr: cname, reqPrevious: prev }
  }, [containerKey, containers, streamNames, allPods])

  // Fetch the logs. Follow polls pass { silent: true } to avoid spinner flicker and
  // { append: true } to request only entries newer than the last line (sinceTime)
  // and append them, so the visible lines don't shift each poll. The initial load
  // and any parameter change fetch the tail and replace the buffer.
  const fetchLogs = useCallback(async ({ silent = false, append = false } = {}) => {
    const reqPodsList = reqPodsStr ? reqPodsStr.split(',') : []
    const reqContainers = reqContainersStr ? reqContainersStr.split(',') : []
    if (reqPodsList.length === 0 || reqContainers.length === 0) return
    // Skip follow polls while a reset is in flight: appending now would use the
    // old buffer's cursor and merge into the resetting buffer.
    if (append && resettingRef.current) return
    // A reset starts a new generation; appends ride on the current one. A fetch
    // applies its result only if its generation is still current (checked on settle),
    // so a stale append poll or superseded reset can't write into the buffer.
    const gen = append ? fetchGenRef.current : ++fetchGenRef.current
    if (!append) resettingRef.current = true
    if (!silent) setLoading(true)
    try {
      // First pod is `name`, the rest repeated `pod` params; containers repeated
      // `container` params. tailLines always bounds the fetch. On a follow poll,
      // single-pod narrows by a global sinceTime, all-pods by per-pod `since`.
      const params = new URLSearchParams({ namespace, name: reqPodsList[0], tailLines: String(tailLines), previous: String(reqPrevious) })
      for (let i = 1; i < reqPodsList.length; i++) {
        params.append('pod', reqPodsList[i])
      }
      for (const c of reqContainers) {
        params.append('container', c)
      }
      if (append) {
        if (reqPodsList.length > 1) {
          for (const p of reqPodsList) {
            const cur = cursorsRef.current.get(p)
            if (cur) params.append('since', `${p}=${cur}`)
          }
        } else if (lastTsRef.current) {
          params.set('sinceTime', lastTsRef.current)
        }
      }
      const resp = await fetchWithMock({
        endpoint: `/api/v1/workload/logs?${params.toString()}`,
        mockPath: '../mock/workload',
        mockExport: 'getMockWorkloadLogs'
      })
      if (fetchGenRef.current !== gen) return
      const incoming = resp?.logs || ''
      if (append) {
        setLogs(prev => mergeLogs(prev, incoming))
      } else {
        setLogs(incoming)
      }
      // The response states whether its lines are pod-tagged and how many requested
      // pods it covered, so the parser and footer don't guess. Refreshed every fetch
      // (a pod becoming readable can clear a partial result).
      setTagged(!!resp?.tagged)
      setPartial(resp?.partial ? { streamed: resp.streamed || 0, total: resp.total || 0, forbidden: resp.forbidden || 0 } : null)
      setError(null)
      setFollowError(null)
    } catch (err) {
      if (fetchGenRef.current !== gen) return
      if (append) {
        // Follow poll failed: keep the buffer, show the error inline at the tail
        // rather than replacing the logs with a banner.
        setFollowError(err.message)
      } else {
        setError(err.message)
        setLogs('')
        setFollowError(null)
      }
    } finally {
      // Only the still-current fetch clears shared state, so a superseded reset
      // settling first doesn't drop the loader or the resetting flag.
      if (fetchGenRef.current === gen) {
        if (!append) resettingRef.current = false
        if (!silent) setLoading(false)
      }
    }
  }, [namespace, reqPodsStr, reqContainersStr, reqPrevious, tailLines])

  // Re-fetch whenever the selected container(s), previous flag or line count changes.
  useEffect(() => {
    fetchLogs()
  }, [fetchLogs])

  // Reset the container selection to "All containers" when the pod changes, so a
  // container picked for the previous pod doesn't carry over. Keyed on the pod only;
  // on mount this re-sets the already-default value, which Preact treats as a no-op.
  useEffect(() => {
    setContainerKey(defaultKey)
  }, [effectivePodKey])

  // Poll for new logs while following: silent (no spinner flicker) and appending
  // (visible lines stay put).
  useEffect(() => {
    if (!follow) {
      // No more polls, so drop any stale inline follow error.
      setFollowError(null)
      return
    }
    const id = setInterval(() => { fetchLogs({ silent: true, append: true }) }, FOLLOW_INTERVAL)
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

  // Split the raw payload into entries once. In the "All pods" view each
  // timestamped line is prefixed with its pod of origin ("<pod> <ts> <msg>"); a
  // token is only treated as a pod tag when it is one of the pods we requested AND
  // the next token parses as a timestamp, so a message that merely starts with a
  // word + timestamp is never mistaken for a tag. The leading timestamp (shown as
  // a separator pill) and any ANSI escapes are then stripped and the level
  // detected. Only re-runs when the payload (or tagging regime) changes.
  const baseLines = useMemo(() => {
    const podSet = tagged ? new Set(reqPodsStr ? reqPodsStr.split(',') : []) : null
    // Strip a trailing CR so a CRLF stream doesn't leak `\r` into the text, fields,
    // or filter. (Download uses the raw payload, so it stays byte-verbatim.)
    const entries = logs.split('\n').map(line => line.replace(/\r$/, '')).filter(line => line.length > 0).map(line => {
      let rest = line
      let pod = ''
      if (podSet) {
        const sp = line.indexOf(' ')
        if (sp > 0) {
          const first = line.slice(0, sp)
          const after = line.slice(sp + 1)
          // Require a real timestamp, not a date-shaped token, so a message
          // beginning "<known-pod> 2026-13-45T.." isn't mistaken for a tag.
          const am = after.match(TIMESTAMP_PREFIX)
          if (podSet.has(first) && am && !Number.isNaN(Date.parse(am[1]))) {
            pod = first
            rest = after
          }
        }
      }
      const m = rest.match(TIMESTAMP_PREFIX)
      const tsMs = m ? Date.parse(m[1]) : NaN
      const hasTs = !Number.isNaN(tsMs)
      const ts = hasTs ? m[1] : ''
      const text = stripAnsi(hasTs ? rest.slice(m[0].length) : rest)
      const podId = pod ? pod.slice(pod.lastIndexOf('-') + 1) : ''
      return { ts, tsMs, text, level: detectLevel(text), pod, podId }
    })

    // In all-pods each pod advances its own `since` cursor, so a lagging pod can
    // deliver a line older than another's buffered tail; in arrival order the buffer
    // would lose chronology. Group each timestamped line with its continuation lines
    // and stable-sort the groups by timestamp, keeping multi-line entries and
    // same-instant lines in backend-merged order. Single-pod/all-containers arrive
    // pre-merged from one cursor, so they're left untouched.
    if (!tagged) return entries
    const groups = []
    for (const entry of entries) {
      if (groups.length === 0 || entry.ts) groups.push([entry])
      else groups[groups.length - 1].push(entry)
    }
    groups.sort((a, b) => {
      const ta = a[0].tsMs, tb = b[0].tsMs
      if (Number.isNaN(ta)) return Number.isNaN(tb) ? 0 : -1
      if (Number.isNaN(tb)) return 1
      return ta - tb
    })
    return groups.flat()
  }, [logs, tagged, reqPodsStr])

  // Track the follow cursors from the buffer so the next poll requests only newer
  // entries: lastTsRef is the single-pod high-watermark (sinceTime), cursorsRef the
  // newest ts per pod for all-pods (repeated `since`). Both clear when the buffer
  // has no timestamped lines, so a poll never reuses a stale cursor. A per-pod
  // cursor keeps a clock-skewed pod from being starved by a faster watermark.
  useEffect(() => {
    let ts = ''
    for (let i = baseLines.length - 1; i >= 0; i--) {
      if (baseLines[i].ts) { ts = baseLines[i].ts; break }
    }
    lastTsRef.current = ts
    // Newest ts per pod. The buffer is chronological, so each pod's last occurrence
    // is its newest.
    const cursors = new Map()
    for (const entry of baseLines) {
      if (entry.pod && entry.ts) cursors.set(entry.pod, entry.ts)
    }
    cursorsRef.current = cursors
  }, [baseLines])

  // Substring filter on the message text. A leading "!" negates, keeping only lines
  // that do NOT contain the text (e.g. "!debug" hides every line mentioning debug).
  const containsLines = useMemo(() => {
    const raw = filter.trim()
    if (!raw) return baseLines
    const negate = raw.startsWith('!')
    const needle = (negate ? raw.slice(1) : raw).trim().toLowerCase()
    if (!needle) return baseLines
    return baseLines.filter(entry => entry.text.toLowerCase().includes(needle) !== negate)
  }, [baseLines, filter])

  // Per-level counts over the contains-filtered set, before the level filter (so the
  // footer shows how many errors exist even while viewing one level). Doubles as the
  // footer legend.
  const levelCounts = useMemo(() => {
    const counts = {}
    for (const entry of containsLines) counts[entry.level] = (counts[entry.level] || 0) + 1
    return counts
  }, [containsLines])

  // Apply the level filter on top of the contains filter (exact level match).
  const logLines = useMemo(() => {
    if (levelFilter === 'all') return containsLines
    return containsLines.filter(entry => entry.level === levelFilter)
  }, [containsLines, levelFilter])

  // In formatted mode every line carries a `fmt` descriptor of colored spans: JSON
  // pretty-printed (`kind: 'json'`), structured loggers reflowed to `block`/`spans`,
  // unmatched lines plain wrapping text (`code: true`). Raw mode renders unstyled
  // (`code: false`). Filtering runs first on raw text, so this only transforms what
  // is shown.
  //
  // Formatted output is cached by raw line text (formatCacheRef): an append reuses
  // cached results and only formats the new lines, not the whole buffer each poll.
  // The cache is rebuilt to the shown lines so it can't outgrow the buffer; raw mode
  // leaves it untouched so it survives a round-trip back to formatted.
  const displayLines = useMemo(() => {
    if (!formatted) {
      return logLines.map(entry => ({ ...entry, code: false, fmt: null }))
    }
    const prev = formatCacheRef.current
    const next = new Map()
    const result = logLines.map(entry => {
      // JSON formatting runs on every line; the klog/zap/logfmt cascade is gated on
      // a parsed timestamp, so a continuation line (no ts) can't be mistaken for a
      // structured entry. The cache key carries the head flag since the same text
      // may appear as both a head and a continuation line.
      const key = `${entry.ts ? 1 : 0}\n${entry.text}`
      let formattedEntry = next.get(key) || prev.get(key)
      if (!formattedEntry) {
        // json, then klog/zap/logfmt, then plain. json/block/spans render as
        // decorated rows; plain keeps a wrapping text div.
        const json = highlightJson(entry.text)
        const d = json || (entry.ts ? decorateLine(entry.text, entry.level) : { kind: 'plain' })
        formattedEntry = d.kind === 'plain'
          ? { text: entry.text, code: true, fmt: null }
          : { text: entry.text, code: false, fmt: d }
      }
      next.set(key, formattedEntry)
      return { ...entry, ...formattedEntry }
    })
    formatCacheRef.current = next
    return result
  }, [logLines, formatted])

  // Briefly highlight the most recent visible line when new entries arrive (e.g.
  // while following). Gated on two conditions so it only signals fresh logs: the raw
  // buffer grew (prevLogs !== logs, skipping the first load) and the newest displayed
  // line changed. The second gate stops a poll that appends only filtered-out lines,
  // or a filter change reshuffling the view, from re-pulsing an unchanged last line.
  useEffect(() => {
    const prevLogs = prevLogsRef.current
    prevLogsRef.current = logs
    const latest = displayLines.length > 0 ? displayLines[displayLines.length - 1] : null
    const latestKey = latest ? `${latest.ts}\n${latest.text}` : null
    const prevKey = prevLatestKeyRef.current
    prevLatestKeyRef.current = latestKey
    if (prevLogs === '' || logs === '' || prevLogs === logs) return
    if (latestKey === null || latestKey === prevKey) return
    setFlashLatest(true)
    const id = setTimeout(() => setFlashLatest(false), HIGHLIGHT_DURATION)
    return () => clearTimeout(id)
  }, [logs, displayLines])

  // Track whether the view is pinned to the bottom. Updated on every user
  // scroll so the auto-scroll below only follows new entries when the user is
  // already at the bottom; scrolling up to read older logs stops the next poll
  // from yanking the view back down.
  const handleScroll = useCallback(() => {
    const el = bodyRef.current
    if (!el) return
    atBottomRef.current = el.scrollHeight - el.scrollTop - el.clientHeight < SCROLL_BOTTOM_THRESHOLD
  }, [])

  // Jump to the latest log line and re-pin to the bottom so following resumes
  // auto-scrolling. Wired to the footer mode indicator.
  const scrollToBottom = useCallback(() => {
    const el = bodyRef.current
    if (!el) return
    el.scrollTop = el.scrollHeight
    atBottomRef.current = true
  }, [])

  // Keep the most recent entry (or the inline follow error) in view after each
  // update, but only while pinned to the bottom, so following doesn't fight a user
  // scrolling through history.
  useEffect(() => {
    if (!bodyRef.current || !atBottomRef.current) return
    bodyRef.current.scrollTop = bodyRef.current.scrollHeight
  }, [displayLines, followError])

  // Download the current logs as a text file, named after the pod for a single
  // pod or the workload for the all-pods view.
  const downloadName = allPods ? (workloadName || 'workload') : effectivePodKey
  const handleDownload = useCallback(() => {
    downloadBlob(new window.Blob([logs], { type: 'text/plain' }), `${downloadName}.log`)
  }, [logs, downloadName])

  return (
    <div
      class={`fixed inset-0 z-50 flex items-center justify-center bg-black/50 ${fullScreen ? 'p-0' : 'p-0 sm:px-6 sm:py-16 lg:px-8'}`}
      onClick={onClose}
      data-testid="logs-viewer-overlay"
    >
      <div
        class={`bg-white dark:bg-gray-900 shadow-xl flex flex-col overflow-hidden border border-gray-200 dark:border-gray-700 ${
          fullScreen
            ? 'w-full h-full max-w-full max-h-full rounded-none'
            : 'w-full h-full max-w-full rounded-none sm:max-w-7xl sm:h-[calc(100vh-8rem)] sm:rounded-lg'
        }`}
        onClick={(e) => e.stopPropagation()}
        role="dialog"
        aria-modal="true"
        aria-label={`Logs for ${[kind, workloadName].filter(Boolean).join(' ')}`}
        data-testid="logs-viewer"
      >
        {/* Header */}
        <div class="flex items-center justify-between gap-2 px-4 py-3 border-b border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800">
          <div class="min-w-0">
            <h2 class="text-sm font-semibold text-gray-900 dark:text-white truncate">Log Viewer</h2>
            <p class="text-xs text-gray-500 dark:text-gray-400 truncate" data-testid="logs-title">
              {[kind, namespace, workloadName].filter(Boolean).join('/')}
            </p>
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

        {/* Toolbar. Mobile: a two-column grid of paired rows (the viewer is
            full-screen there). From sm up: a single-row flex-wrap. */}
        <div class="grid grid-cols-2 items-center gap-2 px-4 py-2 border-b border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800 sm:flex sm:flex-wrap">
          {/* Toggles share the full-width top row on mobile; `sm:contents` dissolves
              the wrapper on desktop so each toggle is a direct flex item. */}
          <div class="col-span-2 flex items-center gap-2 sm:contents">
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

            {/* Formatted (default) pretty-prints structured lines and adds
                timestamp pills and level coloring; raw strips all styling. */}
            <ToggleButton
              active={formatted}
              onClick={() => setFormatted(v => !v)}
              label="Format logs"
              title="Toggle between formatted and raw logs"
              testid="logs-format-toggle"
            >
              <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 3H7a2 2 0 0 0-2 2v5a2 2 0 0 1-2 2 2 2 0 0 1 2 2v5c0 1.1.9 2 2 2h1m8-18h1a2 2 0 0 1 2 2v5c0 1.1.9 2 2 2a2 2 0 0 0-2 2v5a2 2 0 0 1-2 2h-1" />
              </svg>
            </ToggleButton>
          </div>

          {/* Pod select. "All pods" leads (every pod merged, origin-tagged);
              selecting one narrows the view and resets the container dropdown.
              Fixed width so a long name is trimmed, not pushing the toolbar. */}
          {pods.length > 0 && (
            <select
              value={effectivePodKey}
              onChange={(e) => setPodKey(e.target.value)}
              class={`${SELECT_CLASS} w-full truncate sm:w-40`}
              data-testid="logs-pod-select"
              aria-label="Pod"
              title="Select pod"
            >
              {podOptions.map((o) => (
                <option key={o.key} value={o.key}>{o.label}</option>
              ))}
            </select>
          )}

          {/* Container select (always shown). Restarted containers also expose a
              "(previous)" entry for the prior instance's logs. Fixed width so a
              long name is trimmed, not pushing the toolbar. */}
          <select
            value={containerKey}
            onChange={(e) => setContainerKey(e.target.value)}
            class={`${SELECT_CLASS} w-full truncate sm:w-40`}
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
            class={`${INPUT_CLASS} w-full sm:w-40`}
            data-testid="logs-filter-input"
            aria-label="Filter log lines containing text"
            title="Filter by keyword (prefix with ! to exclude e.g. !debug)"
          />

          {/* Minimum level filter */}
          <LevelMenu value={levelFilter} onChange={setLevelFilter} />

          {/* Lines select */}
          <select
            value={tailLines}
            onChange={(e) => setTailLines(Number(e.target.value))}
            class={`${SELECT_CLASS} w-full sm:w-auto`}
            data-testid="logs-lines-select"
            aria-label="Number of lines"
            title="Number of log lines to fetch"
          >
            {LINE_LIMITS.map((n) => (
              <option key={n} value={n}>{n} ln</option>
            ))}
          </select>

          {/* Actions. Mobile: the last grid cell, right-aligned. Desktop: floats
              to the far right of the row. */}
          <div class="flex items-center justify-end gap-1 sm:ml-auto">
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
            {/* Fullscreen hidden on mobile, where the viewer already fills the
                screen. */}
            <button
              onClick={() => setFullScreen(v => !v)}
              class={`${ACTION_CLASS} hidden sm:inline-flex`}
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
        <div ref={bodyRef} onScroll={handleScroll} class="flex-1 overflow-auto overscroll-contain bg-white dark:bg-gray-950" data-testid="logs-body">
          {error ? (
            <div class="m-3 p-2 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded text-xs text-red-800 dark:text-red-200" data-testid="logs-error">
              {error}
            </div>
          ) : loading && !logs ? (
            <p class="p-3 text-xs text-gray-500 dark:text-gray-400" data-testid="logs-loading">Loading logs...</p>
          ) : displayLines.length > 0 || followError || podsGone ? (
            <div class="pb-2" data-testid="logs-content">
              {displayLines.map((entry, i) => {
                // Raw mode strips styling: no timestamp pill, level coloring, or
                // highlight on the freshly appended entry.
                const isLatest = formatted && flashLatest && i === displayLines.length - 1
                const meta = LEVEL_META[entry.level] || LEVEL_META[DEFAULT_LEVEL]
                return (
                  <Fragment key={i}>
                    {formatted && entry.ts && (
                      <div
                        class="flex items-center gap-2 px-3 pt-2 select-none"
                        data-testid="logs-timestamp"
                        data-level={entry.level}
                        data-latest={isLatest ? 'true' : undefined}
                      >
                        <span
                          class={`text-[10px] font-mono px-1.5 py-0.5 rounded-full border bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-400 ${meta.border} ${isLatest ? 'log-glow' : ''}`}
                          style={isLatest ? `--glow-color: ${meta.glow}` : undefined}
                          title={entry.pod ? `${entry.pod} · ${meta.label} level` : `${meta.label} level`}
                        >
                          {entry.podId && <span class="font-semibold text-gray-700 dark:text-gray-300" data-testid="logs-pod-id">{entry.podId} · </span>}
                          {entry.ts}
                        </span>
                        <span
                          class={`flex-1 border-t ${meta.border} ${isLatest ? 'log-glow' : ''}`}
                          style={isLatest ? `--glow-color: ${meta.glow}` : undefined}
                        />
                      </div>
                    )}
                    {entry.fmt && entry.fmt.kind === 'block' ? (
                      // Structured entry: one container per entry (the per-entry
                      // logs-line invariant), one inner row per visual line.
                      <div
                        class="overflow-x-auto font-mono whitespace-pre-wrap break-all text-gray-800 dark:text-gray-200"
                        style="padding: 0.25rem 0.75rem; font-size: 12px; line-height: 1.5;"
                        data-testid="logs-line"
                      >
                        {entry.fmt.rows.map((row, r) => (
                          <div key={r} data-testid="logs-line-row">
                            {row.map((s, j) => (s.cls ? <span key={j} class={s.cls}>{s.text}</span> : s.text))}
                          </div>
                        ))}
                      </div>
                    ) : entry.fmt && entry.fmt.kind === 'spans' ? (
                      // Unstructured entry highlighted in place: a single row.
                      <div
                        class="overflow-x-auto font-mono whitespace-pre-wrap break-all text-gray-800 dark:text-gray-200"
                        style="padding: 0.25rem 0.75rem; font-size: 12px; line-height: 1.5;"
                        data-testid="logs-line"
                      >
                        {entry.fmt.spans.map((s, j) => (s.cls ? <span key={j} class={s.cls}>{s.text}</span> : s.text))}
                      </div>
                    ) : entry.fmt && entry.fmt.kind === 'json' ? (
                      // Pretty-printed JSON: a <pre> preserves the baked-in
                      // indentation/newlines and scrolls long values horizontally.
                      <pre
                        class="overflow-x-auto font-mono text-gray-800 dark:text-gray-200"
                        style="margin: 0; padding: 0.25rem 0.75rem; font-size: 12px; line-height: 1.5;"
                        data-testid="logs-line"
                      >
                        {entry.fmt.spans.map((s, j) => (s.cls ? <span key={j} class={s.cls}>{s.text}</span> : s.text))}
                      </pre>
                    ) : entry.code ? (
                      // Plain line (no formatter match, or a continuation line such
                      // as a stack frame): wrap like the decorated rows, not the
                      // non-wrapping JSON <pre>.
                      <div
                        class="overflow-x-auto font-mono whitespace-pre-wrap break-all text-gray-800 dark:text-gray-200"
                        style="padding: 0.25rem 0.75rem; font-size: 12px; line-height: 1.5;"
                        data-testid="logs-line"
                      >
                        {entry.text}
                      </div>
                    ) : (
                      <div
                        class="px-3 py-0.5 text-sm font-mono whitespace-pre-wrap break-all text-gray-800 dark:text-gray-200"
                        data-testid="logs-line"
                      >
                        {entry.text}
                      </div>
                    )}
                  </Fragment>
                )
              })}
              {followError && (
                <div
                  class="flex items-start gap-2 mx-3 mt-2 p-2 rounded border border-red-200 dark:border-red-800 bg-red-50 dark:bg-red-900/20 text-xs text-red-800 dark:text-red-200"
                  data-testid="logs-follow-error"
                  role="alert"
                >
                  <svg class="w-4 h-4 flex-shrink-0 mt-px" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z" />
                  </svg>
                  <span class="break-all">{followError}</span>
                </div>
              )}
              {podsGone && (
                <div
                  class="flex items-start gap-2 mx-3 mt-2 p-2 rounded border border-red-200 dark:border-red-800 bg-red-50 dark:bg-red-900/20 text-xs text-red-800 dark:text-red-200"
                  data-testid="logs-no-pods"
                  role="alert"
                >
                  <svg class="w-4 h-4 flex-shrink-0 mt-px" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z" />
                  </svg>
                  <span class="break-all">{NO_PODS_MESSAGE}</span>
                </div>
              )}
            </div>
          ) : (
            <p class="p-3 text-xs text-gray-500 dark:text-gray-400" data-testid="logs-empty">
              {filter.trim() || levelFilter !== 'all' ? 'No matching log entries' : 'No logs available'}
            </p>
          )}
        </div>

        {/* Footer: a per-level count summary doubling as the color legend. The
            trailing corner shows the fetch loader while a request is in flight,
            else the live/snapshot mode. */}
        <div
          class="flex items-center justify-between gap-3 px-4 py-2 border-t border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800"
          data-testid="logs-footer"
        >
          <div class="flex flex-wrap items-center gap-x-3 gap-y-1 min-w-0">
            <div class="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-gray-600 dark:text-gray-400" data-testid="logs-level-summary">
              {LEVELS.filter(l => levelCounts[l]).map((l) => (
                <span key={l} class="inline-flex items-center gap-1" title={`${LEVEL_META[l].label}: ${levelCounts[l]}`}>
                  <span class={`w-2 h-2 rounded-full ${LEVEL_META[l].swatch}`} />
                  {LEVEL_META[l].label} {levelCounts[l]}
                </span>
              ))}
            </div>
            {/* Partial-coverage note: all-pods didn't cover every pod (forbidden,
                missing) or the fan-out was capped. The pod-count phrasing shows only
                when pods were dropped; a pure cap reads as "results truncated" to
                avoid a misleading "N of N". */}
            {partial && (
              <span
                class="inline-flex items-center gap-1 text-xs text-amber-600 dark:text-amber-400 flex-shrink-0"
                data-testid="logs-partial"
                title={partial.forbidden > 0
                  ? `${partial.forbidden} pod(s) not readable with your permissions`
                  : 'Some streams could not be shown'}
              >
                <svg class="w-3.5 h-3.5 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z" />
                </svg>
                {partial.streamed < partial.total
                  ? `showing ${partial.streamed} of ${partial.total} pods`
                  : 'results truncated'}
              </span>
            )}
          </div>
          {loading ? (
            <div
              class="flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400 flex-shrink-0"
              data-testid="logs-loader"
              role="status"
              aria-label="Loading logs"
            >
              <div class="animate-spin rounded-full h-4 w-4 border-b-2 border-gray-400" />
              <span>Loading…</span>
            </div>
          ) : (
            <button
              type="button"
              onClick={scrollToBottom}
              class="flex items-center gap-1.5 text-xs text-gray-600 dark:text-gray-400 hover:text-flux-blue dark:hover:text-flux-blue flex-shrink-0 focus:outline-none focus-visible:ring-2 focus-visible:ring-flux-blue rounded"
              data-testid="logs-mode"
              title={`${follow ? 'Following live logs' : 'Snapshot'} — click to scroll to latest`}
              aria-label="Scroll to latest logs"
            >
              {follow ? (
                <>
                  <svg class="w-4 h-4 text-gray-500 dark:text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 13l-7 7-7-7m14-6l-7 7-7-7" />
                  </svg>
                  <span>Following</span>
                </>
              ) : (
                <>
                  <svg class="w-4 h-4 text-gray-500 dark:text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M6.827 6.175A2.31 2.31 0 0 1 5.186 7.23c-.38.054-.757.112-1.134.175C2.999 7.58 2.25 8.507 2.25 9.574V18a2.25 2.25 0 0 0 2.25 2.25h15A2.25 2.25 0 0 0 21.75 18V9.574c0-1.067-.75-1.994-1.802-2.169a47.865 47.865 0 0 0-1.134-.175 2.31 2.31 0 0 1-1.64-1.055l-.822-1.316a2.192 2.192 0 0 0-1.736-1.039 48.774 48.774 0 0 0-5.232 0 2.192 2.192 0 0 0-1.736 1.039l-.821 1.316Z" />
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M16.5 12.75a4.5 4.5 0 1 1-9 0 4.5 4.5 0 0 1 9 0ZM18.75 10.5h.008v.008h-.008V10.5Z" />
                  </svg>
                  <span>Snapshot</span>
                </>
              )}
            </button>
          )}
        </div>
      </div>
    </div>
  )
}
