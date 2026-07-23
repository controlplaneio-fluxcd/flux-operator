// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { toggleFavorite } from './favorites'
import { copyCurrentUrl, parseDetailRoute } from './routing'
import {
  refreshCurrentView,
  toggleFavoriteFromShortcut,
  copyLinkFromShortcut,
  openLogsFromShortcut,
  cycleSectionTab,
  SECTION_TAB_PATHS,
} from './keyboardShortcutActions'
import {
  registerPageRefresh,
  unregisterPageRefresh,
  registerOpenWorkloadLogs,
  unregisterOpenWorkloadLogs,
} from './keyboardShortcuts'

vi.mock('../app', () => ({
  fetchFluxReport: vi.fn(),
}))

vi.mock('../components/search/EventList', () => ({
  fetchEvents: vi.fn(),
}))

vi.mock('../components/search/ResourceList', () => ({
  fetchResourcesStatus: vi.fn(),
}))

vi.mock('../components/search/WorkloadList', () => ({
  fetchWorkloadsStatus: vi.fn(),
}))

vi.mock('../components/favorites/FavoritesPage', () => ({
  fetchFavoritesData: vi.fn(),
}))

vi.mock('./favorites', () => ({
  toggleFavorite: vi.fn(),
}))

vi.mock('./routing', async () => {
  const actual = await vi.importActual('./routing')
  return {
    ...actual,
    copyCurrentUrl: vi.fn(),
  }
})

import { fetchFluxReport } from '../app'
import { fetchEvents } from '../components/search/EventList'
import { fetchResourcesStatus } from '../components/search/ResourceList'
import { fetchWorkloadsStatus } from '../components/search/WorkloadList'
import { fetchFavoritesData } from '../components/favorites/FavoritesPage'

describe('keyboardShortcutActions', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('refreshCurrentView', () => {
    it('refreshes the dashboard', async () => {
      await refreshCurrentView('/')
      expect(fetchFluxReport).toHaveBeenCalledTimes(1)
    })

    it('refreshes list and favorites views', async () => {
      await refreshCurrentView('/favorites')
      await refreshCurrentView('/resources')
      await refreshCurrentView('/workloads')
      await refreshCurrentView('/events')

      expect(fetchFavoritesData).toHaveBeenCalledTimes(1)
      expect(fetchResourcesStatus).toHaveBeenCalledTimes(1)
      expect(fetchWorkloadsStatus).toHaveBeenCalledTimes(1)
      expect(fetchEvents).toHaveBeenCalledTimes(1)
    })

    it('uses the registered page refresh handler on detail pages', async () => {
      const onRefresh = vi.fn()
      registerPageRefresh(onRefresh)

      await refreshCurrentView('/resource/Kustomization/default/flux-system')

      expect(onRefresh).toHaveBeenCalledTimes(1)
      unregisterPageRefresh(onRefresh)
    })
  })

  describe('toggleFavoriteFromShortcut', () => {
    it('toggles favorites on detail pages', () => {
      expect(toggleFavoriteFromShortcut('/workload/Deployment/default/my-app')).toBe(true)
      expect(toggleFavorite).toHaveBeenCalledWith('Deployment', 'default', 'my-app')
    })

    it('ignores non-detail pages', () => {
      expect(toggleFavoriteFromShortcut('/resources')).toBe(false)
      expect(toggleFavorite).not.toHaveBeenCalled()
    })
  })

  describe('copyLinkFromShortcut', () => {
    it('copies the current URL on detail pages', async () => {
      copyCurrentUrl.mockResolvedValue(true)

      await expect(copyLinkFromShortcut('/resource/GitRepository/flux-system/flux-system')).resolves.toBe(true)
      expect(copyCurrentUrl).toHaveBeenCalledTimes(1)
    })

    it('ignores non-detail pages', async () => {
      await expect(copyLinkFromShortcut('/events')).resolves.toBe(false)
      expect(copyCurrentUrl).not.toHaveBeenCalled()
    })
  })

  describe('openLogsFromShortcut', () => {
    it('opens logs on workload detail pages', () => {
      const openLogs = vi.fn()
      registerOpenWorkloadLogs(openLogs)

      expect(openLogsFromShortcut('/workload/Deployment/default/my-app')).toBe(true)
      expect(openLogs).toHaveBeenCalledTimes(1)

      unregisterOpenWorkloadLogs(openLogs)
    })

    it('ignores resource detail pages', () => {
      const openLogs = vi.fn()
      registerOpenWorkloadLogs(openLogs)

      expect(openLogsFromShortcut('/resource/Kustomization/default/flux-system')).toBe(false)
      expect(openLogs).not.toHaveBeenCalled()

      unregisterOpenWorkloadLogs(openLogs)
    })
  })

  describe('parseDetailRoute', () => {
    it('parses encoded detail routes', () => {
      expect(parseDetailRoute('/resource/Kustomization/default/flux%2Dsystem')).toEqual({
        type: 'resource',
        kind: 'Kustomization',
        namespace: 'default',
        name: 'flux-system',
      })
    })
  })

  describe('cycleSectionTab', () => {
    it('cycles section tabs forward and backward', () => {
      const route = vi.fn()

      expect(cycleSectionTab('/resources', 1, route)).toBe(true)
      expect(route).toHaveBeenCalledWith('/workloads')

      route.mockClear()
      expect(cycleSectionTab('/favorites', -1, route)).toBe(true)
      expect(route).toHaveBeenCalledWith('/events')
    })

    it('ignores non-section paths', () => {
      const route = vi.fn()
      expect(cycleSectionTab('/', 1, route)).toBe(false)
      expect(route).not.toHaveBeenCalled()
    })

    it('exports the section tab order used by the tab bar', () => {
      expect(SECTION_TAB_PATHS).toEqual(['/favorites', '/resources', '/workloads', '/events'])
    })
  })
})
