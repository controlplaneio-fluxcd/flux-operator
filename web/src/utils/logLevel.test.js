// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect } from 'vitest'
import { detectLevel, stripAnsi, LEVELS, LEVEL_META } from './logLevel'

describe('detectLevel', () => {
  it('defaults to info for empty or plain unmarked lines', () => {
    expect(detectLevel('')).toBe('info')
    expect(detectLevel('reconciliation finished in 1.2s')).toBe('info')
  })

  describe('JSON', () => {
    it('reads string level fields and their aliases', () => {
      expect(detectLevel('{"level":"error","msg":"boom"}')).toBe('error')
      expect(detectLevel('{"severity":"WARNING","msg":"x"}')).toBe('warn')
      expect(detectLevel('{"lvl":"err"}')).toBe('error')
      expect(detectLevel('{"level":"dpanic"}')).toBe('error')
      expect(detectLevel('{"log.level":"debug"}')).toBe('debug')
      expect(detectLevel('{"level_name":"ERROR","message":"boom"}')).toBe('error')
      expect(detectLevel('{"@t":"2026-06-16T10:00:00Z","@l":"Error","@m":"boom"}')).toBe('error')
      expect(detectLevel('{"@level":"info","@message":"started","@module":"vault"}')).toBe('info')
    })

    it('reads MongoDB logv2 single-char severity codes', () => {
      expect(detectLevel('{"t":{"$date":"2026-06-16T10:00:00Z"},"s":"E","c":"NETWORK","msg":"x"}')).toBe('error')
      expect(detectLevel('{"t":{},"s":"W","c":"STORAGE","msg":"x"}')).toBe('warn')
      expect(detectLevel('{"t":{},"s":"I","c":"NETWORK","msg":"x"}')).toBe('info')
      expect(detectLevel('{"t":{},"s":"D2","c":"INDEX","msg":"x"}')).toBe('debug')
      expect(detectLevel('{"t":{},"s":"F","c":"CONTROL","msg":"x"}')).toBe('fatal')
    })

    it('keeps scanning past an unmappable numeric level to a later string field', () => {
      // Monolog JsonFormatter emits numeric "level":400 before "level_name".
      expect(detectLevel('{"message":"boom","context":{},"level":400,"level_name":"ERROR"}')).toBe('error')
    })

    it('does not treat an arbitrary string "s" field as a severity code', () => {
      expect(detectLevel('{"s":"running","level":"warn"}')).toBe('warn')
      // A real severity-shaped "s" must not shadow an explicit level field.
      expect(detectLevel('{"s":"E","level":"info"}')).toBe('info')
    })

    it('reads only the top-level level, ignoring nested objects', () => {
      expect(detectLevel('{"payload":{"level":"error"},"level":"info"}')).toBe('info')
    })

    it('reads the ECS nested log.level object', () => {
      expect(detectLevel('{"log":{"level":"warn"},"message":"slow"}')).toBe('warn')
    })

    it('does not classify a JSON line by its message content', () => {
      expect(detectLevel('{"msg":"user typed [error]"}')).toBe('info')
      expect(detectLevel('{"msg":"handled ERROR condition"}')).toBe('info')
    })

    it('does not accept out-of-range MongoDB debug verbosity codes', () => {
      expect(detectLevel('{"t":{},"s":"E9","c":"X","msg":"y"}')).toBe('info')
      expect(detectLevel('{"t":{},"s":"D9","c":"X","msg":"y"}')).toBe('info')
    })

    it('reads numeric pino/bunyan levels (10-60)', () => {
      expect(detectLevel('{"level":10,"msg":"x"}')).toBe('trace')
      expect(detectLevel('{"level":30,"msg":"x"}')).toBe('info')
      expect(detectLevel('{"level":40}')).toBe('warn')
      expect(detectLevel('{"level":50}')).toBe('error')
      expect(detectLevel('{"level":60}')).toBe('fatal')
    })

    it('reads syslog numeric levels (0-7) distinct from the pino scale', () => {
      expect(detectLevel('{"level":3}')).toBe('error')
      expect(detectLevel('{"level":7}')).toBe('debug')
      expect(detectLevel('{"level":0}')).toBe('fatal')
    })
  })

  it('reads klog severity prefixes', () => {
    expect(detectLevel('I0616 10:00:00.123456 1 controller.go:42] reconciling')).toBe('info')
    expect(detectLevel('W0616 10:00:00.123456 1 controller.go:42] slow')).toBe('warn')
    expect(detectLevel('E0616 10:00:00.123456 1 controller.go:42] failed')).toBe('error')
    expect(detectLevel('F0616 10:00:00.123456 1 controller.go:42] fatal')).toBe('fatal')
  })

  it('reads logfmt level fields', () => {
    expect(detectLevel('ts=2026-01-01 level=warn msg="slow"')).toBe('warn')
    expect(detectLevel('level=error component=api')).toBe('error')
  })

  it('reads plain-text uppercase level tokens', () => {
    expect(detectLevel('ERROR something broke')).toBe('error')
    expect(detectLevel('2026-06-16 10:00:00.123  INFO 1 --- [main] c.e.App : started')).toBe('info')
    expect(detectLevel('[WARN] disk almost full')).toBe('warn')
    expect(detectLevel('[2026-06-16T10:00:00.000000+00:00] app.ERROR: boom [] []')).toBe('error')
    expect(detectLevel('[2026-06-16T10:00:00.000000+00:00] request.INFO: handled [] []')).toBe('info')
  })

  it('reads .NET Microsoft.Extensions.Logging console prefixes', () => {
    expect(detectLevel('info: Microsoft.Hosting.Lifetime[0]')).toBe('info')
    expect(detectLevel('warn: MyApp.Worker[0]')).toBe('warn')
    expect(detectLevel('fail: MyApp.Worker[0]')).toBe('error')
    expect(detectLevel('crit: MyApp.Worker[0]')).toBe('fatal')
    expect(detectLevel('dbug: MyApp.Worker[0]')).toBe('debug')
    expect(detectLevel('trce: MyApp.Worker[0]')).toBe('trace')
  })

  it('reads Serilog u3 level codes', () => {
    expect(detectLevel('[12:34:56 INF] Starting up')).toBe('info')
    expect(detectLevel('[12:34:56 WRN] disk slow')).toBe('warn')
    expect(detectLevel('[12:34:56 ERR] request failed')).toBe('error')
    expect(detectLevel('[12:34:56 FTL] crashed')).toBe('fatal')
    expect(detectLevel('[12:34:56 DBG] cache hit')).toBe('debug')
    expect(detectLevel('[12:34:56 VRB] trace detail')).toBe('trace')
  })

  it('reads zerolog console codes', () => {
    expect(detectLevel('TRC trace detail foo=bar')).toBe('trace')
    expect(detectLevel('PNC panic recovered')).toBe('fatal')
  })

  it('reads java.util.logging plain levels', () => {
    expect(detectLevel('FINE: cache populated')).toBe('debug')
    expect(detectLevel('FINEST: entering method')).toBe('trace')
  })

  it('reads leading lowercase level tokens (winston, zap console)', () => {
    expect(detectLevel('error: connection refused')).toBe('error')
    expect(detectLevel('debug: cache miss')).toBe('debug')
    expect(detectLevel('verbose: tracing request')).toBe('trace')
    expect(detectLevel('warn: retrying')).toBe('warn')
    expect(detectLevel('info\tcaller\tserver started')).toBe('info')
  })

  it('does not treat a leading prose word as a level', () => {
    expect(detectLevel('error handling middleware registered ok')).toBe('info')
  })

  it('reads a zap console level from the 2nd tab field after a timestamp', () => {
    // zap leads with its own ISO-8601 or epoch timestamp, so the level is hidden
    // from the leading-token matcher; it is also lowercase, so the uppercase text
    // matcher misses it. Read it from the 2nd tab field.
    expect(detectLevel('2026-06-19T14:03:11.500Z\terror\tfoo.go:1\tboom\t{"a":1}')).toBe('error')
    expect(detectLevel('2026-06-19T14:03:11.500Z\twarn\tcontroller\tfoo.go:1\tslow')).toBe('warn')
    expect(detectLevel('1718805791.5\tinfo\tstarting manager')).toBe('info')
    // A tabbed line whose 1st field is not a timestamp is not treated as zap.
    expect(detectLevel('col1\terror\tcol3')).toBe('info')
  })

  it('reads bracketed lowercase/mixed-case level tokens', () => {
    expect(detectLevel('[error] 1#1: *1 connect() failed')).toBe('error') // nginx error_log
    expect(detectLevel('[warn] 1#1: low on memory')).toBe('warn') // nginx
    expect(detectLevel('[crit] 1#1: out of memory')).toBe('fatal') // nginx
    expect(detectLevel('[16][warning][config] xds update rejected')).toBe('warn') // Envoy/Istio
    expect(detectLevel('[16][error][upstream] connection failure')).toBe('error') // Envoy
    expect(detectLevel('[Warning] [MY-010055] connection aborted')).toBe('warn') // MySQL
    expect(detectLevel('[info     ] request handled    method=GET')).toBe('info') // structlog
  })

  it('does not match a non-level bracketed word', () => {
    expect(detectLevel('[main] starting reconciler loop')).toBe('info')
  })

  it('ignores a bracketed level inside a message or logfmt value', () => {
    expect(detectLevel('request completed message="user typed [error]"')).toBe('info')
    expect(detectLevel('msg=processing note=[error] handled gracefully')).toBe('info')
  })

  it('does not capture a partial logfmt level value', () => {
    expect(detectLevel('level=error_code component=api')).toBe('info')
    expect(detectLevel('level=warn component=api')).toBe('warn')
  })

  it('reads a uppercase level token followed by a slash (Celery)', () => {
    expect(detectLevel('[ERROR/MainProcess] task failed')).toBe('error')
  })

  it('does not match the word "error" mid-message (lowercase)', () => {
    expect(detectLevel('handled the error gracefully and continued')).toBe('info')
  })
})

describe('stripAnsi', () => {
  it('removes SGR color escapes', () => {
    expect(stripAnsi('\x1b[31merror\x1b[0m here')).toBe('error here')
  })

  it('returns plain text unchanged', () => {
    expect(stripAnsi('no escapes here')).toBe('no escapes here')
  })

  it('lets level detection work through ANSI codes once stripped', () => {
    expect(detectLevel(stripAnsi('\x1b[33mWARN\x1b[0m disk full'))).toBe('warn')
  })
})

describe('metadata', () => {
  it('has display metadata for every level', () => {
    for (const l of LEVELS) {
      expect(LEVEL_META[l]).toBeDefined()
      expect(LEVEL_META[l].label).toBeTruthy()
      expect(LEVEL_META[l].border).toBeTruthy()
      expect(LEVEL_META[l].swatch).toBeTruthy()
      expect(LEVEL_META[l].glow).toMatch(/^rgba\(\d+,\d+,\d+,[\d.]+\)$/)
    }
  })
})
