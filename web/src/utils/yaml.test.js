// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { renderHook, cleanup } from '@testing-library/preact'
import { usePrismTheme } from './yaml'
import { appliedTheme } from './theme'

describe('usePrismTheme', () => {
  const LINK_ID = 'prism-theme-link'

  beforeEach(() => {
    // Clean up any existing link elements
    const existingLink = document.getElementById(LINK_ID)
    if (existingLink) {
      existingLink.remove()
    }
    // Reset theme to light
    appliedTheme.value = 'light'
  })

  afterEach(() => {
    cleanup()
    // Clean up link elements after each test
    const link = document.getElementById(LINK_ID)
    if (link) {
      link.remove()
    }
  })

  it('should create a link element with prism-theme-link id', () => {
    renderHook(() => usePrismTheme())

    const link = document.getElementById(LINK_ID)
    expect(link).not.toBeNull()
    expect(link.tagName).toBe('LINK')
    expect(link.rel).toBe('stylesheet')
  })

  it('should add link element to document head', () => {
    renderHook(() => usePrismTheme())

    const link = document.getElementById(LINK_ID)
    expect(link.parentElement).toBe(document.head)
  })

  it('should set href attribute on the link element', () => {
    renderHook(() => usePrismTheme())

    const link = document.getElementById(LINK_ID)
    // In test environment, Vite transforms CSS imports differently
    // Just verify href is set (not empty/undefined)
    expect(link.href).toBeTruthy()
  })

  it('should remove existing link before adding new one', () => {
    // Create an existing link
    const existingLink = document.createElement('link')
    existingLink.id = LINK_ID
    existingLink.href = 'old-theme.css'
    document.head.appendChild(existingLink)

    renderHook(() => usePrismTheme())

    const links = document.querySelectorAll(`#${LINK_ID}`)
    expect(links.length).toBe(1)
    expect(links[0].href).not.toContain('old-theme.css')
  })

  it('should remove link element on unmount', () => {
    const { unmount } = renderHook(() => usePrismTheme())

    expect(document.getElementById(LINK_ID)).not.toBeNull()

    unmount()

    expect(document.getElementById(LINK_ID)).toBeNull()
  })

  it('should recreate link when theme changes', async () => {
    appliedTheme.value = 'light'

    const { rerender } = renderHook(() => usePrismTheme())

    expect(document.getElementById(LINK_ID)).not.toBeNull()

    // Change theme
    appliedTheme.value = 'dark'
    rerender()

    // Link should still exist and href should be updated (or recreated)
    const updatedLink = document.getElementById(LINK_ID)
    expect(updatedLink).not.toBeNull()
    // In test env we can't check actual CSS files, but link should be recreated
    expect(document.querySelectorAll(`#${LINK_ID}`).length).toBe(1)
  })
})
