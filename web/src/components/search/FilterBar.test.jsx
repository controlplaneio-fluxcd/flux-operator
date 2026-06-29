// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/preact'
import { FilterBar } from './FilterBar'

describe('FilterBar', () => {
  it('renders the result count and label', () => {
    render(
      <FilterBar count={5} label="resources">
        <div data-testid="form">form</div>
      </FilterBar>
    )
    expect(screen.getByText('5 resources')).toBeInTheDocument()
  })

  it('shows a loader instead of the count while loading', () => {
    const { container } = render(
      <FilterBar count={0} label="events" loading>
        <div data-testid="form">form</div>
      </FilterBar>
    )
    expect(screen.getByText('Loading…')).toBeInTheDocument()
    expect(screen.queryByText('0 events')).not.toBeInTheDocument()
    expect(container.querySelector('.animate-spin')).toBeTruthy()
  })

  it('collapses the form on mobile until the Filters toggle is pressed', () => {
    render(
      <FilterBar count={3} label="events">
        <div data-testid="form">form</div>
      </FilterBar>
    )

    // The form container is hidden on mobile (still sm:block for desktop).
    const formContainer = screen.getByTestId('form').parentElement
    expect(formContainer).toHaveClass('hidden')
    expect(formContainer).toHaveClass('sm:block')
    expect(formContainer).not.toHaveClass('block')

    // Pressing the toggle expands the form.
    fireEvent.click(screen.getByRole('button', { name: /toggle filters/i }))
    expect(formContainer).toHaveClass('block')
    expect(formContainer).not.toHaveClass('hidden')

    // Pressing again collapses it back.
    fireEvent.click(screen.getByRole('button', { name: /toggle filters/i }))
    expect(formContainer).toHaveClass('hidden')
    expect(formContainer).not.toHaveClass('block')
  })

  it('renders the status chart node alongside the form', () => {
    render(
      <FilterBar count={1} label="resource" statusChart={<div data-testid="chart">chart</div>}>
        <div data-testid="form">form</div>
      </FilterBar>
    )
    expect(screen.getByTestId('chart')).toBeInTheDocument()
  })
})
