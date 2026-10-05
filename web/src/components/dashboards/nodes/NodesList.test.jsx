// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/preact'
import { NodesList, sortNodes, NODE_ROW_LIMIT } from './NodesList'

const GiB = 1024 ** 3

function node(name, overrides = {}) {
  return {
    name,
    pool: 'general',
    zone: 'zone-a',
    instanceType: 'standard-8',
    status: 'Ready',
    pressures: [],
    unschedulable: false,
    taints: [],
    conditions: [{ type: 'Ready', status: 'True' }],
    info: { kubeletVersion: 'v1.33.4', osImage: 'Ubuntu', kernelVersion: '6.8', containerRuntimeVersion: 'containerd://2.0.6', architecture: 'amd64' },
    allocatable: { cpu: 8, memory: 32 * GiB, pods: 110 },
    requests: { cpu: 4, memory: 16 * GiB },
    limits: { cpu: 8, memory: 24 * GiB },
    usage: { cpu: 2, memory: 10 * GiB },
    pods: 40,
    heartbeatSeconds: 8,
    findings: [],
    ...overrides
  }
}

const lines = [
  { code: 'unreachable', severity: 'critical' },
  { code: 'memory-high', severity: 'critical' },
  { code: 'oom-kills', severity: 'warn' },
  { code: 'cordoned', severity: 'info' }
]

const nodes = [
  node('ok-1.lab'),
  node('oom-1.lab', { findings: [{ code: 'oom-kills', severity: 'warn', value: 3 }] }),
  node('down-1.lab', { status: 'Unreachable', usage: null, pool: 'batch', findings: [{ code: 'unreachable', severity: 'critical' }] }),
  node('cordoned-1.lab', { status: 'Cordoned', unschedulable: true, findings: [{ code: 'cordoned', severity: 'info' }] }),
  node('hot-1.lab', { usage: { cpu: 2, memory: 30 * GiB }, findings: [{ code: 'memory-high', severity: 'critical', value: 93 }] })
]

const shortName = (n) => n.replace(/\.lab$/, '')
const rowNames = () => screen.queryAllByTestId('node-row').map(r => r.querySelector('button span').textContent)

describe('sortNodes', () => {
  it('sorts by pill color, Kubernetes status, worst finding and memory', () => {
    expect(sortNodes(nodes, lines).map(n => n.name)).toEqual(['down-1.lab', 'hot-1.lab', 'cordoned-1.lab', 'oom-1.lab', 'ok-1.lab'])
  })
})

describe('NodesList', () => {
  it('renders rows worst first with short names and status pills', () => {
    render(<NodesList data={{ nodes, findings: lines, metricsAvailable: true }} shortName={shortName} />)
    expect(rowNames()).toEqual(['down-1', 'hot-1', 'cordoned-1', 'oom-1', 'ok-1'])
    expect(screen.getByText('5 nodes')).toBeInTheDocument()
    const pills = screen.getAllByTestId('node-pill').map(p => p.textContent)
    expect(pills).toEqual(['Unreachable', 'Memory high', 'Cordoned', 'OOM kills', 'Ready'])
  })

  it('keeps the status pill from crowding out the name', () => {
    render(<NodesList data={{ nodes, findings: lines, metricsAvailable: true }} shortName={shortName} />)
    const pill = screen.getAllByTestId('node-pill')[0]
    expect(pill.className).toContain('shrink-0')
    expect(pill.className).toMatch(/max-w-/)
    expect(pill.firstChild.className).toContain('truncate')
  })

  it('filters on the pill color', () => {
    render(<NodesList data={{ nodes, findings: lines, metricsAvailable: true }} shortName={shortName} />)
    fireEvent.click(screen.getByRole('button', { name: 'Critical' }))
    expect(rowNames()).toEqual(['down-1', 'hot-1'])
    fireEvent.click(screen.getByRole('button', { name: 'Warning' }))
    expect(rowNames()).toEqual(['cordoned-1', 'oom-1'])
    fireEvent.click(screen.getByRole('button', { name: 'Ready' }))
    expect(rowNames()).toEqual(['ok-1'])
  })

  it('searches by name, pool, zone and instance type, and clears', () => {
    render(<NodesList data={{ nodes, findings: lines, metricsAvailable: true }} shortName={shortName} />)
    const search = screen.getByLabelText('Search nodes')
    fireEvent.input(search, { target: { value: 'batch' } })
    expect(rowNames()).toEqual(['down-1'])
    fireEvent.input(search, { target: { value: 'nothing' } })
    expect(screen.getByText('No nodes match the filters')).toBeInTheDocument()
    fireEvent.click(screen.getByLabelText('Clear filters'))
    expect(rowNames()).toHaveLength(5)
  })

  it('names each row by its visible status and usage text', () => {
    render(<NodesList data={{ nodes, findings: lines, metricsAvailable: true }} shortName={shortName} />)
    const button = screen.getAllByTestId('node-row')[1].querySelector('button')
    expect(button).not.toHaveAttribute('aria-label')
    expect(button).toHaveAttribute('aria-expanded', 'false')
    expect(button.textContent).toContain('hot-1')
    expect(button.textContent).toContain('Memory high')
    expect(button.textContent).toContain('Memory 93%')
  })

  it('shows no metrics, with requests in the tooltip, for nodes without a usage sample', () => {
    render(<NodesList data={{ nodes, findings: lines, metricsAvailable: true }} shortName={shortName} />)
    const down = screen.getAllByTestId('node-row')[0]
    const token = down.querySelector('[data-testid="node-no-metrics"]')
    expect(token.firstChild.textContent).toBe('no metrics')
    expect(token.getAttribute('title')).toBe('CPU 4.00 cores requested · 8.00 cores allocatable; Memory 16 GiB requested · 32 GiB allocatable')
    expect(token.querySelector('.sr-only')).toHaveTextContent('CPU 4.00 cores requested')
    expect(down.querySelector('button').textContent).not.toMatch(/CPU -|Memory -/)
    expect(document.body.textContent).not.toMatch(/null|NaN/)

    fireEvent.click(down.querySelector('button'))
    expect(down.querySelector('[role="tabpanel"], .max-h-\\[60vh\\]').textContent).toContain('no metrics · 50% requested')
  })

  it('shows request shares without metrics-server', () => {
    render(<NodesList data={{ nodes: [node('a')], findings: [], metricsAvailable: false }} shortName={shortName} />)
    const text = screen.getByTestId('node-row').querySelector('button').textContent
    expect(text).toContain('CPU requests 50%')
    expect(text).toContain('Memory requests 50%')
  })

  it('expands a row into the node detail with its issues', () => {
    const data = { nodes: [nodes[4]], findings: lines, metricsAvailable: true }
    render(<NodesList data={data} shortName={shortName} />)
    const button = screen.getByTestId('node-row').querySelector('button')
    fireEvent.click(button)
    expect(button).toHaveAttribute('aria-expanded', 'true')
    expect(screen.getByText('hot-1.lab')).toBeInTheDocument()
    expect(screen.getByTestId('node-issue')).toHaveTextContent('Critical Memory 93%')
    fireEvent.click(screen.getByRole('button', { name: 'System' }))
    expect(screen.getByText('containerd://2.0.6')).toBeInTheDocument()
    expect(screen.getByText('8s ago')).toBeInTheDocument()
  })

  it(`caps the list at ${NODE_ROW_LIMIT} nodes with Show all`, () => {
    const many = Array.from({ length: 120 }, (_, i) => node(`n-${String(i).padStart(3, '0')}`))
    render(<NodesList data={{ nodes: many, findings: [], metricsAvailable: true }} shortName={shortName} />)
    expect(screen.getAllByTestId('node-row')).toHaveLength(NODE_ROW_LIMIT)
    fireEvent.click(screen.getByRole('button', { name: 'Show all 120 nodes' }))
    expect(screen.getAllByTestId('node-row')).toHaveLength(120)
    expect(screen.queryByRole('button', { name: /Show all/ })).not.toBeInTheDocument()
  })
})
