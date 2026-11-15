// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/preact'
import { FindingsList, checkCounts } from './FindingsList'

const shortName = (n) => n.replace(/\.lab$/, '')

const data = {
  controlPlaneVersion: 'v1.33.4',
  unschedulableReasons: [{ reason: 'Insufficient memory', pods: 3 }],
  unboundedNamespaces: { withoutRequest: [], withoutLimit: [{ namespace: 'monitoring', pods: 2 }] },
  findings: [
    { code: 'unschedulable-pods', severity: 'warn', value: 3 },
    { code: 'unreachable', severity: 'critical', nodes: 1, affected: [{ node: 'n1.lab', severity: 'critical' }] },
    {
      code: 'memory-high', severity: 'critical', nodes: 7, value: 95,
      affected: Array.from({ length: 7 }, (_, i) => ({ node: `m${i}.lab`, severity: i < 2 ? 'critical' : 'warn', value: 80 + i }))
    },
    { code: 'no-memory-limit', severity: 'info', value: 2 }
  ],
  checks: [
    { name: 'Node readiness', status: 'raised', codes: ['unreachable'] },
    { name: 'Memory usage', status: 'raised', codes: ['memory-high'] },
    { name: 'CPU usage', status: 'skipped', codes: ['cpu-high'], fact: 'no metrics-server' },
    { name: 'OOM kills', status: 'passed', codes: ['oom-kills'], fact: 'none in the last hour' },
    { name: 'Heartbeat', status: 'skipped', codes: ['heartbeat-lag'], fact: 'no node leases' }
  ]
}

describe('FindingsList', () => {
  it('lists raised lines worst first, then skipped, then passed checks', () => {
    render(<FindingsList data={data} shortName={shortName} />)
    const rows = screen.getAllByTestId('health-check-row')
    const titles = rows.map(r => r.querySelector('.text-sm.font-semibold').textContent)
    expect(titles).toEqual([
      '1 node unreachable',
      '7 nodes with high memory',
      '3 pods unschedulable',
      '2 pods without memory limit',
      'CPU usage',
      'Heartbeat',
      'OOM kills'
    ])
    expect(rows[0]).toHaveTextContent('Critical')
    expect(rows[2]).toHaveTextContent('Warning')
    expect(rows[3]).toHaveTextContent('Advisory')
    expect(rows[4]).toHaveTextContent('Skipped')
    expect(rows[4]).toHaveTextContent('no metrics-server')
    expect(rows[6]).toHaveTextContent('Passed')
    expect(rows[6]).toHaveTextContent('none in the last hour')
  })

  it('details node lines with short names capped at five', () => {
    render(<FindingsList data={data} shortName={shortName} />)
    expect(screen.getAllByTitle('m1, m0, m6, m5, m4 +2 more').length).toBeGreaterThan(0)
  })

  it('details cluster lines with reasons and namespaces', () => {
    render(<FindingsList data={data} shortName={shortName} />)
    expect(screen.getAllByTitle('Insufficient memory (3)').length).toBeGreaterThan(0)
    expect(screen.getAllByTitle('monitoring (2)').length).toBeGreaterThan(0)
  })

  it('counts rows by kind in the subtitle', () => {
    render(<FindingsList data={data} shortName={shortName} />)
    expect(screen.getByText('2 critical · 1 warning · 1 advisory · 2 skipped · 1 passed')).toBeInTheDocument()
  })

  it('reads all passed on a healthy cluster', () => {
    const checks = Array.from({ length: 20 }, (_, i) => ({ name: `c${i}`, status: 'passed', fact: 'none' }))
    expect(checkCounts([], checks)).toBe('20 passed')
  })
})
