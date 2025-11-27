// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { renderHook, act, waitFor } from '@testing-library/preact'
import { signal } from '@preact/signals'
import { serializeFilters } from './routing'

describe('serializeFilters', () => {
  it('should serialize simple filter object to query string', () => {
    const filters = { kind: 'GitRepository', name: 'flux' }
    expect(serializeFilters(filters)).toBe('kind=GitRepository&name=flux')
  })

  it('should omit empty string values', () => {
    const filters = { kind: 'GitRepository', namespace: '', name: 'flux' }
    expect(serializeFilters(filters)).toBe('kind=GitRepository&name=flux')
  })

  it('should omit null values', () => {
    const filters = { kind: 'GitRepository', namespace: null, name: 'flux' }
    expect(serializeFilters(filters)).toBe('kind=GitRepository&name=flux')
  })

  it('should omit undefined values', () => {
    const filters = { kind: 'GitRepository', namespace: undefined, name: 'flux' }
    expect(serializeFilters(filters)).toBe('kind=GitRepository&name=flux')
  })

  it('should return empty string for empty filter object', () => {
    expect(serializeFilters({})).toBe('')
  })

  it('should return empty string when all values are empty', () => {
    const filters = { kind: '', namespace: '', name: '' }
    expect(serializeFilters(filters)).toBe('')
  })

  it('should handle single filter', () => {
    const filters = { kind: 'HelmRelease' }
    expect(serializeFilters(filters)).toBe('kind=HelmRelease')
  })

  it('should URL encode special characters', () => {
    const filters = { name: 'flux system', namespace: 'test&prod' }
    expect(serializeFilters(filters)).toBe('name=flux+system&namespace=test%26prod')
  })

  it('should preserve order of filters', () => {
    const filters = { a: '1', b: '2', c: '3' }
    expect(serializeFilters(filters)).toBe('a=1&b=2&c=3')
  })

  it('should handle boolean-like string values', () => {
    const filters = { enabled: 'true', disabled: 'false' }
    expect(serializeFilters(filters)).toBe('enabled=true&disabled=false')
  })

  it('should handle numeric string values', () => {
    const filters = { limit: '10', offset: '0' }
    expect(serializeFilters(filters)).toBe('limit=10&offset=0')
  })
})

// Mock module for hook tests
const mockLocationState = {
  query: {},
  path: '/search'
}

vi.mock('preact-iso', () => ({
  useLocation: () => mockLocationState
}))

describe('useRestoreFiltersFromUrl', () => {
  beforeEach(async () => {
    vi.resetModules()
    mockLocationState.query = {}
    mockLocationState.path = '/search'
  })

  it('should restore filter values from URL query params', async () => {
    mockLocationState.query = { kind: 'GitRepository', name: 'flux-system' }

    const { useRestoreFiltersFromUrl } = await import('./routing')

    const kindSignal = signal('')
    const nameSignal = signal('')

    renderHook(() => useRestoreFiltersFromUrl({
      kind: kindSignal,
      name: nameSignal
    }))

    await waitFor(() => {
      expect(kindSignal.value).toBe('GitRepository')
      expect(nameSignal.value).toBe('flux-system')
    })
  })

  it('should set empty string for missing query params', async () => {
    mockLocationState.query = { kind: 'HelmRelease' }

    const { useRestoreFiltersFromUrl } = await import('./routing')

    const kindSignal = signal('initial')
    const nameSignal = signal('initial')
    const namespaceSignal = signal('initial')

    renderHook(() => useRestoreFiltersFromUrl({
      kind: kindSignal,
      name: nameSignal,
      namespace: namespaceSignal
    }))

    await waitFor(() => {
      expect(kindSignal.value).toBe('HelmRelease')
      expect(nameSignal.value).toBe('')
      expect(namespaceSignal.value).toBe('')
    })
  })

  it('should only restore once on mount', async () => {
    mockLocationState.query = { kind: 'GitRepository' }

    const { useRestoreFiltersFromUrl } = await import('./routing')

    const kindSignal = signal('')

    const { rerender } = renderHook(() => useRestoreFiltersFromUrl({
      kind: kindSignal
    }))

    await waitFor(() => {
      expect(kindSignal.value).toBe('GitRepository')
    })

    // Change query and rerender
    mockLocationState.query = { kind: 'HelmRelease' }
    kindSignal.value = 'Kustomization'
    rerender()

    // Should still have the manually set value, not the new query param
    expect(kindSignal.value).toBe('Kustomization')
  })

  it('should handle empty query object', async () => {
    mockLocationState.query = {}

    const { useRestoreFiltersFromUrl } = await import('./routing')

    const kindSignal = signal('initial')
    const nameSignal = signal('initial')

    renderHook(() => useRestoreFiltersFromUrl({
      kind: kindSignal,
      name: nameSignal
    }))

    await waitFor(() => {
      expect(kindSignal.value).toBe('')
      expect(nameSignal.value).toBe('')
    })
  })

  it('should handle undefined query object', async () => {
    mockLocationState.query = undefined

    const { useRestoreFiltersFromUrl } = await import('./routing')

    const kindSignal = signal('initial')

    renderHook(() => useRestoreFiltersFromUrl({
      kind: kindSignal
    }))

    await waitFor(() => {
      expect(kindSignal.value).toBe('')
    })
  })

  it('should restore multiple filter types', async () => {
    mockLocationState.query = {
      kind: 'GitRepository',
      name: 'flux-system',
      namespace: 'flux-system',
      type: 'Normal'
    }

    const { useRestoreFiltersFromUrl } = await import('./routing')

    const kindSignal = signal('')
    const nameSignal = signal('')
    const namespaceSignal = signal('')
    const typeSignal = signal('')

    renderHook(() => useRestoreFiltersFromUrl({
      kind: kindSignal,
      name: nameSignal,
      namespace: namespaceSignal,
      type: typeSignal
    }))

    await waitFor(() => {
      expect(kindSignal.value).toBe('GitRepository')
      expect(nameSignal.value).toBe('flux-system')
      expect(namespaceSignal.value).toBe('flux-system')
      expect(typeSignal.value).toBe('Normal')
    })
  })
})

describe('useSyncFiltersToUrl', () => {
  beforeEach(async () => {
    vi.useFakeTimers()
    vi.resetModules()
    mockLocationState.query = {}
    mockLocationState.path = '/search'

    // Mock window.history.replaceState
    vi.spyOn(window.history, 'replaceState').mockImplementation(() => {})

    // Mock window.location
    Object.defineProperty(window, 'location', {
      value: {
        pathname: '/search',
        search: ''
      },
      writable: true,
      configurable: true
    })
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  it('should skip first render to allow restore hook to run', async () => {
    const { useSyncFiltersToUrl } = await import('./routing')

    const kindSignal = signal('GitRepository')

    renderHook(() => useSyncFiltersToUrl({ kind: kindSignal }))

    vi.advanceTimersByTime(500)

    // Should not have called replaceState on first render
    expect(window.history.replaceState).not.toHaveBeenCalled()
  })

  it('should update URL when filter signal changes', async () => {
    const { useSyncFiltersToUrl } = await import('./routing')

    const kindSignal = signal('')

    const { rerender } = renderHook(() => useSyncFiltersToUrl({ kind: kindSignal }))

    // First render - skipped
    vi.advanceTimersByTime(500)
    expect(window.history.replaceState).not.toHaveBeenCalled()

    // Change signal value
    act(() => {
      kindSignal.value = 'GitRepository'
    })

    rerender()

    // Advance past debounce
    vi.advanceTimersByTime(500)

    expect(window.history.replaceState).toHaveBeenCalledWith(
      null,
      '',
      '/search?kind=GitRepository'
    )
  })

  it('should debounce URL updates', async () => {
    const { useSyncFiltersToUrl } = await import('./routing')

    const kindSignal = signal('')

    const { rerender } = renderHook(() => useSyncFiltersToUrl({ kind: kindSignal }, 300))

    // Skip first render
    vi.advanceTimersByTime(500)

    // Rapid signal changes
    act(() => { kindSignal.value = 'G' })
    rerender()
    vi.advanceTimersByTime(100)

    act(() => { kindSignal.value = 'Gi' })
    rerender()
    vi.advanceTimersByTime(100)

    act(() => { kindSignal.value = 'Git' })
    rerender()
    vi.advanceTimersByTime(100)

    // Should not have updated yet (within debounce)
    expect(window.history.replaceState).not.toHaveBeenCalled()

    // Wait for debounce to complete
    vi.advanceTimersByTime(300)

    // Should only update once with final value
    expect(window.history.replaceState).toHaveBeenCalledTimes(1)
    expect(window.history.replaceState).toHaveBeenCalledWith(
      null,
      '',
      '/search?kind=Git'
    )
  })

  it('should not update URL if it has not changed', async () => {
    const { useSyncFiltersToUrl } = await import('./routing')

    const kindSignal = signal('')

    // Set window.location to match what we'll generate
    window.location.pathname = '/search'
    window.location.search = '?kind=GitRepository'

    const { rerender } = renderHook(() => useSyncFiltersToUrl({ kind: kindSignal }))

    // Skip first render
    vi.advanceTimersByTime(500)

    // Set signal to match current URL
    act(() => {
      kindSignal.value = 'GitRepository'
    })

    rerender()
    vi.advanceTimersByTime(500)

    // Should not call replaceState since URL matches
    expect(window.history.replaceState).not.toHaveBeenCalled()
  })

  it('should remove query string when all filters are empty', async () => {
    const { useSyncFiltersToUrl } = await import('./routing')

    const kindSignal = signal('GitRepository')
    const nameSignal = signal('flux')

    window.location.pathname = '/search'
    window.location.search = '?kind=GitRepository&name=flux'

    const { rerender } = renderHook(() => useSyncFiltersToUrl({
      kind: kindSignal,
      name: nameSignal
    }))

    // Skip first render
    vi.advanceTimersByTime(500)

    // Clear all filters
    act(() => {
      kindSignal.value = ''
      nameSignal.value = ''
    })

    rerender()
    vi.advanceTimersByTime(500)

    expect(window.history.replaceState).toHaveBeenCalledWith(
      null,
      '',
      '/search'
    )
  })

  it('should use custom debounce delay', async () => {
    const { useSyncFiltersToUrl } = await import('./routing')

    const kindSignal = signal('')

    const { rerender } = renderHook(() => useSyncFiltersToUrl({ kind: kindSignal }, 500))

    // Skip first render
    vi.advanceTimersByTime(600)

    act(() => {
      kindSignal.value = 'HelmRelease'
    })

    rerender()

    // At 300ms - should not have updated
    vi.advanceTimersByTime(300)
    expect(window.history.replaceState).not.toHaveBeenCalled()

    // At 500ms - should update
    vi.advanceTimersByTime(200)
    expect(window.history.replaceState).toHaveBeenCalled()
  })

  it('should handle multiple filter signals', async () => {
    const { useSyncFiltersToUrl } = await import('./routing')

    const kindSignal = signal('')
    const nameSignal = signal('')
    const namespaceSignal = signal('')

    const { rerender } = renderHook(() => useSyncFiltersToUrl({
      kind: kindSignal,
      name: nameSignal,
      namespace: namespaceSignal
    }))

    // Skip first render
    vi.advanceTimersByTime(500)

    act(() => {
      kindSignal.value = 'GitRepository'
      nameSignal.value = 'flux-system'
      namespaceSignal.value = 'flux-system'
    })

    rerender()
    vi.advanceTimersByTime(500)

    expect(window.history.replaceState).toHaveBeenCalledWith(
      null,
      '',
      '/search?kind=GitRepository&name=flux-system&namespace=flux-system'
    )
  })

  it('should cleanup timeout on unmount', async () => {
    const { useSyncFiltersToUrl } = await import('./routing')

    const kindSignal = signal('')
    const clearTimeoutSpy = vi.spyOn(window, 'clearTimeout')

    const { rerender, unmount } = renderHook(() => useSyncFiltersToUrl({ kind: kindSignal }))

    // Skip first render
    vi.advanceTimersByTime(500)

    act(() => {
      kindSignal.value = 'GitRepository'
    })

    rerender()

    // Unmount before debounce completes
    unmount()

    expect(clearTimeoutSpy).toHaveBeenCalled()
  })

  it('should use different paths correctly', async () => {
    mockLocationState.path = '/events'

    const { useSyncFiltersToUrl } = await import('./routing')

    const kindSignal = signal('')

    const { rerender } = renderHook(() => useSyncFiltersToUrl({ kind: kindSignal }))

    // Skip first render
    vi.advanceTimersByTime(500)

    act(() => {
      kindSignal.value = 'GitRepository'
    })

    rerender()
    vi.advanceTimersByTime(500)

    expect(window.history.replaceState).toHaveBeenCalledWith(
      null,
      '',
      '/events?kind=GitRepository'
    )
  })

  it('should handle partial filter updates', async () => {
    const { useSyncFiltersToUrl } = await import('./routing')

    const kindSignal = signal('GitRepository')
    const nameSignal = signal('')

    window.location.pathname = '/search'
    window.location.search = '?kind=GitRepository'

    const { rerender } = renderHook(() => useSyncFiltersToUrl({
      kind: kindSignal,
      name: nameSignal
    }))

    // Skip first render
    vi.advanceTimersByTime(500)

    // Add name filter while keeping kind
    act(() => {
      nameSignal.value = 'flux-system'
    })

    rerender()
    vi.advanceTimersByTime(500)

    expect(window.history.replaceState).toHaveBeenCalledWith(
      null,
      '',
      '/search?kind=GitRepository&name=flux-system'
    )
  })
})
