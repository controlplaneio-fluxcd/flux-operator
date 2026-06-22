// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect } from 'vitest'
import { isPodReady, getPodAggregateStatus, getPodPhaseSummary, summarizePods } from './pods'

const readyPod = (name) => ({ name, status: 'Running', podStatus: { conditions: [{ type: 'Ready', status: 'True' }] } })
const notReadyPod = (name) => ({ name, status: 'Running', podStatus: { conditions: [{ type: 'Ready', status: 'False' }] } })

describe('isPodReady', () => {
  it('uses the Ready condition when podStatus is present', () => {
    expect(isPodReady(readyPod('a'))).toBe(true)
    expect(isPodReady(notReadyPod('a'))).toBe(false)
  })

  it('falls back to the phase when podStatus is absent', () => {
    expect(isPodReady({ name: 'a', status: 'Running' })).toBe(true)
    expect(isPodReady({ name: 'a', status: 'Succeeded' })).toBe(true)
    expect(isPodReady({ name: 'a', status: 'Pending' })).toBe(false)
  })
})

describe('getPodAggregateStatus', () => {
  it('returns Unknown for no pods', () => {
    expect(getPodAggregateStatus([], false)).toBe('Unknown')
    expect(getPodAggregateStatus(undefined, false)).toBe('Unknown')
  })

  it('returns Failed when any pod failed', () => {
    expect(getPodAggregateStatus([readyPod('a'), { name: 'b', status: 'Failed' }], false)).toBe('Failed')
  })

  it('returns Ready when all pods are ready', () => {
    expect(getPodAggregateStatus([readyPod('a'), readyPod('b')], false)).toBe('Ready')
  })

  it('returns Progressing when some pods are not ready', () => {
    expect(getPodAggregateStatus([readyPod('a'), notReadyPod('b')], false)).toBe('Progressing')
  })

  it('treats all-succeeded CronJob pods as Ready', () => {
    expect(getPodAggregateStatus([{ name: 'a', status: 'Succeeded' }], true)).toBe('Ready')
  })
})

describe('getPodPhaseSummary', () => {
  it('groups and counts phases', () => {
    expect(getPodPhaseSummary([readyPod('a'), readyPod('b')])).toBe('2 running')
    expect(getPodPhaseSummary([readyPod('a'), { name: 'b', status: 'Pending' }])).toBe('1 running, 1 pending')
  })

  it('returns an empty string for no pods', () => {
    expect(getPodPhaseSummary([])).toBe('')
  })
})

describe('summarizePods', () => {
  it('reports ready count for apps workloads', () => {
    const s = summarizePods([readyPod('a'), notReadyPod('b')], 'Deployment')
    expect(s.primary).toBe('1/2 ready')
    expect(s.detail).toBe('2 running')
    expect(s.status).toBe('Progressing')
    expect(s.total).toBe(2)
  })

  it('reports completed count for CronJobs', () => {
    const s = summarizePods([{ name: 'a', status: 'Succeeded' }, { name: 'b', status: 'Running' }], 'CronJob')
    expect(s.primary).toBe('1/2 completed')
    expect(s.detail).toBe('1 succeeded, 1 running')
  })

  it('reports scaled-to-zero for empty apps workloads', () => {
    const s = summarizePods([], 'Deployment')
    expect(s.primary).toBe('Scaled to zero')
    expect(s.detail).toBe('0 pods')
    expect(s.total).toBe(0)
  })

  it('reports no active pods for empty CronJobs', () => {
    const s = summarizePods(undefined, 'CronJob')
    expect(s.primary).toBe('No active pods')
    expect(s.detail).toBe('0 pods')
  })
})
