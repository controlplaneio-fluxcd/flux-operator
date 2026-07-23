// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, afterEach } from 'vitest'
import {
  workloadLogsOpen,
  setWorkloadLogsOverlayOpen,
  resetWorkloadLogsOverlayState,
} from './keyboardShortcuts'

describe('workload logs overlay guard', () => {
  afterEach(() => {
    resetWorkloadLogsOverlayState()
  })

  it('tracks multiple open viewers with a refcount', () => {
    setWorkloadLogsOverlayOpen(true)
    setWorkloadLogsOverlayOpen(true)
    expect(workloadLogsOpen.value).toBe(true)

    setWorkloadLogsOverlayOpen(false)
    expect(workloadLogsOpen.value).toBe(true)

    setWorkloadLogsOverlayOpen(false)
    expect(workloadLogsOpen.value).toBe(false)
  })
})
