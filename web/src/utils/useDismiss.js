// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useEffect, useRef } from 'preact/hooks'

/**
 * useDismiss - closes a popup (dropdown, menu) on an outside mousedown or the
 * Escape key, but only while `active`. The latest `onDismiss` is read from a
 * ref so the listeners are attached once per open/close, not on every render.
 *
 * @param {object} ref - Ref to the popup container element
 * @param {Function} onDismiss - Called to close the popup
 * @param {boolean} active - Whether the popup is open (listeners attach only then)
 */
export function useDismiss(ref, onDismiss, active) {
  const cb = useRef(onDismiss)
  cb.current = onDismiss

  useEffect(() => {
    if (!active) return
    const onClick = (e) => {
      if (ref.current && !ref.current.contains(e.target)) cb.current()
    }
    const onKey = (e) => {
      if (e.key === 'Escape') {
        // Stop the Escape from bubbling to outer handlers (e.g. a modal that
        // also closes on Escape), so it only dismisses this popup.
        e.stopPropagation()
        cb.current()
      }
    }
    document.addEventListener('mousedown', onClick)
    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('mousedown', onClick)
      document.removeEventListener('keydown', onKey)
    }
  }, [ref, active])
}
