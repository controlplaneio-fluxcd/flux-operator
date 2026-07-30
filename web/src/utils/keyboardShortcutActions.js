// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { fetchFluxReport } from '../app'
import { fetchEvents } from '../components/search/EventList'
import { fetchResourcesStatus } from '../components/search/ResourceList'
import { fetchWorkloadsStatus } from '../components/search/WorkloadList'
import { fetchFavoritesData } from '../components/favorites/FavoritesPage'
import { toggleFavorite } from './favorites'
import { copyCurrentUrl, parseDetailRoute } from './routing'
import {
  getRegisteredOpenWorkloadLogs,
  getRegisteredPageRefresh,
} from './keyboardShortcuts'

const REFRESH_BY_PATH = {
  '/': fetchFluxReport,
  '/favorites': fetchFavoritesData,
  '/resources': fetchResourcesStatus,
  '/workloads': fetchWorkloadsStatus,
  '/events': fetchEvents,
}

/** Section tab views cycled by `[` and `]`. */
export const SECTION_TAB_PATHS = ['/favorites', '/resources', '/workloads', '/events']

/**
 * Refreshes data for the view at the given router path.
 *
 * @param {string} path
 */
export async function refreshCurrentView(path) {
  const refresh = REFRESH_BY_PATH[path]
  if (refresh) {
    return refresh()
  }

  if (parseDetailRoute(path)) {
    const pageRefresh = getRegisteredPageRefresh()
    if (pageRefresh) {
      return pageRefresh()
    }
  }
}

/**
 * Toggles favorite state when `s` is pressed on a detail page.
 *
 * @param {string} path
 * @returns {boolean} True when the shortcut was handled
 */
export function toggleFavoriteFromShortcut(path) {
  const detail = parseDetailRoute(path)
  if (!detail) {
    return false
  }
  toggleFavorite(detail.kind, detail.namespace, detail.name)
  return true
}

/**
 * Copies the current page URL when `c` is pressed on a detail page.
 *
 * @param {string} path
 * @returns {Promise<boolean>} True when the shortcut was handled
 */
export async function copyLinkFromShortcut(path) {
  if (!parseDetailRoute(path)) {
    return false
  }
  await copyCurrentUrl()
  return true
}

/**
 * Opens workload logs when `l` is pressed on a workload detail page.
 *
 * @param {string} path
 * @returns {boolean} True when the shortcut was handled
 */
export function openLogsFromShortcut(path) {
  const detail = parseDetailRoute(path)
  if (!detail || detail.type !== 'workload') {
    return false
  }
  const openLogs = getRegisteredOpenWorkloadLogs()
  if (!openLogs) {
    return false
  }
  openLogs()
  return true
}

/**
 * Cycles section tabs on list views.
 *
 * @param {string} path - Current router path
 * @param {number} direction - -1 for previous, 1 for next
 * @param {Function} route - preact-iso route function
 * @returns {boolean} True when navigation occurred
 */
export function cycleSectionTab(path, direction, route) {
  const idx = SECTION_TAB_PATHS.indexOf(path)
  if (idx === -1) {
    return false
  }
  const next = (idx + direction + SECTION_TAB_PATHS.length) % SECTION_TAB_PATHS.length
  route(SECTION_TAB_PATHS[next])
  return true
}
