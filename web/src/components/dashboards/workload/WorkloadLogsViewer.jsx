// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { Fragment } from 'preact'
import { useState, useEffect, useCallback, useMemo, useRef } from 'preact/hooks'
import Prism from 'prismjs'
import 'prismjs/components/prism-json'
import { fetchWithMock } from '../../../utils/fetch'
import { downloadBlob } from '../../../utils/download'
import { usePrismTheme } from '../common/yaml'
import { LEVELS, LEVEL_META, DEFAULT_LEVEL, detectLevel, stripAnsi } from '../../../utils/logLevel'
import { decorateLine } from '../../../utils/logFormat'
import { useDismiss } from '../../../utils/useDismiss'

// Selectable limits for the number of log lines to fetch from the backend.
const LINE_LIMITS = [100, 500, 1000, 5000]

// Default number of log lines requested from the backend.
const DEFAULT_TAIL_LINES = 100

// Sentinel option key for the "All containers" view, which streams every
// container (init and regular) and interleaves the lines chronologically.
const ALL_CONTAINERS = 'all'

// Sentinel option key for the "All pods" view, which streams every pod of the
// workload and interleaves the lines chronologically, each tagged with its pod of
// origin. Distinct from ALL_CONTAINERS so it can never collide with a pod name.
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

/**
 * mergeLogs - appends the entries of an incoming payload to the accumulated
 * buffer, dropping whole entries already present and capping to MAX_BUFFER_LINES.
 *
 * Follow polls request everything since the last line's timestamp, which the API
 * filters at second granularity, so the last second of entries is re-sent on
 * every poll. Dedup is by the entry's timestamped HEAD line, which is unique (a
 * nanosecond timestamp, plus the pod tag in the all-pods view): a re-sent entry
 * whose head already sits in the buffer is dropped whole — head and all the
 * continuation lines under it. Continuation lines (stack frames, no timestamp)
 * are never deduped on their own, so a genuinely new entry keeps its full stack
 * even when a replica emitted a byte-identical frame earlier; deduping those by
 * text (the previous behaviour) truncated the new trace. Returns the previous
 * buffer unchanged when there is nothing new, so the state update is a no-op.
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
      // Start of an entry: a re-send (head already buffered) is dropped whole, so
      // its continuation lines are skipped until the next head arrives.
      dropping = seen.has(line)
      if (!dropping) { fresh.push(line); seen.add(line) }
    } else if (!dropping) {
      // A continuation line of a kept entry (or a leading orphan before any head):
      // appended verbatim, never deduped against an identical frame elsewhere.
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

// entryTimestamp returns the RFC3339 timestamp that heads a log entry, or null
// for a continuation line (a stack frame and the like) that carries none. A head
// is either "<ts> msg" (single pod) or "<pod> <ts> msg" (all-pods, pod-tagged);
// the timestamp must be a real instant, not merely date-shaped, so a frame
// beginning with a word and a date-like token is not taken for a head.
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

/**
 * formatJson - indents a log line if it is a JSON object or array, otherwise
 * returns null. The cheap first-character check avoids a try/catch on the
 * common case of plain-text lines.
 *
 * @param {string} text - The log line text
 * @returns {string|null} The indented JSON, or null if the line is not JSON
 */
function formatJson(text) {
  const trimmed = text.trim()
  if (trimmed[0] !== '{' && trimmed[0] !== '[') return null
  try {
    return JSON.stringify(JSON.parse(trimmed), null, 2)
  } catch {
    return null
  }
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
    <div class="relative" ref={ref}>
      <button
        type="button"
        onClick={() => setOpen(v => !v)}
        class={LEVEL_BUTTON_CLASS}
        data-testid="logs-level-filter"
        aria-label="Log level"
        aria-expanded={open}
        title="Show only logs of this level"
      >
        {current.swatch && <span class={`w-2 h-2 rounded-full ${current.swatch}`} />}
        <span>{current.label}</span>
        <svg class="w-3 h-3 ml-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
        </svg>
      </button>
      {open && (
        <div
          class="absolute left-0 mt-1 w-36 bg-white dark:bg-gray-800 rounded-md shadow-lg border border-gray-200 dark:border-gray-700 py-1 z-50"
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
 * The header titles the modal "Log Viewer" over the workload's kind/namespace/name.
 * Two toolbar dropdowns scope the stream: a pod dropdown ("All pods" plus each
 * pod) and a container dropdown ("All containers" plus each container). Switching
 * pods resets the container dropdown back to "All containers".
 *
 * Fetches logs from GET /api/v1/workload/logs and renders each log entry on its
 * own row. In the default formatted mode the timestamp is shown as a pill on the
 * row separator (prefixed with the pod id in the "All pods" view), the pill
 * border and rule are colored by the detected log level (JSON/klog/logfmt/plain
 * text), and JSON lines are pretty-printed; the footer summarizes the per-level
 * counts. The raw mode strips all of that styling. Supports following (live
 * polling, appending only new entries), toggling formatted/raw, filtering by
 * substring and minimum level, choosing the number of lines, an "All containers"
 * option (the default) and an "All pods" option (the default) that interleave the
 * lines chronologically. In "All pods" the backend tags each line with its pod of
 * origin; the row pill shows that pod's id. The viewer downloads the logs as a
 * <pod>.log file and has a fullscreen mode. A failed follow poll is shown inline
 * at the tail; an initial/post-reset failure (no buffer) shows a full-pane error.
 *
 * Pod selection is internal: the viewer is given every pod with its containers and
 * keeps the current pod in local state, so it can build the "All pods" request
 * (every pod's regular containers) and resolve the selected pod's containers live.
 *
 * @param {Object} props
 * @param {string} props.kind - Workload kind, shown in the title
 * @param {string} props.namespace - Workload namespace
 * @param {string} props.workloadName - Workload name, shown in the title
 * @param {Array<{name: string, status?: string, containers: Array<{name: string, isInit: boolean, restartCount?: number}>}>} props.pods - Pods of the workload, with their containers
 * @param {string} [props.initialPodName] - Pod to pre-select; defaults to "All pods" when absent
 * @param {Function} props.onClose - Callback to close the viewer
 */
export function WorkloadLogsViewer({ kind, namespace, workloadName, pods = [], initialPodName, onClose }) {
  // Load the Prism theme used to syntax-highlight JSON lines as code blocks.
  usePrismTheme()

  // Selected pod: a specific pod name or ALL_PODS. Initialised from initialPodName
  // (the parent remounts the viewer per open, so this re-inits on each open).
  const [podKey, setPodKey] = useState(initialPodName || ALL_PODS)

  // Resolve the selection against the live pod list. If the selected specific pod
  // has disappeared, fall back to "All pods" so the view keeps working.
  const effectivePodKey = (podKey === ALL_PODS || pods.some(p => p.name === podKey)) ? podKey : ALL_PODS
  const allPods = effectivePodKey === ALL_PODS
  const selectedPod = allPods ? null : (pods.find(p => p.name === effectivePodKey) || null)

  // Commit the fallback when the selected pod disappears (scaled down, deleted).
  // effectivePodKey already renders "All pods" in that case, but persisting it
  // means a later pod that happens to reappear with the same name does not
  // silently snap the view back to it without the user choosing it.
  useEffect(() => {
    if (podKey !== ALL_PODS && !pods.some(p => p.name === podKey)) {
      setPodKey(ALL_PODS)
    }
  }, [podKey, pods])

  // Sorted pod names: the request streams these (name = first, rest as repeated
  // pod params) and the parser uses them as the set of valid pod tags. Sorting
  // keeps the request stable if the backend returns the same pods reordered.
  const podNames = useMemo(() => pods.map(p => p.name).sort(), [pods])

  // Containers driving the container dropdown and the request. For "All pods" it
  // is the union of containers across pods (pod templates are uniform, so this is
  // just every container); for a specific pod it is that pod's live containers.
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

  // Every container name (init and regular) — the set the "All containers" view
  // streams and interleaves. Init containers run to completion before the app
  // starts, so their lines sort to the front of the merged feed.
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
  // Per-pod follow cursors (pod -> newest timestamp seen) for the "All pods" view,
  // sent as repeated `since` params so each pod advances independently of clock
  // skew between nodes. Kept in a ref so updating it doesn't re-create fetchLogs.
  const cursorsRef = useRef(new Map())
  // Monotonic id bumped by every reset (non-append) fetch. A fetch only applies
  // its result if its id is still current, so an in-flight append poll from a
  // previous container/params can't merge stale lines into a reset buffer.
  const fetchGenRef = useRef(0)
  // True while a reset (initial/param-change) fetch is in flight. Follow polls
  // skip while it is set, so a poll started mid-reset can't append using the old
  // buffer's cursor into the resetting buffer (the generation guard alone can't
  // catch this, since a poll fired after the reset shares its generation).
  const resettingRef = useRef(false)
  // Memoizes the formatted (pretty-printed + Prism-highlighted) output per raw
  // line text, so appends only format the new lines instead of re-highlighting
  // the whole buffer on every poll. See displayLines for how it's pruned.
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

  // The pods to stream as a stable comma-joined string: every pod for "All pods",
  // or the single selected pod. Pod names cannot contain commas, so this stays a
  // primitive, value-stable fetch dependency (a pod-set change resets the buffer;
  // a backend reorder of the same set does not, because podNames is sorted).
  const reqPodsStr = useMemo(
    () => (allPods ? podNames : (selectedPod ? [selectedPod.name] : [])).join(','),
    [allPods, podNames, selectedPod]
  )

  // Resolve the selected container option into the request parameters: the
  // container names to stream and whether to read the previous instance. The
  // previous flag applies only to a specific container of a single pod.
  const { reqContainersStr, reqPrevious } = useMemo(() => {
    if (containerKey === ALL_CONTAINERS) {
      // Sort the streamed container names (as reqPodsStr sorts pods) so a backend
      // reorder of the same set keeps the fetch dependency value-stable and does
      // not reset the follow buffer.
      return { reqContainersStr: [...streamNames].sort().join(','), reqPrevious: false }
    }
    const sep = containerKey.lastIndexOf('::')
    const cname = sep >= 0 ? containerKey.slice(0, sep) : containerKey
    const prev = !allPods && sep >= 0 && containerKey.slice(sep + 2) === 'true'
    // Guard the brief window after a pod switch where containerKey still holds the
    // previous pod's container (before the reset effect runs): if it names a
    // container the current set doesn't have, fall back to all containers. Sorted
    // to match the ALL_CONTAINERS branch so the value stays stable and the
    // fallback does not trigger an extra buffer reset.
    if (!containers.some(c => c.name === cname)) {
      return { reqContainersStr: [...streamNames].sort().join(','), reqPrevious: false }
    }
    return { reqContainersStr: cname, reqPrevious: prev }
  }, [containerKey, containers, streamNames, allPods])

  // Fetch the logs. Background follow-polls pass { silent: true } so they don't
  // toggle the loading spinner, which would flicker on every poll; only the
  // initial load and user-driven changes (container, line count) show it.
  //
  // Follow polls also pass { append: true }: once a buffer exists they request
  // only the entries newer than the last line (sinceTime) and append them,
  // instead of re-fetching the whole tail window and replacing it (which would
  // make the visible lines shift on every poll). The initial load and any
  // parameter change fetch the tail and replace the buffer.
  const fetchLogs = useCallback(async ({ silent = false, append = false } = {}) => {
    const reqPodsList = reqPodsStr ? reqPodsStr.split(',') : []
    const reqContainers = reqContainersStr ? reqContainersStr.split(',') : []
    if (reqPodsList.length === 0 || reqContainers.length === 0) return
    // Skip follow polls while a reset is in flight: appending now would use the
    // old buffer's cursor and merge into the resetting buffer.
    if (append && resettingRef.current) return
    // A reset starts a new generation; appends ride on the current one. A fetch
    // only applies its result if its generation is still current (checked on
    // settle below), so an in-flight append poll from a previous pod/container or
    // a superseded reset can't write stale lines into the buffer.
    const gen = append ? fetchGenRef.current : ++fetchGenRef.current
    if (!append) resettingRef.current = true
    if (!silent) setLoading(true)
    try {
      // The required name is the first pod; additional pods are repeated `pod`
      // params (the "All pods" view) and containers repeated `container` params.
      // tailLines is always sent so the backend bounds every fetch to the user's
      // selection. On a follow poll, single-pod mode narrows by a global sinceTime
      // while all-pods mode narrows by a per-pod `since` cursor.
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
      // The response states whether its lines are pod-tagged and how many of the
      // requested pods it covered, so the parser and the footer never have to
      // guess. Both are refreshed on every fetch (they stay constant for a given
      // pod set, but a pod becoming readable can clear a partial result).
      setTagged(!!resp?.tagged)
      setPartial(resp?.partial ? { streamed: resp.streamed || 0, total: resp.total || 0, forbidden: resp.forbidden || 0 } : null)
      setError(null)
      setFollowError(null)
    } catch (err) {
      if (fetchGenRef.current !== gen) return
      if (append) {
        // A follow poll failed: keep the buffer and surface the error inline at
        // the tail of the feed rather than replacing the logs with a banner.
        setFollowError(err.message)
      } else {
        setError(err.message)
        setLogs('')
        setFollowError(null)
      }
    } finally {
      // Only the still-current fetch clears the shared state, so a superseded
      // reset settling first doesn't drop the loader or the resetting flag.
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

  // Reset the container selection to the default ("All containers") whenever the
  // selected pod changes, so a container picked for the previous pod doesn't carry
  // over. Keyed on the pod selection only: poll-driven re-renders keep the same
  // selection untouched. On the initial mount this re-sets the already-default
  // value, which Preact bails out of as a no-op.
  useEffect(() => {
    setContainerKey(defaultKey)
  }, [effectivePodKey])

  // Poll for new logs while following, silently so the spinner doesn't flicker
  // and appending so the visible lines stay put instead of shifting each poll.
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
    // Strip a trailing CR up front so a CRLF stream doesn't leak `\r` into the
    // message text, the decorated field values, or the filter. (The download uses
    // the raw payload, so it stays byte-verbatim.)
    const entries = logs.split('\n').map(line => line.replace(/\r$/, '')).filter(line => line.length > 0).map(line => {
      let rest = line
      let pod = ''
      if (podSet) {
        const sp = line.indexOf(' ')
        if (sp > 0) {
          const first = line.slice(0, sp)
          const after = line.slice(sp + 1)
          // Require a real timestamp, not merely a date-shaped token, so a message
          // that happens to begin "<known-pod> 2026-13-45T.." is not mistaken for
          // a tag and split apart.
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

    // In the all-pods view each pod advances its own `since` cursor, so a pod
    // that lags (slow or clock-skewed) can deliver a line older than another
    // pod's already-buffered tail; appended in arrival order the buffer would no
    // longer be chronological. Regroup each timestamped line with the
    // continuation lines that follow it and stable-sort the groups by timestamp,
    // so the merged view stays ordered while a multi-line entry and same-instant
    // lines keep their original (backend-merged) order. The single-pod and
    // all-containers paths arrive pre-merged from one cursor, so they are left
    // untouched.
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

  // Track the follow cursors from the current buffer so the next poll requests
  // only newer entries. lastTsRef is the global high-watermark for the single-pod
  // path (sinceTime); cursorsRef holds the newest timestamp per pod for the
  // all-pods path (repeated `since`). Both are authoritative over the buffer: they
  // clear when it has no timestamped lines, so a poll never reuses a stale cursor
  // from a previous stream. The second-granular cursor re-sends the last second of
  // overlap, deduped on append; a per-pod cursor keeps a clock-skewed pod from
  // being starved by another pod's faster-advancing watermark.
  useEffect(() => {
    let ts = ''
    for (let i = baseLines.length - 1; i >= 0; i--) {
      if (baseLines[i].ts) { ts = baseLines[i].ts; break }
    }
    lastTsRef.current = ts
    // Newest timestamp per pod. The buffer is chronological, so the last occurrence
    // of each pod is its newest; second-granular cursors make exact ordering moot.
    const cursors = new Map()
    for (const entry of baseLines) {
      if (entry.pod && entry.ts) cursors.set(entry.pod, entry.ts)
    }
    cursorsRef.current = cursors
  }, [baseLines])

  // Apply the substring filter on the message text; cheap pass over the
  // already-split entries. A leading "!" negates the match, keeping only lines
  // that do NOT contain the text (e.g. "!debug" hides every line mentioning debug).
  const containsLines = useMemo(() => {
    const raw = filter.trim()
    if (!raw) return baseLines
    const negate = raw.startsWith('!')
    const needle = (negate ? raw.slice(1) : raw).trim().toLowerCase()
    if (!needle) return baseLines
    return baseLines.filter(entry => entry.text.toLowerCase().includes(needle) !== negate)
  }, [baseLines, filter])

  // Per-level counts over the contains-filtered set, before the minimum-level
  // threshold (so the footer summary shows how many errors exist even while
  // viewing "error and above"). Doubles as the footer legend.
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

  // In formatted mode every line renders as a code block sharing the same
  // monospace font and size as the YAML blocks (`code: true`). Valid JSON gets
  // indented and Prism-highlighted (`html`); other lines keep their plain text
  // in the same styling. In raw mode lines render as unstyled plain text
  // (`code: false`). Filtering happens first on the raw text, so this only
  // transforms what is actually shown.
  //
  // The formatted output is cached by raw line text (formatCacheRef): since the
  // pretty-printed/highlighted form depends only on the line text, an append
  // reuses the cached result for every existing line and only runs formatJson +
  // Prism over the genuinely new lines, instead of re-highlighting the whole
  // buffer on every poll. The cache is rebuilt to hold only the lines currently
  // shown, so it can't outgrow the buffer; raw mode leaves it untouched so it
  // survives a round-trip back to formatted. Returning the same html string for
  // unchanged lines also lets Preact skip re-applying their innerHTML.
  const displayLines = useMemo(() => {
    if (!formatted) {
      return logLines.map(entry => ({ ...entry, code: false, html: null, fmt: null }))
    }
    const prev = formatCacheRef.current
    const next = new Map()
    const result = logLines.map(entry => {
      // JSON formatting runs on every line (unchanged from before). Only the
      // klog/zap/logfmt cascade is gated on a parsed timestamp, so a continuation
      // line (stack frame, no timestamp) can never be mistaken for a structured
      // entry. The cache key carries the head flag because the same stripped text
      // may appear both as a head and as a continuation line.
      const key = `${entry.ts ? 1 : 0} ${entry.text}`
      let formattedEntry = next.get(key) || prev.get(key)
      if (!formattedEntry) {
        const json = formatJson(entry.text)
        if (json != null) {
          formattedEntry = { text: json, code: true, html: Prism.highlight(json, Prism.languages.json, 'json'), fmt: null }
        } else {
          // klog → zap → logfmt → plain. The block/spans kinds render as decorated
          // multi-line / single highlighted rows; plain keeps the existing code
          // path so its output is byte-for-byte unchanged.
          const d = entry.ts ? decorateLine(entry.text, entry.level) : { kind: 'plain' }
          formattedEntry = d.kind === 'plain'
            ? { text: entry.text, code: true, html: null, fmt: null }
            : { text: entry.text, code: false, html: null, fmt: d }
        }
      }
      next.set(key, formattedEntry)
      return { ...entry, ...formattedEntry }
    })
    formatCacheRef.current = next
    return result
  }, [logLines, formatted])

  // Briefly highlight the most recent visible line whenever genuinely new
  // entries arrive (e.g. while following). The highlight is gated on two
  // conditions so it only signals fresh logs: the raw buffer must have grown
  // (prevLogs !== logs, skipping the very first load), and the newest displayed
  // line must have changed. The second gate is what keeps a follow poll that
  // appends only lines filtered out of the view — or a filter change that
  // reshuffles the view — from re-pulsing an unchanged last line.
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
  // update, but only while pinned to the bottom, so following doesn't fight a
  // user scrolling through history.
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
      class={`fixed inset-0 z-50 flex justify-center bg-black/50 ${fullScreen ? 'items-center p-0' : 'items-center px-4 sm:px-6 lg:px-8 py-16'}`}
      onClick={onClose}
      data-testid="logs-viewer-overlay"
    >
      <div
        class={`bg-white dark:bg-gray-900 shadow-xl flex flex-col overflow-hidden border border-gray-200 dark:border-gray-700 ${
          fullScreen ? 'w-full h-full max-w-full max-h-full rounded-none' : 'w-full max-w-7xl h-[calc(100vh-8rem)] rounded-lg'
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

          {/* Toggle between formatted and raw output. Formatted (default)
              pretty-prints structured lines and adds timestamp pills and
              level coloring; raw strips all styling to plain text. */}
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

          {/* Pod select. "All pods" leads and streams every pod merged, tagged
              with its origin; selecting a specific pod narrows the view and
              resets the container dropdown to "All containers". Fixed width so a
              long pod name is trimmed instead of pushing the rest of the toolbar. */}
          {pods.length > 0 && (
            <select
              value={effectivePodKey}
              onChange={(e) => setPodKey(e.target.value)}
              class={`${SELECT_CLASS} w-28 sm:w-40 truncate`}
              data-testid="logs-pod-select"
              aria-label="Pod"
              title="Select pod"
            >
              {podOptions.map((o) => (
                <option key={o.key} value={o.key}>{o.label}</option>
              ))}
            </select>
          )}

          {/* Container select (always shown). Containers that have restarted
              also expose a "(previous)" entry for the prior instance's logs.
              Fixed width so a long container name is trimmed instead of pushing
              the rest of the toolbar. */}
          <select
            value={containerKey}
            onChange={(e) => setContainerKey(e.target.value)}
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

          {/* Minimum level filter */}
          <LevelMenu value={levelFilter} onChange={setLevelFilter} />

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
        <div ref={bodyRef} onScroll={handleScroll} class="flex-1 overflow-auto overscroll-contain bg-white dark:bg-gray-950" data-testid="logs-body">
          {error ? (
            <div class="m-3 p-2 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded text-xs text-red-800 dark:text-red-200" data-testid="logs-error">
              {error}
            </div>
          ) : loading && !logs ? (
            <p class="p-3 text-xs text-gray-500 dark:text-gray-400" data-testid="logs-loading">Loading logs...</p>
          ) : displayLines.length > 0 || followError ? (
            <div class="pb-2" data-testid="logs-content">
              {displayLines.map((entry, i) => {
                // Raw mode strips all styling: no timestamp pill, no level
                // coloring, and no highlight on the freshly appended entry.
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
                      // Structured entry: one container per entry (keeping the
                      // per-entry logs-line invariant), one inner row per visual
                      // line, all flush-left at full width.
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
                    ) : entry.code ? (
                      <pre
                        class="overflow-x-auto language-json"
                        style="margin: 0; padding: 0.25rem 0.75rem; background: transparent; text-shadow: none; font-size: 12px; line-height: 1.5;"
                        data-testid="logs-line"
                      >
                        {entry.html != null ? (
                          <code
                            class="language-json"
                            style="background: transparent; text-shadow: none;"
                            dangerouslySetInnerHTML={{ __html: entry.html }}
                          />
                        ) : (
                          <code class="language-json" style="background: transparent; text-shadow: none;">
                            {entry.text}
                          </code>
                        )}
                      </pre>
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
            </div>
          ) : (
            <p class="p-3 text-xs text-gray-500 dark:text-gray-400" data-testid="logs-empty">
              {filter.trim() || levelFilter !== 'all' ? 'No matching log entries' : 'No logs available'}
            </p>
          )}
        </div>

        {/* Footer: a per-level count summary that doubles as the color legend.
            The trailing corner shows the fetch loader while a request is in
            flight, otherwise the live/snapshot mode reflecting the follow state. */}
        <div
          class="flex items-center justify-between gap-3 px-4 py-2 border-t border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800"
          data-testid="logs-footer"
        >
          <div class="flex items-center gap-3 min-w-0">
            <div class="flex items-center gap-3 text-xs text-gray-600 dark:text-gray-400" data-testid="logs-level-summary">
              {LEVELS.filter(l => levelCounts[l]).map((l) => (
                <span key={l} class="inline-flex items-center gap-1" title={`${LEVEL_META[l].label}: ${levelCounts[l]}`}>
                  <span class={`w-2 h-2 rounded-full ${LEVEL_META[l].swatch}`} />
                  {LEVEL_META[l].label} {levelCounts[l]}
                </span>
              ))}
            </div>
            {/* Partial-coverage note: the all-pods view did not cover every pod
                (forbidden, missing) or the fan-out was capped. The pod-count
                phrasing is shown only when pods were actually dropped; a pure cap
                (every pod streamed, but the stream count was capped) reads as
                "results truncated" instead, to avoid a misleading "N of N". */}
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
