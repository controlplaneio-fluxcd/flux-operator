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

/**
 * groupEntries - fold a multi-line stack trace into its head entry so it renders
 * as one collapsible group instead of N rows.
 *
 * Content-driven (frames may or may not carry a timestamp): a line folds into
 * `prev` (same pod) only while a recognized trace is open — `prev.isTrace &&
 * isCont(e.text, prev.goTrace)` — and the first non-continuation line closes it.
 * A trace whose head isn't in TRACE_HEAD does not open, so its frames stay
 * separate rather than risk folding an unrelated indented/untimestamped line;
 * extend TRACE_HEAD as formats surface. The caller partitions by pod in the
 * all-pods view, so samePod is just a cross-pod backstop.
 *
 * @param {Array<{ts: string, tsMs: number, text: string, level: string, pod: string, podId: string}>} entries
 * @param {boolean} enabled - when false, every entry becomes its own group (no fold)
 * @returns {Array<{head: Object|null, lines: Array, ts: string, tsMs: number, pod: string, podId: string, level: string, isTrace: boolean, goTrace: boolean}>}
 */
export function groupEntries(entries, enabled) {
  const groups = []
  for (const e of entries) {
    const prev = groups[groups.length - 1]
    // An untimestamped orphan (pod '') belongs to its predecessor; a frame from a
    // different (non-empty) pod never folds across.
    const samePod = prev && (e.pod === prev.pod || e.pod === '')
    if (enabled && prev && samePod && prev.isTrace && isCont(e.text, prev.goTrace)) {
      prev.lines.push(e)
      continue
    }
    // isTrace labels the group and keeps the trace open for the following lines.
    const isTrace = enabled && isTraceHead(e.text)
    const goTrace = isTrace && GO_HEAD.test(e.text)
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
      pod: e.pod,
      podId: e.podId,
      level,
      isTrace,
      goTrace,
    })
  }
  return groups
}
