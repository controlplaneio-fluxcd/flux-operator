// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

// Normalized log levels, ordered by severity. `info` is the default for lines
// whose level cannot be determined, so every line carries a level.
export const LEVELS = ['trace', 'debug', 'info', 'warn', 'error', 'fatal']

export const DEFAULT_LEVEL = 'info'

// Per-level display metadata:
// - `label`  human name (legend, menu, tooltips)
// - `border` Tailwind border color for the timestamp pill and its separator
//            rule (light/dark); `info` is a faint blue so every line reads as
//            colored while staying calm, escalating through amber/red.
// - `swatch` solid background for the legend/menu color dots
// - `glow`   ready-to-use rgba for the latest-line border glow (--glow-color in
//            index.css), matching the Tailwind shade noted per row.
export const LEVEL_META = {
  trace: { label: 'Trace', border: 'border-gray-300 dark:border-gray-600', swatch: 'bg-gray-400', glow: 'rgba(156,163,175,0.6)' }, // gray-400
  debug: { label: 'Debug', border: 'border-slate-400 dark:border-slate-500', swatch: 'bg-slate-400', glow: 'rgba(148,163,184,0.6)' }, // slate-400
  info: { label: 'Info', border: 'border-blue-300 dark:border-blue-700', swatch: 'bg-blue-400', glow: 'rgba(96,165,250,0.6)' }, // blue-400
  warn: { label: 'Warn', border: 'border-amber-400 dark:border-amber-500', swatch: 'bg-amber-400', glow: 'rgba(245,158,11,0.6)' }, // amber-500
  error: { label: 'Error', border: 'border-red-400 dark:border-red-500', swatch: 'bg-red-400', glow: 'rgba(239,68,68,0.6)' }, // red-500
  fatal: { label: 'Fatal', border: 'border-red-600 dark:border-red-500', swatch: 'bg-red-600', glow: 'rgba(220,38,38,0.6)' } // red-600
}

// Matches ANSI SGR (color) escape sequences, e.g. "\x1b[31m".
// eslint-disable-next-line no-control-regex
const ANSI_SGR = /\x1b\[[0-9;]*m/g

/**
 * stripAnsi - removes ANSI color escape sequences from a log line so they don't
 * render as garbage and don't break level detection. Cheap-guarded so plain
 * lines (the common case) skip the replace.
 *
 * @param {string} text - The log line text
 * @returns {string} The text without ANSI escapes
 */
export function stripAnsi(text) {
  return text.indexOf('\x1b') === -1 ? text : text.replace(ANSI_SGR, '')
}

// String level aliases across loggers (zap, logrus, slog, klog, GCP, syslog,
// .NET, winston, JUL). The short codes cover Serilog/zerolog u3 codes
// (vrb/trc/dbg/inf/wrn/err/ftl/pnc) and the Microsoft.Extensions.Logging console
// prefixes (trce/dbug/fail/crit). `silly`/`http` are winston's extra npm levels.
const STRING_ALIAS = {
  trace: 'trace', trc: 'trace', trce: 'trace', finest: 'trace', verbose: 'trace', vrb: 'trace', silly: 'trace',
  debug: 'debug', dbg: 'debug', dbug: 'debug', fine: 'debug', finer: 'debug',
  info: 'info', inf: 'info', information: 'info', informational: 'info', notice: 'info', http: 'info',
  warn: 'warn', wrn: 'warn', warning: 'warn',
  error: 'error', err: 'error', eror: 'error', fail: 'error', severe: 'error', dpanic: 'error',
  fatal: 'fatal', ftl: 'fatal', pnc: 'fatal', critical: 'fatal', crit: 'fatal', panic: 'fatal',
  alert: 'fatal', emergency: 'fatal', emerg: 'fatal'
}

// MongoDB logv2 single-char severity codes (F/E/W/I/D; D1..D5 = debug verbosity).
const SEVERITY_CHAR = { F: 'fatal', E: 'error', W: 'warn', I: 'info', D: 'debug' }

// klog/glog leading severity character.
const KLOG_CHAR = { I: 'info', W: 'warn', E: 'error', F: 'fatal' }

// JSON level field names, in priority order, read from the parsed top-level
// object (not a raw substring scan, so a nested or message field can't shadow
// the real level). `level_name` covers Monolog (PHP); `@l`/`@level` Serilog CLEF
// and hclog (Terraform/Vault/Consul/Nomad). ECS nested `log.level` and MongoDB's
// logv2 `s` code are handled separately in levelFromObject.
const JSON_FIELDS = ['level', 'severity', 'lvl', 'log.level', 'level_name', '@level', '@l']
// Fallback for a line that starts with `{` but is not strict JSON (e.g. a
// truncated line): best-effort first level field, string or numeric.
const JSON_LEVEL_FALLBACK = /"(?:@level|@l|level_name|level|severity|lvl|log\.level)"\s*:\s*(?:"([^"]+)"|(-?\d+))/i
// logfmt level=value; the trailing (?!\w) rejects partial captures such as
// `level=error_code` (which would otherwise yield "error").
const LOGFMT_LEVEL = /(?:^|\s)(?:level|lvl|severity)=["']?([a-zA-Z]+)["']?(?!\w)/
// klog: leading severity char + MMDD.
const KLOG = /^([IWEF])\d{4}\s/
// A lowercase level token at line start followed by `:` or a tab. Covers the
// console formatters of winston (Node, "error: msg"), .NET
// Microsoft.Extensions.Logging ("warn: Category[0]"), and zap's console encoder.
const LEADING_LEVEL = /^([a-z]+)[:\t]/
// zap console encoder: "<ts>\t<level>\t...". zap leads with its own timestamp
// (so LEADING_LEVEL can't see the level) and defaults to a lowercase level (so
// the uppercase TEXT_LEVEL misses it); read the level from the 2nd tab field when
// the 1st is an ISO-8601 or epoch timestamp.
const ZAP_TS = /^(?:\d{4}-\d{2}-\d{2}T[\d:.]+(?:Z|[+-]\d{2}:?\d{2})?|\d{9,}(?:\.\d+)?)$/
// A bracketed level token, any case, e.g. nginx "[error]", Envoy/Istio
// "[warning]", MySQL "[Warning]", structlog "[info     ]", Elasticsearch "[INFO ]".
// Global so multi-bracket lines (Envoy "[16][warning][config]") are scanned.
const BRACKET_LEVEL = /[[(]\s*([a-zA-Z]+)\s*[\])]/g
// Plain text: an uppercase level token, bounded so mid-message "error" doesn't
// match. The leading `.` boundary catches Monolog's "channel.LEVEL:" prefix
// (PHP); the trailing `/` catches Celery's "[ERROR/MainProcess]"; the short
// codes (INF/WRN/ERR/FTL/DBG/VRB/TRC/PNC) catch Serilog/zerolog console output,
// and FINE/FINER/FINEST cover java.util.logging.
const TEXT_LEVEL = /(?:^|[\s[(.])(TRACE|TRC|DEBUG|DBG|INFO|INF|WARN(?:ING)?|WRN|ERROR|ERR|FATAL|FTL|SEVERE|CRITICAL|NOTICE|PANIC|PNC|VRB|FINEST|FINER|FINE)(?:[\s\]):/]|$)/

function fromString(s) {
  return STRING_ALIAS[s.toLowerCase()] || null
}

// fromSeverityChar maps a MongoDB logv2 severity code to a normalized level:
// F/E/W/I, or D with an optional debug verbosity digit (D, D1..D5).
function fromSeverityChar(s) {
  return /^(?:[FEWI]|D[1-5]?)$/.test(s) ? SEVERITY_CHAR[s[0]] : null
}

// syslog/RFC 5424 severities 0..7 mapped to normalized levels.
const SYSLOG_LEVELS = ['fatal', 'fatal', 'fatal', 'error', 'warn', 'info', 'info', 'debug']

// Numeric levels: pino/bunyan use 10..60 (multiples of ten, same order as
// LEVELS); syslog uses 0..7. zap emits negatives for debug/trace.
function fromNumber(n) {
  if (n < 0) return 'debug'
  if (n % 10 === 0 && n >= 10 && n <= 60) return LEVELS[n / 10 - 1]
  if (n >= 0 && n <= 7) return SYSLOG_LEVELS[n]
  return null
}

// levelFromObject reads the severity from a parsed JSON log object: the known
// fields in priority order, then ECS nested `log.level`, then MongoDB's
// single-char `s`. Reads only top-level keys, so a nested or message field
// never shadows the real level. Returns null when none resolve.
function levelFromObject(obj) {
  if (!obj || typeof obj !== 'object') return null
  for (const f of JSON_FIELDS) {
    const v = obj[f]
    if (v == null) continue
    const lvl = typeof v === 'number' ? fromNumber(v) : typeof v === 'string' ? fromString(v) : null
    if (lvl) return lvl
  }
  if (obj.log && typeof obj.log.level === 'string') {
    const lvl = fromString(obj.log.level)
    if (lvl) return lvl
  }
  if (typeof obj.s === 'string') return fromSeverityChar(obj.s)
  return null
}

/**
 * detectLevel - determine the normalized severity of a log line across the
 * common formats (JSON, klog, logfmt, bracketed and plain text). Runs a cheap
 * cascade, cheapest checks first, and falls back to the default `info`.
 *
 * @param {string} text - The log line text (timestamp and ANSI already stripped)
 * @returns {string} A value from LEVELS
 */
export function detectLevel(text) {
  if (!text) return DEFAULT_LEVEL

  // JSON: parse the object and read known level fields, top-level only. A line
  // that looks like JSON is classified solely by its fields — never by message
  // content — so it does not fall through to the text matchers below.
  if (text[0] === '{') {
    let lvl
    try {
      lvl = levelFromObject(JSON.parse(text))
    } catch {
      const m = JSON_LEVEL_FALLBACK.exec(text)
      lvl = m ? (m[1] != null ? fromString(m[1]) : fromNumber(Number(m[2]))) : null
    }
    return lvl || DEFAULT_LEVEL
  }

  // klog/glog severity prefix.
  const k = KLOG.exec(text)
  if (k) return KLOG_CHAR[k[1]]

  // Leading lowercase level token + ":"/tab (winston, .NET).
  const d = LEADING_LEVEL.exec(text)
  if (d) {
    const lvl = fromString(d[1])
    if (lvl) return lvl
  }

  // zap console encoder: timestamp-led, level in the 2nd tab field.
  if (text.indexOf('\t') !== -1) {
    const parts = text.split('\t')
    if (parts.length >= 2 && ZAP_TS.test(parts[0])) {
      const lvl = fromString(parts[1])
      if (lvl) return lvl
    }
  }

  // logfmt level=...
  if (text.indexOf('=') !== -1) {
    const lf = LOGFMT_LEVEL.exec(text)
    if (lf) {
      const lvl = fromString(lf[1])
      if (lvl) return lvl
    }
  }

  // Bracketed level token, any case (nginx, Envoy/Istio, MySQL, structlog).
  // Scan only the leading segment, before the first quote or `=`, so a stray
  // "[error]" inside a message or logfmt value is not treated as the level.
  if (text.indexOf('[') !== -1 || text.indexOf('(') !== -1) {
    const cut = text.search(/["'=]/)
    const head = cut === -1 ? text : text.slice(0, cut)
    BRACKET_LEVEL.lastIndex = 0
    let b
    while ((b = BRACKET_LEVEL.exec(head)) !== null) {
      const lvl = fromString(b[1])
      if (lvl) return lvl
    }
  }

  // Plain text uppercase level token.
  const t = TEXT_LEVEL.exec(text)
  if (t) {
    const lvl = fromString(t[1])
    if (lvl) return lvl
  }

  return DEFAULT_LEVEL
}
