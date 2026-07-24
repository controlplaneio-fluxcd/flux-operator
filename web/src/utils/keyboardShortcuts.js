// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { signal } from '@preact/signals'
import { quickSearchOpen } from '../components/search/QuickSearch'
import { userMenuOpen } from '../components/layout/UserMenu'

/** Whether the keyboard shortcuts help modal is open. */
export const keyboardShortcutsOpen = signal(false)

/** Whether the workload logs viewer modal is open. */
export const workloadLogsOpen = signal(false)

let workloadLogsOpenCount = 0

/**
 * Tracks an open workload logs overlay for global shortcut guards.
 * Supports multiple viewers registering independently via refcounting.
 *
 * @param {boolean} open
 */
export function setWorkloadLogsOverlayOpen(open) {
  if (open) {
    workloadLogsOpenCount += 1
  } else {
    workloadLogsOpenCount = Math.max(0, workloadLogsOpenCount - 1)
  }
  workloadLogsOpen.value = workloadLogsOpenCount > 0
}

/** Resets logs overlay refcount state (for tests). */
export function resetWorkloadLogsOverlayState() {
  workloadLogsOpenCount = 0
  workloadLogsOpen.value = false
}

/** Timeout for `g`-chord second key (GitHub-style navigation). */
export const G_CHORD_TIMEOUT_MS = 1000

/** Second-key → route mapping for `g`-chord navigation. */
export const NAV_ROUTES = {
  d: '/',
  f: '/favorites',
  r: '/resources',
  w: '/workloads',
  e: '/events',
}

let registeredPageRefresh = null
let registeredOpenWorkloadLogs = null

/**
 * Registers the active page's refresh handler (favorites and detail pages).
 *
 * @param {Function} handler
 */
export function registerPageRefresh(handler) {
  registeredPageRefresh = handler
}

/**
 * Clears the active page refresh handler when it unmounts.
 *
 * @param {Function} handler
 */
export function unregisterPageRefresh(handler) {
  if (registeredPageRefresh === handler) {
    registeredPageRefresh = null
  }
}

/**
 * Registers the workload detail page logs opener.
 *
 * @param {Function} handler
 */
export function registerOpenWorkloadLogs(handler) {
  registeredOpenWorkloadLogs = handler
}

/**
 * Clears the workload logs opener when it unmounts.
 *
 * @param {Function} handler
 */
export function unregisterOpenWorkloadLogs(handler) {
  if (registeredOpenWorkloadLogs === handler) {
    registeredOpenWorkloadLogs = null
  }
}

/** Returns the registered page refresh handler, if any. */
export function getRegisteredPageRefresh() {
  return registeredPageRefresh
}

/** Returns the registered workload logs opener, if any. */
export function getRegisteredOpenWorkloadLogs() {
  return registeredOpenWorkloadLogs
}

/**
 * Returns true when global shortcuts should not fire (e.g. typing in a field
 * or when another overlay owns the keyboard).
 *
 * @param {KeyboardEvent} e
 * @param {object} [options]
 * @param {boolean} [options.allowOverlays=false] - When true, ignore only text inputs
 *   (and still block when the logs viewer is open). Search and user menu may be dismissed
 *   by g-chords via closeOverlays().
 * @returns {boolean}
 */
export function shouldIgnoreShortcut(e, { allowOverlays = false } = {}) {
  const tag = e.target.tagName
  if (tag === 'INPUT' || tag === 'TEXTAREA' || e.target.isContentEditable) {
    return true
  }
  // Logs always own the keyboard; allowOverlays only bypasses search/user menu
  // so g-chords can dismiss those overlays without navigating away from logs.
  if (workloadLogsOpen.value) {
    return true
  }
  if (!allowOverlays && (quickSearchOpen.value || userMenuOpen.value)) {
    return true
  }
  return false
}

/** Closes search and user menu before navigating via a shortcut. */
export function closeOverlays() {
  quickSearchOpen.value = false
  userMenuOpen.value = false
}

/** Opens the keyboard shortcuts help modal and closes the user menu. */
export function openKeyboardShortcuts() {
  userMenuOpen.value = false
  keyboardShortcutsOpen.value = true
}
