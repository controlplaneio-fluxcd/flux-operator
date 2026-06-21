// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect } from 'vitest'
import { groupEntries } from './logGroup'
import { parseLogLine } from '../components/dashboards/workload/WorkloadLogsViewer'

// The fixtures below are verbatim `kubectl logs --timestamps` output captured from
// throwaway pods on a containerd cluster (the same wire format the backend serves
// with PodLogOptions{Timestamps: true}). Every physical line — frames included —
// carries its own RFC3339 timestamp, and the kubelet separates it from the verbatim
// line with a single space, so the frame's own indentation survives. `\t` is a real
// tab; the lone "<ts> " line is a timestamped blank. Run through the production
// parseLogLine so the {ts,text} shape under test equals what the viewer builds.

// Go panic: flush-left function frames (`main.c(...)`), tab-indented file lines
// (`\t/m.go:N`), a leading blank, bracketed by stdout `starting` / `exit status 2`.
const GO_PANIC = [
  '2026-06-20T10:09:30.111653171Z starting',
  '2026-06-20T10:09:30.115719629Z panic: boom from c',
  '2026-06-20T10:09:30.115726921Z ',
  '2026-06-20T10:09:30.115727796Z goroutine 1 [running]:',
  '2026-06-20T10:09:30.115728504Z main.c(...)',
  '2026-06-20T10:09:30.115729171Z \t/m.go:2',
  '2026-06-20T10:09:30.115729754Z main.b(...)',
  '2026-06-20T10:09:30.115730296Z \t/m.go:3',
  '2026-06-20T10:09:30.115730921Z main.a(...)',
  '2026-06-20T10:09:30.115731463Z \t/m.go:4',
  '2026-06-20T10:09:30.115731963Z main.main()',
  '2026-06-20T10:09:30.115732504Z \t/m.go:5 +0x4c',
  '2026-06-20T10:09:30.115891088Z exit status 2',
]

// Python traceback: 2/4-space-indented File/source/caret frames, flush-left final
// `ValueError:`, then stdout `starting` interleaved after the trace.
const PY_TRACE = [
  '2026-06-20T10:10:08.026235800Z Traceback (most recent call last):',
  '2026-06-20T10:10:08.026462300Z   File "<string>", line 4, in <module>',
  '2026-06-20T10:10:08.026465467Z     print("starting"); a()',
  '2026-06-20T10:10:08.026466300Z                        ~^^',
  '2026-06-20T10:10:08.026466925Z   File "<string>", line 1, in a',
  '2026-06-20T10:10:08.026467508Z     def a(): b()',
  '2026-06-20T10:10:08.026468175Z              ~^^',
  '2026-06-20T10:10:08.026468675Z   File "<string>", line 2, in b',
  '2026-06-20T10:10:08.026469258Z     def b(): c()',
  '2026-06-20T10:10:08.026469842Z              ~^^',
  '2026-06-20T10:10:08.026470342Z   File "<string>", line 3, in c',
  '2026-06-20T10:10:08.026470842Z     def c(): raise ValueError("boom from c")',
  '2026-06-20T10:10:08.026471425Z              ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^',
  '2026-06-20T10:10:08.026472092Z ValueError: boom from c',
  '2026-06-20T10:10:08.026505175Z starting',
]

// Node error: a flush-left preamble (`[eval]:1`, source echo, caret) precedes the
// bare `Error:` head; frames are 4-space `    at …`; a `Node.js vX` footer closes.
const NODE_ERR = [
  '2026-06-20T10:12:05.325763924Z starting',
  '2026-06-20T10:12:05.326386049Z [eval]:1',
  '2026-06-20T10:12:05.326389049Z function a(){b()} function b(){c()} function c(){throw new Error("boom from c")} console.log("starting"); a()',
  '2026-06-20T10:12:05.326390007Z                                                  ^',
  '2026-06-20T10:12:05.326390590Z ',
  '2026-06-20T10:12:05.326391382Z Error: boom from c',
  '2026-06-20T10:12:05.326391965Z     at c ([eval]:1:56)',
  '2026-06-20T10:12:05.326392465Z     at b ([eval]:1:32)',
  '2026-06-20T10:12:05.326392965Z     at a ([eval]:1:14)',
  '2026-06-20T10:12:05.326393507Z     at [eval]:1:107',
  '2026-06-20T10:12:05.326394049Z     at runScriptInThisContext (node:internal/vm:219:10)',
  '2026-06-20T10:12:05.326394715Z     at node:internal/process/execution:483:12',
  '2026-06-20T10:12:05.326395340Z     at [eval]-wrapper:6:24',
  '2026-06-20T10:12:05.326396007Z     at runScriptInContext (node:internal/process/execution:481:60)',
  '2026-06-20T10:12:05.326396549Z     at evalFunction (node:internal/process/execution:315:30)',
  '2026-06-20T10:12:05.326397007Z     at evalTypeScript (node:internal/process/execution:327:3)',
  '2026-06-20T10:12:05.326397465Z ',
  '2026-06-20T10:12:05.326398049Z Node.js v26.3.1',
]

// Java exception: tab-indented `\tat …` frames and a flush-left `Caused by:` chain.
const JAVA_TRACE = [
  '2026-06-20T10:31:05.433962257Z starting',
  '2026-06-20T10:31:05.434200507Z Exception in thread "main" java.lang.IllegalStateException: boom from c',
  '2026-06-20T10:31:05.434203507Z \tat App.c(App.java:2)',
  '2026-06-20T10:31:05.434233965Z \tat App.b(App.java:3)',
  '2026-06-20T10:31:05.434235882Z \tat App.a(App.java:4)',
  '2026-06-20T10:31:05.434236465Z \tat App.main(App.java:5)',
  '2026-06-20T10:31:05.434296965Z Caused by: java.lang.RuntimeException: root cause',
  '2026-06-20T10:31:05.434298882Z \tat App.c(App.java:2)',
  '2026-06-20T10:31:05.434299549Z \tat App.b(App.java:3)',
  '2026-06-20T10:31:05.434326882Z \tat App.a(App.java:4)',
  '2026-06-20T10:31:05.434328424Z \tat App.main(App.java:5)',
  '2026-06-20T10:31:05.434329299Z \tat java.base/jdk.internal.reflect.DirectMethodHandleAccessor.invoke(DirectMethodHandleAccessor.java:103)',
  '2026-06-20T10:31:05.434375465Z \tat java.base/java.lang.reflect.Method.invoke(Method.java:580)',
  '2026-06-20T10:31:05.434378632Z \tat jdk.compiler/com.sun.tools.javac.launcher.Main.execute(Main.java:484)',
  '2026-06-20T10:31:05.434379632Z \tat jdk.compiler/com.sun.tools.javac.launcher.Main.run(Main.java:208)',
  '2026-06-20T10:31:05.434411215Z \tat jdk.compiler/com.sun.tools.javac.launcher.Main.main(Main.java:135)',
]

// parse turns a fixture (array of raw payload lines) into log entries via the
// production parser, single-pod (no pod tags).
const parse = (lines) => lines.map((l) => parseLogLine(l, null))

// mk builds a single timestamped entry for the constructed (non-fixture) cases.
let seq = 0
const mk = (text, pod = '') => {
  const ts = `2026-06-20T11:00:${String(seq++ % 60).padStart(2, '0')}.000000000Z`
  return { ...parseLogLine(`${ts} ${text}`, null), pod }
}

describe('groupEntries', () => {
  describe('Go panic', () => {
    const groups = groupEntries(parse(GO_PANIC), true)

    it('folds the panic, goroutine, flush-left frames and \\t file lines into one group', () => {
      expect(groups).toHaveLength(3)
      const trace = groups[1]
      expect(trace.head.text).toBe('panic: boom from c')
      expect(trace.lines).toHaveLength(11)
      expect(trace.isTrace).toBe(true)
      expect(trace.goTrace).toBe(true)
    })

    it('detects the panic as fatal and keeps stdout lines as their own entries', () => {
      expect(groups[1].level).toBe('fatal')
      expect(groups[0].lines).toHaveLength(1)
      expect(groups[0].head.text).toBe('starting')
      expect(groups[2].lines).toHaveLength(1)
      expect(groups[2].head.text).toBe('exit status 2')
    })
  })

  describe('Python traceback', () => {
    const groups = groupEntries(parse(PY_TRACE), true)

    it('folds the indented frames and the flush-left final exception into one group', () => {
      expect(groups).toHaveLength(2)
      const trace = groups[0]
      expect(trace.head.text).toBe('Traceback (most recent call last):')
      expect(trace.lines).toHaveLength(14)
      expect(trace.isTrace).toBe(true)
      expect(trace.lines.at(-1).text).toBe('ValueError: boom from c')
    })

    it('bumps an unleveled trace head to error and closes on the trailing stdout', () => {
      expect(groups[0].level).toBe('error')
      expect(groups[1].head.text).toBe('starting')
    })
  })

  describe('Node error', () => {
    const groups = groupEntries(parse(NODE_ERR), true)
    const trace = groups.find((g) => g.head && g.head.text === 'Error: boom from c')

    it('folds the 4-space at-frames under the bare Error head', () => {
      expect(trace).toBeTruthy()
      expect(trace.isTrace).toBe(true)
      expect(trace.level).toBe('error')
      expect(trace.lines.some((l) => l.text === '    at c ([eval]:1:56)')).toBe(true)
    })

    it('does not fold the Node.js footer into the trace', () => {
      const footer = groups.find((g) => g.head && g.head.text === 'Node.js v26.3.1')
      expect(footer).toBeTruthy()
      expect(footer.lines).toHaveLength(1)
    })
  })

  describe('Java exception', () => {
    const groups = groupEntries(parse(JAVA_TRACE), true)

    it('folds the tab-indented frames and the Caused by chain into one group', () => {
      expect(groups).toHaveLength(2)
      const trace = groups[1]
      expect(trace.head.text).toContain('IllegalStateException')
      expect(trace.isTrace).toBe(true)
      expect(trace.level).toBe('error')
      expect(trace.lines).toHaveLength(15)
      expect(trace.lines.some((l) => l.text === 'Caused by: java.lang.RuntimeException: root cause')).toBe(true)
    })
  })

  describe('SIGQUIT goroutine dump (no panic head)', () => {
    it('opens a Go trace on a bare goroutine line and folds its frames', () => {
      const groups = groupEntries([
        mk('goroutine 1 [running]:'),
        mk('main.f(...)'),
        mk('\t/m.go:9'),
      ], true)
      expect(groups).toHaveLength(1)
      expect(groups[0].isTrace).toBe(true)
      expect(groups[0].goTrace).toBe(true)
      expect(groups[0].lines).toHaveLength(3)
    })
  })

  describe('negatives', () => {
    it('does not fold a flush-left paren line with no open trace', () => {
      const groups = groupEntries([mk('server listening'), mk('connect(addr)')], true)
      expect(groups).toHaveLength(2)
    })

    it('does not fold a flush-left line under a non-Go (bare Error) head', () => {
      // GO_FRAME is Go-gated, so a `)`-line is not consulted under an `Error:` head.
      const groups = groupEntries([
        mk('Error: cannot reach upstream'),
        mk('falling back to cache (stale)'),
      ], true)
      expect(groups).toHaveLength(2)
      expect(groups[0].isTrace).toBe(true)
      expect(groups[0].lines).toHaveLength(1)
    })

    it('closes a trace on the first plain line after it', () => {
      const groups = groupEntries([
        mk('panic: x'),
        mk('goroutine 1 [running]:'),
        mk('main.f(...)'),
        mk('server resumed'),
      ], true)
      expect(groups).toHaveLength(2)
      expect(groups[0].lines).toHaveLength(3)
      expect(groups[1].head.text).toBe('server resumed')
    })

    it('never folds an unambiguous frame across pods (samePod guard)', () => {
      const groups = groupEntries([
        mk('panic: boom', 'pod-a'),
        mk('\tat App.c(App.java:2)', 'pod-b'),
      ], true)
      expect(groups).toHaveLength(2)
    })

    it('emits one group per entry when disabled', () => {
      const entries = parse(GO_PANIC)
      const groups = groupEntries(entries, false)
      expect(groups).toHaveLength(entries.length)
      expect(groups.every((g) => g.lines.length === 1)).toBe(true)
    })

    it('does not swallow an unrelated untimestamped line', () => {
      // A lone untimestamped non-frame following a plain head is its own entry.
      const groups = groupEntries([mk('ok'), { ...parseLogLine('retry later', null), pod: '' }], true)
      expect(groups).toHaveLength(2)
    })

    it('does not fold an indented at/File line with no open trace', () => {
      // Without a recognized head the frames stay separate, so a stray indented
      // line like "  at startup" can never collapse into the previous log.
      const groups = groupEntries([mk('cache ready'), mk('  at startup: warm')], true)
      expect(groups).toHaveLength(2)
    })
  })

  describe('extended head and marker coverage', () => {
    it('recognizes a bracketed Node error-code head and folds its frames', () => {
      const groups = groupEntries([
        mk('TypeError [ERR_INVALID_ARG_TYPE]: bad arg'),
        mk('    at parse (node:internal/x:1:2)'),
        mk('    at run (node:internal/y:3:4)'),
      ], true)
      expect(groups).toHaveLength(1)
      expect(groups[0].isTrace).toBe(true)
      expect(groups[0].level).toBe('error')
      expect(groups[0].lines).toHaveLength(3)
    })

    it('folds a suffix-less Python final (KeyboardInterrupt) into the trace', () => {
      const groups = groupEntries([
        mk('Traceback (most recent call last):'),
        mk('  File "<string>", line 1, in <module>'),
        mk('KeyboardInterrupt'),
      ], true)
      expect(groups).toHaveLength(1)
      expect(groups[0].lines).toHaveLength(3)
      expect(groups[0].lines.at(-1).text).toBe('KeyboardInterrupt')
    })

    it('folds a fully-qualified Java throwable final into the trace', () => {
      const groups = groupEntries([
        mk('Exception in thread "main" java.lang.RuntimeException: wrap'),
        mk('\tat App.run(App.java:9)'),
        mk('java.lang.OutOfMemoryError: heap'),
      ], true)
      expect(groups).toHaveLength(1)
      expect(groups[0].lines).toHaveLength(3)
    })
  })
})

describe('groupEntries — unstructured bursts', () => {
  // be builds a timestamped entry at a ms offset from a base, with an explicit
  // `structured` flag (the viewer sets this from the format layer before grouping).
  const T0 = Date.parse('2026-06-20T11:00:00.000Z')
  const be = (text, offsetMs, structured, pod = '') => {
    const iso = new Date(T0 + offsetMs).toISOString()
    return { ...parseLogLine(`${iso} ${text}`, null), pod, podId: pod, structured }
  }

  it('groups a co-timestamped curl -v dump into one unstructured run', () => {
    const dump = [
      be('* Host kubernetes.default.svc.cluster.local:443 was resolved.', 0, false),
      be('*   Trying 10.96.0.1:443...', 1, false),
      be('> GET /healthz HTTP/2', 2, false),
      be('< HTTP/2 200', 3, false),
      be('* shutting down connection #0', 4, false),
    ]
    const groups = groupEntries(dump, true)
    expect(groups).toHaveLength(1)
    expect(groups[0].unstructuredRun).toBe(true)
    expect(groups[0].isTrace).toBe(false)
    expect(groups[0].lines).toHaveLength(5)
    expect(groups[0].head.text).toBe('* Host kubernetes.default.svc.cluster.local:443 was resolved.')
  })

  it('does not group structured lines milliseconds apart', () => {
    const lines = [
      be('2026-06-20 10:03:22.644 [main] DEBUG com.x - a', 0, true),
      be('2026-06-20 10:03:22.646 [main] DEBUG com.x - b', 2, true),
      be('2026-06-20 10:03:22.648 [main] DEBUG com.x - c', 4, true),
    ]
    const groups = groupEntries(lines, true)
    expect(groups).toHaveLength(3)
    expect(groups.every(g => g.unstructuredRun === false)).toBe(true)
  })

  it('does not group unstructured lines spaced beyond the burst window', () => {
    const lines = [be('* line one', 0, false), be('* line two', 500, false)]
    expect(groupEntries(lines, true)).toHaveLength(2)
  })

  it('caps a run at the total span even when every gap is within the window', () => {
    // A continuous plain stream, each line 200 ms after the last (< 250 ms window)
    // but running past the 1000 ms span: the run closes at the span boundary and a
    // fresh group starts, so a steady stream never coalesces into one giant group.
    const lines = [0, 200, 400, 600, 800, 1000, 1200].map(ms => be('* tick', ms, false))
    const groups = groupEntries(lines, true)
    expect(groups).toHaveLength(2)
    expect(groups[0].lines).toHaveLength(5)   // offsets 0..800 (< 1000 ms span)
    expect(groups[1].lines).toHaveLength(2)   // offsets 1000, 1200 (new run)
  })

  it('opens a trace (not a run) for a structure-less trace head', () => {
    const lines = [
      be('Error: boom from c', 0, false),
      be('    at c (eval:1:2)', 1, false),
      be('    at b (eval:3:4)', 2, false),
    ]
    const groups = groupEntries(lines, true)
    expect(groups).toHaveLength(1)
    expect(groups[0].isTrace).toBe(true)
    expect(groups[0].unstructuredRun).toBe(false)
    expect(groups[0].lines).toHaveLength(3)
  })

  it('closes the run on a lone structured line mid-burst (curl JSON body split)', () => {
    const lines = [
      be('* request begin', 0, false),
      be('> GET /api/info', 1, false),
      be('{"hostname":"frontend","version":"6.14"}', 2, true),  // valid JSON → structured
      be('* request end', 3, false),
    ]
    const groups = groupEntries(lines, true)
    expect(groups).toHaveLength(3)
    expect(groups[0].lines).toHaveLength(2)   // pre-JSON run
    expect(groups[1].lines).toHaveLength(1)   // the structured JSON line
    expect(groups[1].unstructuredRun).toBe(false)
    expect(groups[2].lines).toHaveLength(1)   // post-JSON run
  })

  it('never groups a burst across pods', () => {
    const lines = [be('* a one', 0, false, 'pod-a'), be('* b one', 1, false, 'pod-b')]
    expect(groupEntries(lines, true)).toHaveLength(2)
  })

  it('does not group when disabled', () => {
    const lines = [be('* one', 0, false), be('* two', 1, false), be('* three', 2, false)]
    const groups = groupEntries(lines, false)
    expect(groups).toHaveLength(3)
    expect(groups.every(g => g.unstructuredRun === false)).toBe(true)
  })
})

describe('groupEntries — container scoping', () => {
  // ce builds a timestamped entry tagged with a container (and optional pod), via
  // the production parser, mirroring what the viewer builds in the all-containers
  // view where each line carries its container of origin.
  let cseq = 0
  const ce = (text, container, pod = '') => {
    const ts = `2026-06-20T12:00:${String(cseq++ % 60).padStart(2, '0')}.000000000Z`
    return { ...parseLogLine(`${ts} ${text}`, null), container, pod, podId: pod }
  }

  it('folds a trace within one container', () => {
    const groups = groupEntries([
      ce('panic: boom', 'app'),
      ce('goroutine 1 [running]:', 'app'),
      ce('main.main()', 'app'),
    ], true)
    expect(groups).toHaveLength(1)
    expect(groups[0].isTrace).toBe(true)
    expect(groups[0].lines).toHaveLength(3)
    expect(groups[0].container).toBe('app')
  })

  it('does not fold a continuation from a different container into an open trace', () => {
    // app opens a Go panic; envoy emits a co-timestamped frame-shaped line — the
    // container guard keeps it out of app's trace so two interleaved containers
    // never merge.
    const groups = groupEntries([
      ce('panic: boom', 'app'),
      ce('\t/main.go:2', 'envoy'),
    ], true)
    expect(groups).toHaveLength(2)
    expect(groups[0].lines).toHaveLength(1)
    expect(groups[1].container).toBe('envoy')
  })

  it('never merges containers across two interleaved traces', () => {
    // app and side crash together; their frames interleave on the wire. No resulting
    // group may mix containers.
    const groups = groupEntries([
      ce('panic: app boom', 'app'),
      ce('Traceback (most recent call last):', 'side'),
      ce('  File "x.py", line 1', 'side'),
      ce('\tmain.main()', 'app'),
    ], true)
    for (const g of groups) {
      const containers = new Set(g.lines.map(l => l.container).filter(Boolean))
      expect(containers.size).toBeLessThanOrEqual(1)
    }
  })

  it('an orphan continuation (no container) inherits its predecessor and folds in', () => {
    const groups = groupEntries([
      ce('panic: boom', 'app'),
      { ...parseLogLine('  at frame', null), container: '', pod: '' },
    ], true)
    expect(groups).toHaveLength(1)
    expect(groups[0].lines).toHaveLength(2)
  })

  it('does not group an unstructured burst across containers', () => {
    // Two structure-less lines 1 ms apart (within the burst window) but from
    // different containers must not join one run.
    const T0 = Date.parse('2026-06-20T12:30:00.000Z')
    const bc = (text, offsetMs, container) => ({
      ...parseLogLine(`${new Date(T0 + offsetMs).toISOString()} ${text}`, null),
      container, pod: '', podId: '', structured: false,
    })
    const groups = groupEntries([
      bc('* app request', 0, 'app'),
      bc('* envoy upstream', 1, 'envoy'),
    ], true)
    expect(groups).toHaveLength(2)
    expect(groups[0].container).toBe('app')
    expect(groups[1].container).toBe('envoy')
  })
})

describe('parseLogLine', () => {
  it('preserves a frame\'s leading tab after stripping the timestamp', () => {
    const entry = parseLogLine('2026-06-20T10:00:00.000000000Z \t/m.go:2', null)
    expect(entry.text).toBe('\t/m.go:2')
    expect(entry.ts).toBe('2026-06-20T10:00:00.000000000Z')
  })

  it('preserves leading spaces of an indented frame', () => {
    const entry = parseLogLine('2026-06-20T10:00:00.000000000Z     at c ([eval]:1:56)', null)
    expect(entry.text).toBe('    at c ([eval]:1:56)')
  })

  it('parses a plain head line and leaves a non-timestamped line untimestamped', () => {
    expect(parseLogLine('2026-06-20T10:00:00.000000000Z hello', null).text).toBe('hello')
    const cont = parseLogLine('  some continuation', null)
    expect(cont.ts).toBe('')
    expect(cont.text).toBe('  some continuation')
  })

  it('peels a container tag when a real timestamp follows', () => {
    const cset = new Set(['app', 'envoy'])
    const entry = parseLogLine('app 2026-06-20T10:00:00.000000000Z hello', null, cset)
    expect(entry.container).toBe('app')
    expect(entry.ts).toBe('2026-06-20T10:00:00.000000000Z')
    expect(entry.text).toBe('hello')
  })

  it('peels pod then container in order in the combined view', () => {
    const pset = new Set(['frontend-abc-x1'])
    const cset = new Set(['app'])
    const entry = parseLogLine('frontend-abc-x1 app 2026-06-20T10:00:00.000000000Z hello', pset, cset)
    expect(entry.pod).toBe('frontend-abc-x1')
    expect(entry.podId).toBe('x1')
    expect(entry.container).toBe('app')
    expect(entry.text).toBe('hello')
  })

  it('does not treat a name-shaped token as a container tag without a following timestamp', () => {
    const cset = new Set(['app'])
    // A message that merely begins with a known container name and a date-shaped
    // (but invalid) token keeps its full text and no container.
    const entry = parseLogLine('app 2026-13-45Tnope started', null, cset)
    expect(entry.container).toBe('')
    expect(entry.ts).toBe('')
    expect(entry.text).toBe('app 2026-13-45Tnope started')
  })

  it('leaves a container-shaped continuation line untagged', () => {
    const cset = new Set(['app'])
    const entry = parseLogLine('  at app (x:1)', null, cset)
    expect(entry.container).toBe('')
    expect(entry.text).toBe('  at app (x:1)')
  })
})
