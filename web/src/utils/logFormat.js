// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

// Pretty-printing for the workload log viewer's Formatted mode beyond JSON: the
// common non-JSON loggers seen in Kubernetes workloads. Go: klog/glog, logfmt
// (logrus text, go-kit/log, Go slog TextHandler), and zap's console encoder. Java:
// the Spring Boot, Log4j2 and Logback default console layouts. .NET: the Serilog
// Console sink and NLog default layouts (log4net's log4j-style layout reuses the
// Java Log4j2 matcher). Scripting: Python (a common `logging` format, gunicorn,
// uvicorn), Ruby `Logger`, and PHP Monolog. `decorateLine` runs a cheap cascade and
// returns a serializable descriptor (never a VNode), which the viewer caches.
//
// The JSON step of the cascade is `highlightJson` (below): it pretty-prints a JSON
// object/array into colored spans, preserving nesting, and also covers the
// JSON-encoded Java loggers (logstash-logback-encoder, Log4j2 ECS layout) and .NET
// ones (Serilog CLEF, the MEL JSON console). `decorateLine` covers the rest:
// klog → zap → java → dotnet → python → ruby → monolog → logfmt → plain. A structured
// entry reflows onto multiple visual lines (a bare message line, then one
// `key: value` field per line); an unstructured line is highlighted in place.
//
// Safety: descriptors are plain data and the viewer maps spans to auto-escaped
// Preact text nodes, so there is no innerHTML path here.

import { fromString } from './logLevel'

// Span classes. Field keys and the `:` separator are a muted gray so they recede;
// values inherit the container's default body color (CLS.val is empty), reading as
// the high-contrast foreground — near-black on light, near-white on dark. Green is
// deliberately avoided for values since it is the app's status-ready color. The
// scaffolding kinds (caller, muted ts / thread / logger, message) carry their own
// Tailwind utility classes.
const CLS = {
  key: 'text-gray-500 dark:text-gray-400',
  val: '', // empty: values inherit the container's default body color
  op: 'text-gray-400 dark:text-gray-500',
  caller: 'font-semibold text-violet-600 dark:text-violet-300',
  muted: 'text-gray-400 dark:text-gray-500',
  msg: '' // empty: inherits the container's default body color
}

// klog severity letter tint, by normalized level (the same palette family as
// LEVEL_META's pill borders). Only the leading I/W/E/F character is tinted; the
// rest of the header is muted.
const SEV_CLS = {
  trace: 'text-gray-500 dark:text-gray-400',
  debug: 'text-slate-500 dark:text-slate-400',
  info: 'text-blue-500 dark:text-blue-400',
  warn: 'text-amber-500 dark:text-amber-400',
  error: 'text-red-500 dark:text-red-400',
  fatal: 'text-red-600 dark:text-red-400'
}

// sp builds a span descriptor. An empty class renders as a bare (still escaped)
// text node in the viewer.
function sp(cls, text) {
  return { cls, text }
}

// matchKeyEnd returns the index just past a logfmt/klog key starting at i
// (`[A-Za-z_][\w.-]*`), or -1 when no key starts there. Dotted/dashed keys cover
// slog group keys (`req.method`) and hyphenated keys.
function matchKeyEnd(s, i) {
  if (i >= s.length || !/[A-Za-z_]/.test(s[i])) return -1
  let j = i + 1
  while (j < s.length && /[\w.-]/.test(s[j])) j++
  return j
}

// readQuoted reads a double-quoted run starting at s[i], returning the inner
// value and the index past the closing quote. Only `\"` and `\\` are unescaped;
// any other backslash sequence (`\n`, `\t`, …) is kept verbatim so a value never
// expands into multiple visual lines. An unterminated quote consumes to end of
// string.
function readQuoted(s, i) {
  const q = s[i]
  let j = i + 1
  let out = ''
  while (j < s.length) {
    if (s[j] === '\\' && j + 1 < s.length) {
      const nx = s[j + 1]
      if (nx === '"' || nx === '\\') { out += nx; j += 2; continue }
      out += s[j]; j++; continue
    }
    if (s[j] === q) { j++; break }
    out += s[j]; j++
  }
  return { val: out, next: j }
}

// readValue reads a single logfmt/klog value at s[i]: a double-quoted run, or a
// bare token that ends only at the next ` key=` boundary. Single quotes are NOT
// value delimiters (logfmt/slog/go-kit never single-quote, and a leading `'` in a
// bare value must be preserved). A bare token may contain spaces, `=`, and
// unbalanced brackets — covering klog's unquoted `%+v` struct/map values
// (`{Host:a Port:1}`, `map[a:1 b:2]`) without tearing or swallowing the next
// field. An empty value (`key=` then a space or end) yields ''.
function readValue(s, i) {
  if (i >= s.length || s[i] === ' ') return { val: '', kind: 'str', next: i }
  if (s[i] === '"') {
    const r = readQuoted(s, i)
    return { val: r.val, kind: 'str', next: r.next }
  }
  let j = i
  while (j < s.length) {
    if (s[j] === ' ') {
      // A space only ends the value when the next non-space token is a `key=`
      // pair; otherwise it belongs to an unquoted multi-word value.
      let k = j + 1
      while (k < s.length && s[k] === ' ') k++
      const ke = matchKeyEnd(s, k)
      if (ke !== -1 && s[ke] === '=') break
      // The space run is part of the value: jump past it in one step. Stepping by
      // one would rescan the whole run at every space — O(m²) on a long run.
      j = k
      continue
    }
    j++
  }
  const val = s.slice(i, j)
  const kind = /^-?\d+(?:\.\d+)?$/.test(val) ? 'num' : 'str'
  return { val, kind, next: j }
}

// oneLine renders control characters visibly so a value carrying embedded
// newlines or tabs (e.g. a zap `stacktrace` field decoded from its JSON blob)
// stays on its own single field line instead of spilling flush-left across the
// pane and breaking the one-field-per-line model.
function oneLine(s) {
  return s.indexOf('\n') === -1 && s.indexOf('\r') === -1 && s.indexOf('\t') === -1
    ? s
    : s.replace(/\r/g, '\\r').replace(/\n/g, '\\n').replace(/\t/g, '\\t')
}

/**
 * tokenizeKV - splits a logfmt-style tail into its leading bare-text fragment and
 * its `key=value` fields. Quote- and bracket-aware: a value may be a quoted run
 * with escapes, a balanced `{…}`/`[…]` struct (klog `%+v`), or a bare token that
 * stops only at the next ` key=` boundary, so unquoted values containing spaces or
 * `=` stay intact.
 *
 * @param {string} s - The text to tokenize
 * @returns {{pre: string, fields: Array<{key: string, val: string, kind: string}>}}
 *   `pre` is any text before the first key (trimmed); `kind` is 'num' or 'str'.
 */
export function tokenizeKV(s) {
  const fields = []
  let pre = null
  let i = 0
  while (i < s.length) {
    while (i < s.length && s[i] === ' ') i++
    if (i >= s.length) break
    const start = i
    const ke = matchKeyEnd(s, i)
    if (ke !== -1 && s[ke] === '=') {
      if (pre === null) pre = s.slice(0, start)
      const key = s.slice(i, ke)
      const r = readValue(s, ke + 1)
      fields.push({ key, val: r.val, kind: r.kind })
      i = r.next
    } else if (fields.length === 0) {
      // Still in the leading bare-text fragment: skip this word and keep looking
      // for the first key.
      while (i < s.length && s[i] !== ' ') i++
    } else {
      // Trailing non-kv text after the fields; stop rather than misparse it.
      break
    }
  }
  return { pre: pre === null ? '' : pre.trim(), fields }
}

// fieldRows turns parsed fields into one descriptor row each, rendered
// `key: value` (single space, no alignment). The `=` is dropped in favour of a
// colon to mirror the JSON view.
function fieldRows(fields) {
  return fields.map(f => [
    sp(CLS.key, f.key),
    sp(CLS.op, ':'),
    sp(CLS.msg, ' '),
    sp(CLS.val, f.val)
  ])
}

// klog header: severity char + MMDD + time + thread + caller. klog space-pads the
// goroutine/PID to a fixed width (e.g. PID `1` becomes `      1`); that run is
// collapsed to a single space since the formatted view has no columns to align.
const KLOG_HEADER = /^([IWEF])(\d{4}) (\d{2}:\d{2}:\d{2}(?:\.\d+)?)\s+(\d+)\s(\S+?:\d+)\]\s?/

/**
 * parseKlog - decorates a klog/glog line. Classic lines (`Immdd … file:line] msg`)
 * highlight in place as a single `spans` row; structured `klog.InfoS/ErrorS` lines
 * (`… ] "msg" key="val"`) reflow to a `block` only when the body is a quoted
 * message followed by at least one valid field — the message stays on the header
 * row and the fields stack beneath it. Returns null when the header does not match.
 *
 * @param {string} text - The log line (timestamp and ANSI already stripped)
 * @param {string} level - The normalized level, for the severity-char tint
 * @returns {object|null} A descriptor or null
 */
export function parseKlog(text, level) {
  const m = KLOG_HEADER.exec(text)
  if (!m) return null
  const [, sev, mmdd, time, thread, caller] = m
  const header = [
    sp(SEV_CLS[level] || CLS.muted, sev),
    sp(CLS.muted, `${mmdd} ${time} ${thread} `),
    sp(CLS.caller, caller),
    sp(CLS.muted, ']')
  ]
  const body = text.slice(m[0].length)

  // Structured: a quoted message that the first field abuts, with ≥1 valid field.
  // `pre` must be empty — genuine klog.InfoS/ErrorS emits the first field right
  // after the message quote, so a quoted body with intervening prose (e.g.
  // `"podinfo" scaled replicas=3`) or no field tail (`"GET /healthz" completed`)
  // is classic, and reflowing it would drop the prose between message and field.
  if (body[0] === '"') {
    const q = readQuoted(body, 0)
    const { pre, fields } = tokenizeKV(body.slice(q.next))
    if (pre === '' && fields.length > 0) {
      return {
        kind: 'block',
        rows: [[...header, sp(CLS.msg, ` ${q.val}`)], ...fieldRows(fields)]
      }
    }
  }

  // Classic: the whole body is the message on the header line.
  return { kind: 'spans', spans: [...header, sp(CLS.msg, ` ${body}`)] }
}

// logfmt must start with a key=value pair (rejecting prose such as
// `Setting env GOFLAGS=-mod=vendor GOOS=linux`, which starts with a bare word)
// and carry at least one anchor key, so a stray `x=y` in a sentence never reflows.
const LOGFMT_START = /^[A-Za-z_][\w.-]*=/
const LOGFMT_ANCHOR = /(?:^|\s)(?:level|lvl|severity|ts|time|msg|message|logger|caller)=/

/**
 * parseLogfmt - decorates a logfmt line (logrus text, go-kit/log, Go slog
 * TextHandler). Promotes `msg`/`message` to a bare message line and stacks the
 * remaining fields one per line; a line with only a message renders as a single
 * highlighted row. Returns null when the line is not anchored logfmt.
 *
 * @param {string} text - The log line (timestamp and ANSI already stripped)
 * @returns {object|null} A descriptor or null
 */
export function parseLogfmt(text) {
  if (!LOGFMT_START.test(text) || !LOGFMT_ANCHOR.test(text)) return null
  const { fields } = tokenizeKV(text)
  if (fields.length === 0) return null
  let message = null
  const rest = []
  for (const f of fields) {
    if (message === null && (f.key === 'msg' || f.key === 'message')) message = f.val
    else rest.push(f)
  }
  if (rest.length === 0) {
    if (!message) return null
    return { kind: 'spans', spans: [sp(CLS.msg, oneLine(message))] }
  }
  const rows = []
  // Skip an empty promoted message so it doesn't render a blank leading row.
  if (message) rows.push([sp(CLS.msg, oneLine(message))])
  rows.push(...fieldRows(rest))
  return { kind: 'block', rows }
}

// zap console field shapes. The app timestamp and level are conveyed by the
// viewer's pill (kubelet time + detected level) and dropped from the inline body.
// ZAP_TS is end-anchored AND requires the ISO `T` separator (or an epoch), so a
// date-led TSV/audit line with a space separator — `2020-01-02 03:04:05.123\tERROR…`
// (Java/Python `asctime`) — is not absorbed as zap. zap console emits ISO8601 (`T`)
// or an epoch float by default.
const ZAP_TS = /^(?:\d{4}-\d{2}-\d{2}T[\d:.]+(?:Z|[+-]\d{2}:?\d{2})?|\d{9,}(?:\.\d+)?)$/
const ZAP_LEVEL = /^(?:trace|debug|info|warn|warning|error|dpanic|panic|fatal)$/i
// Caller `pkg/file.go:line`, tolerating an optional `:column` suffix.
const ZAP_CALLER = /^\S+\.go:\d+(?::\d+)?$/

function zapIsBlob(p) {
  const t = p.trim()
  if (t[0] !== '{') return null
  try {
    const obj = JSON.parse(t)
    return obj && typeof obj === 'object' && !Array.isArray(obj) ? obj : null
  } catch {
    return null
  }
}

// zapField renders one expanded blob entry as a `key: value` row, compacting
// objects/arrays to JSON. Control characters in a scalar value (a `stacktrace`/
// `error` field's newlines and tabs) are escaped so the field stays on one line.
function zapField(key, val) {
  const text = val !== null && typeof val === 'object' ? JSON.stringify(val) : oneLine(String(val))
  return [sp(CLS.key, key), sp(CLS.op, ':'), sp(CLS.msg, ' '), sp(CLS.val, text)]
}

/**
 * parseZap - decorates a zap console-encoder line
 * (`ts⇥LEVEL⇥[logger]⇥[caller]⇥msg⇥{json}`). The field count is variable —
 * controller-runtime inserts a named-logger field and caller/blob are optional —
 * so parts are classified by shape, not fixed position. With a trailing JSON blob
 * the line reflows to a `block` (message line led by logger/caller, then expanded
 * fields); otherwise it highlights in place as `spans`. Returns null when the line
 * is not confidently zap (needs a tab plus a recognized timestamp and level).
 *
 * @param {string} text - The log line (timestamp and ANSI already stripped)
 * @returns {object|null} A descriptor or null
 */
export function parseZap(text) {
  if (text.indexOf('\t') === -1) return null
  const parts = text.split('\t')
  let lo = 0
  let hi = parts.length - 1
  if (!ZAP_TS.test(parts[lo])) return null
  lo++
  if (lo > hi || !ZAP_LEVEL.test(parts[lo])) return null
  lo++

  // Locate the caller first (over the full remaining range) so blob extraction can
  // tell a context blob from a JSON-string message. zap always emits the message,
  // so the trailing JSON part is the field blob only when a message part still
  // remains after removing it (and the caller); otherwise the JSON *is* the message.
  let callerIdx = -1
  for (let k = lo; k <= hi; k++) {
    if (ZAP_CALLER.test(parts[k])) { callerIdx = k; break }
  }
  let blob = null
  const canStripBlob = callerIdx !== -1 ? hi - 1 > callerIdx : hi > lo
  if (hi >= lo && canStripBlob) {
    const obj = zapIsBlob(parts[hi])
    if (obj) { blob = obj; hi-- }
  }
  let logger = ''
  let caller = ''
  let msg = ''
  if (callerIdx !== -1) {
    caller = parts[callerIdx]
    logger = parts.slice(lo, callerIdx).join('\t')
    msg = parts.slice(callerIdx + 1, hi + 1).join('\t')
  } else {
    const mid = parts.slice(lo, hi + 1)
    // Treat the leading part as a logger only when it is shaped like a zap logger
    // name (dotted/slashed identifier, no spaces or punctuation); otherwise the
    // whole middle is the message (a message containing a literal tab must not have
    // its first word mistaken for a logger).
    if (mid.length >= 2 && /^[\w./-]+$/.test(mid[0])) { logger = mid[0]; msg = mid.slice(1).join('\t') }
    else { msg = mid.join('\t') }
  }

  // Message line: logger (muted) + caller (accent) + message (default), each
  // separated by two spaces for legibility.
  const line = []
  if (logger) line.push(sp(CLS.muted, logger))
  if (caller) { if (line.length) line.push(sp(CLS.msg, '  ')); line.push(sp(CLS.caller, caller)) }
  if (msg) { if (line.length) line.push(sp(CLS.msg, '  ')); line.push(sp(CLS.msg, oneLine(msg))) }
  // An empty message with fields (e.g. `logger.Info("", zap.String(...))`) still
  // reflows the blob; only a wholly empty line with no blob is not zap.
  if (line.length === 0 && !blob) return null

  if (!blob) return { kind: 'spans', spans: line }
  const rows = line.length ? [line] : []
  for (const k of Object.keys(blob)) rows.push(zapField(k, blob[k]))
  return { kind: 'block', rows }
}

// Java-family console patterns (Spring Boot / Log4j2 / Logback). After the kubelet
// timestamp is stripped the line still leads with the app's own date/time, so both
// shapes begin with a digit. The JSON-encoded Java loggers (logstash-logback-encoder,
// Log4j2 ECS layout) are handled by the viewer's JSON path, not here.
//
// SPRING: <ts> LEVEL [PID] --- [brackets…] [logger] : msg. The PID is optional
// (Spring's default `${PID:- }` is blank when undiscoverable) and the logger is
// optional (the root logger renders empty). The bracket run requires `\]\s+`, so a
// logger that itself contains brackets (Tomcat's `o.a.c.c.C.[Tomcat].[localhost].[/]`)
// is captured whole by \S+ instead of being torn into the run.
const SPRING = /^\d{4}-\d{2}-\d{2}[ T]\d{2}:\d{2}:\d{2}[.,]\d{3}(?:Z|[+-]\d{2}:?\d{2})?\s+(TRACE|DEBUG|INFO|WARN|ERROR|FATAL)\s+(?:\d+\s+)?---\s+((?:\[[^\]]*\]\s+)+)(?:(\S+)\s+)?:\s(.*)$/
// LOG4J2: <time|datetime> [thread] LEVEL [logger] - msg. Thread precedes the level
// and the separator is ` - ` (no PID, no `---`); logger optional (root logger empty).
const LOG4J2 = /^(?:\d{2}:\d{2}:\d{2}[.,]\d{3}|\d{4}-\d{2}-\d{2}[ T]\d{2}:\d{2}:\d{2}[.,]\d{3}(?:Z|[+-]\d{2}:?\d{2})?)\s+(\[[^\]]+\])\s+(TRACE|DEBUG|INFO|WARN|ERROR|FATAL)\s+(?:(\S+)\s+)?-\s+(.*)$/

// levelSpan tints a parsed level token by its normalized level, falling back to
// muted for an unrecognized token. Uses logLevel's fromString so a level word maps
// the same way here (the in-body tint) as in the pill — one source of truth — while
// still being driven by the word literally parsed from the line.
function levelSpan(word) {
  return sp(SEV_CLS[fromString(word)] || CLS.muted, word)
}

// trimBrackets normalizes a captured bracket run (`[demo] [           main] `) to a
// compact muted form (`[demo] [main]`): each `[…]` content is trimmed and empty
// brackets are dropped. Returns '' when nothing remains.
function trimBrackets(run) {
  const out = []
  const re = /\[([^\]]*)\]/g
  let m
  while ((m = re.exec(run)) !== null) {
    const inner = m[1].trim()
    if (inner) out.push(`[${inner}]`)
  }
  return out.join(' ')
}

// decoratedRow assembles the rendered row from the ordered muted parts, the accented
// logger, and the default message, joined by two spaces and skipping empties — the
// same shape as parseZap's message line.
function decoratedRow(muted, logger, msg) {
  const spans = []
  const push = (sp) => { if (spans.length) spans.push({ cls: CLS.msg, text: '  ' }); spans.push(sp) }
  for (const m of muted) if (m && m.text) push(m)
  if (logger) push(sp(CLS.caller, logger))
  if (msg) push(sp(CLS.msg, oneLine(msg)))
  return spans.length ? { kind: 'spans', spans } : null
}

/**
 * parseJava - decorates a Java-family console line (Spring Boot, Log4j2, Logback).
 * Drops the app's own timestamp (redundant with the pill), the PID and the
 * `---`/`:` scaffolding, but keeps the level word (tinted by the parsed word, so
 * the in-body level is authoritative even when the pill's heuristic misfires), the
 * thread bracket(s) and the logger. Renders a single `spans` row in each format's
 * natural field order. Returns null when the line is not a recognized Java shape.
 *
 * @param {string} text - The log line (timestamp and ANSI already stripped)
 * @returns {object|null} A `spans` descriptor or null
 */
export function parseJava(text) {
  // Gate: Java console layouts are space-delimited, so reject any tab (this also
  // keeps a non-zap tab-separated line out of the \s+-tolerant regexes below); both
  // shapes lead with the app date/time, so require a leading digit (which also
  // routes stack frames and exception headers to plain).
  if (text.indexOf('\t') !== -1) return null
  const c = text.charCodeAt(0)
  if (c < 48 || c > 57) return null

  // Spring first, then Log4j2 — mutually exclusive (Spring needs `---`; Log4j2 needs
  // a [thread] where Spring has the level), so the order is for clarity only.
  const s = SPRING.exec(text)
  if (s) {
    const [, level, brackets, logger, msg] = s
    const br = trimBrackets(brackets)
    return decoratedRow([levelSpan(level), br ? sp(CLS.muted, br) : null], logger || '', msg)
  }
  const l = LOG4J2.exec(text)
  if (l) {
    const [, thread, level, logger, msg] = l
    const br = trimBrackets(thread)
    return decoratedRow([br ? sp(CLS.muted, br) : null, levelSpan(level)], logger || '', msg)
  }
  return null
}

// .NET-family console patterns. Two text loggers are covered here; the JSON ones
// (Serilog CLEF, the MEL JSON console formatter) go through the viewer's JSON path,
// and log4net's log4j-style PatternLayout is already matched by parseJava's LOG4J2
// branch. Microsoft.Extensions.Logging's Simple console is intentionally NOT here:
// its default output is two physical lines (a `level: Category[id]` header then the
// message on the next, indented line), so it belongs to the future stack-trace
// grouping phase, when the header and message can be joined.
//
// SERILOG: the Console sink default template `[{Timestamp:HH:mm:ss} {Level:u3}]
// {Message:lj}`. The real default has no date and no fractional seconds; the
// optional date/fractional/zone tolerate common ISO custom timestamps. The u3 code
// is a fixed 3-char token and there is no logger in the default template.
const SERILOG = /^\[(?:\d{4}-\d{2}-\d{2}[ T])?\d{2}:\d{2}:\d{2}(?:[.,]\d+)?(?:Z|[+-]\d{2}:?\d{2})? (VRB|DBG|INF|WRN|ERR|FTL)\] (.*)$/
// NLOG: the default `TargetWithLayout` layout
// `${longdate}|${level:uppercase=true}|${logger}|${message:withexception=true}`.
// Pipe-delimited; the logger field may be empty. With an exception the 4th field
// carries it on this physical line (continuations go plain).
const NLOG = /^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}[.,]\d+\|(TRACE|DEBUG|INFO|WARN|ERROR|FATAL)\|([^|]*)\|(.*)$/

/**
 * parseDotnet - decorates a .NET-family console line (Serilog Console sink, NLog
 * default layout). Drops the app's own timestamp (redundant with the pill) but
 * keeps the level token (tinted by the parsed token), the NLog logger, and the
 * message. Renders a single `spans` row. Returns null when the line is not a
 * recognized .NET shape.
 *
 * @param {string} text - The log line (timestamp and ANSI already stripped)
 * @returns {object|null} A `spans` descriptor or null
 */
export function parseDotnet(text) {
  // Serilog leads with `[`; NLog leads with a longdate and is pipe-delimited. The
  // two shapes are disjoint by first token, so dispatch cheaply.
  if (text[0] === '[') {
    const s = SERILOG.exec(text)
    if (s) {
      const [, code, msg] = s
      return decoratedRow([levelSpan(code)], '', msg)
    }
    return null
  }
  if (text.indexOf('|') !== -1) {
    const n = NLOG.exec(text)
    if (n) {
      const [, level, logger, msg] = n
      return decoratedRow([levelSpan(level)], logger || '', msg)
    }
  }
  return null
}

// Scripting-language console patterns.
//
// Python logging, a common custom `format='%(asctime)s - %(name)s - %(levelname)s -
// %(message)s'` (asctime default millis is a comma). Not the stdlib default (which is
// the deferred `LEVEL:name:msg`), but the most common configured shape.
const PY_BASIC = /^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}(?:[.,]\d+)? - (\S+) - (DEBUG|INFO|WARNING|ERROR|CRITICAL) - (.*)$/
// gunicorn glogging default `[%(asctime)s] [%(process)d] [%(levelname)s] %(message)s`
// (the datefmt brackets the date, with a ` %z` zone inside).
const GUNICORN = /^\[\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}[^\]]*\] \[(\d+)\] \[(DEBUG|INFO|WARNING|ERROR|CRITICAL)\] (.*)$/
// uvicorn `%(levelprefix)s %(message)s`, where levelprefix is `LEVEL:` right-padded
// so the message starts at a fixed column: spaces-after-colon = 9 - len(level)
// (CRITICAL gets just one, TRACE four). The exact-width check (level.length + spaces
// === 9) both admits those and rejects prose like `INFO: hi`. TRACE is uvicorn's
// extra `--log-level trace` level.
const UVICORN = /^(TRACE|DEBUG|INFO|WARNING|ERROR|CRITICAL):( +)(.*)$/

/**
 * parsePython - decorates a Python-ecosystem console line: the common timestamped
 * `logging` format, gunicorn, and uvicorn. Keeps the level word (tinted), accents
 * the logger name (when the format carries one), drops the app timestamp. Returns
 * null when the line is not a recognized Python shape.
 *
 * @param {string} text - The log line (timestamp and ANSI already stripped)
 * @returns {object|null} A `spans` descriptor or null
 */
export function parsePython(text) {
  // gunicorn leads with `[`; the timestamped logging format leads with a digit;
  // uvicorn leads with an uppercase level word.
  if (text[0] === '[') {
    const g = GUNICORN.exec(text)
    if (g) {
      const [, pid, level, msg] = g
      return decoratedRow([sp(CLS.muted, `[${pid}]`), levelSpan(level)], '', msg)
    }
    return null
  }
  const c = text.charCodeAt(0)
  if (c >= 48 && c <= 57) {
    const b = PY_BASIC.exec(text)
    if (b) {
      const [, name, level, msg] = b
      return decoratedRow([levelSpan(level)], name, msg)
    }
    return null
  }
  const u = UVICORN.exec(text)
  if (u && u[1].length + u[2].length === 9) {
    const [, level, , msg] = u
    return decoratedRow([levelSpan(level)], '', msg)
  }
  return null
}

// Ruby Logger default `%.1s, [%s #%d] %5s -- %s: %s` (severityID, datetime, pid,
// label, progname, message). Labels DEBUG/INFO/WARN/ERROR/FATAL/ANY (ANY = UNKNOWN,
// an unknown severity). `\s+` before `#` tolerates the stdlib's trailing-space
// datetime (which yields two spaces); progname is commonly empty.
const RUBY = /^[DIWEFA], \[\d{4}-\d{2}-\d{2}T[\d:.]+\s+#\d+\]\s+(DEBUG|INFO|WARN|ERROR|FATAL|ANY)\s+-- ([^:]*): (.*)$/

/**
 * parseRuby - decorates a Ruby `Logger` default-format line. Keeps the level word
 * (tinted; ANY → fatal), accents the progname (when present), drops the severity
 * char, datetime and pid. Returns null when the line is not the Ruby shape.
 *
 * @param {string} text - The log line (timestamp and ANSI already stripped)
 * @returns {object|null} A `spans` descriptor or null
 */
export function parseRuby(text) {
  // Cheap gate on the "S, [" shape; the anchored regex validates the severity char
  // (D/I/W/E/F/A — not a contiguous range, so left to the regex).
  if (text[1] !== ',') return null
  const m = RUBY.exec(text)
  if (!m) return null
  const [, level, progname, msg] = m
  // DEBUG/INFO/WARN/ERROR/FATAL tint by their level; ANY (Ruby's UNKNOWN) is an
  // unknown severity, so levelSpan renders it muted like any unrecognized token —
  // which keeps it consistent with the pill and level filter (detectLevel also
  // defaults it to info). Mapping ANY to a real level would require adding it to
  // TEXT_LEVEL, which would mis-tint the common word "ANY" in ordinary messages.
  return decoratedRow([levelSpan(level)], progname || '', msg)
}

// PHP Monolog default LineFormatter `[%datetime%] %channel%.%level_name%: %message%
// %context% %extra%` (default datetime ISO-8601 with offset, optional microseconds).
// The context/extra JSON ride along in the message — splitting them is the JSON
// phase. PSR-3 level set.
const MONOLOG = /^\[\d{4}-\d{2}-\d{2}[T ][\d:.+-]+\] (\S+)\.(DEBUG|INFO|NOTICE|WARNING|ERROR|CRITICAL|ALERT|EMERGENCY): (.*)$/

/**
 * parseMonolog - decorates a PHP Monolog default LineFormatter line. Keeps the level
 * word (tinted), accents the channel, drops the datetime. The trailing context/extra
 * stay in the message. Returns null when the line is not the Monolog shape.
 *
 * @param {string} text - The log line (timestamp and ANSI already stripped)
 * @returns {object|null} A `spans` descriptor or null
 */
export function parseMonolog(text) {
  if (text[0] !== '[') return null
  const m = MONOLOG.exec(text)
  if (!m) return null
  const [, channel, level, msg] = m
  return decoratedRow([levelSpan(level)], channel, msg)
}

// jsonScalar renders a JSON primitive as a single value-colored span (CLS.val, the
// body color used for field values). JSON.stringify renders strings quoted/escaped,
// finite numbers verbatim, booleans/null plainly, and non-finite numbers (Infinity/
// NaN from over-range literals) as `null` — matching JSON.stringify(value, null, 2)
// rather than String()'s `Infinity`.
function jsonScalar(v) {
  return sp(CLS.val, JSON.stringify(v))
}

// emitJson appends the pretty-printed spans for a parsed JSON value at the given
// indent depth, mirroring JSON.stringify(value, null, 2) byte-for-byte but with each
// token wrapped in a colored span: keys and structural punctuation (braces, brackets,
// `:` and `,`) muted gray, scalars in the body color. Indentation and newlines are
// emitted as plain spans so a <pre> lays the nesting out. Empty objects/arrays stay
// on one line.
function emitJson(spans, value, depth) {
  if (value === null || typeof value !== 'object') { spans.push(jsonScalar(value)); return }
  const isArr = Array.isArray(value)
  const keys = isArr ? null : Object.keys(value)
  const len = isArr ? value.length : keys.length
  if (len === 0) { spans.push(sp(CLS.op, isArr ? '[]' : '{}')); return }
  const pad = '  '.repeat(depth + 1)
  spans.push(sp(CLS.op, isArr ? '[' : '{'))
  for (let i = 0; i < len; i++) {
    spans.push(sp(CLS.msg, `\n${pad}`))
    if (!isArr) spans.push(sp(CLS.key, JSON.stringify(keys[i])), sp(CLS.op, ':'), sp(CLS.msg, ' '))
    emitJson(spans, isArr ? value[i] : value[keys[i]], depth + 1)
    if (i < len - 1) spans.push(sp(CLS.op, ','))
  }
  spans.push(sp(CLS.msg, `\n${'  '.repeat(depth)}`), sp(CLS.op, isArr ? ']' : '}'))
}

// emitMembers appends the members of the top-level object/array WITHOUT the wrapping
// braces/brackets, flush-left, one per line — a lone `{`/`}` (or `[`/`]`) would waste
// two lines per entry and the keys already give context. Nested values keep their own
// braces via emitJson. The caller guarantees a non-empty object/array.
function emitMembers(spans, value, depth) {
  const isArr = Array.isArray(value)
  const keys = isArr ? null : Object.keys(value)
  const len = isArr ? value.length : keys.length
  const pad = '  '.repeat(depth)
  for (let i = 0; i < len; i++) {
    if (i > 0) spans.push(sp(CLS.msg, `\n${pad}`))
    if (!isArr) spans.push(sp(CLS.key, JSON.stringify(keys[i])), sp(CLS.op, ':'), sp(CLS.msg, ' '))
    emitJson(spans, isArr ? value[i] : value[keys[i]], depth)
    if (i < len - 1) spans.push(sp(CLS.op, ','))
  }
}

// globToTest compiles a `*`-glob pattern (the only wildcard) into a linear,
// backtracking-free matcher. The pattern is split on `*` and matched as an ordered
// sequence of case-insensitive literal segments — a required prefix, in-order middle
// substrings, and a required suffix. This deliberately avoids a regex like
// `^.*a.*b…$`, whose chained `.*` suffers catastrophic backtracking on crafted input
// (a user could otherwise freeze the viewer by typing `*a*a*…*b`).
function globToTest(pat) {
  const parts = pat.toLowerCase().split('*')
  const last = parts[parts.length - 1]
  return (key) => {
    const s = key.toLowerCase()
    if (!s.startsWith(parts[0])) return false
    if (!s.endsWith(last)) return false
    const end = s.length - last.length // start index of the suffix region
    let pos = parts[0].length
    for (let i = 1; i < parts.length - 1; i++) {
      const seg = parts[i]
      if (seg === '') continue
      const idx = s.indexOf(seg, pos)
      if (idx === -1 || idx + seg.length > end) return false
      pos = idx + seg.length
    }
    return pos <= end
  }
}

/**
 * fieldMatcher - compiles a field-selection expression into a predicate over field
 * names, or null when the expression selects everything. The expression is a list of
 * tokens separated by spaces, commas, or pipes (`msg error`, `msg, error`,
 * `msg|message|error`). Matching is case-insensitive and exact by default; a `*`
 * acts as a glob wildcard (`*id`, `controller*`). A token prefixed with `!` excludes
 * the matching field. With only exclusions, every other field is kept; with any
 * inclusion, the result is a whitelist (minus anything also excluded). An empty
 * expression (or one with no usable tokens) returns null, meaning "all fields".
 *
 * @param {string} expr - The field-selection expression
 * @returns {((key: string) => boolean)|null} A predicate, or null for "all fields"
 */
export function fieldMatcher(expr) {
  const tokens = (expr || '').split(/[\s,|]+/).filter(Boolean)
  const includes = []
  const excludes = []
  for (const t of tokens) {
    if (t[0] === '!') { if (t.length > 1) excludes.push(t.slice(1)) }
    else includes.push(t)
  }
  if (includes.length === 0 && excludes.length === 0) return null
  const tester = (pat) => {
    if (pat.includes('*')) return globToTest(pat)
    const lower = pat.toLowerCase()
    return (k) => k.toLowerCase() === lower
  }
  const inc = includes.map(tester)
  const exc = excludes.map(tester)
  return (key) => {
    if (exc.some(t => t(key))) return false
    return inc.length === 0 || inc.some(t => t(key))
  }
}

/**
 * topLevelJsonKeys - returns the top-level keys of a JSON object log line, or null
 * when the line is not a JSON object (a scalar, an array — which has no keys — or
 * non-JSON). Used to discover the set of structured fields a stream emits so the
 * viewer can offer a field picker. Keys are returned in source order.
 *
 * @param {string} text - The log line (timestamp and ANSI already stripped)
 * @returns {string[]|null} The object's top-level keys, or null
 */
export function topLevelJsonKeys(text) {
  const t = text.trim()
  if (t.charCodeAt(0) !== 123) return null // fast gate: only '{' (objects have keys)
  try {
    const parsed = JSON.parse(t)
    if (parsed === null || typeof parsed !== 'object' || Array.isArray(parsed)) return null
    return Object.keys(parsed)
  } catch {
    return null
  }
}

/**
 * highlightJson - pretty-prints a JSON object/array log line into colored spans,
 * preserving nesting (indentation + newlines) so complex structures stay readable.
 * The outer braces/brackets are dropped and the members sit flush-left so the entry
 * does not waste a line on a lone `{` and `}`; nested structures keep their braces.
 * Returns null for a non-JSON line or a bare scalar that merely parses (`42`,
 * `"hi"`), leaving those to the text cascade. The descriptor is plain data with no
 * innerHTML; the viewer renders the spans inside a <pre> so the layout survives.
 *
 * When `selected` is a Set, a top-level object is projected to only those keys (in
 * source order) so the viewer can show a subset of fields; arrays and nested values
 * are unaffected. A null/undefined `selected` shows every field.
 *
 * @param {string} text - The log line (timestamp and ANSI already stripped)
 * @param {Set<string>} [selected] - Top-level keys to keep, or null/undefined for all
 * @returns {{kind: 'json', spans: Array}|null} A descriptor or null
 */
export function highlightJson(text, selected) {
  const t = text.trim()
  const c = t.charCodeAt(0)
  if (c !== 123 && c !== 91) return null // fast gate: not '{' or '['
  let parsed
  try {
    parsed = JSON.parse(t)
  } catch {
    return null
  }
  if (parsed === null || typeof parsed !== 'object') return null
  // Project a top-level object to the selected fields (source order preserved). A
  // selection never applies to a top-level array, which carries no field keys.
  let value = parsed
  if (selected && !Array.isArray(parsed)) {
    // Null-prototype object so a literal "__proto__" field is kept as an own key
    // (a plain {} would assign it to the prototype and drop it from Object.keys).
    value = Object.create(null)
    for (const k of Object.keys(parsed)) if (selected.has(k)) value[k] = parsed[k]
  }
  // An empty top-level object/array (or one projected empty by the field filter)
  // keeps its braces on one line (emitMembers would render nothing, leaving a blank).
  if ((Array.isArray(value) ? value.length : Object.keys(value).length) === 0) {
    return { kind: 'json', spans: [sp(CLS.op, Array.isArray(value) ? '[]' : '{}')] }
  }
  const spans = []
  try {
    emitMembers(spans, value, 0)
  } catch {
    // Pathologically deep nesting overflows the recursion (RangeError). Fall back to
    // the plain (raw text) renderer rather than crashing the viewer on one bad line.
    return null
  }
  return { kind: 'json', spans }
}

/**
 * decorateLine - the non-JSON formatting cascade for one timestamped log line.
 * The viewer handles the JSON step (cascade position 1) and the continuation-line
 * gate before calling this; here the order is klog → zap → java → dotnet → python →
 * ruby → monolog → logfmt → plain. Returns a serializable descriptor: `block`
 * (multi-line structured), `spans`
 * (single highlighted line), or `plain` (unchanged).
 *
 * @param {string} text - The log line (timestamp and ANSI already stripped)
 * @param {string} level - The normalized level, for the klog severity-char tint
 * @returns {{kind: string, rows?: Array, spans?: Array}} The descriptor
 */
export function decorateLine(text, level) {
  if (!text) return { kind: 'plain' }
  // Three parsers sniff a leading `[`: parseDotnet (Serilog `[time u3]`), parsePython
  // (gunicorn `[date] [pid] [LEVEL]`) and parseMonolog (`[date] chan.LEVEL:`). Each
  // returns null for a `[`-line it does not own, so order among them only sets
  // precedence; they are mutually disjoint by the tokens after the first bracket.
  return parseKlog(text, level) || parseZap(text) || parseJava(text) || parseDotnet(text) ||
    parsePython(text) || parseRuby(text) || parseMonolog(text) || parseLogfmt(text) || { kind: 'plain' }
}
