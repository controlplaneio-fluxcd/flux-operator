// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, cleanup } from '@testing-library/preact'
import { KeyboardShortcutsModal } from './KeyboardShortcutsModal'
import { keyboardShortcutsOpen } from '../../utils/keyboardShortcuts'

describe('KeyboardShortcutsModal', () => {
  beforeEach(() => {
    keyboardShortcutsOpen.value = false
  })

  afterEach(() => {
    cleanup()
    keyboardShortcutsOpen.value = false
    document.body.style.overflow = ''
  })

  it('does not render when closed', () => {
    render(<KeyboardShortcutsModal />)
    expect(screen.queryByTestId('keyboard-shortcuts-modal')).not.toBeInTheDocument()
  })

  it('renders shortcut groups when open', () => {
    keyboardShortcutsOpen.value = true
    render(<KeyboardShortcutsModal />)

    expect(screen.getByRole('dialog', { name: 'Keyboard shortcuts' })).toBeInTheDocument()
    expect(screen.getByText('Navigation')).toBeInTheDocument()
    expect(screen.getByText('Go to Dashboard')).toBeInTheDocument()
    expect(screen.getByText('Go to Favorites')).toBeInTheDocument()
    expect(screen.getByText('Open quick search')).toBeInTheDocument()
  })

  it('closes when the close button is clicked', () => {
    keyboardShortcutsOpen.value = true
    render(<KeyboardShortcutsModal />)

    fireEvent.click(screen.getByTestId('keyboard-shortcuts-close'))
    expect(keyboardShortcutsOpen.value).toBe(false)
  })

  it('closes on Escape', () => {
    keyboardShortcutsOpen.value = true
    render(<KeyboardShortcutsModal />)

    fireEvent.keyDown(document, { key: 'Escape' })
    expect(keyboardShortcutsOpen.value).toBe(false)
  })

  it('closes when clicking the overlay', () => {
    keyboardShortcutsOpen.value = true
    render(<KeyboardShortcutsModal />)

    fireEvent.click(screen.getByTestId('keyboard-shortcuts-overlay'))
    expect(keyboardShortcutsOpen.value).toBe(false)
  })

  it('locks body scroll while open', () => {
    keyboardShortcutsOpen.value = true
    const { rerender } = render(<KeyboardShortcutsModal />)
    expect(document.body.style.overflow).toBe('hidden')

    keyboardShortcutsOpen.value = false
    rerender(<KeyboardShortcutsModal />)
    expect(document.body.style.overflow).toBe('')
  })
})
