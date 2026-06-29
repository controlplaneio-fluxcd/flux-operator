// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect } from 'vitest'
import { compileSearch } from './inventorySearch'

const items = [
  { name: 'podinfo', namespace: 'apps', kind: 'Deployment', apiVersion: 'apps/v1' },
  { name: 'podinfo', namespace: 'flux-system', kind: 'Kustomization', apiVersion: 'kustomize.toolkit.fluxcd.io/v1' },
  { name: 'redis-secret', namespace: 'apps', kind: 'Secret', apiVersion: 'v1' },
  { name: 'cluster-reader', namespace: '', kind: 'ClusterRole', apiVersion: 'rbac.authorization.k8s.io/v1' },
]

const filter = (q) => items.filter(compileSearch(q))

describe('compileSearch', () => {
  it('matches everything for an empty query', () => {
    expect(filter('')).toHaveLength(4)
    expect(filter('   ')).toHaveLength(4)
  })

  it('includes by name', () => {
    expect(filter('podinfo').map(i => i.kind).sort()).toEqual(['Deployment', 'Kustomization'])
  })

  it('includes by namespace', () => {
    expect(filter('flux-system')).toHaveLength(1)
  })

  it('includes by kind', () => {
    expect(filter('secret')).toHaveLength(1)
    expect(filter('secret')[0].name).toBe('redis-secret')
  })

  it('includes by apiVersion group', () => {
    expect(filter('fluxcd.io')).toHaveLength(1)
    expect(filter('rbac.authorization')[0].kind).toBe('ClusterRole')
  })

  it('is case-insensitive', () => {
    expect(filter('DEPLOYMENT')).toHaveLength(1)
    expect(filter('PodInfo')).toHaveLength(2)
  })

  it('requires every include term (AND)', () => {
    // "apps" matches two items by namespace/apiVersion; adding "deployment" narrows to one.
    expect(filter('apps deployment')).toHaveLength(1)
    expect(filter('apps deployment')[0].kind).toBe('Deployment')
  })

  it('excludes with a ! prefix', () => {
    // All "podinfo" items minus anything matching "kustomization".
    const r = filter('podinfo !kustomization')
    expect(r).toHaveLength(1)
    expect(r[0].kind).toBe('Deployment')
  })

  it('excludes across all items when only a ! term is given', () => {
    expect(filter('!secret').some(i => i.kind === 'Secret')).toBe(false)
    expect(filter('!secret')).toHaveLength(3)
  })

  it('supports * wildcards within a term', () => {
    // "podinfo" appears in both names; "clu*reader" matches the cluster role name.
    expect(filter('clu*reader')).toHaveLength(1)
    expect(filter('clu*reader')[0].name).toBe('cluster-reader')
    // Unanchored: a wildcard term still matches as contains-in-order.
    expect(filter('redis*secret')).toHaveLength(1)
  })

  it('treats a *-only term as match-all', () => {
    expect(filter('*')).toHaveLength(4)
    expect(filter('**')).toHaveLength(4)
  })

  it('excludes with a wildcard term', () => {
    // "!cluster*" drops the cluster role; the rest remain.
    const r = filter('!cluster*')
    expect(r).toHaveLength(3)
    expect(r.some(i => i.name === 'cluster-reader')).toBe(false)
  })

  it('treats a bare ! as no term', () => {
    expect(filter('!')).toHaveLength(4)
  })
})
