// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { renderHook } from '@testing-library/preact'
import { setPageTitle, usePageTitle } from './title'

describe('title utilities', () => {
  let originalTitle

  beforeEach(() => {
    originalTitle = document.title
  })

  afterEach(() => {
    document.title = originalTitle
  })

  describe('setPageTitle', () => {
    it('should set home page title when pageTitle is null', () => {
      setPageTitle(null)
      expect(document.title).toBe('Flux Status Page')
    })

    it('should set home page title when pageTitle is undefined', () => {
      setPageTitle(undefined)
      expect(document.title).toBe('Flux Status Page')
    })

    it('should set home page title when pageTitle is empty string', () => {
      setPageTitle('')
      expect(document.title).toBe('Flux Status Page')
    })

    it('should set page-specific title with suffix', () => {
      setPageTitle('Favorites')
      expect(document.title).toBe('Favorites - Flux Status')
    })

    it('should set page-specific title for Resources', () => {
      setPageTitle('Resources')
      expect(document.title).toBe('Resources - Flux Status')
    })

    it('should set page-specific title for Events', () => {
      setPageTitle('Events')
      expect(document.title).toBe('Events - Flux Status')
    })

    it('should set page-specific title for resource names', () => {
      setPageTitle('my-kustomization')
      expect(document.title).toBe('my-kustomization - Flux Status')
    })

    it('should handle resource names with special characters', () => {
      setPageTitle('flux-system-config')
      expect(document.title).toBe('flux-system-config - Flux Status')
    })
  })

  describe('usePageTitle', () => {
    it('should set home page title when called with null', () => {
      renderHook(() => usePageTitle(null))
      expect(document.title).toBe('Flux Status Page')
    })

    it('should set page-specific title when called with a string', () => {
      renderHook(() => usePageTitle('Favorites'))
      expect(document.title).toBe('Favorites - Flux Status')
    })

    it('should update title when pageTitle changes', () => {
      const { rerender } = renderHook(({ title }) => usePageTitle(title), {
        initialProps: { title: 'Favorites' }
      })
      expect(document.title).toBe('Favorites - Flux Status')

      rerender({ title: 'Resources' })
      expect(document.title).toBe('Resources - Flux Status')
    })

    it('should update title from page to home', () => {
      const { rerender } = renderHook(({ title }) => usePageTitle(title), {
        initialProps: { title: 'Events' }
      })
      expect(document.title).toBe('Events - Flux Status')

      rerender({ title: null })
      expect(document.title).toBe('Flux Status Page')
    })
  })
})
