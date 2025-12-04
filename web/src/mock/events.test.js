// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect } from 'vitest'
import { getMockEvents } from './events'

describe('getMockEvents', () => {
  it('should return all events when no filters are provided', () => {
    const result = getMockEvents('/api/v1/events')
    expect(result.events).toBeDefined()
    expect(result.events.length).toBeGreaterThan(0)
  })

  it('should filter events by type (Normal)', () => {
    const result = getMockEvents('/api/v1/events?type=Normal')
    expect(result.events.every(e => e.type === 'Normal')).toBe(true)
    expect(result.events.length).toBeGreaterThan(0)
  })

  it('should filter events by type (Warning)', () => {
    const result = getMockEvents('/api/v1/events?type=Warning')
    expect(result.events.every(e => e.type === 'Warning')).toBe(true)
    expect(result.events.length).toBeGreaterThan(0)
  })

  it('should filter events by kind', () => {
    const result = getMockEvents('/api/v1/events?kind=Kustomization')
    expect(result.events.every(e => e.involvedObject.startsWith('Kustomization/'))).toBe(true)
    expect(result.events.length).toBeGreaterThan(0)
  })

  it('should filter events by namespace', () => {
    const result = getMockEvents('/api/v1/events?namespace=flux-system')
    expect(result.events.every(e => e.namespace === 'flux-system')).toBe(true)
    expect(result.events.length).toBeGreaterThan(0)
  })

  it('should filter events by kind and type', () => {
    const result = getMockEvents('/api/v1/events?kind=Kustomization&type=Warning')
    expect(result.events.every(e =>
      e.involvedObject.startsWith('Kustomization/') && e.type === 'Warning'
    )).toBe(true)
  })

  it('should filter events by name with wildcard', () => {
    const result = getMockEvents('/api/v1/events?name=flux*')
    expect(result.events.every(e => {
      const name = e.involvedObject.split('/')[1]
      return name.toLowerCase().startsWith('flux')
    })).toBe(true)
  })

  it('should filter events by exact name match', () => {
    const result = getMockEvents('/api/v1/events?name=flux-system')
    expect(result.events.every(e => {
      const name = e.involvedObject.split('/')[1]
      return name === 'flux-system'
    })).toBe(true)
  })

  it('should filter events by all filters combined', () => {
    const result = getMockEvents('/api/v1/events?kind=HelmRelease&type=Warning&namespace=default')
    expect(result.events.every(e =>
      e.involvedObject.startsWith('HelmRelease/') &&
      e.type === 'Warning' &&
      e.namespace === 'default'
    )).toBe(true)
  })

  it('should return empty array when no events match filters', () => {
    const result = getMockEvents('/api/v1/events?kind=NonExistentKind')
    expect(result.events).toEqual([])
  })

  it('should handle URL encoding in query parameters', () => {
    const result = getMockEvents('/api/v1/events?namespace=flux-system&kind=Kustomization')
    expect(result.events.length).toBeGreaterThan(0)
  })
})
