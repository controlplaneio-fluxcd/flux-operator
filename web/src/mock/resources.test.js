// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect } from 'vitest'
import { getMockResources } from './resources'

describe('getMockResources', () => {
  it('should return all resources when no filters are provided', () => {
    const result = getMockResources('/api/v1/resources')
    expect(result.resources).toBeDefined()
    expect(result.resources.length).toBeGreaterThan(0)
  })

  it('should filter resources by status (Ready)', () => {
    const result = getMockResources('/api/v1/resources?status=Ready')
    expect(result.resources.every(r => r.status === 'Ready')).toBe(true)
    expect(result.resources.length).toBeGreaterThan(0)
  })

  it('should filter resources by status (Failed)', () => {
    const result = getMockResources('/api/v1/resources?status=Failed')
    expect(result.resources.every(r => r.status === 'Failed')).toBe(true)
    expect(result.resources.length).toBeGreaterThan(0)
  })

  it('should filter resources by status (Progressing)', () => {
    const result = getMockResources('/api/v1/resources?status=Progressing')
    expect(result.resources.every(r => r.status === 'Progressing')).toBe(true)
    expect(result.resources.length).toBeGreaterThan(0)
  })

  it('should filter resources by status (Suspended)', () => {
    const result = getMockResources('/api/v1/resources?status=Suspended')
    expect(result.resources.every(r => r.status === 'Suspended')).toBe(true)
    expect(result.resources.length).toBeGreaterThan(0)
  })

  it('should filter resources by kind', () => {
    const result = getMockResources('/api/v1/resources?kind=Kustomization')
    expect(result.resources.every(r => r.kind === 'Kustomization')).toBe(true)
    expect(result.resources.length).toBeGreaterThan(0)
  })

  it('should filter resources by namespace', () => {
    const result = getMockResources('/api/v1/resources?namespace=flux-system')
    expect(result.resources.every(r => r.namespace === 'flux-system')).toBe(true)
    expect(result.resources.length).toBeGreaterThan(0)
  })

  it('should filter resources by kind and status', () => {
    const result = getMockResources('/api/v1/resources?kind=Bucket&status=Ready')
    expect(result.resources.every(r => r.kind === 'Bucket' && r.status === 'Ready')).toBe(true)
    expect(result.resources.length).toBe(2) // We have 2 Ready Buckets in the mock
  })

  it('should filter Buckets by status (Suspended)', () => {
    const result = getMockResources('/api/v1/resources?kind=Bucket&status=Suspended')
    expect(result.resources.every(r => r.kind === 'Bucket' && r.status === 'Suspended')).toBe(true)
    expect(result.resources.length).toBe(1) // We have 1 Suspended Bucket in the mock
  })

  it('should filter Buckets by status (Failed)', () => {
    const result = getMockResources('/api/v1/resources?kind=Bucket&status=Failed')
    expect(result.resources.every(r => r.kind === 'Bucket' && r.status === 'Failed')).toBe(true)
    expect(result.resources.length).toBe(1) // We have 1 Failed Bucket in the mock
  })

  it('should filter resources by name with wildcard', () => {
    const result = getMockResources('/api/v1/resources?name=flux*')
    expect(result.resources.every(r => r.name.toLowerCase().startsWith('flux'))).toBe(true)
    expect(result.resources.length).toBeGreaterThan(0)
  })

  it('should filter resources by exact name match', () => {
    const result = getMockResources('/api/v1/resources?name=flux-system')
    expect(result.resources.every(r => r.name === 'flux-system')).toBe(true)
    expect(result.resources.length).toBeGreaterThan(0)
  })

  it('should filter resources by all filters combined', () => {
    const result = getMockResources('/api/v1/resources?kind=HelmRelease&status=Ready&namespace=kube-system')
    expect(result.resources.every(r =>
      r.kind === 'HelmRelease' &&
      r.status === 'Ready' &&
      r.namespace === 'kube-system'
    )).toBe(true)
  })

  it('should return empty array when no resources match filters', () => {
    const result = getMockResources('/api/v1/resources?kind=NonExistentKind')
    expect(result.resources).toEqual([])
  })

  it('should return empty array when status filter matches nothing', () => {
    const result = getMockResources('/api/v1/resources?kind=FluxInstance&status=Failed')
    expect(result.resources).toEqual([])
  })

  it('should handle multiple resources of same kind with different statuses', () => {
    const allBuckets = getMockResources('/api/v1/resources?kind=Bucket')
    const readyBuckets = getMockResources('/api/v1/resources?kind=Bucket&status=Ready')
    const failedBuckets = getMockResources('/api/v1/resources?kind=Bucket&status=Failed')
    const suspendedBuckets = getMockResources('/api/v1/resources?kind=Bucket&status=Suspended')
    const progressingBuckets = getMockResources('/api/v1/resources?kind=Bucket&status=Progressing')
    const unknownBuckets = getMockResources('/api/v1/resources?kind=Bucket&status=Unknown')

    expect(allBuckets.resources.length).toBe(6) // Total Buckets
    expect(readyBuckets.resources.length).toBe(2) // Ready Buckets (prod-configs, dev-configs)
    expect(failedBuckets.resources.length).toBe(1) // Failed Buckets (aws-configs)
    expect(suspendedBuckets.resources.length).toBe(1) // Suspended Buckets (preview-configs)
    expect(progressingBuckets.resources.length).toBe(1) // Progressing Buckets (staging-configs)
    expect(unknownBuckets.resources.length).toBe(1) // Unknown Buckets (default-configs)
  })
})
