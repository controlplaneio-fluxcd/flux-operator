// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { renderHook, act } from '@testing-library/preact'
import { parseHash, buildHash, useHashTab, clearHash } from './hash'

describe('parseHash', () => {
  it('should parse valid hash with panel and tab', () => {
    expect(parseHash('#reconciler-events')).toEqual({ panel: 'reconciler', tab: 'events' })
    expect(parseHash('#inventory-graph')).toEqual({ panel: 'inventory', tab: 'graph' })
  })

  it('should handle hash without leading #', () => {
    expect(parseHash('reconciler-overview')).toEqual({ panel: 'reconciler', tab: 'overview' })
  })

  it('should handle tabs with hyphens', () => {
    expect(parseHash('#reconciler-some-tab-name')).toEqual({ panel: 'reconciler', tab: 'some-tab-name' })
  })

  it('should return null for empty hash', () => {
    expect(parseHash('')).toBeNull()
    expect(parseHash('#')).toBeNull()
  })

  it('should return null for null/undefined', () => {
    expect(parseHash(null)).toBeNull()
    expect(parseHash(undefined)).toBeNull()
  })

  it('should return null for hash without hyphen', () => {
    expect(parseHash('#reconciler')).toBeNull()
    expect(parseHash('overview')).toBeNull()
  })

  it('should return null for hash with empty panel', () => {
    expect(parseHash('#-events')).toBeNull()
  })

  it('should return null for hash with empty tab', () => {
    expect(parseHash('#reconciler-')).toBeNull()
  })
})

describe('buildHash', () => {
  it('should build hash from panel and tab', () => {
    expect(buildHash('reconciler', 'events')).toBe('#reconciler-events')
    expect(buildHash('inventory', 'graph')).toBe('#inventory-graph')
  })

  it('should handle tabs with special characters', () => {
    expect(buildHash('panel', 'tab-name')).toBe('#panel-tab-name')
  })
})

describe('useHashTab', () => {
  const validTabs = ['overview', 'history', 'events', 'spec', 'status']

  beforeEach(() => {
    // Reset location hash
    window.location.hash = ''

    // Mock history.replaceState
    vi.spyOn(window.history, 'replaceState').mockImplementation(() => {})
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('should return default tab when no hash is present', () => {
    const { result } = renderHook(() => useHashTab('reconciler', 'overview', validTabs))

    expect(result.current[0]).toBe('overview')
  })

  it('should return tab from hash when it matches panel', () => {
    window.location.hash = '#reconciler-events'

    const { result } = renderHook(() => useHashTab('reconciler', 'overview', validTabs))

    expect(result.current[0]).toBe('events')
  })

  it('should return default tab when hash panel does not match', () => {
    window.location.hash = '#inventory-graph'

    const { result } = renderHook(() => useHashTab('reconciler', 'overview', validTabs))

    expect(result.current[0]).toBe('overview')
  })

  it('should return default tab when hash tab is not in validTabs', () => {
    window.location.hash = '#reconciler-invalid'

    const { result } = renderHook(() => useHashTab('reconciler', 'overview', validTabs))

    expect(result.current[0]).toBe('overview')
  })

  it('should update hash when setActiveTab is called', () => {
    Object.defineProperty(window, 'location', {
      value: {
        hash: '',
        pathname: '/resource/GitRepository/flux-system/repo',
        search: ''
      },
      writable: true,
      configurable: true
    })

    const { result } = renderHook(() => useHashTab('reconciler', 'overview', validTabs))

    act(() => {
      result.current[1]('events')
    })

    expect(result.current[0]).toBe('events')
    expect(window.history.replaceState).toHaveBeenCalledWith(
      null,
      '',
      '/resource/GitRepository/flux-system/repo#reconciler-events'
    )
  })

  it('should preserve query params when updating hash', () => {
    Object.defineProperty(window, 'location', {
      value: {
        hash: '',
        pathname: '/resource/GitRepository/flux-system/repo',
        search: '?foo=bar'
      },
      writable: true,
      configurable: true
    })

    const { result } = renderHook(() => useHashTab('reconciler', 'overview', validTabs))

    act(() => {
      result.current[1]('spec')
    })

    expect(window.history.replaceState).toHaveBeenCalledWith(
      null,
      '',
      '/resource/GitRepository/flux-system/repo?foo=bar#reconciler-spec'
    )
  })

  it('should respond to hashchange events for matching panel', () => {
    const { result } = renderHook(() => useHashTab('reconciler', 'overview', validTabs))

    expect(result.current[0]).toBe('overview')

    // Simulate hashchange
    act(() => {
      window.location.hash = '#reconciler-history'
      window.dispatchEvent(new window.Event('hashchange'))
    })

    expect(result.current[0]).toBe('history')
  })

  it('should ignore hashchange events for non-matching panel', () => {
    const { result } = renderHook(() => useHashTab('reconciler', 'overview', validTabs))

    expect(result.current[0]).toBe('overview')

    // Simulate hashchange for different panel
    act(() => {
      window.location.hash = '#inventory-graph'
      window.dispatchEvent(new window.Event('hashchange'))
    })

    // Should keep current tab, not reset to default
    expect(result.current[0]).toBe('overview')
  })

  it('should ignore hashchange events for invalid tabs', () => {
    window.location.hash = '#reconciler-events'

    const { result } = renderHook(() => useHashTab('reconciler', 'overview', validTabs))

    expect(result.current[0]).toBe('events')

    // Simulate hashchange with invalid tab
    act(() => {
      window.location.hash = '#reconciler-invalid'
      window.dispatchEvent(new window.Event('hashchange'))
    })

    // Should keep current tab
    expect(result.current[0]).toBe('events')
  })

  it('should work with multiple panels independently', () => {
    window.location.hash = '#reconciler-events'

    const { result: reconcilerResult } = renderHook(() =>
      useHashTab('reconciler', 'overview', validTabs)
    )
    const { result: inventoryResult } = renderHook(() =>
      useHashTab('inventory', 'overview', ['overview', 'graph', 'inventory', 'workloads'])
    )

    // Reconciler should read from hash
    expect(reconcilerResult.current[0]).toBe('events')
    // Inventory should use default (hash doesn't match)
    expect(inventoryResult.current[0]).toBe('overview')
  })

  it('should reset to default when panel/validTabs change and hash does not match', () => {
    window.location.hash = '#reconciler-events'

    const { result, rerender } = renderHook(
      ({ panel, defaultTab, validTabs }) => useHashTab(panel, defaultTab, validTabs),
      { initialProps: { panel: 'reconciler', defaultTab: 'overview', validTabs } }
    )

    expect(result.current[0]).toBe('events')

    // Change panel
    rerender({ panel: 'inventory', defaultTab: 'overview', validTabs: ['overview', 'graph'] })

    // Hash doesn't match new panel, should reset to default
    expect(result.current[0]).toBe('overview')
  })
})

describe('clearHash', () => {
  beforeEach(() => {
    vi.spyOn(window.history, 'replaceState').mockImplementation(() => {})
    Object.defineProperty(window, 'location', {
      value: {
        hash: '#reconciler-events',
        pathname: '/resource/GitRepository/flux-system/repo',
        search: '?foo=bar'
      },
      writable: true,
      configurable: true
    })
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('should clear hash while preserving path and query', () => {
    clearHash()

    expect(window.history.replaceState).toHaveBeenCalledWith(
      null,
      '',
      '/resource/GitRepository/flux-system/repo?foo=bar'
    )
  })
})
