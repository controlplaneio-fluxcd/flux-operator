// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect } from 'vitest'
import { decorateLine, fieldMatcher, highlightJson, parseDotnet, parseJava, parseKlog, parseLogfmt, parseMonolog, parsePython, parseRuby, parseZap, tokenizeKV, topLevelJsonKeys } from './logFormat'

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

  it('keeps internal multi-space runs in a bare value and stops at the next key=', () => {
    const { fields } = tokenizeKV('msg=word1    word2     word3 next=ok')
    expect(fields).toEqual([
      { key: 'msg', val: 'word1    word2     word3', kind: 'str' },
      { key: 'next', val: 'ok', kind: 'str' }
    ])
  })

  it('parses a long consecutive-space run in a bare value in linear time', () => {
    // Pre-fix, readValue rescanned the whole space run at every space (O(m²)): this
    // size took several seconds. Post-fix it jumps past the run, so it is linear and
    // returns the value intact. The tight time bound is the regression guard.
    const spaces = ' '.repeat(120000)
    const input = `msg=start${spaces}end tail=ok`
    const t0 = Date.now()
    const { fields } = tokenizeKV(input)
    const elapsed = Date.now() - t0
    expect(fields).toHaveLength(2)
    expect(fields[0].key).toBe('msg')
    expect(fields[0].val).toBe(`start${spaces}end`)
    expect(fields[1]).toEqual({ key: 'tail', val: 'ok', kind: 'str' })
    expect(elapsed).toBeLessThan(1000)
  })
})

describe('parseKlog', () => {
  it('decorates a structured klog.ErrorS line into a block (header+message, fields)', () => {
    const line = 'E0526 23:03:57.521582       1 leaderelection.go:452] "Error retrieving lease lock" err="Get \\"https://10.96.0.1:443\\": i/o timeout" logger="cert-manager.controller" lock="kube-system/cert-manager-controller"'
    const d = parseKlog(line, 'error')
    expect(d.kind).toBe('block')
    const rows = rowsText(d)
    // Header rebuilt with klog's PID padding collapsed to a single space, and the
    // unquoted message kept on the same row as the header.
    expect(rows[0]).toBe('E0526 23:03:57.521582 1 leaderelection.go:452] Error retrieving lease lock')
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
    // PID padding collapsed to a single space.
    expect(spansText(d)).toBe('I0612 14:03:11.123456 12 server.go:80] "GET /healthz" completed')
  })

  it('decorates a classic klog line as a single highlighted row', () => {
    const line = 'I0612 14:03:11.123456   12 reflector.go:243] Watch close - watch chan closed'
    const d = parseKlog(line, 'info')
    expect(d.kind).toBe('spans')
    // PID padding collapsed to a single space.
    expect(spansText(d)).toBe('I0612 14:03:11.123456 12 reflector.go:243] Watch close - watch chan closed')
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

describe('parseJava', () => {
  describe('Spring Boot', () => {
    it('decorates a 3.x line with app + thread brackets, keeping the level and accenting the logger', () => {
      const line = '2025-12-18T07:25:01.584Z  INFO 132568 --- [myapp] [           main] o.s.b.d.f.logexample.MyApplication       : Started in 3.1s'
      const d = parseJava(line)
      expect(d.kind).toBe('spans')
      // Level kept, brackets trimmed (15-char thread padding collapsed), logger, message.
      expect(spansText(d)).toBe('INFO  [myapp] [main]  o.s.b.d.f.logexample.MyApplication  Started in 3.1s')
      // Level tinted blue (info), brackets muted, logger violet.
      expect(d.spans.some(s => s.text === 'INFO' && /blue/.test(s.cls))).toBe(true)
      expect(d.spans.some(s => s.text === '[myapp] [main]' && /gray-4/.test(s.cls))).toBe(true)
      expect(d.spans.some(s => s.text === 'o.s.b.d.f.logexample.MyApplication' && /violet/.test(s.cls))).toBe(true)
    })

    it('handles a line with only the padded thread bracket (no app name)', () => {
      const d = parseJava('2026-06-16T10:00:00.123Z  WARN 12345 --- [  scheduler-1] c.e.Scheduler : tick')
      expect(spansText(d)).toBe('WARN  [scheduler-1]  c.e.Scheduler  tick')
      expect(d.spans.some(s => s.text === 'WARN' && /amber/.test(s.cls))).toBe(true)
    })

    it('keeps a Micrometer correlation bracket', () => {
      const d = parseJava('2026-06-16T10:00:00.123Z  INFO 7 --- [demo] [  main] [abc123-def456] c.e.Trace : handled')
      expect(spansText(d)).toBe('INFO  [demo] [main] [abc123-def456]  c.e.Trace  handled')
    })

    it('matches a 2.x space-separated local timestamp with dot millis and no zone', () => {
      const d = parseJava('2019-08-30 12:30:04.031  INFO 22174 --- [  nio-8080-exec-0] demo.Controller : handled')
      expect(d.kind).toBe('spans')
      expect(spansText(d)).toBe('INFO  [nio-8080-exec-0]  demo.Controller  handled')
    })

    it('matches when the PID is blank (${PID:- })', () => {
      const d = parseJava('2026-06-16T10:00:00.123Z  INFO  --- [  main] c.e.App : up')
      expect(d.kind).toBe('spans')
      expect(spansText(d)).toBe('INFO  [main]  c.e.App  up')
    })

    it('matches the root logger (empty logger name), omitting the logger span', () => {
      const d = parseJava('2026-06-16T10:00:00.123Z ERROR 1 --- [  main]  : boom')
      expect(d.kind).toBe('spans')
      expect(spansText(d)).toBe('ERROR  [main]  boom')
    })

    it('captures a logger that itself contains brackets (Tomcat) whole', () => {
      const d = parseJava('2026-06-16T10:00:00.123Z  INFO 1 --- [  main] o.a.c.c.C.[Tomcat].[localhost].[/]       : init')
      expect(d.kind).toBe('spans')
      expect(d.spans.some(s => s.text === 'o.a.c.c.C.[Tomcat].[localhost].[/]' && /violet/.test(s.cls))).toBe(true)
      expect(spansText(d)).toBe('INFO  [main]  o.a.c.c.C.[Tomcat].[localhost].[/]  init')
    })
  })

  describe('Log4j2 / Logback', () => {
    it('decorates a default time-only line (thread before level, " - " separator)', () => {
      const d = parseJava('10:00:00.123 [main] INFO  com.example.Demo - Started')
      expect(d.kind).toBe('spans')
      expect(spansText(d)).toBe('[main]  INFO  com.example.Demo  Started')
      expect(d.spans.some(s => s.text === '[main]' && /gray-4/.test(s.cls))).toBe(true)
      expect(d.spans.some(s => s.text === 'com.example.Demo' && /violet/.test(s.cls))).toBe(true)
    })

    it('matches a Logback full date-time with comma millis and ERROR', () => {
      const d = parseJava('2026-06-16 10:00:00,123 [main] ERROR com.example.Demo - boom')
      expect(d.kind).toBe('spans')
      expect(spansText(d)).toBe('[main]  ERROR  com.example.Demo  boom')
      expect(d.spans.some(s => s.text === 'ERROR' && /red/.test(s.cls))).toBe(true)
    })

    it('matches the root logger (empty logger name)', () => {
      const d = parseJava('10:00:00.123 [main] ERROR  - Root logger message')
      expect(d.kind).toBe('spans')
      expect(spansText(d)).toBe('[main]  ERROR  Root logger message')
    })

    it('does not mis-split a message containing " - "', () => {
      const d = parseJava('10:00:00.123 [main] INFO com.example.Demo - task a - b done')
      expect(spansText(d)).toBe('[main]  INFO  com.example.Demo  task a - b done')
    })
  })

  describe('negatives', () => {
    it('returns null for a tab-separated line (Java console is space-delimited)', () => {
      expect(parseJava('2026-06-16 10:00:00.123\t[worker]\tERROR\tpayments\t-\tdeclined')).toBeNull()
    })

    it('returns null for a Go zap TSV line and a klog line', () => {
      expect(parseJava('2026-06-19T14:03:11.500Z\tinfo\tfoo.go:1\thi')).toBeNull()
      expect(parseJava('I0612 14:03:11.123456 12 reflector.go:243] watch closed')).toBeNull()
    })

    it('returns null for Java stack-trace continuation lines (routed to plain)', () => {
      expect(parseJava('\tat com.example.Foo.bar(Foo.java:42)')).toBeNull()
      expect(parseJava('Caused by: java.lang.NullPointerException: x')).toBeNull()
      expect(parseJava('\t... 3 more')).toBeNull()
      expect(parseJava('java.lang.IllegalStateException: bad')).toBeNull()
    })

    it('returns null for plain prose', () => {
      expect(parseJava('just a plain sentence')).toBeNull()
      expect(parseJava('2026 was a good year for logging')).toBeNull()
    })
  })
})

describe('parseDotnet', () => {
  describe('Serilog', () => {
    it('decorates the default no-date template, keeping and tinting the u3 code', () => {
      const d = parseDotnet('[14:30:00 INF] Starting up')
      expect(d.kind).toBe('spans')
      expect(spansText(d)).toBe('INF  Starting up')
      expect(d.spans.some(s => s.text === 'INF' && /blue/.test(s.cls))).toBe(true)
    })

    it('maps every u3 code to its tint', () => {
      expect(parseDotnet('[14:30:00 ERR] x').spans.some(s => s.text === 'ERR' && /red/.test(s.cls))).toBe(true)
      expect(parseDotnet('[14:30:00 WRN] x').spans.some(s => s.text === 'WRN' && /amber/.test(s.cls))).toBe(true)
      expect(parseDotnet('[14:30:00 FTL] x').spans.some(s => s.text === 'FTL' && /red-6/.test(s.cls))).toBe(true)
      expect(parseDotnet('[14:30:00 DBG] x').spans.some(s => s.text === 'DBG' && /slate/.test(s.cls))).toBe(true)
      expect(parseDotnet('[14:30:00 VRB] x').spans.some(s => s.text === 'VRB' && /gray-5/.test(s.cls))).toBe(true)
    })

    it('tolerates an ISO timestamp variant (date + fractional + zone)', () => {
      const d = parseDotnet('[2024-01-15T14:30:00.123Z INF] booted')
      expect(d.kind).toBe('spans')
      expect(spansText(d)).toBe('INF  booted')
    })

    it('returns null when the bracketed token is not a u3 level code', () => {
      expect(parseDotnet('[12:00:00 ABC] not serilog')).toBeNull()
    })
  })

  describe('NLog', () => {
    it('decorates the default pipe layout: level tinted, logger accented, message', () => {
      const d = parseDotnet('2024-01-15 10:30:00.0000|WARN|MyApp.Program|slow start')
      expect(d.kind).toBe('spans')
      expect(spansText(d)).toBe('WARN  MyApp.Program  slow start')
      expect(d.spans.some(s => s.text === 'WARN' && /amber/.test(s.cls))).toBe(true)
      expect(d.spans.some(s => s.text === 'MyApp.Program' && /violet/.test(s.cls))).toBe(true)
    })

    it('handles an empty logger field', () => {
      const d = parseDotnet('2024-01-15 10:30:00.0000|ERROR||boom')
      expect(d.kind).toBe('spans')
      expect(spansText(d)).toBe('ERROR  boom')
    })

    it('keeps an exception riding the message field (first line)', () => {
      const d = parseDotnet('2024-01-15 10:30:00.0000|ERROR|MyApp.Svc|failed: System.Exception: x')
      expect(spansText(d)).toBe('ERROR  MyApp.Svc  failed: System.Exception: x')
    })

    it('does not match a free-form pipe line without a leading longdate', () => {
      expect(parseDotnet('result|ERROR|stack trace follows')).toBeNull()
    })
  })

  describe('negatives', () => {
    it('returns null for zap, klog, and plain prose', () => {
      expect(parseDotnet('2026-06-19T14:03:11.500Z\tinfo\tfoo.go:1\thi')).toBeNull()
      expect(parseDotnet('I0612 14:03:11.123456 12 reflector.go:243] watch closed')).toBeNull()
      expect(parseDotnet('just a plain sentence')).toBeNull()
    })
  })
})

describe('parsePython', () => {
  it('decorates a common timestamped logging format (comma millis), level tinted, name accented', () => {
    const d = parsePython('2024-01-15 10:30:00,123 - myapp.module - WARNING - disk almost full')
    expect(d.kind).toBe('spans')
    expect(spansText(d)).toBe('WARNING  myapp.module  disk almost full')
    expect(d.spans.some(s => s.text === 'WARNING' && /amber/.test(s.cls))).toBe(true)
    expect(d.spans.some(s => s.text === 'myapp.module' && /violet/.test(s.cls))).toBe(true)
  })

  it('keeps a hyphenated logger name and maps CRITICAL to fatal', () => {
    const d = parsePython('2024-01-15 10:30:00.000 - my-app - CRITICAL - dead')
    expect(spansText(d)).toBe('CRITICAL  my-app  dead')
    expect(d.spans.some(s => s.text === 'CRITICAL' && /red-6/.test(s.cls))).toBe(true)
  })

  it('decorates a gunicorn line: pid muted, level tinted', () => {
    const d = parsePython('[2024-01-15 10:30:00 +0000] [8] [ERROR] Worker (pid:8) was sent SIGKILL!')
    expect(d.kind).toBe('spans')
    expect(spansText(d)).toBe('[8]  ERROR  Worker (pid:8) was sent SIGKILL!')
    expect(d.spans.some(s => s.text === '[8]' && /gray-4/.test(s.cls))).toBe(true)
    expect(d.spans.some(s => s.text === 'ERROR' && /red/.test(s.cls))).toBe(true)
  })

  it('decorates uvicorn padded prefixes, including CRITICAL (one space)', () => {
    expect(spansText(parsePython('INFO:     Started server process [1]'))).toBe('INFO  Started server process [1]')
    expect(spansText(parsePython('CRITICAL: Application startup failed. Exiting.'))).toBe('CRITICAL  Application startup failed. Exiting.')
    // TRACE (uvicorn --log-level trace): 5-char level → 4 spaces (still width 9).
    expect(spansText(parsePython('TRACE:    ASGI [1] Started'))).toBe('TRACE  ASGI [1] Started')
    // Access-log line with colons/dashes in the message stays intact.
    expect(spansText(parsePython('INFO:     127.0.0.1:0 - "GET / HTTP/1.1" 200 OK'))).toBe('INFO  127.0.0.1:0 - "GET / HTTP/1.1" 200 OK')
  })

  it('rejects prose that is not uvicorn-padded (single space after the level colon)', () => {
    expect(parsePython('INFO: just one space here')).toBeNull()
    expect(parsePython('ERROR: see above')).toBeNull()
  })

  it('returns null for non-Python lines', () => {
    expect(parsePython('[14:30:00 INF] serilog')).toBeNull()
    expect(parsePython('a plain sentence')).toBeNull()
  })
})

describe('parseRuby', () => {
  it('decorates a line with an empty progname', () => {
    const d = parseRuby('I, [2024-01-15T10:30:00.123456 #1]  INFO -- : Started')
    expect(d.kind).toBe('spans')
    expect(spansText(d)).toBe('INFO  Started')
  })

  it('accents the progname and tints the level when present', () => {
    const d = parseRuby('F, [2024-01-15T10:30:00.123456 #1] FATAL -- MyApp: boom')
    expect(spansText(d)).toBe('FATAL  MyApp  boom')
    expect(d.spans.some(s => s.text === 'MyApp' && /violet/.test(s.cls))).toBe(true)
    expect(d.spans.some(s => s.text === 'FATAL' && /red/.test(s.cls))).toBe(true)
  })

  it('tolerates the stdlib trailing-space datetime (two spaces before #)', () => {
    const d = parseRuby('W, [2024-01-15T10:30:00.123456  #1]  WARN -- : careful')
    expect(spansText(d)).toBe('WARN  careful')
  })

  it('formats the ANY (UNKNOWN) level, tinting it muted (unknown severity)', () => {
    const d = parseRuby('A, [2024-01-15T10:30:00.123456 #1]  ANY -- : unknown-level')
    expect(spansText(d)).toBe('ANY  unknown-level')
    // ANY is not a recognized severity, so it renders muted — consistent with the
    // info pill/filter (see logLevel.test for the matching detection assertion).
    expect(d.spans.some(s => s.text === 'ANY' && /gray-4/.test(s.cls))).toBe(true)
  })

  it('returns null for non-Ruby lines', () => {
    expect(parseRuby('Info, something happened')).toBeNull()
    expect(parseRuby('a plain sentence')).toBeNull()
  })
})

describe('parseMonolog', () => {
  it('decorates an ISO+offset line, channel accented, context kept in the message', () => {
    const d = parseMonolog('[2024-01-15T10:30:00.123456+00:00] app.ERROR: Something failed {"k":"v"} []')
    expect(d.kind).toBe('spans')
    expect(spansText(d)).toBe('ERROR  app  Something failed {"k":"v"} []')
    expect(d.spans.some(s => s.text === 'app' && /violet/.test(s.cls))).toBe(true)
    expect(d.spans.some(s => s.text === 'ERROR' && /red/.test(s.cls))).toBe(true)
  })

  it('maps the upper PSR-3 levels (NOTICE→info, EMERGENCY→fatal)', () => {
    expect(parseMonolog('[2024-01-15 10:30:00] app.NOTICE: heads up [] []').spans.some(s => s.text === 'NOTICE' && /blue/.test(s.cls))).toBe(true)
    expect(parseMonolog('[2024-01-15 10:30:00] app.EMERGENCY: down [] []').spans.some(s => s.text === 'EMERGENCY' && /red-6/.test(s.cls))).toBe(true)
  })

  it('returns null for non-Monolog bracketed lines', () => {
    expect(parseMonolog('[14:30:00 INF] serilog')).toBeNull()
    expect(parseMonolog('[2024-01-15 10:30:00 +0000] [1] [INFO] gunicorn')).toBeNull()
  })
})

describe('decorateLine cascade', () => {
  it('routes klog, zap, java, dotnet, and logfmt to their parsers and prose to plain', () => {
    expect(decorateLine('I0612 14:03:11.123456 12 reflector.go:243] watch closed', 'info').kind).toBe('spans')
    expect(decorateLine('2026-06-19T14:03:11.5Z\tinfo\tfoo.go:1\thi\t{"a":1}', 'info').kind).toBe('block')
    expect(decorateLine('2025-12-18T07:25:01.584Z  INFO 1 --- [  main] c.e.App : up', 'info').kind).toBe('spans')
    expect(decorateLine('10:00:00.123 [main] INFO com.example.Demo - started', 'info').kind).toBe('spans')
    expect(decorateLine('[14:30:00 INF] Starting up', 'info').kind).toBe('spans')
    expect(decorateLine('2024-01-15 10:30:00.0000|WARN|MyApp.Program|slow', 'warn').kind).toBe('spans')
    expect(decorateLine('2024-01-15 10:30:00,123 - app - INFO - up', 'info').kind).toBe('spans')
    expect(decorateLine('[2024-01-15 10:30:00 +0000] [1] [INFO] gunicorn', 'info').kind).toBe('spans')
    expect(decorateLine('INFO:     uvicorn', 'info').kind).toBe('spans')
    expect(decorateLine('I, [2024-01-15T10:30:00.1 #1]  INFO -- : ruby', 'info').kind).toBe('spans')
    expect(decorateLine('[2024-01-15T10:30:00+00:00] app.ERROR: monolog [] []', 'error').kind).toBe('spans')
    expect(decorateLine('level=info msg=hi controller=foo', 'info').kind).toBe('block')
    expect(decorateLine('a plain sentence with no structure', 'info').kind).toBe('plain')
  })

  it('routes a log4net log4j-style PatternLayout line via the Java Log4j2 branch', () => {
    const d = decorateLine('2024-12-21 14:07:41,517 [main] WARN  Animals.Carnivora.Dog - Meow!', 'warn')
    expect(d.kind).toBe('spans')
    expect(d.spans.some(s => s.text === 'Animals.Carnivora.Dog' && /violet/.test(s.cls))).toBe(true)
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

describe('highlightJson', () => {
  it('pretty-prints an object flush-left, dropping the outer braces', () => {
    const d = highlightJson('{"level":"info","msg":"hello"}')
    expect(d.kind).toBe('json')
    expect(spansText(d)).toBe('"level": "info",\n"msg": "hello"')
  })

  it('colors keys gray and leaves scalars in the body color', () => {
    const d = highlightJson('{"n":1}')
    expect(d.spans.find(s => s.text === '"n"').cls).toMatch(/gray-5/)
    expect(d.spans.find(s => s.text === '1').cls).toBe('') // CLS.val inherits body color
  })

  it('keeps nested objects and arrays indented, with only the outer braces dropped', () => {
    const src = '{"a":{"b":[1,2,{"c":true}]},"d":null}'
    expect(spansText(highlightJson(src))).toBe(
      '"a": {\n  "b": [\n    1,\n    2,\n    {\n      "c": true\n    }\n  ]\n},\n"d": null'
    )
  })

  it('renders a top-level array flush-left without the outer brackets', () => {
    expect(spansText(highlightJson('[1,"x",false]'))).toBe('1,\n"x",\nfalse')
  })

  it('keeps an empty top-level object or array on one line', () => {
    expect(spansText(highlightJson('{}'))).toBe('{}')
    expect(spansText(highlightJson('[]'))).toBe('[]')
  })

  it('keeps nested empty objects/arrays inline', () => {
    expect(spansText(highlightJson('{"a":{},"b":[]}'))).toBe('"a": {},\n"b": []')
  })

  it('escapes control characters in string values, keeping them on one line', () => {
    expect(spansText(highlightJson('{"m":"a\\nb"}'))).toBe('"m": "a\\nb"')
  })

  it('renders a non-finite number (over-range literal) as null, like JSON.stringify', () => {
    // 1e999 parses to Infinity; JSON.stringify emits null, not "Infinity".
    expect(spansText(highlightJson('{"n":1e999}'))).toBe('"n": null')
  })

  it('does not throw on pathologically deep nesting, falling back to plain', () => {
    const deep = '['.repeat(200000) + ']'.repeat(200000)
    expect(() => highlightJson(deep)).not.toThrow()
    expect(highlightJson(deep)).toBeNull()
  })

  it('ignores whitespace around the JSON', () => {
    expect(spansText(highlightJson('  {"a":1}  '))).toBe('"a": 1')
  })

  it('returns null for non-JSON, a bare scalar, or malformed input', () => {
    expect(highlightJson('plain text line')).toBeNull()
    expect(highlightJson('42')).toBeNull() // parses, but not an object/array
    expect(highlightJson('"hi"')).toBeNull()
    expect(highlightJson('{not json}')).toBeNull()
    expect(highlightJson('{"a":1')).toBeNull() // truncated
  })

  describe('field selection', () => {
    it('projects a top-level object to the selected keys, in source order', () => {
      const d = highlightJson('{"level":"info","ts":"t","msg":"hi","ns":"x"}', new Set(['msg', 'ns']))
      expect(spansText(d)).toBe('"msg": "hi",\n"ns": "x"')
    })

    it('shows every field when the selection is null/undefined', () => {
      expect(spansText(highlightJson('{"a":1,"b":2}'))).toBe('"a": 1,\n"b": 2')
      expect(spansText(highlightJson('{"a":1,"b":2}', null))).toBe('"a": 1,\n"b": 2')
    })

    it('renders {} when the selection keeps none of the line\'s fields', () => {
      expect(spansText(highlightJson('{"a":1}', new Set(['other'])))).toBe('{}')
    })

    it('keeps a selected nested value whole', () => {
      const d = highlightJson('{"msg":"hi","obj":{"x":1}}', new Set(['obj']))
      expect(spansText(d)).toBe('"obj": {\n  "x": 1\n}')
    })

    it('leaves a top-level array unaffected by a selection (no field keys)', () => {
      expect(spansText(highlightJson('[1,2]', new Set(['msg'])))).toBe('1,\n2')
    })

    it('keeps a literal "__proto__" field when selected (no prototype mutation)', () => {
      const d = highlightJson('{"__proto__":"x","msg":"hi"}', new Set(['__proto__']))
      expect(spansText(d)).toBe('"__proto__": "x"')
    })
  })
})

describe('fieldMatcher', () => {
  const keep = (expr, keys) => keys.filter(fieldMatcher(expr) || (() => true))

  it('returns null for an empty or token-less expression (all fields)', () => {
    expect(fieldMatcher('')).toBeNull()
    expect(fieldMatcher('   ')).toBeNull()
    expect(fieldMatcher('!')).toBeNull() // a lone "!" yields no usable token
  })

  it('matches exact field names, case-insensitively, not as substrings', () => {
    const m = fieldMatcher('msg')
    expect(m('msg')).toBe(true)
    expect(m('MSG')).toBe(true)
    expect(m('msgCount')).toBe(false)
    expect(m('message')).toBe(false)
  })

  it('splits on spaces, commas, and pipes interchangeably', () => {
    const keys = ['level', 'ts', 'msg', 'message', 'error']
    expect(keep('msg message error', keys)).toEqual(['msg', 'message', 'error'])
    expect(keep('msg, message, error', keys)).toEqual(['msg', 'message', 'error'])
    expect(keep('msg|message|error', keys)).toEqual(['msg', 'message', 'error'])
  })

  it('treats * as a glob wildcard', () => {
    const keys = ['reconcileID', 'controllerKind', 'controllerGroup', 'msg']
    expect(keep('*id', keys)).toEqual(['reconcileID'])
    expect(keep('controller*', keys)).toEqual(['controllerKind', 'controllerGroup'])
    expect(keep('*o*', keys)).toEqual(['reconcileID', 'controllerKind', 'controllerGroup'])
  })

  it('excludes fields prefixed with !, keeping everything else', () => {
    const keys = ['level', 'ts', 'msg', 'ns']
    expect(keep('!level !ts', keys)).toEqual(['msg', 'ns'])
    expect(keep('!controller*', ['controllerKind', 'msg', 'level'])).toEqual(['msg', 'level'])
  })

  it('lets an exclusion override an inclusion', () => {
    const keys = ['msg', 'error', 'level']
    expect(keep('msg error !error', keys)).toEqual(['msg'])
  })

  it('matches multi-* globs in order without catastrophic backtracking', () => {
    const m = fieldMatcher('a*b*c')
    expect(m('axxbxxc')).toBe(true)
    expect(m('abc')).toBe(true)
    expect(m('axbxd')).toBe(false) // no trailing c
    expect(m('ac')).toBe(false)    // missing b between
  })

  it('resolves a pathological *-heavy pattern on a long key quickly (no ReDoS)', () => {
    // A chained-.* regex would hang here; the linear matcher returns at once.
    const m = fieldMatcher('*a*a*a*a*a*a*a*a*a*a*a*a*a*a*a*a*b')
    const key = 'a'.repeat(2000)
    const start = Date.now()
    expect(m(key)).toBe(false) // no trailing b
    expect(Date.now() - start).toBeLessThan(1000)
  })
})

describe('topLevelJsonKeys', () => {
  it('returns the top-level keys of a JSON object in source order', () => {
    expect(topLevelJsonKeys('{"level":"info","msg":"hi","ns":"x"}')).toEqual(['level', 'msg', 'ns'])
  })

  it('ignores surrounding whitespace', () => {
    expect(topLevelJsonKeys('  {"a":1}  ')).toEqual(['a'])
  })

  it('returns null for an array, a scalar, or non-JSON', () => {
    expect(topLevelJsonKeys('[1,2]')).toBeNull()
    expect(topLevelJsonKeys('42')).toBeNull()
    expect(topLevelJsonKeys('"hi"')).toBeNull()
    expect(topLevelJsonKeys('plain text')).toBeNull()
    expect(topLevelJsonKeys('{"a":1')).toBeNull() // truncated
  })
})
