// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect } from 'vitest'
import { getKindAlias, getKindChipAlias, parseHelmChartRef } from './constants'

describe('getKindAlias', () => {
  it('returns the Flux CRD alias for known kinds', () => {
    expect(getKindAlias('FluxInstance')).toBe('instance')
    expect(getKindAlias('GitRepository')).toBe('gitrepo')
  })

  it('falls back to the lowercased kind when no alias is known', () => {
    expect(getKindAlias('Deployment')).toBe('deployment')
    expect(getKindAlias('Pod')).toBe('pod')
  })
})

describe('getKindChipAlias', () => {
  it('uses the Flux CRD alias for known kinds', () => {
    expect(getKindChipAlias('FluxInstance')).toBe('instance')
    expect(getKindChipAlias('ResourceSet')).toBe('rset')
    expect(getKindChipAlias('Kustomization')).toBe('ks')
    expect(getKindChipAlias('HelmRelease')).toBe('hr')
  })

  it('uses the kubectl-style short name for workload kinds', () => {
    expect(getKindChipAlias('Deployment')).toBe('deploy')
    expect(getKindChipAlias('StatefulSet')).toBe('sts')
    expect(getKindChipAlias('DaemonSet')).toBe('ds')
    expect(getKindChipAlias('CronJob')).toBe('cj')
  })

  it('falls back to the full kind when no alias is known', () => {
    expect(getKindChipAlias('Pod')).toBe('Pod')
    expect(getKindChipAlias('Unknown')).toBe('Unknown')
  })
})

describe('parseHelmChartRef', () => {
  it('parses namespace/name as a HelmChart', () => {
    expect(parseHelmChartRef('flux-system/flux-system-podinfo')).toEqual({
      kind: 'HelmChart',
      namespace: 'flux-system',
      name: 'flux-system-podinfo'
    })
  })

  it('parses kind/namespace/name', () => {
    expect(parseHelmChartRef('OCIRepository/apps/apps-podinfo')).toEqual({
      kind: 'OCIRepository',
      namespace: 'apps',
      name: 'apps-podinfo'
    })
  })

  it('parses an explicit HelmChart kind', () => {
    expect(parseHelmChartRef('HelmChart/apps/apps-podinfo')).toEqual({
      kind: 'HelmChart',
      namespace: 'apps',
      name: 'apps-podinfo'
    })
  })

  it('returns null for empty values', () => {
    expect(parseHelmChartRef('')).toBeNull()
    expect(parseHelmChartRef(undefined)).toBeNull()
    expect(parseHelmChartRef(null)).toBeNull()
  })

  it('returns null for malformed values', () => {
    expect(parseHelmChartRef('podinfo')).toBeNull()
    expect(parseHelmChartRef('a/b/c/d')).toBeNull()
    expect(parseHelmChartRef('/podinfo')).toBeNull()
    expect(parseHelmChartRef('apps/')).toBeNull()
    expect(parseHelmChartRef('OCIRepository//podinfo')).toBeNull()
  })
})
