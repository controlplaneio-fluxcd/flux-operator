// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/preact'
import { NotFoundPage } from './NotFoundPage'

// Mock preact-iso
const mockRoute = vi.fn()
vi.mock('preact-iso', () => ({
  useLocation: () => ({
    route: mockRoute
  })
}))

describe('NotFoundPage component', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('should render the 404 page with correct elements', () => {
    render(<NotFoundPage />)

    // Check for heading
    expect(screen.getByText('Page Not Found')).toBeInTheDocument()

    // Check for description
    expect(screen.getByText("The page you're looking for doesn't exist or has been moved.")).toBeInTheDocument()

    // Check for Go Home button
    expect(screen.getByRole('button', { name: /go to home/i })).toBeInTheDocument()
  })

  it('should have the correct test id', () => {
    render(<NotFoundPage />)

    expect(screen.getByTestId('not-found-page')).toBeInTheDocument()
  })

  it('should navigate to home when Go Home button is clicked', () => {
    render(<NotFoundPage />)

    const goHomeButton = screen.getByRole('button', { name: /go to home/i })
    fireEvent.click(goHomeButton)

    expect(mockRoute).toHaveBeenCalledWith('/')
  })

  it('should render with proper styling classes', () => {
    render(<NotFoundPage />)

    const main = screen.getByTestId('not-found-page')
    expect(main).toHaveClass('max-w-7xl')
    expect(main).toHaveClass('mx-auto')
  })

  it('should render the 404 icon', () => {
    render(<NotFoundPage />)

    // The icon container should be present
    const iconContainer = document.querySelector('.w-20.h-20.rounded-full')
    expect(iconContainer).toBeInTheDocument()
  })

  it('should render the home icon in the button', () => {
    render(<NotFoundPage />)

    const button = screen.getByRole('button', { name: /go to home/i })
    const svg = button.querySelector('svg')
    expect(svg).toBeInTheDocument()
  })
})
