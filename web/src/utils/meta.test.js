// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { renderHook } from '@testing-library/preact'
import { setPageTitle, setPageDescription, usePageTitle, usePageMeta } from './meta'

describe('meta utilities', () => {
  let originalTitle
  let metaDescription
  let ogTitle
  let ogDescription

  beforeEach(() => {
    originalTitle = document.title
    // Create meta description tag if it doesn't exist
    metaDescription = document.querySelector('meta[name="description"]')
    if (!metaDescription) {
      metaDescription = document.createElement('meta')
      metaDescription.setAttribute('name', 'description')
      metaDescription.setAttribute('content', 'Real-time visibility into your GitOps pipelines')
      document.head.appendChild(metaDescription)
    }
    // Create og:title tag if it doesn't exist
    ogTitle = document.querySelector('meta[property="og:title"]')
    if (!ogTitle) {
      ogTitle = document.createElement('meta')
      ogTitle.setAttribute('property', 'og:title')
      ogTitle.setAttribute('content', 'Flux Status')
      document.head.appendChild(ogTitle)
    }
    // Create og:description tag if it doesn't exist
    ogDescription = document.querySelector('meta[property="og:description"]')
    if (!ogDescription) {
      ogDescription = document.createElement('meta')
      ogDescription.setAttribute('property', 'og:description')
      ogDescription.setAttribute('content', 'Real-time visibility into your GitOps pipelines')
      document.head.appendChild(ogDescription)
    }
  })

  afterEach(() => {
    document.title = originalTitle
    // Reset meta tags to defaults
    if (metaDescription) {
      metaDescription.setAttribute('content', 'Real-time visibility into your GitOps pipelines')
    }
    if (ogTitle) {
      ogTitle.setAttribute('content', 'Flux Status')
    }
    if (ogDescription) {
      ogDescription.setAttribute('content', 'Real-time visibility into your GitOps pipelines')
    }
  })

  describe('setPageTitle', () => {
    it('should set home page title when pageTitle is null', () => {
      setPageTitle(null)
      expect(document.title).toBe('Flux Status')
      expect(ogTitle.getAttribute('content')).toBe('Flux Status')
    })

    it('should set home page title when pageTitle is undefined', () => {
      setPageTitle(undefined)
      expect(document.title).toBe('Flux Status')
      expect(ogTitle.getAttribute('content')).toBe('Flux Status')
    })

    it('should set home page title when pageTitle is empty string', () => {
      setPageTitle('')
      expect(document.title).toBe('Flux Status')
      expect(ogTitle.getAttribute('content')).toBe('Flux Status')
    })

    it('should set page-specific title with suffix', () => {
      setPageTitle('Favorites')
      expect(document.title).toBe('Favorites - Flux Status')
      expect(ogTitle.getAttribute('content')).toBe('Favorites - Flux Status')
    })

    it('should set page-specific title for Resources', () => {
      setPageTitle('Resources')
      expect(document.title).toBe('Resources - Flux Status')
      expect(ogTitle.getAttribute('content')).toBe('Resources - Flux Status')
    })

    it('should set page-specific title for Events', () => {
      setPageTitle('Events')
      expect(document.title).toBe('Events - Flux Status')
      expect(ogTitle.getAttribute('content')).toBe('Events - Flux Status')
    })

    it('should set page-specific title for resource names', () => {
      setPageTitle('my-kustomization')
      expect(document.title).toBe('my-kustomization - Flux Status')
      expect(ogTitle.getAttribute('content')).toBe('my-kustomization - Flux Status')
    })

    it('should handle resource names with special characters', () => {
      setPageTitle('flux-system-config')
      expect(document.title).toBe('flux-system-config - Flux Status')
      expect(ogTitle.getAttribute('content')).toBe('flux-system-config - Flux Status')
    })
  })

  describe('setPageDescription', () => {
    it('should set description when provided', () => {
      setPageDescription('Test description')
      expect(metaDescription.getAttribute('content')).toBe('Test description')
      expect(ogDescription.getAttribute('content')).toBe('Test description')
    })

    it('should set default description when null is provided', () => {
      setPageDescription('Custom')
      setPageDescription(null)
      expect(metaDescription.getAttribute('content')).toBe('Real-time visibility into your GitOps pipelines')
      expect(ogDescription.getAttribute('content')).toBe('Real-time visibility into your GitOps pipelines')
    })

    it('should set default description when undefined is provided', () => {
      setPageDescription('Custom')
      setPageDescription(undefined)
      expect(metaDescription.getAttribute('content')).toBe('Real-time visibility into your GitOps pipelines')
      expect(ogDescription.getAttribute('content')).toBe('Real-time visibility into your GitOps pipelines')
    })

    it('should handle resource-specific descriptions', () => {
      setPageDescription('Kustomization/flux-system/my-app dashboard')
      expect(metaDescription.getAttribute('content')).toBe('Kustomization/flux-system/my-app dashboard')
      expect(ogDescription.getAttribute('content')).toBe('Kustomization/flux-system/my-app dashboard')
    })
  })

  describe('usePageTitle', () => {
    it('should set home page title when called with null', () => {
      renderHook(() => usePageTitle(null))
      expect(document.title).toBe('Flux Status')
      expect(ogTitle.getAttribute('content')).toBe('Flux Status')
    })

    it('should set page-specific title when called with a string', () => {
      renderHook(() => usePageTitle('Favorites'))
      expect(document.title).toBe('Favorites - Flux Status')
      expect(ogTitle.getAttribute('content')).toBe('Favorites - Flux Status')
    })

    it('should update title when pageTitle changes', () => {
      const { rerender } = renderHook(({ title }) => usePageTitle(title), {
        initialProps: { title: 'Favorites' }
      })
      expect(document.title).toBe('Favorites - Flux Status')
      expect(ogTitle.getAttribute('content')).toBe('Favorites - Flux Status')

      rerender({ title: 'Resources' })
      expect(document.title).toBe('Resources - Flux Status')
      expect(ogTitle.getAttribute('content')).toBe('Resources - Flux Status')
    })

    it('should update title from page to home', () => {
      const { rerender } = renderHook(({ title }) => usePageTitle(title), {
        initialProps: { title: 'Events' }
      })
      expect(document.title).toBe('Events - Flux Status')
      expect(ogTitle.getAttribute('content')).toBe('Events - Flux Status')

      rerender({ title: null })
      expect(document.title).toBe('Flux Status')
      expect(ogTitle.getAttribute('content')).toBe('Flux Status')
    })
  })

  describe('usePageMeta', () => {
    it('should set both title and description on mount', () => {
      renderHook(() => usePageMeta('Favorites', 'Favorites dashboard'))
      expect(document.title).toBe('Favorites - Flux Status')
      expect(ogTitle.getAttribute('content')).toBe('Favorites - Flux Status')
      expect(metaDescription.getAttribute('content')).toBe('Favorites dashboard')
      expect(ogDescription.getAttribute('content')).toBe('Favorites dashboard')
    })

    it('should set default title and description when nulls are provided', () => {
      renderHook(() => usePageMeta(null, null))
      expect(document.title).toBe('Flux Status')
      expect(ogTitle.getAttribute('content')).toBe('Flux Status')
      expect(metaDescription.getAttribute('content')).toBe('Real-time visibility into your GitOps pipelines')
      expect(ogDescription.getAttribute('content')).toBe('Real-time visibility into your GitOps pipelines')
    })

    it('should update both title and description when props change', () => {
      const { rerender } = renderHook(
        ({ title, description }) => usePageMeta(title, description),
        { initialProps: { title: 'Favorites', description: 'Favorites dashboard' } }
      )
      expect(document.title).toBe('Favorites - Flux Status')
      expect(ogTitle.getAttribute('content')).toBe('Favorites - Flux Status')
      expect(metaDescription.getAttribute('content')).toBe('Favorites dashboard')
      expect(ogDescription.getAttribute('content')).toBe('Favorites dashboard')

      rerender({ title: 'Resources', description: 'Resources dashboard' })
      expect(document.title).toBe('Resources - Flux Status')
      expect(ogTitle.getAttribute('content')).toBe('Resources - Flux Status')
      expect(metaDescription.getAttribute('content')).toBe('Resources dashboard')
      expect(ogDescription.getAttribute('content')).toBe('Resources dashboard')
    })

    it('should handle resource page with dynamic description', () => {
      renderHook(() => usePageMeta('my-app', 'Kustomization/flux-system/my-app dashboard'))
      expect(document.title).toBe('my-app - Flux Status')
      expect(ogTitle.getAttribute('content')).toBe('my-app - Flux Status')
      expect(metaDescription.getAttribute('content')).toBe('Kustomization/flux-system/my-app dashboard')
      expect(ogDescription.getAttribute('content')).toBe('Kustomization/flux-system/my-app dashboard')
    })

    it('should handle navigation from resource to home', () => {
      const { rerender } = renderHook(
        ({ title, description }) => usePageMeta(title, description),
        { initialProps: { title: 'my-app', description: 'Kustomization/flux-system/my-app dashboard' } }
      )
      expect(document.title).toBe('my-app - Flux Status')
      expect(ogTitle.getAttribute('content')).toBe('my-app - Flux Status')
      expect(metaDescription.getAttribute('content')).toBe('Kustomization/flux-system/my-app dashboard')
      expect(ogDescription.getAttribute('content')).toBe('Kustomization/flux-system/my-app dashboard')

      rerender({ title: null, description: null })
      expect(document.title).toBe('Flux Status')
      expect(ogTitle.getAttribute('content')).toBe('Flux Status')
      expect(metaDescription.getAttribute('content')).toBe('Real-time visibility into your GitOps pipelines')
      expect(ogDescription.getAttribute('content')).toBe('Real-time visibility into your GitOps pipelines')
    })
  })
})
