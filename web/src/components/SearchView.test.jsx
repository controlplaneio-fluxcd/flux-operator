// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, fireEvent, cleanup } from '@testing-library/preact'
import { SearchView } from './SearchView'
import { activeSearchTab } from '../app'

// Mock child components to simplify testing
vi.mock('./EventList', () => ({
  EventList: () => <div data-testid="event-list">Event List</div>
}))
vi.mock('./ResourceList', () => ({
  ResourceList: () => <div data-testid="resource-list">Resource List</div>
}))
afterEach(cleanup)
describe('SearchView', () => {
  describe('Tab Rendering', () => {
    it('should render Events tab button', () => {
      render(<SearchView />)
      expect(screen.getByRole('button', { name: 'Events' })).toBeInTheDocument()
    })
    it('should render Resources tab button', () => {
      render(<SearchView />)
      expect(screen.getByRole('button', { name: 'Resources' })).toBeInTheDocument()
    })
    it('should render Events tab as active by default', () => {
      activeSearchTab.value = 'events'
      render(<SearchView />)
      const eventsTab = screen.getByRole('button', { name: 'Events' })
      expect(eventsTab.className).toContain('border-flux-blue')
    })
    it('should render Resources tab as inactive by default', () => {
      activeSearchTab.value = 'events'
      render(<SearchView />)
      const resourcesTab = screen.getByRole('button', { name: 'Resources' })
      expect(resourcesTab.className).toContain('border-transparent')
    })
    it('should render Resources tab as active when activeSearchTab is set', () => {
      activeSearchTab.value = 'resources'
      render(<SearchView />)
      const resourcesTab = screen.getByRole('button', { name: 'Resources' })
      expect(resourcesTab.className).toContain('border-flux-blue')
    })
  })
  describe('Tab Content Display', () => {
    it('should show EventList component by default', () => {
      activeSearchTab.value = 'events'
      render(<SearchView />)
      expect(screen.getByTestId('event-list')).toBeInTheDocument()
      expect(screen.queryByTestId('resource-list')).not.toBeInTheDocument()
    })
    it('should show ResourceList component after clicking Resources tab', () => {
      render(<SearchView />)
      fireEvent.click(screen.getByRole('button', { name: 'Resources' }))
      expect(screen.getByTestId('resource-list')).toBeInTheDocument()
      expect(screen.queryByTestId('event-list')).not.toBeInTheDocument()
    })
  })
  describe('Tab Switching', () => {
    it('should switch content and styles when tabs are clicked', () => {
      activeSearchTab.value = 'events'
      render(<SearchView />)
      const eventsTab = screen.getByRole('button', { name: 'Events' })
      const resourcesTab = screen.getByRole('button', { name: 'Resources' })
      // Initial state
      expect(screen.getByTestId('event-list')).toBeInTheDocument()
      expect(eventsTab.className).toContain('border-flux-blue')
      expect(resourcesTab.className).toContain('border-transparent')
      // Click Resources
      fireEvent.click(resourcesTab)
      expect(screen.getByTestId('resource-list')).toBeInTheDocument()
      expect(screen.queryByTestId('event-list')).not.toBeInTheDocument()
      expect(resourcesTab.className).toContain('border-flux-blue')
      expect(eventsTab.className).toContain('border-transparent')
      // Click Events
      fireEvent.click(eventsTab)
      expect(screen.getByTestId('event-list')).toBeInTheDocument()
      expect(screen.queryByTestId('resource-list')).not.toBeInTheDocument()
      expect(eventsTab.className).toContain('border-flux-blue')
      expect(resourcesTab.className).toContain('border-transparent')
    })
  })
})
