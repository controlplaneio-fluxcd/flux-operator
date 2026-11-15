// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/preact'
import { BurstingWorkloadsPanel, workloadDetails } from './BurstingWorkloadsPanel'

const GiB = 1024 ** 3

const workloads = [
  { kind: 'Deployment', namespace: 'apps', name: 'api', pods: 3, nodes: 2, usage: 2 * GiB, requests: 1 * GiB, limits: 4 * GiB, aboveRequests: 1 * GiB, noLimit: false },
  { kind: 'StatefulSet', namespace: 'data', name: 'db', pods: 1, nodes: 1, usage: 8 * GiB, requests: 2 * GiB, limits: null, aboveRequests: 6 * GiB, noLimit: true },
  { kind: 'CronJob', namespace: 'batch', name: 'report', pods: 2, nodes: 2, usage: 0.5 * GiB, requests: 0.25 * GiB, limits: 1 * GiB, aboveRequests: 0.25 * GiB, noLimit: false }
]

const rows = () => screen.getAllByTestId('bursting-workload-row')
const links = () => rows().map(r => r.querySelector('a.hidden').getAttribute('href'))

describe('BurstingWorkloadsPanel', () => {
  it('renders one list row per workload, largest memory above requests first', () => {
    render(<BurstingWorkloadsPanel data={{ metricsAvailable: true, workloads }} />)
    expect(screen.getByText('Bursting Workloads')).toBeInTheDocument()
    expect(screen.getByText('3 workloads')).toBeInTheDocument()
    expect(rows()[0].closest('.card')).toHaveClass('overflow-hidden', 'p-0', 'sm:p-2')
    expect(links()).toEqual([
      '/workload/StatefulSet/data/db',
      '/workload/Deployment/apps/api',
      '/workload/CronJob/batch/report'
    ])
    expect(rows()[0]).toHaveTextContent('6 GiB')
    expect(rows()[0].querySelector('.usage-bar-fill-memory').style.width).toBe('100%')
    expect(rows()[1].querySelector('.usage-bar-fill-memory').style.width).toMatch(/^16\.6/)
  })

  it('shows the kind chip on desktop and on a mobile second line', () => {
    render(<BurstingWorkloadsPanel data={{ metricsAvailable: true, workloads }} />)
    const chips = [...rows()[1].querySelectorAll('[title="Deployment"]')]
    expect(chips.map(c => c.textContent)).toEqual(['deploy', 'deploy'])
    expect(chips[0]).toHaveClass('hidden', 'sm:inline-block')
  })

  it('puts used, requests, limits and pods in the tooltip and accessible name', () => {
    render(<BurstingWorkloadsPanel data={{ metricsAvailable: true, workloads }} />)
    const api = rows()[1]
    expect(api.firstChild).toHaveAttribute('title', 'Deployment apps/api · used 2 GiB · requests 1 GiB · limits 4 GiB · 3 pods on 2 nodes')
    expect(api.querySelector('.sr-only')).toHaveTextContent('above requests · used 2 GiB · requests 1 GiB · limits 4 GiB · 3 pods on 2 nodes')
  })

  it('flags workloads without a memory limit', () => {
    render(<BurstingWorkloadsPanel data={{ metricsAvailable: true, workloads }} />)
    expect(rows()[0]).toHaveTextContent('no limit')
    expect(rows()[1]).not.toHaveTextContent('no limit')
    expect(workloadDetails(workloads[1])).toBe('used 8 GiB · requests 2 GiB · no limit · 1 pod on 1 node')
  })

  it('states when no workload uses memory above requests', () => {
    render(<BurstingWorkloadsPanel data={{ metricsAvailable: true, workloads: [] }} />)
    expect(screen.getByText('0 workloads')).toBeInTheDocument()
    expect(screen.getByText('No workloads above memory requests')).toBeInTheDocument()
  })

  it('shows a failed fetch instead of rows', () => {
    render(<BurstingWorkloadsPanel data={null} error="boom" />)
    expect(screen.getByText('Failed to load workloads')).toBeInTheDocument()
    expect(screen.queryByTestId('bursting-workload-row')).not.toBeInTheDocument()
  })

  it('states when pod metrics are unavailable', () => {
    render(<BurstingWorkloadsPanel data={{ metricsAvailable: false, workloads: [] }} />)
    expect(screen.getByText('No pod metrics')).toBeInTheDocument()
    expect(screen.queryByText(/workloads$/)).not.toBeInTheDocument()
  })
})
