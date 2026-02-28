// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect } from 'vitest'
import { getMockWorkload } from './workload'

describe('getMockWorkload', () => {
  it('should return workload data for Deployment/flux-system/source-controller', () => {
    const result = getMockWorkload('/api/v1/workload?kind=Deployment&name=source-controller&namespace=flux-system')
    expect(result).toBeDefined()
    expect(result.kind).toBe('Deployment')
    expect(result.metadata.name).toBe('source-controller')
    expect(result.metadata.namespace).toBe('flux-system')
    expect(result.workloadInfo.status).toBe('Current')
    expect(result.workloadInfo.pods).toBeDefined()
    expect(result.workloadInfo.pods.length).toBeGreaterThan(0)
  })

  it('should return workload data for Deployment/flux-system/kustomize-controller', () => {
    const result = getMockWorkload('/api/v1/workload?kind=Deployment&name=kustomize-controller&namespace=flux-system')
    expect(result).toBeDefined()
    expect(result.kind).toBe('Deployment')
    expect(result.metadata.name).toBe('kustomize-controller')
    expect(result.metadata.namespace).toBe('flux-system')
  })

  it('should return workload data for StatefulSet/registry/zot-registry', () => {
    const result = getMockWorkload('/api/v1/workload?kind=StatefulSet&name=zot-registry&namespace=registry')
    expect(result).toBeDefined()
    expect(result.kind).toBe('StatefulSet')
    expect(result.metadata.name).toBe('zot-registry')
    expect(result.metadata.namespace).toBe('registry')
    expect(result.workloadInfo.status).toBe('Current')
  })

  it('should return empty object when kind parameter is missing', () => {
    const result = getMockWorkload('/api/v1/workload?name=source-controller&namespace=flux-system')
    expect(result).toEqual({})
  })

  it('should return empty object when name parameter is missing', () => {
    const result = getMockWorkload('/api/v1/workload?kind=Deployment&namespace=flux-system')
    expect(result).toEqual({})
  })

  it('should return empty object when namespace parameter is missing', () => {
    const result = getMockWorkload('/api/v1/workload?kind=Deployment&name=source-controller')
    expect(result).toEqual({})
  })

  it('should return empty object when workload does not exist', () => {
    const result = getMockWorkload('/api/v1/workload?kind=Deployment&name=non-existent&namespace=default')
    expect(result).toEqual({})
  })

  it('should return containerImages array in workloadInfo', () => {
    const result = getMockWorkload('/api/v1/workload?kind=Deployment&name=source-controller&namespace=flux-system')
    expect(result.workloadInfo.containerImages).toBeDefined()
    expect(Array.isArray(result.workloadInfo.containerImages)).toBe(true)
    expect(result.workloadInfo.containerImages.length).toBeGreaterThan(0)
  })

  it('should return pods array with pod status in workloadInfo', () => {
    const result = getMockWorkload('/api/v1/workload?kind=Deployment&name=source-controller&namespace=flux-system')
    expect(result.workloadInfo.pods).toBeDefined()
    expect(Array.isArray(result.workloadInfo.pods)).toBe(true)
    expect(result.workloadInfo.pods[0]).toHaveProperty('name')
    expect(result.workloadInfo.pods[0]).toHaveProperty('status')
    expect(result.workloadInfo.pods[0]).toHaveProperty('statusMessage')
  })

  it('should return statusMessage in workloadInfo', () => {
    const result = getMockWorkload('/api/v1/workload?kind=Deployment&name=source-controller&namespace=flux-system')
    expect(result.workloadInfo.statusMessage).toBeDefined()
    expect(typeof result.workloadInfo.statusMessage).toBe('string')
    expect(result.workloadInfo.statusMessage.length).toBeGreaterThan(0)
  })

  it('should include reconciler data in workloadInfo', () => {
    const result = getMockWorkload('/api/v1/workload?kind=Deployment&name=source-controller&namespace=flux-system')
    expect(result.workloadInfo.reconciler).toBeDefined()
    expect(result.workloadInfo.reconciler.kind).toBe('Kustomization')
    expect(result.workloadInfo.reconciler.metadata.name).toBe('flux-controllers')
    expect(result.workloadInfo.reconciler.status.sourceRef).toBeDefined()
    expect(result.workloadInfo.reconciler.status.reconcilerRef.status).toBe('Ready')
  })
})
