// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { render, screen } from '@testing-library/preact'
import { describe, it, expect } from 'vitest'
import { StatusPill } from './StatusPill'

describe('StatusPill', () => {
  it('renders the computing placeholder with the given testid when status is absent', () => {
    render(<StatusPill computingTestid="x-computing" />)
    expect(screen.getByTestId('x-computing')).toBeInTheDocument()
  })

  it('renders the formatted status badge when a status is provided', () => {
    // formatWorkloadStatus maps kstatus "Current" to "Ready".
    render(<StatusPill status="Current" computingTestid="x-computing" />)
    expect(screen.queryByTestId('x-computing')).toBeNull()
    expect(screen.getByText('Ready')).toBeInTheDocument()
  })
})
