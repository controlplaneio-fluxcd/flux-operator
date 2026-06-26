// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render, renderHook, screen, fireEvent, waitFor, cleanup } from '@testing-library/preact'
import { EditableYamlBlock, usePrismTheme } from './yaml'
import { fetchWithMock } from '../../../utils/fetch'

vi.mock('../../../utils/fetch', () => ({
  fetchWithMock: vi.fn()
}))
import { appliedTheme } from '../../../utils/theme'

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
    fetchWithMock.mockReset()
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

  it('keeps the link while another consumer is still mounted (reference counted)', () => {
    // Two components use the shared theme link concurrently.
    const first = renderHook(() => usePrismTheme())
    const second = renderHook(() => usePrismTheme())
    expect(document.getElementById(LINK_ID)).not.toBeNull()

    // Unmounting one must NOT remove the link the other still relies on.
    first.unmount()
    expect(document.getElementById(LINK_ID)).not.toBeNull()

    // Removed only once the last consumer unmounts.
    second.unmount()
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

describe('EditableYamlBlock', () => {
  beforeEach(() => {
    fetchWithMock.mockReset()
    fetchWithMock.mockResolvedValue({ success: true, message: 'Updated ConfigMap default/app-config' })
  })

  afterEach(() => {
    cleanup()
  })

  it('edits YAML and saves it through the object edit API', async () => {
    const onSaved = vi.fn()
    const data = {
      apiVersion: 'v1',
      kind: 'ConfigMap',
      metadata: { name: 'app-config', namespace: 'default' },
      data: { key: 'old-value' }
    }

    render(<EditableYamlBlock data={data} onSaved={onSaved} />)

    fireEvent.click(screen.getByRole('button', { name: 'Edit YAML' }))

    const editor = screen.getByLabelText('YAML object editor')
    expect(editor.value).toContain('old-value')

    fireEvent.input(editor, {
      target: { value: editor.value.replace('old-value', 'new-value') }
    })
    fireEvent.click(screen.getByRole('button', { name: 'Save YAML' }))

    await waitFor(() => {
      expect(fetchWithMock).toHaveBeenCalledWith({
        endpoint: '/api/v1/object',
        mockPath: '../mock/action',
        mockExport: 'mockObjectEdit',
        method: 'PUT',
        body: expect.objectContaining({
          yaml: expect.stringContaining('new-value'),
          apiVersion: 'v1',
          kind: 'ConfigMap',
          namespace: 'default',
          name: 'app-config'
        })
      })
      expect(onSaved).toHaveBeenCalledTimes(1)
    })
  })

  it('shows save errors without leaving edit mode', async () => {
    fetchWithMock.mockRejectedValue(new Error('forbidden'))

    render(<EditableYamlBlock data={{ apiVersion: 'v1', kind: 'ConfigMap', metadata: { name: 'app-config' } }} />)

    fireEvent.click(screen.getByRole('button', { name: 'Edit YAML' }))
    fireEvent.click(screen.getByRole('button', { name: 'Save YAML' }))

    await waitFor(() => {
      expect(screen.getByText('forbidden')).toBeInTheDocument()
      expect(screen.getByLabelText('YAML object editor')).toBeInTheDocument()
    })
  })
})
