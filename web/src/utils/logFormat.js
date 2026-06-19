// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

// Pretty-printing for the workload log viewer's Formatted mode beyond JSON: the
// three most common Go non-JSON loggers seen in Kubernetes workloads — klog/glog,
// logfmt (logrus text, go-kit/log, Go slog TextHandler), and zap's console
// encoder. `decorateLine` runs a cheap cascade and returns a serializable
// descriptor (never a VNode), which the viewer caches per raw line and renders.
//
// The JSON step of the cascade lives in the viewer (it owns the Prism highlight
// and the formatJson helper); this module covers klog → zap → logfmt → plain. A
// structured entry reflows onto multiple visual lines (a bare message line, then
// one `key: value` field per line); an unstructured line is highlighted in place.
//
// Safety: descriptors are plain data and the viewer maps spans to auto-escaped
// Preact text nodes, so there is no innerHTML path here.

// Span classes. Field key/value/separator reuse Prism's global JSON token classes
// (loaded by usePrismTheme), so they are pixel-identical to the JSON view and
// inherit the active light/dark theme. The scaffolding kinds (caller, muted ts /
// thread / logger, message) carry their own Tailwind utility classes. Each Prism
// span must include the base `token` class for the global rule to apply.
const CLS = {
  key: 'token property',
  str: 'token string',
  num: 'token number',
  op: 'token operator',
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
    sp(f.kind === 'num' ? CLS.num : CLS.str, f.val)
  ])
}

// klog header: severity char + MMDD + time + thread + caller. The whitespace runs
// are captured so the header line is rebuilt verbatim.
const KLOG_HEADER = /^([IWEF])(\d{4}) (\d{2}:\d{2}:\d{2}(?:\.\d+)?)(\s+)(\d+)\s(\S+?:\d+)\]\s?/

/**
 * parseKlog - decorates a klog/glog line. Classic lines (`Immdd … file:line] msg`)
 * highlight in place as a single `spans` row; structured `klog.InfoS/ErrorS` lines
 * (`… ] "msg" key="val"`) reflow to a `block` only when the body is a quoted
 * message followed by at least one valid field. Returns null when the header does
 * not match.
 *
 * @param {string} text - The log line (timestamp and ANSI already stripped)
 * @param {string} level - The normalized level, for the severity-char tint
 * @returns {object|null} A descriptor or null
 */
export function parseKlog(text, level) {
  const m = KLOG_HEADER.exec(text)
  if (!m) return null
  const [, sev, mmdd, time, gap, thread, caller] = m
  const header = [
    sp(SEV_CLS[level] || CLS.muted, sev),
    sp(CLS.muted, `${mmdd} ${time}${gap}${thread} `),
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
        rows: [header, [sp(CLS.msg, q.val)], ...fieldRows(fields)]
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

// zapField renders one expanded blob entry as a `key: value` row, typing the
// value by its JSON type (objects/arrays as compact JSON). Control characters in
// a scalar value (a `stacktrace`/`error` field's newlines and tabs) are escaped
// so the field stays on one line.
function zapField(key, val) {
  const kind = typeof val === 'number' ? 'num' : 'str'
  const text = val !== null && typeof val === 'object' ? JSON.stringify(val) : oneLine(String(val))
  return [sp(CLS.key, key), sp(CLS.op, ':'), sp(CLS.msg, ' '), sp(kind === 'num' ? CLS.num : CLS.str, text)]
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

/**
 * decorateLine - the non-JSON formatting cascade for one timestamped log line.
 * The viewer handles the JSON step (cascade position 1) and the continuation-line
 * gate before calling this; here the order is klog → zap → logfmt → plain. Returns
 * a serializable descriptor: `block` (multi-line structured), `spans` (single
 * highlighted line), or `plain` (unchanged).
 *
 * @param {string} text - The log line (timestamp and ANSI already stripped)
 * @param {string} level - The normalized level, for the klog severity-char tint
 * @returns {{kind: string, rows?: Array, spans?: Array}} The descriptor
 */
export function decorateLine(text, level) {
  if (!text) return { kind: 'plain' }
  return parseKlog(text, level) || parseZap(text) || parseLogfmt(text) || { kind: 'plain' }
}
