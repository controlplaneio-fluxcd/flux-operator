// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/preact'
import { QuickSearch, quickSearchOpen, quickSearchQuery, quickSearchResults, quickSearchLoading } from './QuickSearch'

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
})
