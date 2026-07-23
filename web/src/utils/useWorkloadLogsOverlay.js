// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useEffect } from 'preact/hooks'
import { setWorkloadLogsOverlayOpen } from './keyboardShortcuts'

/**
 * useWorkloadLogsOverlay - marks the workload logs viewer as open for shortcut guards.
 *
 * @param {boolean} active - Whether this component currently shows the logs viewer
 */
export function useWorkloadLogsOverlay(active) {
  useEffect(() => {
    if (!active) {
      return
    }
    setWorkloadLogsOverlayOpen(true)
    return () => setWorkloadLogsOverlayOpen(false)
  }, [active])
}
