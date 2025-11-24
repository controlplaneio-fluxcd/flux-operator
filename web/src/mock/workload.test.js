// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect } from 'vitest'
import { getMockWorkload } from './workload'

describe('getMockWorkload', () => {
  it('should return workload data for Deployment/flux-system/source-controller', () => {
    const result = getMockWorkload('/api/v1/workload?kind=Deployment&name=source-controller&namespace=flux-system')
    expect(result).toBeDefined()
    expect(result.kind).toBe('Deployment')
    expect(result.name).toBe('source-controller')
    expect(result.namespace).toBe('flux-system')
    expect(result.status).toBe('Current')
    expect(result.pods).toBeDefined()
    expect(result.pods.length).toBeGreaterThan(0)
  })

  it('should return workload data for Deployment/flux-system/kustomize-controller', () => {
    const result = getMockWorkload('/api/v1/workload?kind=Deployment&name=kustomize-controller&namespace=flux-system')
    expect(result).toBeDefined()
    expect(result.kind).toBe('Deployment')
    expect(result.name).toBe('kustomize-controller')
    expect(result.namespace).toBe('flux-system')
  })

  it('should return workload data for StatefulSet/registry/zot-registry', () => {
    const result = getMockWorkload('/api/v1/workload?kind=StatefulSet&name=zot-registry&namespace=registry')
    expect(result).toBeDefined()
    expect(result.kind).toBe('StatefulSet')
    expect(result.name).toBe('zot-registry')
    expect(result.namespace).toBe('registry')
    expect(result.status).toBe('Current')
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

  it('should return containerImages array', () => {
    const result = getMockWorkload('/api/v1/workload?kind=Deployment&name=source-controller&namespace=flux-system')
    expect(result.containerImages).toBeDefined()
    expect(Array.isArray(result.containerImages)).toBe(true)
    expect(result.containerImages.length).toBeGreaterThan(0)
  })

  it('should return pods array with pod status', () => {
    const result = getMockWorkload('/api/v1/workload?kind=Deployment&name=source-controller&namespace=flux-system')
    expect(result.pods).toBeDefined()
    expect(Array.isArray(result.pods)).toBe(true)
    expect(result.pods[0]).toHaveProperty('name')
    expect(result.pods[0]).toHaveProperty('status')
    expect(result.pods[0]).toHaveProperty('statusMessage')
  })

  it('should return statusMessage for workload', () => {
    const result = getMockWorkload('/api/v1/workload?kind=Deployment&name=source-controller&namespace=flux-system')
    expect(result.statusMessage).toBeDefined()
    expect(typeof result.statusMessage).toBe('string')
    expect(result.statusMessage.length).toBeGreaterThan(0)
  })
})
