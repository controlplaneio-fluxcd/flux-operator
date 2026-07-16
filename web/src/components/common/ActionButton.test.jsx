// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/preact'
import userEvent from '@testing-library/user-event'
import { ActionButton } from './ActionButton'

describe('ActionButton component', () => {
  it('renders an enabled button with a title attribute', () => {
    render(
      <ActionButton title="Do something" data-testid="action-button">
        Click me
      </ActionButton>
    )

    const button = screen.getByTestId('action-button')
    expect(button).not.toBeDisabled()
    expect(button).toHaveAttribute('title', 'Do something')
  })

  it('wraps disabled buttons in a span with the tooltip title', () => {
    render(
      <ActionButton title="Not allowed" disabled data-testid="action-button">
        Click me
      </ActionButton>
    )

    const button = screen.getByTestId('action-button')
    expect(button).toBeDisabled()
    expect(button.parentElement).toHaveAttribute('title', 'Not allowed')
    expect(button).not.toHaveAttribute('title')
  })

  it('does not fire onClick when disabled', async () => {
    const user = userEvent.setup()
    const onClick = vi.fn()

    render(
      <ActionButton title="Not allowed" disabled onClick={onClick} data-testid="action-button">
        Click me
      </ActionButton>
    )

    await user.click(screen.getByTestId('action-button'))
    expect(onClick).not.toHaveBeenCalled()
  })
})
