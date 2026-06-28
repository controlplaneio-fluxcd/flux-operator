// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent, renderHook, act } from '@testing-library/preact'
import {
  Spinner,
  Star,
  KindChip,
  Reveal,
  useDisclosure,
  NEUTRAL_CHIP,
} from './compactRow'

describe('Spinner', () => {
  it('renders an animated spinner svg', () => {
    const { container } = render(<Spinner />)
    const svg = container.querySelector('svg')
    expect(svg).not.toBeNull()
    expect(svg).toHaveClass('animate-spin')
  })
})

describe('Star', () => {
  it('renders as inactive (neutral) when not active', () => {
    render(<Star active={false} onClick={() => {}} />)
    const button = screen.getByRole('button')
    expect(button).toHaveAttribute('title', 'Add to favorites')
    expect(button).not.toHaveClass('text-yellow-500')
  })

  it('renders the active (yellow) state when favorited', () => {
    render(<Star active={true} onClick={() => {}} />)
    const button = screen.getByRole('button')
    expect(button).toHaveAttribute('title', 'Remove from favorites')
    expect(button).toHaveClass('text-yellow-500')
  })

  it('calls onClick when pressed', () => {
    const onClick = vi.fn()
    render(<Star active={false} onClick={onClick} />)
    fireEvent.click(screen.getByRole('button'))
    expect(onClick).toHaveBeenCalledTimes(1)
  })
})

describe('KindChip', () => {
  it('renders the short kind as its label', () => {
    render(<KindChip kind="Kustomization" colorClass={NEUTRAL_CHIP} />)
    expect(screen.getByText('ks')).toBeInTheDocument()
  })

  it('defaults the tooltip to the full kind', () => {
    render(<KindChip kind="Deployment" colorClass={NEUTRAL_CHIP} />)
    const chip = screen.getByText('deploy')
    expect(chip).toHaveAttribute('title', 'Deployment')
  })

  it('applies the color class and extra responsive classes', () => {
    render(<KindChip kind="Deployment" colorClass={NEUTRAL_CHIP} cls="hidden sm:block" title="custom" />)
    const chip = screen.getByText('deploy')
    expect(chip).toHaveClass('hidden')
    expect(chip).toHaveAttribute('title', 'custom')
  })
})

describe('Reveal', () => {
  it('applies the open classes when revealed', () => {
    const { container } = render(<Reveal open={true}><span>content</span></Reveal>)
    const grid = container.firstChild
    expect(grid).toHaveClass('grid-rows-[1fr]')
    expect(grid).toHaveClass('opacity-100')
    expect(screen.getByText('content')).toBeInTheDocument()
  })

  it('applies the collapsed classes but keeps children mounted', () => {
    const { container } = render(<Reveal open={false}><span>content</span></Reveal>)
    const grid = container.firstChild
    expect(grid).toHaveClass('grid-rows-[0fr]')
    expect(grid).toHaveClass('opacity-0')
    // Children stay in the DOM so a lazily mounted panel can fetch while collapsed.
    expect(screen.getByText('content')).toBeInTheDocument()
  })
})

describe('useDisclosure', () => {
  it('starts closed, unmounted and not loading', () => {
    const { result } = renderHook(() => useDisclosure())
    expect(result.current.open).toBe(false)
    expect(result.current.mounted).toBe(false)
    expect(result.current.loading).toBe(false)
  })

  it('mounts and enters the loading state on first toggle without opening yet', () => {
    const { result } = renderHook(() => useDisclosure())
    act(() => result.current.toggle())
    expect(result.current.mounted).toBe(true)
    expect(result.current.loading).toBe(true)
    expect(result.current.open).toBe(false)
  })

  it('opens and clears loading when the panel signals onReady', () => {
    const { result } = renderHook(() => useDisclosure())
    act(() => result.current.toggle())
    act(() => result.current.onReady())
    expect(result.current.open).toBe(true)
    expect(result.current.loading).toBe(false)
    expect(result.current.mounted).toBe(true)
  })

  it('collapses on toggle while keeping the content mounted', () => {
    const { result } = renderHook(() => useDisclosure())
    act(() => result.current.toggle())
    act(() => result.current.onReady())
    act(() => result.current.toggle())
    expect(result.current.open).toBe(false)
    expect(result.current.mounted).toBe(true)
    expect(result.current.loading).toBe(false)
  })

  it('re-opens instantly without a second loading pass once loaded', () => {
    const { result } = renderHook(() => useDisclosure())
    act(() => result.current.toggle())
    act(() => result.current.onReady())
    act(() => result.current.toggle()) // collapse
    act(() => result.current.toggle()) // re-open
    expect(result.current.open).toBe(true)
    expect(result.current.loading).toBe(false)
  })

  it('cancels an in-flight fetch when toggled while still loading', () => {
    const { result } = renderHook(() => useDisclosure())
    act(() => result.current.toggle()) // mount + loading
    act(() => result.current.toggle()) // cancel before onReady
    expect(result.current.loading).toBe(false)
    expect(result.current.mounted).toBe(false)
    expect(result.current.open).toBe(false)
  })
})
