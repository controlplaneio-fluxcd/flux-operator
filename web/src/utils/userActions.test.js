// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach, vi } from 'vitest'
import { signal } from '@preact/signals'
import {
  AUTH_NOT_CONFIGURED_TOOLTIP,
  getActionTooltip,
  isActionBlockedByAccess,
  isUserActionsEnabled
} from './userActions'

vi.mock('../app', () => ({
  reportData: signal(null)
}))

import { reportData } from '../app'

describe('isUserActionsEnabled', () => {
  beforeEach(() => {
    reportData.value = null
  })

  it('reads userActionsEnabled from workload context', () => {
    expect(isUserActionsEnabled({ userActionsEnabled: true })).toBe(true)
    expect(isUserActionsEnabled({ userActionsEnabled: false })).toBe(false)
  })

  it('reads userActionsEnabled from resource status', () => {
    expect(isUserActionsEnabled({ status: { userActionsEnabled: true } })).toBe(true)
  })

  it('falls back to report userInfo', () => {
    reportData.value = { spec: { userInfo: { userActionsEnabled: true } } }
    expect(isUserActionsEnabled({})).toBe(true)
  })
})

describe('getActionTooltip', () => {
  it('returns auth message when user actions are disabled', () => {
    expect(getActionTooltip({
      userActionsEnabled: false,
      hasPermission: false,
      actionLabel: 'reconcile'
    })).toBe(AUTH_NOT_CONFIGURED_TOOLTIP)
  })

  it('returns permission message when user lacks RBAC', () => {
    expect(getActionTooltip({
      userActionsEnabled: true,
      hasPermission: false,
      actionLabel: 'suspend reconciliation'
    })).toBe("You don't have permission to suspend reconciliation")
  })

  it('returns state reason when user has permission but action is state-blocked', () => {
    expect(getActionTooltip({
      userActionsEnabled: true,
      hasPermission: true,
      actionLabel: 'reconcile',
      stateReason: 'Reconciliation in progress',
      enabledTitle: 'Trigger a reconciliation'
    })).toBe('Reconciliation in progress')
  })

  it('returns enabled title when action is available', () => {
    expect(getActionTooltip({
      userActionsEnabled: true,
      hasPermission: true,
      actionLabel: 'reconcile',
      enabledTitle: 'Trigger a reconciliation'
    })).toBe('Trigger a reconciliation')
  })
})

describe('isActionBlockedByAccess', () => {
  it('blocks when auth is not configured or permission is missing', () => {
    expect(isActionBlockedByAccess(false, true)).toBe(true)
    expect(isActionBlockedByAccess(true, false)).toBe(true)
    expect(isActionBlockedByAccess(true, true)).toBe(false)
  })
})
