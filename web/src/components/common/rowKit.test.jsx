// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent, renderHook, act } from '@testing-library/preact'
import {
  Spinner,
  Star,
  KindChip,
  NameLink,
  Reveal,
  useDisclosure,
  NEUTRAL_CHIP,
} from './rowKit'

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

describe('NameLink', () => {
  it('renders anchor links to the dashboard when href is set', () => {
    render(<NameLink href="/dash" namespace="flux-system" name="podinfo" />)
    const links = screen.getAllByRole('link')
    expect(links.length).toBe(2) // mobile + desktop variant
    links.forEach((a) => expect(a).toHaveAttribute('href', '/dash'))
    // Two-tone label: namespace prefix + name.
    expect(screen.getAllByText('flux-system/').length).toBe(2)
    expect(screen.getAllByText('podinfo').length).toBe(2)
  })

  it('renders plain text (no link) when href is absent', () => {
    render(<NameLink namespace="default" name="my-config" />)
    expect(screen.queryByRole('link')).toBeNull()
    expect(screen.getAllByText('default/').length).toBe(2)
    expect(screen.getAllByText('my-config').length).toBe(2)
  })

  it('omits the namespace prefix for cluster-scoped objects', () => {
    render(<NameLink name="cluster-reader" />)
    expect(screen.queryByText('/', { exact: false })).toBeNull()
    expect(screen.getAllByText('cluster-reader').length).toBe(2)
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

  it('collapses and unmounts the content on toggle', () => {
    const { result } = renderHook(() => useDisclosure())
    act(() => result.current.toggle())
    act(() => result.current.onReady())
    act(() => result.current.toggle())
    expect(result.current.open).toBe(false)
    expect(result.current.mounted).toBe(false)
    expect(result.current.loading).toBe(false)
  })

  it('re-mounts and re-fetches on each expand (no cached snapshot)', () => {
    const { result } = renderHook(() => useDisclosure())
    act(() => result.current.toggle())   // mount + load
    act(() => result.current.onReady())  // reveal
    act(() => result.current.toggle())   // collapse + unmount
    act(() => result.current.toggle())   // re-expand
    expect(result.current.mounted).toBe(true)
    expect(result.current.loading).toBe(true)
    expect(result.current.open).toBe(false)
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
