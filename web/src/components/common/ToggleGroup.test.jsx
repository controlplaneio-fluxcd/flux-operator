// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { render, screen, fireEvent } from '@testing-library/preact'
import { describe, it, expect, vi } from 'vitest'
import { ToggleGroup } from './ToggleGroup'

const OPTIONS = [
  { value: 'all', label: 'All', testid: 'opt-all' },
  { value: 'flux', label: 'Flux', testid: 'opt-flux' },
  { value: 'other', label: 'Other', testid: 'opt-other' }
]

describe('ToggleGroup', () => {
  it('renders one button per option', () => {
    render(<ToggleGroup ariaLabel="Category" options={OPTIONS} value="all" onChange={() => {}} testid="cat" />)
    expect(screen.getByTestId('opt-all')).toBeInTheDocument()
    expect(screen.getByTestId('opt-flux')).toBeInTheDocument()
    expect(screen.getByTestId('opt-other')).toBeInTheDocument()
  })

  it('marks the selected option as pressed', () => {
    render(<ToggleGroup ariaLabel="Category" options={OPTIONS} value="flux" onChange={() => {}} />)
    expect(screen.getByTestId('opt-flux')).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByTestId('opt-all')).toHaveAttribute('aria-pressed', 'false')
  })

  it('calls onChange with the clicked value', () => {
    const onChange = vi.fn()
    render(<ToggleGroup ariaLabel="Category" options={OPTIONS} value="all" onChange={onChange} />)
    fireEvent.click(screen.getByTestId('opt-other'))
    expect(onChange).toHaveBeenCalledWith('other')
  })

  it('renders a visible label when provided', () => {
    render(<ToggleGroup label="Category" options={OPTIONS} value="all" onChange={() => {}} testid="cat" />)
    expect(screen.getByText('Category')).toBeInTheDocument()
    expect(screen.getByTestId('cat')).toHaveAttribute('aria-label', 'Category')
  })

  it('uses ariaLabel for the group when there is no visible label', () => {
    render(<ToggleGroup ariaLabel="Filter" options={OPTIONS} value="all" onChange={() => {}} testid="cat" />)
    expect(screen.getByTestId('cat')).toHaveAttribute('aria-label', 'Filter')
  })
})
