// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/preact'
import { StatusHeroCard } from './StatusHeroCard'

const icon = <svg data-testid="hero-icon" />

describe('StatusHeroCard', () => {
  it('renders the structured layout (kind / name / namespace)', () => {
    render(
      <StatusHeroCard
        bgColor="bg-green-50"
        borderColor="border-success"
        icon={icon}
        kind="Deployment"
        name="frontend"
        namespace="apps"
      />
    )

    expect(screen.getByRole('heading', { name: 'frontend' })).toBeInTheDocument()
    expect(screen.getByText('Deployment')).toBeInTheDocument()
    expect(screen.getByText('Namespace: apps')).toBeInTheDocument()
    expect(screen.getByTestId('hero-icon')).toBeInTheDocument()
  })

  it('renders the icon disc with the darker shade of the card tint', () => {
    const { container } = render(
      <StatusHeroCard bgColor="bg-green-50" borderColor="border-success" icon={icon} name="x" />
    )

    // Card shell carries the pale tint; the disc carries the one-step-darker shade.
    expect(container.querySelector('.card.bg-green-50')).toBeInTheDocument()
    const disc = container.querySelector('.rounded-full')
    expect(disc.className).toContain('bg-green-100')
    expect(disc.className).not.toContain('bg-green-50')
  })

  it('renders the title action next to the name', () => {
    render(
      <StatusHeroCard
        bgColor="bg-green-50"
        borderColor="border-success"
        icon={icon}
        name="frontend"
        titleAction={<button>star</button>}
      />
    )
    expect(screen.getByRole('button', { name: 'star' })).toBeInTheDocument()
  })

  it('renders the Last Updated block only when lastUpdatedAt is provided', () => {
    const { rerender } = render(
      <StatusHeroCard bgColor="bg-green-50" borderColor="border-success" icon={icon} name="x" />
    )
    expect(screen.queryByText('Last Updated')).not.toBeInTheDocument()

    rerender(
      <StatusHeroCard bgColor="bg-green-50" borderColor="border-success" icon={icon} name="x" lastUpdatedAt={new Date('2026-06-30T12:00:00Z')} />
    )
    expect(screen.getByText('Last Updated')).toBeInTheDocument()
  })

  it('renders as a div by default and as a link when href is set', () => {
    const { container, rerender } = render(
      <StatusHeroCard bgColor="bg-gray-50" borderColor="border-gray-400" icon={icon}>
        <span>custom content</span>
      </StatusHeroCard>
    )
    // Custom (children) mode: no structured name heading, children shown.
    expect(screen.getByText('custom content')).toBeInTheDocument()
    expect(container.querySelector('a')).toBeNull()

    rerender(
      <StatusHeroCard bgColor="bg-red-50" borderColor="border-danger" icon={icon} href="/resources?status=Failed">
        <span>custom content</span>
      </StatusHeroCard>
    )
    const link = container.querySelector('a')
    expect(link).toBeInTheDocument()
    expect(link.getAttribute('href')).toBe('/resources?status=Failed')
  })
})
