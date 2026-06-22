// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { DEFAULT_LEVEL } from './logLevel'

// Opens a stack trace: Go panic/fatal/goroutine dump, Python Traceback, Java
// `Exception in thread`/FQCN throwable, Node/generic bare, prefixed, or bracketed
// `Error:`/`Exception:` (e.g. `TypeError [ERR_X]:`). Derived from real captures.
const TRACE_HEAD = /^(?:panic:|fatal error:|goroutine\s+\d+\s+\[|Traceback \(most recent call last\):|Exception in thread\s+"|(?:[a-z][\w$]*\.)+[A-Z][\w$]*(?:Exception|Error|Throwable)\b|(?:[A-Z][\w$]*)?(?:Error|Exception)(?:\s*\[[^\]]*\])?:)/

// Flush-left Go function frame (`main.c(...)`). No space before the paren (so
// `falling back (stale)` is excluded), and Go-gated via goTrace so a non-Go
// `Error:` trace never absorbs a stray `)`-line.
const GO_FRAME = /^\S+\(.*\)\s*$/

// Go panic/fatal/goroutine head — the only context GO_FRAME may fold in.
const GO_HEAD = /^(?:panic:|fatal error:|goroutine\s+\d+\s+\[)/

// Flush-left lines that continue an open trace: Java `Caused by:`/`Suppressed:`/
// `… N more`, goroutine separator, `created by`, `[signal …]`, suffix-less Python
// finals (KeyboardInterrupt…), a FQCN throwable, and a final `<Name>Error:`.
const MARKER = /^(?:Caused by:|Suppressed:|\.{3}\s+\d+\s+more\b|goroutine\s+\d+\s+\[|created by\s|\[signal\s|(?:KeyboardInterrupt|SystemExit|StopIteration|StopAsyncIteration|GeneratorExit)\b|(?:[a-z][\w$]*\.)+[A-Z][\w$]*(?:Exception|Error|Throwable)\b|(?:[A-Z][\w$]*)?(?:Error|Exception)(?:\s*\[[^\]]*\])?:)/

// isTraceHead reports whether a line opens a recognized stack trace.
function isTraceHead(text) { return TRACE_HEAD.test(text) }

// isCont reports whether a line continues an open trace: blank, indented, a
// flush-left marker, or — only in a Go trace — a flush-left Go frame.
function isCont(text, goTrace) {
  return text === '' || /^\s/.test(text) || MARKER.test(text) || (goTrace && GO_FRAME.test(text))
}

// Max gap between consecutive lines of an unstructured run. A real multi-line
// write (a curl -v dump, a banner) lands within milliseconds; a primitive app
// printing structure-less lines seconds apart stays separate entries.
const BURST_WINDOW_MS = 250

// Max total span of an unstructured run, from its head to the line being joined.
// The per-gap window alone would let a steady plain stream (each line < 250 ms
// after the last) coalesce without bound into one group under a single, ever more
// stale timestamp. A genuine burst (curl dump, banner) lands well within a second;
// past this span the run closes and a fresh group starts.
const BURST_SPAN_MS = 1000

/**
 * groupEntries - collect a multi-line run into its head entry so it renders as one
 * timestamp block instead of N rows. Two kinds of run group, each scoped to one
 * (pod, container) — a line from a different non-empty pod or container never joins
 * across, so streams interleaved in the all-pods/all-containers view can't merge:
 *
 *  - **stack trace** (`isTrace`) — a recognized head opens it and each continuation
 *    joins while `prev.isTrace && isCont(e.text, prev.goTrace)`. The renderer *folds*
 *    a trace: head visible, frames hidden behind a click-to-expand control. A head
 *    not in TRACE_HEAD does not open, so unknown-format frames stay separate;
 *    extend TRACE_HEAD as needed.
 *  - **unstructured burst** (`unstructuredRun`) — a structure-less line opens a run
 *    and each following structure-less line joins while it lands within BURST_WINDOW_MS
 *    of the previous line AND within BURST_SPAN_MS of the run's head; the first
 *    structured line, a too-large gap, or an over-long total span closes it.
 *    `e.structured` is set by the caller from the format layer (highlightJson/
 *    decorateLine); a curl -v dump groups, a structured logger 1–3 ms apart does not,
 *    and a continuous plain stream caps out instead of growing one group unbounded.
 *    The renderer does NOT fold a burst: every line shows under the one timestamp
 *    pill, nothing hidden. Mutually exclusive with a trace head (a bare `Error:`
 *    opens a trace, not a run).
 *
 * Content-driven: a frame may or may not carry a timestamp. The caller also
 * partitions by pod in the all-pods view, so samePod is a backstop; the container
 * guard is the load-bearing scope within a single pod's interleaved containers.
 *
 * @param {Array<{ts: string, tsMs: number, text: string, level: string, pod: string, podId: string, container: string, structured?: boolean}>} entries
 * @param {boolean} enabled - when false, every entry becomes its own group (no run)
 * @returns {Array<{head: Object|null, lines: Array, ts: string, tsMs: number, lastTsMs: number, pod: string, podId: string, container: string, level: string, isTrace: boolean, goTrace: boolean, unstructuredRun: boolean}>}
 */
export function groupEntries(entries, enabled) {
  const groups = []
  for (const e of entries) {
    const prev = groups[groups.length - 1]
    // An untimestamped orphan (pod/container '') belongs to its predecessor; a line
    // from a different (non-empty) pod or container never joins across.
    const samePod = prev && (e.pod === prev.pod || e.pod === '')
    const sameContainer = prev && (e.container === prev.container || e.container === '')
    const scoped = prev && samePod && sameContainer
    // isTrace labels the group and keeps the trace open for the following lines; a
    // structure-less, non-trace head opens an unstructured run instead.
    const isTrace = enabled && isTraceHead(e.text)
    const joinTrace = scoped && prev.isTrace && isCont(e.text, prev.goTrace)
    // A trace head is excluded from a burst (`!isTrace`) so it breaks the run and
    // opens its own trace rather than joining as a plain line. Two timers bound a
    // run: the per-gap window (close together) and the total span from the head
    // (`prev.tsMs`), so a continuous plain stream doesn't grow one group unbounded.
    const joinBurst = scoped && prev.unstructuredRun && !isTrace && !e.structured &&
      !Number.isNaN(e.tsMs) && e.tsMs - prev.lastTsMs < BURST_WINDOW_MS &&
      e.tsMs - prev.tsMs < BURST_SPAN_MS
    if (enabled && (joinTrace || joinBurst)) {
      prev.lines.push(e)
      if (!Number.isNaN(e.tsMs)) prev.lastTsMs = e.tsMs   // advance the burst cursor
      continue
    }
    const goTrace = isTrace && GO_HEAD.test(e.text)
    const unstructuredRun = enabled && !isTrace && !e.structured
    // A bare exception head has no level word (detectLevel → default); bump a
    // trace head to error so the exact-match level filter surfaces it. (`panic:`
    // is already fatal, so the bump is a no-op.)
    let level = e.level
    if (isTrace && level === DEFAULT_LEVEL) level = 'error'
    groups.push({
      head: e.ts ? e : null,
      lines: [e],
      ts: e.ts,
      tsMs: e.tsMs,
      lastTsMs: e.tsMs,
      pod: e.pod,
      podId: e.podId,
      container: e.container,
      level,
      isTrace,
      goTrace,
      unstructuredRun,
    })
  }
  return groups
}
