// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/preact'
import { useState } from 'preact/hooks'
import { TabbedPanel, Field } from './detailPanel'

const tabs = [
  { id: 'overview', label: 'Overview' },
  { id: 'spec', label: 'Specification' },
  { id: 'status', label: 'Status' },
]

describe('TabbedPanel', () => {
  it('renders a button for every tab and the active content', () => {
    render(
      <TabbedPanel tabs={tabs} active="overview" onSelect={() => {}}>
        <div>overview content</div>
      </TabbedPanel>
    )

    tabs.forEach(t => {
      expect(screen.getByRole('button', { name: t.label })).toBeInTheDocument()
    })
    expect(screen.getByText('overview content')).toBeInTheDocument()
  })

  it('calls onSelect with the tab id when a tab is clicked', () => {
    const onSelect = vi.fn()
    render(
      <TabbedPanel tabs={tabs} active="overview" onSelect={onSelect}>
        <div>content</div>
      </TabbedPanel>
    )

    fireEvent.click(screen.getByRole('button', { name: 'Specification' }))
    expect(onSelect).toHaveBeenCalledWith('spec')
  })

  it('switches the active tab styling and content when a tab is selected', () => {
    // A small controlled wrapper exercises the active/onSelect contract end to end.
    function Harness() {
      const [active, setActive] = useState('overview')
      return (
        <TabbedPanel tabs={tabs} active={active} onSelect={setActive}>
          <div>{active} panel</div>
        </TabbedPanel>
      )
    }

    render(<Harness />)
    expect(screen.getByText('overview panel')).toBeInTheDocument()

    // The active tab merges into the panel by sharing its background.
    expect(screen.getByRole('button', { name: 'Overview' })).toHaveClass('bg-gray-100')
    expect(screen.getByRole('button', { name: 'Status' })).not.toHaveClass('bg-gray-100')

    fireEvent.click(screen.getByRole('button', { name: 'Status' }))

    expect(screen.getByText('status panel')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Status' })).toHaveClass('bg-gray-100')
    expect(screen.getByRole('button', { name: 'Overview' })).not.toHaveClass('bg-gray-100')
  })
})

describe('Field', () => {
  it('renders the label and value', () => {
    render(<Field label="Namespace">flux-system</Field>)
    expect(screen.getByText('Namespace')).toBeInTheDocument()
    expect(screen.getByText('flux-system')).toBeInTheDocument()
  })

  it('renders nothing when the value is an empty string', () => {
    const { container } = render(<Field label="Source">{''}</Field>)
    expect(container.firstChild).toBeNull()
  })

  it('renders nothing when the value is null', () => {
    const { container } = render(<Field label="Source">{null}</Field>)
    expect(container.firstChild).toBeNull()
  })

  it('renders nothing when the value is undefined', () => {
    const { container } = render(<Field label="Source">{undefined}</Field>)
    expect(container.firstChild).toBeNull()
  })
})
