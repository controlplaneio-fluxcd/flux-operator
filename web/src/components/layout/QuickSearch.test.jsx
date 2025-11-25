// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/preact'

// Mock preact-iso
const mockRoute = vi.fn()
vi.mock('preact-iso', () => ({
  useLocation: vi.fn(() => ({
    path: '/',
    query: {},
    route: mockRoute
  }))
}))

// Mock fetchWithMock
vi.mock('../../utils/fetch', () => ({
  fetchWithMock: vi.fn()
}))

// Mock reportData from app.jsx with inline signal
vi.mock('../../app', async () => {
  const { signal } = await import('@preact/signals')
  return {
    reportData: signal({
      spec: {
        namespaces: ['automation', 'cert-manager', 'default', 'flux-system', 'monitoring', 'registry', 'tailscale']
      }
    })
  }
})

import { QuickSearch, quickSearchOpen, quickSearchQuery, quickSearchResults, quickSearchLoading, parseSearchQuery } from './QuickSearch'
import { fetchWithMock } from '../../utils/fetch'


describe('QuickSearch', () => {
  beforeEach(() => {
    // Reset signals
    quickSearchOpen.value = false
    quickSearchQuery.value = ''
    quickSearchResults.value = []
    quickSearchLoading.value = false

    // Reset mocks
    vi.clearAllMocks()
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  describe('Search Button', () => {
    it('should render search button when closed', () => {
      render(<QuickSearch />)

      const searchButton = screen.getByLabelText('Open search')
      expect(searchButton).toBeInTheDocument()
    })

    it('should show search icon', () => {
      render(<QuickSearch />)

      // Search icon has specific path
      const searchIcon = document.querySelector('path[d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"]')
      expect(searchIcon).toBeInTheDocument()
    })

    it('should open search input when button is clicked', async () => {
      render(<QuickSearch />)

      const searchButton = screen.getByLabelText('Open search')
      fireEvent.click(searchButton)

      expect(quickSearchOpen.value).toBe(true)
      expect(screen.getByPlaceholderText('Search appliers...')).toBeInTheDocument()
    })
  })

  describe('Search Input', () => {
    beforeEach(() => {
      quickSearchOpen.value = true
    })

    it('should render input when search is open', () => {
      render(<QuickSearch />)

      expect(screen.getByPlaceholderText('Search appliers...')).toBeInTheDocument()
    })

    it('should render close button', () => {
      render(<QuickSearch />)

      expect(screen.getByLabelText('Close search')).toBeInTheDocument()
    })

    it('should close search when close button is clicked', () => {
      render(<QuickSearch />)

      const closeButton = screen.getByLabelText('Close search')
      fireEvent.click(closeButton)

      expect(quickSearchOpen.value).toBe(false)
      expect(quickSearchQuery.value).toBe('')
    })

    it('should close search when Escape key is pressed', () => {
      render(<QuickSearch />)

      const input = screen.getByPlaceholderText('Search appliers...')
      fireEvent.keyDown(input, { key: 'Escape' })

      expect(quickSearchOpen.value).toBe(false)
    })

    it('should update query signal on input', () => {
      render(<QuickSearch />)

      const input = screen.getByPlaceholderText('Search appliers...')
      fireEvent.input(input, { target: { value: 'flux' } })

      expect(quickSearchQuery.value).toBe('flux')
    })
  })

  describe('Debounced Search', () => {
    beforeEach(() => {
      quickSearchOpen.value = true
      fetchWithMock.mockResolvedValue({ resources: [] })
    })

    it('should not call API for queries less than 2 characters', async () => {
      render(<QuickSearch />)

      const input = screen.getByPlaceholderText('Search appliers...')
      fireEvent.input(input, { target: { value: 'f' } })

      vi.advanceTimersByTime(500)

      expect(fetchWithMock).not.toHaveBeenCalled()
    })

    it('should call API after debounce delay for valid queries', async () => {
      render(<QuickSearch />)

      const input = screen.getByPlaceholderText('Search appliers...')
      fireEvent.input(input, { target: { value: 'flux' } })

      // Should show loading immediately
      expect(quickSearchLoading.value).toBe(true)

      // Advance past debounce delay
      vi.advanceTimersByTime(300)

      expect(fetchWithMock).toHaveBeenCalledWith({
        endpoint: '/api/v1/search?name=flux',
        mockPath: '../mock/resources',
        mockExport: 'getMockSearchResults'
      })
    })

    it('should debounce multiple rapid inputs', async () => {
      render(<QuickSearch />)

      const input = screen.getByPlaceholderText('Search appliers...')

      // Type multiple times rapidly
      fireEvent.input(input, { target: { value: 'f' } })
      vi.advanceTimersByTime(100)
      fireEvent.input(input, { target: { value: 'fl' } })
      vi.advanceTimersByTime(100)
      fireEvent.input(input, { target: { value: 'flu' } })
      vi.advanceTimersByTime(100)
      fireEvent.input(input, { target: { value: 'flux' } })

      // Advance past debounce delay
      vi.advanceTimersByTime(300)

      // Should only call API once with final value
      expect(fetchWithMock).toHaveBeenCalledTimes(1)
      expect(fetchWithMock).toHaveBeenCalledWith({
        endpoint: '/api/v1/search?name=flux',
        mockPath: '../mock/resources',
        mockExport: 'getMockSearchResults'
      })
    })
  })

  describe('Search Results', () => {
    beforeEach(() => {
      quickSearchOpen.value = true
      quickSearchQuery.value = 'flux'
    })

    it('should display loading state', () => {
      quickSearchLoading.value = true

      render(<QuickSearch />)

      expect(screen.getByText('Searching...')).toBeInTheDocument()
    })

    it('should display results with status dot and Kind/Namespace/Name', () => {
      quickSearchResults.value = [
        { kind: 'FluxInstance', namespace: 'flux-system', name: 'flux', status: 'Ready' },
        { kind: 'Kustomization', namespace: 'flux-system', name: 'flux-system', status: 'Failed' }
      ]

      render(<QuickSearch />)

      // Kind is in a separate span, check for the parts
      expect(screen.getByText('FluxInstance/')).toBeInTheDocument()
      expect(screen.getByText('flux-system/flux')).toBeInTheDocument()
      expect(screen.getByText('Kustomization/')).toBeInTheDocument()
      expect(screen.getByText('flux-system/flux-system')).toBeInTheDocument()
    })

    it('should display empty state when no results found', () => {
      quickSearchResults.value = []

      render(<QuickSearch />)

      expect(screen.getByText('No resources found')).toBeInTheDocument()
    })

    it('should show green dot for Ready status', () => {
      quickSearchResults.value = [
        { kind: 'FluxInstance', namespace: 'flux-system', name: 'flux', status: 'Ready' }
      ]

      render(<QuickSearch />)

      const dot = document.querySelector('.bg-green-500')
      expect(dot).toBeInTheDocument()
    })

    it('should show red dot for Failed status', () => {
      quickSearchResults.value = [
        { kind: 'Kustomization', namespace: 'flux-system', name: 'flux-system', status: 'Failed' }
      ]

      render(<QuickSearch />)

      const dot = document.querySelector('.bg-red-500')
      expect(dot).toBeInTheDocument()
    })

    it('should show blue dot for Progressing status', () => {
      quickSearchResults.value = [
        { kind: 'ResourceSet', namespace: 'flux-system', name: 'test', status: 'Progressing' }
      ]

      render(<QuickSearch />)

      const dot = document.querySelector('.bg-blue-500')
      expect(dot).toBeInTheDocument()
    })

    it('should show yellow dot for Suspended status', () => {
      quickSearchResults.value = [
        { kind: 'HelmRelease', namespace: 'flux-system', name: 'suspended', status: 'Suspended' }
      ]

      render(<QuickSearch />)

      const dot = document.querySelector('.bg-yellow-500')
      expect(dot).toBeInTheDocument()
    })
  })

  describe('Result Navigation', () => {
    beforeEach(() => {
      quickSearchOpen.value = true
      quickSearchQuery.value = 'flux'
      quickSearchResults.value = [
        { kind: 'FluxInstance', namespace: 'flux-system', name: 'flux', status: 'Ready' }
      ]
    })

    it('should navigate to resource dashboard when result is clicked', () => {
      render(<QuickSearch />)

      const resultButton = screen.getByText('flux-system/flux').closest('button')
      fireEvent.click(resultButton)

      expect(mockRoute).toHaveBeenCalledWith('/resource/FluxInstance/flux-system/flux')
    })

    it('should close search after navigating', () => {
      render(<QuickSearch />)

      const resultButton = screen.getByText('flux-system/flux').closest('button')
      fireEvent.click(resultButton)

      expect(quickSearchOpen.value).toBe(false)
      expect(quickSearchQuery.value).toBe('')
      expect(quickSearchResults.value).toEqual([])
    })
  })

  describe('parseSearchQuery', () => {
    it('should return empty namespace for regular queries', () => {
      const result = parseSearchQuery('flux')
      expect(result).toEqual({
        namespace: null,
        name: 'flux',
        isSelectingNamespace: false,
        namespacePartial: ''
      })
    })

    it('should return empty values for empty query', () => {
      const result = parseSearchQuery('')
      expect(result).toEqual({
        namespace: null,
        name: '',
        isSelectingNamespace: false,
        namespacePartial: ''
      })
    })

    it('should detect namespace selection mode when typing ns:', () => {
      const result = parseSearchQuery('ns:')
      expect(result).toEqual({
        namespace: null,
        name: '',
        isSelectingNamespace: true,
        namespacePartial: ''
      })
    })

    it('should detect namespace selection mode with partial namespace', () => {
      const result = parseSearchQuery('ns:flux')
      expect(result).toEqual({
        namespace: null,
        name: '',
        isSelectingNamespace: true,
        namespacePartial: 'flux'
      })
    })

    it('should extract namespace and name when namespace is complete', () => {
      const result = parseSearchQuery('ns:flux-system podinfo')
      expect(result).toEqual({
        namespace: 'flux-system',
        name: 'podinfo',
        isSelectingNamespace: false,
        namespacePartial: ''
      })
    })

    it('should handle namespace with empty name', () => {
      const result = parseSearchQuery('ns:flux-system ')
      expect(result).toEqual({
        namespace: 'flux-system',
        name: '',
        isSelectingNamespace: false,
        namespacePartial: ''
      })
    })

    it('should handle case-insensitive ns: prefix', () => {
      expect(parseSearchQuery('NS:').isSelectingNamespace).toBe(true)
      expect(parseSearchQuery('Ns:').isSelectingNamespace).toBe(true)
      expect(parseSearchQuery('nS:').isSelectingNamespace).toBe(true)
      expect(parseSearchQuery('NS:flux-system ').namespace).toBe('flux-system')
    })
  })

  describe('Namespace Filtering', () => {
    beforeEach(() => {
      quickSearchOpen.value = true
      fetchWithMock.mockResolvedValue({ resources: [] })
    })

    it('should show namespace suggestions when typing ns:', () => {
      render(<QuickSearch />)

      const input = screen.getByPlaceholderText('Search appliers...')
      fireEvent.input(input, { target: { value: 'ns:' } })

      expect(screen.getByText('Select namespace')).toBeInTheDocument()
      expect(screen.getByText('automation')).toBeInTheDocument()
      expect(screen.getByText('flux-system')).toBeInTheDocument()
    })

    it('should filter namespace suggestions based on partial input', () => {
      render(<QuickSearch />)

      const input = screen.getByPlaceholderText('Search appliers...')
      fireEvent.input(input, { target: { value: 'ns:flux' } })

      expect(screen.getByText('flux-system')).toBeInTheDocument()
      expect(screen.queryByText('automation')).not.toBeInTheDocument()
    })

    it('should show no matching namespaces message', () => {
      render(<QuickSearch />)

      const input = screen.getByPlaceholderText('Search appliers...')
      fireEvent.input(input, { target: { value: 'ns:nonexistent' } })

      expect(screen.getByText('No matching namespaces')).toBeInTheDocument()
    })

    it('should select namespace on click', () => {
      render(<QuickSearch />)

      const input = screen.getByPlaceholderText('Search appliers...')
      fireEvent.input(input, { target: { value: 'ns:' } })

      const namespaceButton = screen.getByText('flux-system')
      fireEvent.click(namespaceButton)

      expect(quickSearchQuery.value).toBe('ns:flux-system ')
    })

    it('should show namespace badge when namespace is selected', () => {
      quickSearchQuery.value = 'ns:flux-system '

      render(<QuickSearch />)

      expect(screen.getByText('ns:flux-system')).toBeInTheDocument()
      // Badge should have blue background
      const badge = screen.getByText('ns:flux-system')
      expect(badge.className).toContain('bg-blue-100')
    })

    it('should show different placeholder when namespace is selected', () => {
      quickSearchQuery.value = 'ns:flux-system '

      render(<QuickSearch />)

      expect(screen.getByPlaceholderText('Search in namespace...')).toBeInTheDocument()
    })

    it('should call API with namespace parameter', async () => {
      render(<QuickSearch />)

      // First select a namespace
      quickSearchQuery.value = 'ns:flux-system '

      // Re-render to reflect the namespace selection
      render(<QuickSearch />)

      const input = screen.getByPlaceholderText('Search in namespace...')
      fireEvent.input(input, { target: { value: 'podinfo' } })

      vi.advanceTimersByTime(300)

      expect(fetchWithMock).toHaveBeenCalledWith({
        endpoint: '/api/v1/search?name=podinfo&namespace=flux-system',
        mockPath: '../mock/resources',
        mockExport: 'getMockSearchResults'
      })
    })

    it('should remove namespace badge on backspace when input is empty', () => {
      quickSearchQuery.value = 'ns:flux-system '

      render(<QuickSearch />)

      const input = screen.getByPlaceholderText('Search in namespace...')
      fireEvent.keyDown(input, { key: 'Backspace' })

      expect(quickSearchQuery.value).toBe('')
    })

    it('should not call API when typing ns prefix', async () => {
      render(<QuickSearch />)

      const input = screen.getByPlaceholderText('Search appliers...')
      fireEvent.input(input, { target: { value: 'ns' } })

      vi.advanceTimersByTime(500)

      expect(fetchWithMock).not.toHaveBeenCalled()
    })

    it('should not call API when typing ns: without completing namespace', async () => {
      render(<QuickSearch />)

      const input = screen.getByPlaceholderText('Search appliers...')
      fireEvent.input(input, { target: { value: 'ns:flux' } })

      vi.advanceTimersByTime(500)

      expect(fetchWithMock).not.toHaveBeenCalled()
    })
  })

  describe('Namespace Keyboard Navigation', () => {
    beforeEach(() => {
      quickSearchOpen.value = true
    })

    it('should navigate namespace suggestions with arrow keys', () => {
      render(<QuickSearch />)

      const input = screen.getByPlaceholderText('Search appliers...')
      fireEvent.input(input, { target: { value: 'ns:' } })

      // Press ArrowDown to select first item
      fireEvent.keyDown(input, { key: 'ArrowDown' })

      // First namespace (automation) should be highlighted
      const firstItem = screen.getByText('automation').closest('button')
      expect(firstItem.className).toContain('bg-gray-100')
    })

    it('should select namespace on Enter key', () => {
      render(<QuickSearch />)

      const input = screen.getByPlaceholderText('Search appliers...')
      fireEvent.input(input, { target: { value: 'ns:' } })

      // Navigate to first item and select
      fireEvent.keyDown(input, { key: 'ArrowDown' })
      fireEvent.keyDown(input, { key: 'Enter' })

      expect(quickSearchQuery.value).toBe('ns:automation ')
    })

    it('should navigate up with ArrowUp', () => {
      render(<QuickSearch />)

      const input = screen.getByPlaceholderText('Search appliers...')
      fireEvent.input(input, { target: { value: 'ns:' } })

      // Navigate down twice then up once
      fireEvent.keyDown(input, { key: 'ArrowDown' })
      fireEvent.keyDown(input, { key: 'ArrowDown' })
      fireEvent.keyDown(input, { key: 'ArrowUp' })

      // First namespace should be highlighted again
      const firstItem = screen.getByText('automation').closest('button')
      expect(firstItem.className).toContain('bg-gray-100')
    })
  })

  describe('Search Hint', () => {
    beforeEach(() => {
      quickSearchOpen.value = true
    })

    it('should show hint when typing 1 character', () => {
      render(<QuickSearch />)

      const input = screen.getByPlaceholderText('Search appliers...')
      fireEvent.input(input, { target: { value: 'f' } })

      expect(screen.getByText('Type at least 2 characters to search')).toBeInTheDocument()
      expect(screen.getByText('ns:')).toBeInTheDocument()
      expect(screen.getByText(/to filter by namespace/)).toBeInTheDocument()
    })

    it('should not show hint when typing 2+ characters', () => {
      render(<QuickSearch />)

      const input = screen.getByPlaceholderText('Search appliers...')
      fireEvent.input(input, { target: { value: 'fl' } })

      expect(screen.queryByText('Type at least 2 characters to search')).not.toBeInTheDocument()
    })

    it('should set ns: prefix when clicking hint link', () => {
      render(<QuickSearch />)

      const input = screen.getByPlaceholderText('Search appliers...')
      fireEvent.input(input, { target: { value: 'f' } })

      const nsLink = screen.getByText('ns:')
      fireEvent.click(nsLink)

      expect(quickSearchQuery.value).toBe('ns:')
    })

    it('should not show hint when typing ns prefix', () => {
      render(<QuickSearch />)

      const input = screen.getByPlaceholderText('Search appliers...')
      fireEvent.input(input, { target: { value: 'n' } })

      // Hint should appear for single char that's not 'n' leading to 'ns'
      expect(screen.getByText('Type at least 2 characters to search')).toBeInTheDocument()

      // But when typing 'ns', no hint or results panel should show
      fireEvent.input(input, { target: { value: 'ns' } })
      expect(screen.queryByText('Type at least 2 characters to search')).not.toBeInTheDocument()
    })
  })
})
