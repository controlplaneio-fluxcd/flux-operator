// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect } from 'vitest'
import { getKindAlias, getKindChipAlias } from './constants'

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
