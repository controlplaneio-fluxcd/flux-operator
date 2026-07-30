// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useEffect, useRef } from 'preact/hooks'
import {
  registerPageRefresh,
  unregisterPageRefresh,
  registerOpenWorkloadLogs,
  unregisterOpenWorkloadLogs,
} from './keyboardShortcuts'

/**
 * useRegisterPageShortcuts - registers page-local handlers for global shortcuts.
 *
 * @param {object} options
 * @param {Function} [options.onRefresh] - Called by Shift+R on this page
 * @param {Function} [options.onOpenLogs] - Called by `l` on workload detail pages
 */
export function useRegisterPageShortcuts({ onRefresh, onOpenLogs } = {}) {
  const refreshRef = useRef(onRefresh)
  const openLogsRef = useRef(onOpenLogs)
  refreshRef.current = onRefresh
  openLogsRef.current = onOpenLogs

  useEffect(() => {
    const refreshHandler = onRefresh ? () => refreshRef.current?.() : null
    const openLogsHandler = onOpenLogs ? () => openLogsRef.current?.() : null

    if (refreshHandler) {
      registerPageRefresh(refreshHandler)
    }
    if (openLogsHandler) {
      registerOpenWorkloadLogs(openLogsHandler)
    }

    return () => {
      if (refreshHandler) {
        unregisterPageRefresh(refreshHandler)
      }
      if (openLogsHandler) {
        unregisterOpenWorkloadLogs(openLogsHandler)
      }
    }
  }, [Boolean(onRefresh), Boolean(onOpenLogs)])
}
