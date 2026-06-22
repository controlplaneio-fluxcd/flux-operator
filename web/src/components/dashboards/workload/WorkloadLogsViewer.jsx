// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useState, useEffect, useCallback, useMemo, useRef } from 'preact/hooks'
import { fetchWithMock } from '../../../utils/fetch'
import { downloadBlob } from '../../../utils/download'
import { LEVELS, LEVEL_META, DEFAULT_LEVEL, detectLevel, stripAnsi } from '../../../utils/logLevel'
import { decorateLine, highlightJson, topLevelJsonKeys, fieldMatcher } from '../../../utils/logFormat'
import { groupEntries } from '../../../utils/logGroup'
import { logSettings, TAIL_LINES, FONT_SIZES } from '../../../utils/logSettings'

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

/**
 * countNewEntries - counts how many logical log entries an incoming follow payload
 * adds to the buffer. An entry is a timestamped HEAD line (continuation lines such
 * as stack frames carry no timestamp and fold into their head, so they aren't
 * counted). Dedup mirrors mergeLogs: a head already buffered is a re-send, not a
 * new entry. Used to show a transient "+N new" indicator after a poll appends.
 *
 * @param {string} prev - The accumulated log payload before the append
 * @param {string} incoming - The newly fetched log payload
 * @returns {number} The number of new entries the append introduces
 */
function countNewEntries(prev, incoming) {
  const add = incoming.split('\n').filter(Boolean)
  if (add.length === 0) return 0
  const seen = new Set(prev.split('\n').filter(Boolean))
  let count = 0
  for (const line of add) {
    if (entryTimestamp(line) && !seen.has(line)) count++
  }
  return count
}

// Captures the leading RFC3339 timestamp the API prepends to each log line.
// Anchored to a date so an untimestamped line (a stack frame) isn't mangled. The
// separator is a single space (kubelet emits exactly one), not greedy `\s+`, so a
// frame's own indentation (`\tat …`, `  File …`) survives to drive trace folding.
const TIMESTAMP_PREFIX = /^(\d{4}-\d{2}-\d{2}T\S+) /

// entryTimestamp returns the RFC3339 timestamp heading a log entry, or null for a
// continuation line (stack frame) that carries none. A head is "<ts> msg" or
// carries up to two leading origin tags before the ts ("<pod> <ts> msg", "<container>
// <ts> msg", or "<pod> <container> <ts> msg"), so the ts may sit after 0, 1, or 2
// tokens. The ts must be a real instant, not merely date-shaped, so a frame
// beginning with words and a date-like token isn't a head. The two-token cap bounds
// misdetection; an indented continuation breaks on its leading space.
function entryTimestamp(line) {
  let s = line
  for (let i = 0; i < 3; i++) {
    const m = s.match(TIMESTAMP_PREFIX)
    if (m && !Number.isNaN(Date.parse(m[1]))) return m[1]
    const sp = s.indexOf(' ')
    if (sp <= 0) break
    s = s.slice(sp + 1)
  }
  return null
}

/**
 * parseLogLine - parse one raw payload line into a log entry. A multi-stream line
 * carries its origin tags before the timestamp: "<pod> <ts> <msg>" (all-pods),
 * "<container> <ts> <msg>" (all-containers, one pod), or "<pod> <container> <ts>
 * <msg>" (both). Each tag is peeled only when its token is one of the requested
 * names AND a real timestamp follows the peels (so a message starting "<word>
 * <date>" isn't mistaken for a tag, and a continuation line keeps its full text).
 * The timestamp and ANSI are then stripped and the level detected; a continuation
 * line (no ts) keeps its indentation (see TIMESTAMP_PREFIX). Exported so tests hit
 * the real parse path. Level is NOT detected here — it is formatted-mode only and
 * computed lazily in formattedLines (raw mode is verbatim with no level semantics).
 *
 * @param {string} line - One raw payload line (CR already stripped, non-empty)
 * @param {Set<string>|null} podSet - Requested pods in the pod-tagged view, else null
 * @param {Set<string>|null} containerSet - Requested containers in the container-tagged view, else null
 * @returns {{ts: string, tsMs: number, text: string, pod: string, podId: string, container: string}}
 */
export function parseLogLine(line, podSet, containerSet) {
  // Peel pod then container, but commit the peel only if a real timestamp follows:
  // a name-shaped first token is a tag only on an actual entry head.
  const peel = (s, set) => {
    const sp = s.indexOf(' ')
    if (sp > 0 && set.has(s.slice(0, sp))) return [s.slice(0, sp), s.slice(sp + 1)]
    return null
  }
  let cursor = line
  let pod = ''
  let container = ''
  if (podSet) { const r = peel(cursor, podSet); if (r) { pod = r[0]; cursor = r[1] } }
  if (containerSet) { const r = peel(cursor, containerSet); if (r) { container = r[0]; cursor = r[1] } }
  const am = cursor.match(TIMESTAMP_PREFIX)
  // Require a real timestamp, not a date-shaped token, after the peels; otherwise
  // the tokens weren't tags — restore the original line and drop the peeled names.
  const tagged = !!am && !Number.isNaN(Date.parse(am[1]))
  const rest = tagged ? cursor : line
  if (!tagged) { pod = ''; container = '' }
  const m = rest.match(TIMESTAMP_PREFIX)
  const tsMs = m ? Date.parse(m[1]) : NaN
  const hasTs = !Number.isNaN(tsMs)
  const ts = hasTs ? m[1] : ''
  const text = stripAnsi(hasTs ? rest.slice(m[0].length) : rest)
  const podId = pod ? pod.slice(pod.lastIndexOf('-') + 1) : ''
  return { ts, tsMs, text, pod, podId, container }
}

/**
 * reconcileEntries - incrementally parse `lines` against the previously parsed
 * buffer, reusing the cached entry objects for lines unchanged since the last
 * poll and calling `parse` only on the appended tail. mergeLogs only appends and
 * front-evicts, so `prevLines` is either an exact prefix of `lines` (the buffer
 * grew) or, once the buffer is capped, a suffix of `prevLines` equals the head of
 * `lines` (front-eviction dropped some heads).
 *
 * Two paths:
 * - No eviction: `lines` starts with all of `prevLines`. The endpoint check is a
 *   cheap guard, not the correctness basis — it is sound only because the caller
 *   guarantees an append (mergeLogs is append-only and every non-append fetch
 *   bumps fetchGenRef, which forces a full re-parse before this is ever reached).
 * - Eviction: scan for the drop offset, then VERIFY the whole overlap and fall
 *   back to a full parse on any divergence, so the result always equals
 *   `lines.map(parse)` and the parsed mirror can never drift from the buffer.
 *
 * Exported so tests can prove the reuse mechanism (parse call count + object
 * identity) directly, not just the end-to-end output.
 *
 * @param {string[]} prevLines - the previously parsed lines, in arrival order
 * @param {Array} prevEntries - their parsed entries, 1:1 with prevLines
 * @param {string[]} lines - the current buffer lines, in arrival order
 * @param {(line: string) => object} parse - parses one raw line into an entry
 * @returns {Array} entries, 1:1 with lines
 */
export function reconcileEntries(prevLines, prevEntries, lines, parse) {
  const p = prevLines.length
  if (p === 0) return lines.map(parse)
  // No eviction: the buffer grew (or is unchanged), so prevLines is an exact
  // prefix. Reuse every cached entry, parse only the appended tail.
  if (lines.length >= p && lines[0] === prevLines[0] && lines[p - 1] === prevLines[p - 1]) {
    return prevEntries.concat(lines.slice(p).map(parse))
  }
  // Front-eviction at the buffer cap dropped some head lines. Find the eviction
  // offset (prevLines.slice(drop) == lines head), verify the overlap, then reuse
  // the surviving entries and parse the appended tail. The verify loop (not just
  // the offset match) makes this correct even when lines[0] also appears earlier
  // as a duplicate; any divergence falls through to a full parse.
  let drop = 1
  while (drop < p && prevLines[drop] !== lines[0]) drop++
  const reuse = p - drop
  if (lines.length >= reuse) {
    let ok = true
    for (let i = 0; ok && i < reuse; i++) {
      if (prevLines[drop + i] !== lines[i]) ok = false
    }
    if (ok) return prevEntries.slice(drop).concat(lines.slice(reuse).map(parse))
  }
  return lines.map(parse)
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
 * @param {boolean} [props.disabled] - When set, the button is non-interactive and dimmed
 * @param {any} props.children - The button icon
 */
function ToggleButton({ active, onClick, label, title, testid, disabled, children }) {
  return (
    <button
      onClick={onClick}
      disabled={disabled}
      class={`${ICON_TOGGLE_CLASS} ${active ? ACTIVE_CLASS : INACTIVE_CLASS} ${disabled ? 'opacity-40 cursor-not-allowed' : ''}`}
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
 * ToggleGroup - a labelled segmented control in the settings panel. One button
 * per option, the selected one highlighted; clicking calls onChange with its
 * value. Used for both the tail-lines and font-size settings.
 *
 * @param {Object} props
 * @param {string} props.label - Row label and group aria-label
 * @param {Array<{value: any, label: string, testid: string}>} props.options - The choices
 * @param {any} props.value - The currently selected option value
 * @param {Function} props.onChange - Called with the chosen value
 * @param {string} props.testid - data-testid for the group container
 */
function ToggleGroup({ label, options, value, onChange, testid }) {
  return (
    <div class="flex items-center gap-3">
      <span class="text-xs font-medium text-gray-600 dark:text-gray-300 w-20 flex-shrink-0">{label}</span>
      <div
        class="flex w-60 max-w-full rounded-md border border-gray-300 dark:border-gray-600 overflow-hidden"
        role="group"
        aria-label={label}
        data-testid={testid}
      >
        {options.map((o, i) => (
          <button
            key={o.value}
            type="button"
            onClick={() => onChange(o.value)}
            class={`flex-1 px-3 py-1 text-xs text-center focus:outline-none focus-visible:ring-2 focus-visible:ring-flux-blue focus-visible:ring-inset ${
              i > 0 ? 'border-l border-gray-300 dark:border-gray-600' : ''
            } ${
              value === o.value
                ? 'bg-flux-blue text-white'
                : 'bg-white dark:bg-gray-700 text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-600'
            }`}
            data-testid={o.testid}
            aria-pressed={value === o.value}
          >
            {o.label}
          </button>
        ))}
      </div>
    </div>
  )
}

// Level filter options for the native select: "All levels" plus one entry per
// level. Constant.
const LEVEL_OPTIONS = [
  { value: 'all', label: 'All levels' },
  ...LEVELS.map((l) => ({ value: l, label: LEVEL_META[l].label }))
]

// Shared monospace body styling for a formatted log line: a horizontally
// scrollable, wrapping row. The block/spans/plain-code variants share it; the
// JSON <pre> reuses the style with a margin reset.
const LINE_CLASS = 'overflow-x-auto font-mono whitespace-pre-wrap break-all text-gray-800 dark:text-gray-200'
// font-size is driven by the --logs-font-size CSS variable set on the log body
// container from the persisted font-size setting, falling back to 12px.
const LINE_STYLE = 'padding: 0.25rem 0.75rem; font-size: var(--logs-font-size, 12px); line-height: 1.5;'

/**
 * LogLineBody - renders the formatted body of one log line: a structured block,
 * highlighted spans, pretty-printed JSON, or plain wrapping text. Shared by a
 * single-line entry, a folded group's head, and its expanded frames so all three
 * render identically.
 *
 * @param {Object} props
 * @param {{text: string, code: boolean, fmt: Object|null}} props.entry - A formatted line
 */
function LogLineBody({ entry }) {
  if (entry.fmt && entry.fmt.kind === 'block') {
    // Structured entry: one container per entry (the per-entry logs-line
    // invariant), one inner row per visual line.
    return (
      <div class={LINE_CLASS} style={LINE_STYLE} data-testid="logs-line">
        {entry.fmt.rows.map((row, r) => (
          <div key={r} data-testid="logs-line-row">
            {row.map((s, j) => (s.cls ? <span key={j} class={s.cls}>{s.text}</span> : s.text))}
          </div>
        ))}
      </div>
    )
  }
  if (entry.fmt && entry.fmt.kind === 'spans') {
    // Unstructured entry highlighted in place: a single row.
    return (
      <div class={LINE_CLASS} style={LINE_STYLE} data-testid="logs-line">
        {entry.fmt.spans.map((s, j) => (s.cls ? <span key={j} class={s.cls}>{s.text}</span> : s.text))}
      </div>
    )
  }
  if (entry.fmt && entry.fmt.kind === 'json') {
    // Pretty-printed JSON: a <pre> preserves the baked-in indentation/newlines
    // and scrolls long values horizontally.
    return (
      <pre
        class="overflow-x-auto font-mono text-gray-800 dark:text-gray-200"
        style={`margin: 0; ${LINE_STYLE}`}
        data-testid="logs-line"
      >
        {entry.fmt.spans.map((s, j) => (s.cls ? <span key={j} class={s.cls}>{s.text}</span> : s.text))}
      </pre>
    )
  }
  if (entry.code) {
    // Plain line (no formatter match, or a continuation line such as a stack
    // frame): wrap like the decorated rows, not the non-wrapping JSON <pre>.
    return (
      <div class={LINE_CLASS} style={LINE_STYLE} data-testid="logs-line">
        {entry.text}
      </div>
    )
  }
  return (
    <div
      class="px-3 py-0.5 font-mono whitespace-pre-wrap break-all text-gray-800 dark:text-gray-200"
      style="font-size: var(--logs-font-size, 12px); line-height: 1.5;"
      data-testid="logs-line"
    >
      {entry.text}
    </div>
  )
}

/**
 * LogSeparator - the level-colored timestamp pill on a logical entry's separator
 * row. Shows the pod id in the all-pods view and glows when this is the freshly
 * appended entry. `level` is the group level (a recognized trace head is bumped to
 * error), so the pill matches the level filter and footer counts.
 *
 * @param {Object} props
 * @param {{ts: string, pod: string, podId: string}} props.entry - The head entry
 * @param {string} props.level - The group's normalized level
 * @param {boolean} props.isLatest - Whether to glow as the freshly appended entry
 */
function LogSeparator({ entry, level, isLatest }) {
  const meta = LEVEL_META[level] || LEVEL_META[DEFAULT_LEVEL]
  return (
    <div
      class="flex items-center gap-2 px-3 pt-2 select-none"
      data-testid="logs-timestamp"
      data-level={level}
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
  )
}

// Folded lines shown when a group is expanded before a "show all" link, so a
// pathological multi-thousand-frame trace doesn't render all at once.
const EXPANDED_LINE_CAP = 500

/**
 * LogGroup - one logical log entry. Two multi-line shapes render differently:
 *
 *  - **stack trace** (`group.isTrace`) is *folded*: head visible, frames hidden
 *    behind a click-to-expand control that caps a long expansion with a "show all"
 *    link, so a multi-thousand-frame trace doesn't render at once.
 *  - **unstructured burst** is *grouped but not folded*: every line renders under
 *    the one timestamp pill, nothing hidden — a curl -v dump reads as a single
 *    timestamped block.
 *
 * A single-line entry renders just its head. A headless group (head === null, a
 * buffer opened mid-trace) skips the separator pill and renders its lines through
 * the same path.
 *
 * @param {Object} props
 * @param {Object} props.group - A display group: head, formatted lines, and metadata
 * @param {boolean} props.formatted - Whether formatted mode is on (renders the pill)
 * @param {boolean} props.isLatest - Whether to glow as the freshly appended entry
 */
function LogGroup({ group, formatted, isLatest }) {
  const [expanded, setExpanded] = useState(false)
  const [showAll, setShowAll] = useState(false)
  const lines = group.lines

  const multi = lines.length > 1
  // A trace folds (head + click-to-expand); a burst groups without folding.
  const foldable = group.isTrace && multi
  // Frames exclude folded blank lines (a Go panic's blank, a trailing Node blank)
  // so the "N frames" count isn't inflated.
  const frameCount = lines.slice(1).filter(l => l.text !== '').length
  const hiddenCount = lines.length - 1
  const shown = showAll ? lines.length : Math.min(lines.length, EXPANDED_LINE_CAP + 1)

  return (
    <>
      {formatted && group.head && <LogSeparator entry={group.head} level={group.level} isLatest={isLatest} />}
      <LogLineBody entry={lines[0]} />
      {foldable && (
        <>
          <button
            type="button"
            onClick={() => setExpanded(v => !v)}
            class="flex items-center gap-1 px-3 py-0.5 text-[11px] font-mono text-gray-500 hover:text-flux-blue dark:text-gray-400 dark:hover:text-flux-blue select-none focus:outline-none focus-visible:ring-2 focus-visible:ring-flux-blue rounded"
            data-testid="logs-group-fold"
            aria-expanded={expanded}
          >
            <span aria-hidden="true">{expanded ? '▾' : '▸'}</span>
            {frameCount > 0
              ? `${frameCount} frame${frameCount === 1 ? '' : 's'}`
              : `+${hiddenCount} line${hiddenCount === 1 ? '' : 's'}`}
          </button>
          {expanded && (
            <div class="border-l-2 border-gray-200 dark:border-gray-700 ml-3" data-testid="logs-group-frames">
              {lines.slice(1, shown).map((l, j) => <LogLineBody key={j} entry={l} />)}
              {shown < lines.length && (
                <button
                  type="button"
                  onClick={() => setShowAll(true)}
                  class="px-3 py-0.5 text-[11px] font-mono text-flux-blue hover:underline select-none"
                  data-testid="logs-group-showall"
                >
                  show all {hiddenCount} lines
                </button>
              )}
            </div>
          )}
        </>
      )}
      {/* Burst: not folded — render every remaining line under the one pill. */}
      {!group.isTrace && multi && lines.slice(1).map((l, j) => <LogLineBody key={j} entry={l} />)}
    </>
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
  // Follow, formatted and tailLines are seeded from the persisted log viewer
  // settings (peek, so seeding doesn't subscribe the component) and written back
  // by the effect below, so they carry across sessions.
  const [tailLines, setTailLines] = useState(() => logSettings.peek().tail)
  const [follow, setFollow] = useState(() => logSettings.peek().follow)
  const [filter, setFilter] = useState('')
  const [levelFilter, setLevelFilter] = useState('all')
  const [formatted, setFormatted] = useState(() => logSettings.peek().formatted)
  // Monospace font size for the log body, seeded from the persisted setting and
  // applied via the --logs-font-size CSS variable on the body container.
  const [fontSize, setFontSize] = useState(() => logSettings.peek().fontSize)
  // Whether the settings panel (tail-lines slider, font size) is expanded below
  // the toolbar. Local to the session, not persisted.
  const [showSettings, setShowSettings] = useState(false)
  // Field-selection expression for the formatted JSON body (e.g. "msg|error",
  // "!level ts", "*id"). Seeded from and persisted to the log settings so it carries
  // across sessions. See fieldMatcher for the syntax; empty = all fields.
  const [fieldExpr, setFieldExpr] = useState(() => logSettings.peek().fields)
  const [fullScreen, setFullScreen] = useState(false)
  const [logs, setLogs] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState(null)
  // A failed follow poll, shown inline at the tail of the feed so the buffer
  // stays visible. The full-pane `error` is used only when there is no buffer.
  const [followError, setFollowError] = useState(null)
  const [flashLatest, setFlashLatest] = useState(false)
  // Number of entries the most recent follow poll appended, shown as a clickable
  // "+N new" indicator until the next poll replaces the count (zero hides it).
  const [appendedCount, setAppendedCount] = useState(0)
  // Whether the current buffer's lines are pod-tagged and/or container-tagged (set
  // from the response, so the parser never has to guess the wire format). Pod
  // tagging marks the all-pods view, container tagging the all-containers view;
  // both can be set in the combined view. Reset with the buffer.
  const [tagged, setTagged] = useState(false)
  const [containerTagged, setContainerTagged] = useState(false)
  // Coverage of the "All pods" view: { streamed, total, forbidden } when the
  // response did not cover every requested pod, else null.
  const [partial, setPartial] = useState(null)

  const bodyRef = useRef(null)
  const prevLogsRef = useRef('')
  // Mirror of the current `logs` buffer, so a follow poll can count the entries it
  // appends against the live buffer without listing `logs` in fetchLogs' deps.
  const logsRef = useRef('')
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
  // lines instead of the whole buffer each poll. See formattedLines for pruning.
  const formatCacheRef = useRef(new Map())
  // Memoizes each line's top-level JSON keys (string[] | null) by text, so the field
  // discovery pass parses each unique line once. Rebuilt over the buffer like
  // formatCacheRef, so it can't outgrow it.
  const jsonKeysCacheRef = useRef(new Map())
  // The parsed buffer, mirroring `logs` line-for-line in arrival order (pre-sort,
  // pre-format), so a follow poll re-parses only the appended lines instead of the
  // whole buffer. `key` is the tag regime and `gen` the reset-fetch id; a change in
  // either forces a full re-parse (see baseLines). `logs` (the string) stays
  // authoritative — this is a derived mirror that must never diverge from it.
  const parsedRef = useRef({ key: null, gen: -1, lines: [], entries: [] })

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
        // Count the new entries against the live buffer before merging, so the
        // "+N new" indicator reflects what this poll appended. It's set on every
        // poll (0 hides it), so the count persists until the next refresh replaces it.
        // The merge itself stays exact via the functional update against the real
        // prev; the count is computed against logsRef (a render-synced mirror) and is
        // best-effort: if two follow polls overlap (a fetch slower than the interval),
        // it may briefly over-report, never corrupting the deduped buffer.
        const added = countNewEntries(logsRef.current, incoming)
        setLogs(prev => mergeLogs(prev, incoming))
        setAppendedCount(added)
      } else {
        // A reset replaces the buffer; no "appended" count applies.
        setAppendedCount(0)
        setLogs(incoming)
      }
      // The response states whether its lines are pod-tagged and how many requested
      // pods it covered, so the parser and footer don't guess. Refreshed every fetch
      // (a pod becoming readable can clear a partial result).
      setTagged(!!resp?.tagged)
      setContainerTagged(!!resp?.containerTagged)
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
        // The buffer is gone, so a leftover "+N new" from a prior poll is stale.
        setAppendedCount(0)
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

  // Raw mode is verbatim text with no level semantics: the level filter is hidden
  // and reset to "all" so a filter set in formatted mode doesn't silently narrow the
  // raw view (where the level pills/legend are gone). On mount formatted is true, so
  // this is a no-op until the user switches to raw.
  useEffect(() => {
    if (!formatted) setLevelFilter('all')
  }, [formatted])

  // Persist the follow/formatted/tailLines/fontSize/fields settings so they carry
  // across sessions. The signal's own effect writes them to localStorage. On mount
  // this writes the seeded values back (a redundant but harmless write of identical
  // content). The component seeds via peek() and never reads the signal reactively,
  // so writing it here cannot re-render the viewer or feed back into this effect.
  useEffect(() => {
    logSettings.value = { follow, formatted, tail: tailLines, fontSize, fields: fieldExpr }
  }, [follow, formatted, tailLines, fontSize, fieldExpr])

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

  // Split the raw payload into entries. In the "All pods" view each timestamped
  // line is prefixed with its pod of origin ("<pod> <ts> <msg>"); a token is only
  // treated as a pod tag when it is one of the pods we requested AND the next token
  // parses as a timestamp, so a message that merely starts with a word + timestamp
  // is never mistaken for a tag. The leading timestamp (shown as a separator pill)
  // and any ANSI escapes are then stripped.
  //
  // Parsing is incremental: a follow poll re-parses only the lines mergeLogs
  // appended, reusing the cached entry objects for lines already parsed (parsedRef).
  // A full re-parse is forced only when the tag regime changes (peeling differs) or
  // a reset fetch replaced the buffer (fetchGenRef bumps on every non-append fetch).
  // mergeLogs only appends and front-evicts, so on an append the surviving cached
  // lines are a suffix of the previous buffer equal to the head of the new one.
  const baseLines = useMemo(() => {
    const podSet = tagged ? new Set(reqPodsStr ? reqPodsStr.split(',') : []) : null
    const containerSet = containerTagged ? new Set(reqContainersStr ? reqContainersStr.split(',') : []) : null
    // Strip a trailing CR so a CRLF stream doesn't leak `\r` into the text, fields,
    // or filter. (Download uses the raw payload, so it stays byte-verbatim.)
    const lines = logs.split('\n').map(line => line.replace(/\r$/, '')).filter(line => line.length > 0)
    const parse = (line) => parseLogLine(line, podSet, containerSet)

    // The parse regime is keyed by the tagging signals; `gen` (fetchGenRef, bumped
    // only on a non-append fetch) is the replace signal. A change in either means
    // the buffer was rebuilt, not appended, so the prior mirror can't be trusted
    // and every line re-parses. Otherwise reconcileEntries reuses the cached
    // entries for unchanged lines and parses only the appended tail. Reading the
    // ref here (not via deps) is safe because Preact's useMemo recomputes only on a
    // dep change, and an append is the only same-gen/same-key mutation of `logs`.
    const cache = parsedRef.current
    const gen = fetchGenRef.current
    const key = `${tagged}|${reqPodsStr}|${containerTagged}|${reqContainersStr}`
    const entries = (cache.key !== key || cache.gen !== gen)
      ? lines.map(parse)
      : reconcileEntries(cache.lines, cache.entries, lines, parse)
    // Mirror the arrival-order parse before the derived sort below, so the next poll
    // diffs against the unsorted buffer. The sort returns a fresh array and never
    // mutates these entries.
    parsedRef.current = { key, gen, lines, entries }

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
  }, [logs, tagged, reqPodsStr, containerTagged, reqContainersStr])

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

  // Grouping is a formatted-mode affordance: raw mode renders every line flat, so it
  // is bypassed when not formatted. There is no separate toggle — a trace starts
  // folded (click to expand), a burst renders all its lines under one timestamp pill.
  const grouping = formatted

  // Format each base line once, before grouping, so a line's `level`, `fmt`
  // descriptor, and `structured` flag are known when groupEntries decides whether to
  // join an unstructured burst. Level is detected HERE (formatted mode only), not in
  // parseLogLine: raw mode is verbatim with no level semantics, so it skips
  // detectLevel entirely. In formatted mode a line gains { level, code, fmt,
  // structured }; raw mode passes baseLines through (no level/fmt — grouping is off,
  // the level filter/legend are hidden, LogLineBody falls through to plain).
  //
  // Cached by raw line text (formatCacheRef): an append reuses cached results and
  // only parses+formats the new lines. The cache is rebuilt over the whole base
  // buffer (≤ MAX_BUFFER_LINES), so it can't outgrow it; raw mode leaves it untouched
  // so it survives a round-trip back to formatted.
  // Discover the JSON fields the buffer emits (union of top-level keys, in first-seen
  // order), so the settings panel can offer a field picker. Independent of the field
  // selection, so filtering never removes options. Scans every line that projection
  // (highlightJson in formattedLines) sees — topLevelJsonKeys returns null for
  // non-JSON lines, which contribute nothing — so a JSON line without a leading
  // timestamp is still discovered, keeping the input visible and consistent with the
  // projected body. Each unique line's keys are cached by text (jsonKeysCacheRef) to
  // parse it once. Empty in raw mode (no field concept).
  const availableFields = useMemo(() => {
    if (!formatted) return []
    const prev = jsonKeysCacheRef.current
    const next = new Map()
    const seen = new Set()
    for (const entry of baseLines) {
      let keys = next.get(entry.text)
      if (keys === undefined) keys = prev.has(entry.text) ? prev.get(entry.text) : topLevelJsonKeys(entry.text)
      next.set(entry.text, keys)
      if (keys) for (const k of keys) seen.add(k)
    }
    jsonKeysCacheRef.current = next
    return [...seen]
  }, [baseLines, formatted])

  // Resolve the field expression to the concrete set of top-level keys to show, or
  // null for "all". Applying the matcher to the discovered fields (the union over the
  // buffer) yields a stable set every JSON line is projected against; a pure-exclusion
  // expression keeps newly seen fields once they enter availableFields.
  const selectedFields = useMemo(() => {
    const match = fieldMatcher(fieldExpr)
    if (!match) return null
    return new Set(availableFields.filter(match))
  }, [fieldExpr, availableFields])

  // Field-selection signature for the format cache key: 'all' when unfiltered, else a
  // JSON-encoded array of the sorted selected keys (encoded so a key containing the
  // delimiter can't make two different selections collide). A change busts only the
  // JSON lines' cached formatting.
  const fieldSig = useMemo(
    () => (selectedFields === null ? 'all' : JSON.stringify([...selectedFields].sort())),
    [selectedFields]
  )

  const formattedLines = useMemo(() => {
    if (!formatted) return baseLines
    const prev = formatCacheRef.current
    const next = new Map()
    const result = baseLines.map(entry => {
      // The cache key carries the head flag since the same text can appear as both a
      // head and a continuation line, and the field signature since the JSON body
      // depends on the selected fields. JSON runs on every line; the klog/zap/logfmt
      // cascade only on a timestamped one (a frame's shape falls through to plain).
      const key = `${entry.ts ? 1 : 0}\n${fieldSig}\n${entry.text}`
      let f = next.get(key) || prev.get(key)
      if (!f) {
        const level = detectLevel(entry.text)
        const json = highlightJson(entry.text, selectedFields)
        const d = json || (entry.ts ? decorateLine(entry.text, level) : { kind: 'plain' })
        // `structured` (whether the burst grouper treats the line as a real log
        // entry vs structure-less noise) is true when the format layer decorates it,
        // OR when a level was detected even though decorateLine fell through to
        // plain. The level catch closes the decorateLine gap: a date-less console
        // line like `[main] DEBUG com.x - msg` renders plain (no leading digit for
        // parseJava) but detectLevel finds `debug`, so it is not swept into a burst.
        // A curl -v dump carries no level (default info), so it still groups.
        const leveled = level !== DEFAULT_LEVEL
        f = d.kind === 'plain'
          ? { level, code: true, fmt: null, structured: leveled }
          : { level, code: false, fmt: d, structured: true }
      }
      next.set(key, f)
      return { ...entry, ...f }
    })
    formatCacheRef.current = next
    return result
  }, [baseLines, formatted, selectedFields, fieldSig])

  // Group recognized stack traces and unstructured bursts, one run per group (off →
  // one group per entry, keeping the chain uniform). When the stream is tagged (more
  // than one pod and/or container), partition by origin (pod + container) then order
  // groups by head ts: every line is timestamped, so two streams crashing together
  // interleave frame-by-frame and grouping the flat list would fragment each run. The
  // partition keeps each origin's trace intact; groupEntries' samePod/sameContainer
  // guard is then a backstop. A ts==='' orphan inherits the previous line's origin,
  // so it buckets with its head. A single untagged stream skips the partition.
  const groups = useMemo(() => {
    if (!tagged && !containerTagged) return groupEntries(formattedLines, grouping)
    const byOrigin = new Map()
    let lastPod = ''
    let lastContainer = ''
    for (const e of formattedLines) {
      const pod = e.pod || lastPod
      const container = e.container || lastContainer
      if (e.pod) lastPod = e.pod
      if (e.container) lastContainer = e.container
      const origin = `${pod}\n${container}`
      let bucket = byOrigin.get(origin)
      if (!bucket) { bucket = []; byOrigin.set(origin, bucket) }
      bucket.push(e)
    }
    // A group with no timestamped head inherits the previous group's ts so it stays
    // adjacent instead of floating to the front; a leading orphan keeps NaN (first).
    const ordered = [...byOrigin.values()].flatMap(es => {
      const gs = groupEntries(es, grouping)
      let sortTs = NaN
      for (const g of gs) {
        if (!Number.isNaN(g.tsMs)) sortTs = g.tsMs
        g.sortTs = sortTs
      }
      return gs
    })
    return ordered.sort((a, b) => {
      if (Number.isNaN(a.sortTs)) return Number.isNaN(b.sortTs) ? 0 : -1
      if (Number.isNaN(b.sortTs)) return 1
      return a.sortTs - b.sortTs
    })
  }, [formattedLines, tagged, containerTagged, grouping])

  // Substring filter over each group's whole block. Positive: keep if any line has
  // the needle (a match keeps the whole trace). "!needle": keep only if no line has
  // it (the exact complement — a buried frame drops the trace).
  const containsGroups = useMemo(() => {
    const raw = filter.trim()
    if (!raw) return groups
    const negate = raw.startsWith('!')
    const needle = (negate ? raw.slice(1) : raw).trim().toLowerCase()
    if (!needle) return groups
    return groups.filter(g => g.lines.some(l => l.text.toLowerCase().includes(needle)) !== negate)
  }, [groups, filter])

  // Per-level tally over the contains-filtered groups, before the level filter (so
  // the footer shows error counts while viewing one level). One per logical entry,
  // so the legend matches the rendered list.
  const levelCounts = useMemo(() => {
    const counts = {}
    for (const g of containsGroups) counts[g.level] = (counts[g.level] || 0) + 1
    return counts
  }, [containsGroups])

  // Apply the level filter on top of the contains filter (exact group-level match).
  // Lines already carry their fmt/code from formattedLines, so this is the final set
  // the render maps — no separate formatting pass. The filter is forced to "all" in
  // raw mode (which carries no level): a setLevelFilter('all') effect resets the
  // state, but deriving the effective level here too closes the one-frame window
  // where the toggle has flipped to raw before that effect runs — otherwise a
  // lingering 'warn' filter would blank the raw list for a frame (no group has a
  // level), flashing "No matching log entries".
  const effectiveLevel = formatted ? levelFilter : 'all'
  const logGroups = useMemo(() => {
    if (effectiveLevel === 'all') return containsGroups
    return containsGroups.filter(g => g.level === effectiveLevel)
  }, [containsGroups, effectiveLevel])

  // Briefly highlight the most recent visible entry when new entries arrive (e.g.
  // while following). Gated on two conditions so it only signals fresh logs: the raw
  // buffer grew (prevLogs !== logs, skipping the first load) and the newest displayed
  // line changed. The second gate stops a poll that appends only filtered-out lines,
  // or a filter change reshuffling the view, from re-pulsing an unchanged last entry.
  // Keys off the last group's last line, so a frame appended to an open trace (which
  // folds into the group rather than adding a row) still triggers the highlight.
  useEffect(() => {
    const prevLogs = prevLogsRef.current
    prevLogsRef.current = logs
    const lastGroup = logGroups.length > 0 ? logGroups[logGroups.length - 1] : null
    const latest = lastGroup ? lastGroup.lines[lastGroup.lines.length - 1] : null
    const latestKey = latest ? `${latest.ts}\n${latest.text}` : null
    const prevKey = prevLatestKeyRef.current
    prevLatestKeyRef.current = latestKey
    if (prevLogs === '' || logs === '' || prevLogs === logs) return
    if (latestKey === null || latestKey === prevKey) return
    setFlashLatest(true)
    const id = setTimeout(() => setFlashLatest(false), HIGHLIGHT_DURATION)
    return () => clearTimeout(id)
  }, [logs, logGroups])

  // Keep logsRef in sync with the buffer so a follow poll can diff its append
  // against the live buffer (fetchLogs intentionally omits `logs` from its deps).
  useEffect(() => { logsRef.current = logs }, [logs])

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
  }, [logGroups, followError])

  // Download the current logs as a text file, named after the pod for a single
  // pod or the workload for the all-pods view.
  // Resolve the selected font-size key to its pixel value for the CSS variable,
  // defaulting to medium if a stale key somehow slips through.
  const fontSizePx = useMemo(
    () => (FONT_SIZES.find(f => f.key === fontSize) || FONT_SIZES[1]).px,
    [fontSize]
  )

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

        {/* Toolbar. Mobile: two stacked rows — controls+filter, then selectors
            (the viewer is full-screen there). From sm up: a single-row flex-wrap.
            Each row wrapper is `sm:contents` so it dissolves into that one row. */}
        <div class="flex flex-col gap-2 px-4 py-2 border-b border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800 sm:flex-row sm:flex-wrap sm:items-center">
          {/* Row 1 (mobile): follow/format toggles, lines, and the contains filter. */}
          <div class="flex items-center gap-2 sm:contents">
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

            {/* Settings — expands a panel below the toolbar with the tail-lines
                slider and the font size toggle group. */}
            <ToggleButton
              active={showSettings}
              onClick={() => setShowSettings(v => !v)}
              label="Log settings"
              title="Tail lines and font size"
              testid="logs-settings-toggle"
            >
              <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.343 3.94c.09-.542.56-.94 1.11-.94h1.093c.55 0 1.02.398 1.11.94l.149.894c.07.424.384.764.78.93.398.164.855.142 1.205-.108l.737-.527a1.125 1.125 0 0 1 1.45.12l.773.774c.39.389.44 1.002.12 1.45l-.527.737c-.25.35-.272.806-.107 1.204.165.397.505.71.93.78l.893.15c.543.09.94.56.94 1.109v1.094c0 .55-.397 1.02-.94 1.11l-.894.149c-.424.07-.764.383-.929.78-.165.398-.143.854.107 1.204l.527.738c.32.447.27 1.06-.12 1.45l-.774.773a1.125 1.125 0 0 1-1.449.12l-.738-.527c-.35-.25-.806-.272-1.203-.107-.397.165-.71.505-.781.929l-.149.894c-.09.542-.56.94-1.11.94h-1.094c-.55 0-1.019-.398-1.11-.94l-.148-.894c-.071-.424-.384-.764-.781-.93-.398-.164-.854-.142-1.204.108l-.738.527c-.447.32-1.06.27-1.45-.12l-.773-.774a1.125 1.125 0 0 1-.12-1.45l.527-.737c.25-.35.273-.806.108-1.204-.165-.397-.505-.71-.93-.78l-.893-.15c-.543-.09-.94-.56-.94-1.109v-1.094c0-.55.397-1.02.94-1.11l.894-.149c.424-.07.764-.383.929-.78.165-.398.143-.854-.107-1.204l-.527-.738a1.125 1.125 0 0 1 .12-1.45l.773-.773a1.125 1.125 0 0 1 1.45-.12l.737.527c.35.25.807.272 1.204.107.397-.165.71-.505.78-.929l.15-.894Z" />
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z" />
              </svg>
            </ToggleButton>

            {/* Separator (desktop only) between the view controls and the filter. */}
            <div class="hidden sm:block w-px h-5 bg-gray-300 dark:bg-gray-600" />

            {/* Contains filter — takes the remaining width of row 1 on mobile. */}
            <input
              type="text"
              value={filter}
              onInput={(e) => setFilter(e.target.value)}
              placeholder="contains…"
              class={`${INPUT_CLASS} flex-1 min-w-0 sm:flex-none sm:w-40`}
              data-testid="logs-filter-input"
              aria-label="Filter log lines containing text"
              title="Filter by keyword (prefix with ! to exclude e.g. !debug)"
            />
          </div>

          {/* Row 2 (mobile): the pod/container/level selectors, sharing the row in
              equal parts. `sm:contents` dissolves the wrapper on desktop so they
              rejoin the single wrapping toolbar row. */}
          <div class="flex items-center gap-2 sm:contents">
            {/* Pod select. "All pods" leads (every pod merged, origin-tagged);
                selecting one narrows the view and resets the container dropdown. A
                long name truncates instead of pushing the row. */}
            {pods.length > 0 && (
              <select
                value={effectivePodKey}
                onChange={(e) => setPodKey(e.target.value)}
                class={`${SELECT_CLASS} flex-1 min-w-0 truncate sm:flex-none sm:w-40`}
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
                "(previous)" entry for the prior instance's logs. */}
            <select
              value={containerKey}
              onChange={(e) => setContainerKey(e.target.value)}
              class={`${SELECT_CLASS} flex-1 min-w-0 truncate sm:flex-none sm:w-40`}
              data-testid="logs-container-select"
              aria-label="Container"
              title="Select container (a previous entry reads the prior instance's logs)"
            >
              {containerOptions.map((o) => (
                <option key={o.key} value={o.key}>{o.label}</option>
              ))}
            </select>

            {/* Minimum level filter. Formatted mode only — raw is verbatim with no
                level semantics, so the filter is hidden (and reset to "all"). */}
            {formatted && (
              <select
                value={levelFilter}
                onChange={(e) => setLevelFilter(e.target.value)}
                class={`${SELECT_CLASS} flex-1 min-w-0 truncate sm:flex-none sm:w-40`}
                data-testid="logs-level-filter"
                aria-label="Log level"
                title="Show only logs of this level"
              >
                {LEVEL_OPTIONS.map((o) => (
                  <option key={o.value} value={o.value}>{o.label}</option>
                ))}
              </select>
            )}
          </div>

          {/* Actions (desktop only): download + fullscreen are hidden on mobile,
              where the viewer already fills the screen. */}
          <div class="hidden sm:flex items-center justify-end gap-1 sm:ml-auto">
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

        {/* Settings panel: expands below the toolbar with the tail-lines slider and
            the font size toggle group. Both apply on the spot and persist via the
            settings effect above. */}
        {showSettings && (
          <div
            class="flex flex-col gap-3 px-4 py-3 border-b border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800"
            data-testid="logs-settings-panel"
          >
            <ToggleGroup
              label="Tail lines"
              testid="logs-tail-group"
              value={tailLines}
              onChange={setTailLines}
              options={TAIL_LINES.map(n => ({ value: n, label: n >= 1000 ? `${n / 1000}K` : String(n), testid: `logs-tail-${n}` }))}
            />
            <ToggleGroup
              label="Font size"
              testid="logs-fontsize-group"
              value={fontSize}
              onChange={setFontSize}
              options={FONT_SIZES.map(f => ({ value: f.key, label: f.label, testid: `logs-fontsize-${f.key}` }))}
            />
            {/* JSON field filter. Formatted mode only, and only when the buffer emits
                structured JSON fields. A list of field names (space/comma/pipe
                separated) with `*` globs and `!` to exclude; empty shows all. The
                placeholder shows example syntax; the expression persists via the log
                settings (see fieldExpr). */}
            {formatted && availableFields.length > 0 && (
              <div class="flex items-center gap-3">
                <label for="logs-fields-input" class="text-xs font-medium text-gray-600 dark:text-gray-300 w-20 flex-shrink-0">Fields</label>
                <input
                  id="logs-fields-input"
                  type="text"
                  value={fieldExpr}
                  onInput={(e) => setFieldExpr(e.target.value)}
                  placeholder="msg error !level *id"
                  class={`${INPUT_CLASS} w-60 max-w-full font-mono`}
                  data-testid="logs-fields-input"
                  aria-label="Show only these JSON fields"
                  title="Field names separated by space, comma or pipe. Use * to glob and ! to exclude (e.g. msg|error, !level, *id). Empty shows all."
                  autocomplete="off"
                  autocapitalize="off"
                  spellcheck={false}
                />
              </div>
            )}
          </div>
        )}

        {/* Body */}
        <div ref={bodyRef} onScroll={handleScroll} style={`--logs-font-size: ${fontSizePx}px`} class="flex-1 overflow-auto overscroll-contain bg-white dark:bg-gray-950" data-testid="logs-body">
          {error ? (
            <div class="m-3 p-2 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded text-xs text-red-800 dark:text-red-200" data-testid="logs-error">
              {error}
            </div>
          ) : loading && !logs ? (
            <p class="p-3 text-xs text-gray-500 dark:text-gray-400" data-testid="logs-loading">Loading logs...</p>
          ) : logGroups.length > 0 || followError || podsGone ? (
            <div class="pb-2" data-testid="logs-content">
              {logGroups.map((group, i) => {
                // Content-identity key, not index: front-eviction and the all-pods
                // re-sort shift indices, which would bleed a folded group's expand
                // state onto another. Container + head text disambiguate a shared
                // pod+ts (two containers in one pod can emit the same line at the
                // same instant); a headless run falls back to the index.
                const key = group.head ? `${group.pod} ${group.container} ${group.ts} ${group.head.text}` : `orphan ${i}`
                // Raw mode strips styling: the highlight on the freshly appended
                // entry only fires in formatted mode.
                const isLatest = formatted && flashLatest && i === logGroups.length - 1
                return (
                  <LogGroup
                    key={key}
                    group={group}
                    formatted={formatted}
                    isLatest={isLatest}
                  />
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

        {/* Footer: in formatted mode a per-level count summary doubling as the
            color legend; in raw mode (no level semantics) a plain line count. The
            trailing corner shows the fetch loader while a request is in flight,
            else the live/snapshot mode. */}
        <div
          class="flex items-center justify-between gap-3 px-4 py-2 border-t border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800"
          data-testid="logs-footer"
        >
          <div class="flex flex-wrap items-center gap-x-3 gap-y-1 min-w-0">
            {formatted ? (
              <div class="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-gray-600 dark:text-gray-400" data-testid="logs-level-summary">
                {LEVELS.filter(l => levelCounts[l]).map((l) => (
                  <span key={l} class="inline-flex items-center gap-1" title={`${LEVEL_META[l].label}: ${levelCounts[l]}`}>
                    <span class={`w-2 h-2 rounded-full ${LEVEL_META[l].swatch}`} />
                    {LEVEL_META[l].label} {levelCounts[l]}
                  </span>
                ))}
              </div>
            ) : (
              <span class="text-xs text-gray-600 dark:text-gray-400" data-testid="logs-line-count">
                {logGroups.length} log line{logGroups.length === 1 ? '' : 's'}
              </span>
            )}
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
            {/* Transient indicator of how many entries the last follow poll appended.
                Clicking jumps to the latest line and re-pins the view to the bottom. */}
            {appendedCount > 0 && (
              <button
                type="button"
                onClick={scrollToBottom}
                class="inline-flex items-center gap-1 text-xs font-medium text-flux-blue hover:underline flex-shrink-0 focus:outline-none focus-visible:ring-2 focus-visible:ring-flux-blue rounded"
                data-testid="logs-appended-count"
                aria-live="polite"
                title="Scroll to latest logs"
              >
                <svg class="w-3.5 h-3.5 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19.5 8.25l-7.5 7.5-7.5-7.5" />
                </svg>
                +{appendedCount} new
              </button>
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
