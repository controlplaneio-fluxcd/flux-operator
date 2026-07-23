// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render, cleanup, fireEvent } from '@testing-library/preact'
import { useKeyboardShortcuts } from './useKeyboardShortcuts'
import { keyboardShortcutsOpen, G_CHORD_TIMEOUT_MS } from './keyboardShortcuts'
import { quickSearchOpen } from '../components/search/QuickSearch'
import { userMenuOpen } from '../components/layout/UserMenu'

const mockRoute = vi.fn()
const mockLocation = { path: '/', route: mockRoute }

vi.mock('preact-iso', () => ({
  useLocation: () => mockLocation,
}))

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
    copyCurrentUrl: vi.fn().mockResolvedValue(true),
  }
})

vi.mock('./hash', async () => {
  const actual = await vi.importActual('./hash')
  return {
    ...actual,
    cycleHashTab: vi.fn(() => true),
  }
})

import { fetchFluxReport } from '../app'
import { fetchEvents } from '../components/search/EventList'
import { toggleFavorite } from './favorites'
import { copyCurrentUrl } from './routing'
import { cycleHashTab } from './hash'
import {
  registerOpenWorkloadLogs,
  unregisterOpenWorkloadLogs,
  workloadLogsOpen,
} from './keyboardShortcuts'

function ShortcutHarness() {
  useKeyboardShortcuts()
  return <div data-testid="harness" />
}

describe('useKeyboardShortcuts', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    mockLocation.path = '/'
    keyboardShortcutsOpen.value = false
    quickSearchOpen.value = false
    userMenuOpen.value = false
    workloadLogsOpen.value = false
    vi.clearAllMocks()
    mockRoute.mockClear()
    render(<ShortcutHarness />)
  })

  afterEach(() => {
    cleanup()
    vi.useRealTimers()
    keyboardShortcutsOpen.value = false
    quickSearchOpen.value = false
    userMenuOpen.value = false
    workloadLogsOpen.value = false
  })

  it('opens the help modal on ?', () => {
    fireEvent.keyDown(document, { key: '?' })
    expect(keyboardShortcutsOpen.value).toBe(true)
  })

  it('opens the help modal on Shift+/', () => {
    fireEvent.keyDown(document, { key: '/', shiftKey: true })
    expect(keyboardShortcutsOpen.value).toBe(true)
  })

  it('toggles the help modal closed on ? when already open', () => {
    keyboardShortcutsOpen.value = true
    fireEvent.keyDown(document, { key: '?' })
    expect(keyboardShortcutsOpen.value).toBe(false)
  })

  it('does not open help when quick search is open', () => {
    quickSearchOpen.value = true
    fireEvent.keyDown(document, { key: '?' })
    expect(keyboardShortcutsOpen.value).toBe(false)
  })

  it('ignores shortcuts while typing in an input', () => {
    const input = document.createElement('input')
    document.body.appendChild(input)
    input.focus()

    fireEvent.keyDown(input, { key: '?' })
    expect(keyboardShortcutsOpen.value).toBe(false)

    fireEvent.keyDown(input, { key: 'g' })
    fireEvent.keyDown(input, { key: 'f' })
    expect(mockRoute).not.toHaveBeenCalled()

    document.body.removeChild(input)
  })

  it('navigates with g-chord shortcuts', () => {
    const chords = [
      ['d', '/'],
      ['f', '/favorites'],
      ['r', '/resources'],
      ['w', '/workloads'],
      ['e', '/events'],
    ]

    for (const [key, path] of chords) {
      mockRoute.mockClear()
      fireEvent.keyDown(document, { key: 'g' })
      fireEvent.keyDown(document, { key })
      expect(mockRoute).toHaveBeenCalledWith(path)
    }
  })

  it('closes overlays before g-chord navigation', () => {
    quickSearchOpen.value = true
    userMenuOpen.value = true

    fireEvent.keyDown(document, { key: 'g' })
    fireEvent.keyDown(document, { key: 'f' })

    expect(quickSearchOpen.value).toBe(false)
    expect(userMenuOpen.value).toBe(false)
    expect(mockRoute).toHaveBeenCalledWith('/favorites')
  })

  it('expires g-chord after timeout', () => {
    fireEvent.keyDown(document, { key: 'g' })
    vi.advanceTimersByTime(G_CHORD_TIMEOUT_MS + 1)
    fireEvent.keyDown(document, { key: 'f' })
    expect(mockRoute).not.toHaveBeenCalled()
  })

  it('cancels g-chord on an unknown second key', () => {
    fireEvent.keyDown(document, { key: 'g' })
    fireEvent.keyDown(document, { key: 'x' })
    fireEvent.keyDown(document, { key: 'f' })
    expect(mockRoute).not.toHaveBeenCalled()
  })

  it('refreshes the current view on Shift+R', () => {
    fireEvent.keyDown(document, { key: 'R', shiftKey: true })
    expect(fetchFluxReport).toHaveBeenCalledTimes(1)

    mockLocation.path = '/events'
    fireEvent.keyDown(document, { key: 'R', shiftKey: true })
    expect(fetchEvents).toHaveBeenCalledTimes(1)
  })

  it('toggles favorite on detail pages with s', () => {
    mockLocation.path = '/resource/Kustomization/default/flux-system'
    fireEvent.keyDown(document, { key: 's' })
    expect(toggleFavorite).toHaveBeenCalledWith('Kustomization', 'default', 'flux-system')
  })

  it('copies the page link on detail pages with c', () => {
    mockLocation.path = '/workload/Deployment/default/my-app'
    fireEvent.keyDown(document, { key: 'c' })
    expect(copyCurrentUrl).toHaveBeenCalledTimes(1)
  })

  it('opens workload logs on detail pages with l', () => {
    const openLogs = vi.fn()
    registerOpenWorkloadLogs(openLogs)
    mockLocation.path = '/workload/Deployment/default/my-app'

    fireEvent.keyDown(document, { key: 'l' })

    expect(openLogs).toHaveBeenCalledTimes(1)
    unregisterOpenWorkloadLogs(openLogs)
  })

  it('ignores context shortcuts while the logs viewer is open', () => {
    workloadLogsOpen.value = true
    mockLocation.path = '/workload/Deployment/default/my-app'

    fireEvent.keyDown(document, { key: 'R', shiftKey: true })
    fireEvent.keyDown(document, { key: 's' })

    expect(fetchFluxReport).not.toHaveBeenCalled()
    expect(toggleFavorite).not.toHaveBeenCalled()
  })

  it('suppresses g-chord navigation while the logs viewer is open', () => {
    workloadLogsOpen.value = true

    fireEvent.keyDown(document, { key: 'g' })
    fireEvent.keyDown(document, { key: 'f' })

    expect(mockRoute).not.toHaveBeenCalled()
  })

  it('cycles section tabs with [ and ]', () => {
    mockLocation.path = '/resources'

    fireEvent.keyDown(document, { key: ']' })
    expect(mockRoute).toHaveBeenCalledWith('/workloads')

    mockRoute.mockClear()
    fireEvent.keyDown(document, { key: '[' })
    expect(mockRoute).toHaveBeenCalledWith('/favorites')
  })

  it('cycles detail page tabs with [ and ]', () => {
    mockLocation.path = '/resource/Kustomization/default/flux-system'

    fireEvent.keyDown(document, { key: ']' })
    expect(cycleHashTab).toHaveBeenCalledWith(1)

    vi.mocked(cycleHashTab).mockClear()
    fireEvent.keyDown(document, { key: '[' })
    expect(cycleHashTab).toHaveBeenCalledWith(-1)
  })

  it('does not cycle tabs for Cmd+[ or Ctrl+[ (browser / reserved chords)', () => {
    mockLocation.path = '/resources'
    fireEvent.keyDown(document, { key: ']', metaKey: true })
    fireEvent.keyDown(document, { key: '[', ctrlKey: true })
    expect(mockRoute).not.toHaveBeenCalled()

    mockLocation.path = '/resource/Kustomization/default/flux-system'
    fireEvent.keyDown(document, { key: ']', metaKey: true })
    fireEvent.keyDown(document, { key: '[', ctrlKey: true })
    expect(cycleHashTab).not.toHaveBeenCalled()
  })

  it('cycles tabs when [ or ] are typed via Option or AltGr', () => {
    // German Mac: Option+5/6 → [ ]; Windows DE: AltGr (ctrl+alt) → [ ]
    mockLocation.path = '/resources'
    fireEvent.keyDown(document, { key: ']', altKey: true })
    expect(mockRoute).toHaveBeenCalledWith('/workloads')

    mockRoute.mockClear()
    fireEvent.keyDown(document, { key: '[', ctrlKey: true, altKey: true })
    expect(mockRoute).toHaveBeenCalledWith('/favorites')
  })
})
