// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, fireEvent, cleanup } from '@testing-library/preact'
import { useRef } from 'preact/hooks'
import { useDismiss } from './useDismiss'

// Minimal harness: a panel that dismisses via the hook.
function Panel({ active, onDismiss }) {
  const ref = useRef(null)
  useDismiss(ref, onDismiss, active)
  return (
    <div>
      <div ref={ref} data-testid="panel">inside</div>
      <button data-testid="outside">outside</button>
    </div>
  )
}

describe('useDismiss', () => {
  afterEach(cleanup)

  it('dismisses on an outside mousedown while active', () => {
    const onDismiss = vi.fn()
    render(<Panel active={true} onDismiss={onDismiss} />)
    fireEvent.mouseDown(screen.getByTestId('outside'))
    expect(onDismiss).toHaveBeenCalledTimes(1)
  })

  it('does not dismiss on a mousedown inside the panel', () => {
    const onDismiss = vi.fn()
    render(<Panel active={true} onDismiss={onDismiss} />)
    fireEvent.mouseDown(screen.getByTestId('panel'))
    expect(onDismiss).not.toHaveBeenCalled()
  })

  it('dismisses on Escape while active', () => {
    const onDismiss = vi.fn()
    render(<Panel active={true} onDismiss={onDismiss} />)
    fireEvent.keyDown(document, { key: 'Escape' })
    expect(onDismiss).toHaveBeenCalledTimes(1)
  })

  it('stops Escape from reaching outer handlers', () => {
    const onDismiss = vi.fn()
    const outer = vi.fn()
    window.addEventListener('keydown', outer)
    render(<Panel active={true} onDismiss={onDismiss} />)
    fireEvent.keyDown(document, { key: 'Escape' })
    expect(onDismiss).toHaveBeenCalledTimes(1)
    expect(outer).not.toHaveBeenCalled()
    window.removeEventListener('keydown', outer)
  })

  it('does not listen while inactive', () => {
    const onDismiss = vi.fn()
    render(<Panel active={false} onDismiss={onDismiss} />)
    fireEvent.mouseDown(screen.getByTestId('outside'))
    fireEvent.keyDown(document, { key: 'Escape' })
    expect(onDismiss).not.toHaveBeenCalled()
  })
})
