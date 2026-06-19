// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect } from 'vitest'
import { decorateLine, parseKlog, parseLogfmt, parseZap, tokenizeKV } from './logFormat'

// rowsText flattens a block descriptor to one string per visual row (spans joined),
// so a test can assert the rendered text without caring about span boundaries.
function rowsText(d) {
  return d.rows.map(row => row.map(s => s.text).join(''))
}

// spansText flattens a spans descriptor to its single rendered line.
function spansText(d) {
  return d.spans.map(s => s.text).join('')
}

// fieldRowsOf returns the field rows of a block (everything after a leading header
// and/or message row is matched by its `key: value` shape).
function kvRows(d) {
  return rowsText(d).filter(t => /^[\w.-]+: /.test(t))
}

describe('tokenizeKV', () => {
  it('splits simple key=value pairs and types numbers', () => {
    const { pre, fields } = tokenizeKV('controller=gitrepository name=flux-system duration=1.2s retries=3')
    expect(pre).toBe('')
    expect(fields).toEqual([
      { key: 'controller', val: 'gitrepository', kind: 'str' },
      { key: 'name', val: 'flux-system', kind: 'str' },
      { key: 'duration', val: '1.2s', kind: 'str' },
      { key: 'retries', val: '3', kind: 'num' }
    ])
  })

  it('keeps escaped quotes inside a quoted value', () => {
    const { fields } = tokenizeKV('err="Get \\"https://x\\": timeout" code=503')
    expect(fields[0]).toEqual({ key: 'err', val: 'Get "https://x": timeout', kind: 'str' })
    expect(fields[1]).toEqual({ key: 'code', val: '503', kind: 'num' })
  })

  it('keeps an unquoted %+v value with spaces and = intact (bracket-balanced)', () => {
    const { fields } = tokenizeKV('config={Host:a Port:1} m=map[a:1 b:2] next=ok')
    expect(fields.map(f => f.val)).toEqual(['{Host:a Port:1}', 'map[a:1 b:2]', 'ok'])
    expect(fields.map(f => f.key)).toEqual(['config', 'm', 'next'])
  })

  it('handles an empty value and a = inside a quoted value', () => {
    const { fields } = tokenizeKV('empty= url="https://x?a=b&c=d" tail=z')
    expect(fields).toEqual([
      { key: 'empty', val: '', kind: 'str' },
      { key: 'url', val: 'https://x?a=b&c=d', kind: 'str' },
      { key: 'tail', val: 'z', kind: 'str' }
    ])
  })

  it('captures a leading bare-text fragment before the first key', () => {
    const { pre, fields } = tokenizeKV('some leading words key=val')
    expect(pre).toBe('some leading words')
    expect(fields).toEqual([{ key: 'key', val: 'val', kind: 'str' }])
  })
})

describe('parseKlog', () => {
  it('decorates a structured klog.ErrorS line into a block (header, message, fields)', () => {
    const line = 'E0526 23:03:57.521582       1 leaderelection.go:452] "Error retrieving lease lock" err="Get \\"https://10.96.0.1:443\\": i/o timeout" logger="cert-manager.controller" lock="kube-system/cert-manager-controller"'
    const d = parseKlog(line, 'error')
    expect(d.kind).toBe('block')
    const rows = rowsText(d)
    // Header rebuilt verbatim, including the original padding.
    expect(rows[0]).toBe('E0526 23:03:57.521582       1 leaderelection.go:452]')
    // Message unquoted on its own line.
    expect(rows[1]).toBe('Error retrieving lease lock')
    // Three fields, one per line, with the escaped-quote value kept intact.
    expect(kvRows(d)).toEqual([
      'err: Get "https://10.96.0.1:443": i/o timeout',
      'logger: cert-manager.controller',
      'lock: kube-system/cert-manager-controller'
    ])
    // The severity char carries the level tint (error → red); the caller is its own span.
    expect(d.rows[0][0]).toMatchObject({ text: 'E' })
    expect(d.rows[0][0].cls).toMatch(/red/)
    expect(d.rows[0].some(s => s.text === 'leaderelection.go:452' && /violet/.test(s.cls))).toBe(true)
  })

  it('treats a quoted body with no field tail as classic (not block)', () => {
    const line = 'I0612 14:03:11.123456   12 server.go:80] "GET /healthz" completed'
    const d = parseKlog(line, 'info')
    expect(d.kind).toBe('spans')
    expect(spansText(d)).toBe('I0612 14:03:11.123456   12 server.go:80] "GET /healthz" completed')
  })

  it('decorates a classic klog line as a single highlighted row', () => {
    const line = 'I0612 14:03:11.123456   12 reflector.go:243] Watch close - watch chan closed'
    const d = parseKlog(line, 'info')
    expect(d.kind).toBe('spans')
    expect(spansText(d)).toBe(line)
    // Caller highlighted, severity tinted.
    expect(d.spans.some(s => s.text === 'reflector.go:243' && /violet/.test(s.cls))).toBe(true)
  })

  it('returns null when the klog header does not match', () => {
    expect(parseKlog('just a plain sentence', 'info')).toBeNull()
    expect(parseKlog('level=info msg=hi', 'info')).toBeNull()
  })
})

describe('parseLogfmt', () => {
  it('promotes msg to a bare message line and stacks the rest', () => {
    const d = parseLogfmt('level=info msg="reconcile complete" controller=gitrepository name=flux-system duration=1.2s')
    expect(d.kind).toBe('block')
    expect(rowsText(d)[0]).toBe('reconcile complete')
    expect(kvRows(d)).toEqual([
      'level: info',
      'controller: gitrepository',
      'name: flux-system',
      'duration: 1.2s'
    ])
  })

  it('renders a message-only logfmt line as a single row', () => {
    const d = parseLogfmt('msg="just a message"')
    expect(d.kind).toBe('spans')
    expect(spansText(d)).toBe('just a message')
  })

  it('renders a fields-only line with no blank message row', () => {
    const d = parseLogfmt('level=warn controller=foo count=2')
    expect(d.kind).toBe('block')
    expect(rowsText(d)).toEqual(['level: warn', 'controller: foo', 'count: 2'])
  })

  it('does NOT reflow prose with stray key=value fragments', () => {
    expect(parseLogfmt('Setting env GOFLAGS=-mod=vendor GOOS=linux')).toBeNull()
    expect(parseLogfmt('exec failed: PATH=/bin USER=root ./run')).toBeNull()
  })

  describe('slog TextHandler', () => {
    it('renders dotted group keys flat as fields', () => {
      const d = parseLogfmt('time=2009-11-10T23:00:00Z level=INFO msg=hi req.method=GET req.user.id=42')
      expect(kvRows(d)).toEqual([
        'time: 2009-11-10T23:00:00Z',
        'level: INFO',
        'req.method: GET',
        'req.user.id: 42'
      ])
    })

    it('keeps a bare RFC3339 time= as a normal field', () => {
      const d = parseLogfmt('time=2009-11-10T23:00:00Z level=INFO msg=ok')
      expect(rowsText(d)).toContain('time: 2009-11-10T23:00:00Z')
    })

    it('renders a level offset field verbatim', () => {
      const d = parseLogfmt('time=2009-11-10T23:00:00Z level=INFO+2 msg=ok')
      expect(rowsText(d)).toContain('level: INFO+2')
    })
  })
})

describe('parseZap', () => {
  it('expands a zap console line with a JSON blob into a block', () => {
    const line = '2026-06-19T14:03:11.500Z\twarn\tinternal/controller/foo.go:88\treconciliation slow\t{"duration":"4.7s","retries":3}'
    const d = parseZap(line)
    expect(d.kind).toBe('block')
    expect(rowsText(d)[0]).toBe('internal/controller/foo.go:88  reconciliation slow')
    expect(kvRows(d)).toEqual(['duration: 4.7s', 'retries: 3'])
  })

  it('does not mistake a named logger field for the caller', () => {
    const line = '2026-06-19T14:03:11.500Z\tinfo\tcontroller.gitrepository\tinternal/controller/foo.go:88\tstarting\t{"name":"flux-system"}'
    const d = parseZap(line)
    expect(d.kind).toBe('block')
    // Logger (muted) precedes caller (accent) on the message line.
    expect(rowsText(d)[0]).toBe('controller.gitrepository  internal/controller/foo.go:88  starting')
    expect(d.rows[0].some(s => s.text === 'controller.gitrepository' && /gray-4/.test(s.cls))).toBe(true)
    expect(d.rows[0].some(s => s.text === 'internal/controller/foo.go:88' && /violet/.test(s.cls))).toBe(true)
  })

  it('renders a zap line without caller or blob as a single row', () => {
    const line = '2026-06-19T14:03:11.500Z\tinfo\tstarting manager'
    const d = parseZap(line)
    expect(d.kind).toBe('spans')
    expect(spansText(d)).toBe('starting manager')
  })

  it('keeps a {…}-looking tail that does not parse as text', () => {
    const line = '2026-06-19T14:03:11.500Z\tinfo\tfoo.go:1\tgot {partial json'
    const d = parseZap(line)
    expect(d.kind).toBe('spans')
    expect(spansText(d)).toBe('foo.go:1  got {partial json')
  })

  it('returns null for a tabbed line without a timestamp+level', () => {
    expect(parseZap('a\tb\tc')).toBeNull()
    expect(parseZap('2026-06-19T14:03:11.500Z\tnot-a-level\tmsg')).toBeNull()
  })

  it('returns null when there is no tab', () => {
    expect(parseZap('level=info msg=hi')).toBeNull()
  })
})

describe('decorateLine cascade', () => {
  it('routes klog, zap, and logfmt to their parsers and prose to plain', () => {
    expect(decorateLine('I0612 14:03:11.123456 12 reflector.go:243] watch closed', 'info').kind).toBe('spans')
    expect(decorateLine('2026-06-19T14:03:11.5Z\tinfo\tfoo.go:1\thi\t{"a":1}', 'info').kind).toBe('block')
    expect(decorateLine('level=info msg=hi controller=foo', 'info').kind).toBe('block')
    expect(decorateLine('a plain sentence with no structure', 'info').kind).toBe('plain')
  })

  it('leaves a stray = or [error] in prose as plain', () => {
    expect(decorateLine('rerun with x=1 to debug', 'info').kind).toBe('plain')
    expect(decorateLine('the [error] was transient', 'error').kind).toBe('plain')
  })

  it('returns plain for empty text', () => {
    expect(decorateLine('', 'info').kind).toBe('plain')
  })

  it('routes a non-Go space-delimited console line (zerolog) to plain', () => {
    expect(decorateLine('2026-06-19T14:03:11Z INF hello foo=bar', 'info').kind).toBe('plain')
  })
})

// Regressions for issues found by the adversarial review.
describe('review regressions', () => {
  it('klog: intervening prose between message and field stays classic, nothing dropped', () => {
    const line = 'I0612 14:03:11.123456 12 x.go:1] "podinfo" scaled replicas=3'
    const d = parseKlog(line, 'info')
    expect(d.kind).toBe('spans')
    expect(spansText(d)).toBe(line) // "scaled" is not dropped
  })

  it('klog: an unbalanced %+v brace does not swallow the following field', () => {
    const open = parseKlog('I0612 14:03:11.123456 1 x.go:1] "s" cfg={Name:a{b Port:1} next=ok', 'info')
    expect(kvRows(open)).toEqual(['cfg: {Name:a{b Port:1}', 'next: ok'])
    const close = parseKlog('I0612 14:03:11.123456 1 x.go:1] "s" cfg={Msg:a}b} next=ok', 'info')
    expect(kvRows(close)).toEqual(['cfg: {Msg:a}b}', 'next: ok'])
  })

  it('logfmt: a value beginning with a single quote does not swallow the next field', () => {
    const { fields } = tokenizeKV("user='admin level=info")
    expect(fields).toEqual([
      { key: 'user', val: "'admin", kind: 'str' },
      { key: 'level', val: 'info', kind: 'str' }
    ])
  })

  it('logfmt: a quoted value keeps \\n literal (one line) and the next field survives', () => {
    const { fields } = tokenizeKV('msg="line1\\nline2" next=ok')
    expect(fields[0]).toEqual({ key: 'msg', val: 'line1\\nline2', kind: 'str' })
    expect(fields[0].val).not.toContain('\n')
    expect(fields[1]).toEqual({ key: 'next', val: 'ok', kind: 'str' })
  })

  it('zap: a JSON-string message with no fields is preserved, not swallowed as a blob', () => {
    const withCaller = parseZap('2026-06-19T14:03:11.5Z\tinfo\twebhook.go:88\t{"kind":"Pod","op":"CREATE"}')
    expect(withCaller.kind).toBe('spans')
    expect(spansText(withCaller)).toBe('webhook.go:88  {"kind":"Pod","op":"CREATE"}')
    const noCaller = parseZap('2026-06-19T14:03:11.5Z\tinfo\t{"a":1}')
    expect(noCaller.kind).toBe('spans')
    expect(spansText(noCaller)).toBe('{"a":1}')
  })

  it('zap: a no-caller message containing a tab is not mis-split into a logger', () => {
    const d = parseZap('2026-06-19T14:03:11.5Z\tinfo\theader:\tvalue')
    expect(d.kind).toBe('spans')
    expect(d.spans).toHaveLength(1)
    expect(spansText(d)).toBe('header:\\tvalue')
  })

  it('zap: a caller with a column suffix is classified as the caller', () => {
    const d = parseZap('2026-06-19T14:03:11.5Z\tinfo\tfoo.go:42:13\thi\t{"a":1}')
    expect(d.kind).toBe('block')
    expect(d.rows[0].some(s => s.text === 'foo.go:42:13' && /violet/.test(s.cls))).toBe(true)
  })

  it('zap: a date-led non-zap TSV line is not treated as zap (comma or dot millis, space separator)', () => {
    expect(parseZap('2020-01-02 03:04:05,123\tERROR\tdb\tconnection refused')).toBeNull()
    expect(parseZap('2020-01-02 03:04:05.123\tERROR\tdb\tconnection refused')).toBeNull()
    expect(decorateLine('2020-01-02 03:04:05.123\tERROR\tdb\tconnection refused', 'error').kind).toBe('plain')
  })

  it('zap: an empty message with a JSON blob still reflows the fields', () => {
    const d = parseZap('2026-06-19T14:03:11.5Z\tinfo\t\t{"foo":"bar","n":2}')
    expect(d.kind).toBe('block')
    expect(rowsText(d)).toEqual(['foo: bar', 'n: 2']) // no blank leading message row
  })

  it('zap: a blob field with embedded newlines renders on one line (escaped)', () => {
    const d = parseZap('2026-06-19T14:03:11Z\terror\tfoo.go:1\tboom\t{"error":"x","stacktrace":"goroutine 1\\nmain.foo()"}')
    expect(d.kind).toBe('block')
    expect(rowsText(d).every(t => !t.includes('\n'))).toBe(true)
    expect(kvRows(d)).toContain('stacktrace: goroutine 1\\nmain.foo()')
  })

  it('logfmt: an empty promoted message renders no blank leading row', () => {
    const d = parseLogfmt('level=info msg="" controller=foo')
    expect(d.kind).toBe('block')
    expect(rowsText(d)).toEqual(['level: info', 'controller: foo'])
  })
})
