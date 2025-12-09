// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect } from 'vitest'
import { render, screen, waitFor } from '@testing-library/preact'
import userEvent from '@testing-library/user-event'
import { DashboardPanel, TabButton } from './panel'

/**
 * Helper function to get panel by data-id attribute
 */
export function getPanelById(container, id) {
  return container.querySelector(`[data-id="${id}"]`)
}

describe('DashboardPanel component', () => {
  describe('Rendering', () => {
    it('should render the panel with title', () => {
      const { container } = render(
        <DashboardPanel title="Test Panel" id="test-panel">
          <div>Content</div>
        </DashboardPanel>
      )

      expect(getPanelById(container, 'test-panel')).toBeInTheDocument()
      expect(screen.getByText('Test Panel')).toBeInTheDocument()
    })

    it('should render subtitle when provided', () => {
      render(
        <DashboardPanel title="Test Panel" subtitle={<span>Subtitle text</span>} id="test-panel">
          <div>Content</div>
        </DashboardPanel>
      )

      expect(screen.getByText('Subtitle text')).toBeInTheDocument()
    })

    it('should not render subtitle when not provided', () => {
      render(
        <DashboardPanel title="Test Panel" id="test-panel">
          <div>Content</div>
        </DashboardPanel>
      )

      expect(screen.queryByText('Subtitle text')).not.toBeInTheDocument()
    })

    it('should apply correct data-id attribute', () => {
      const { container } = render(
        <DashboardPanel title="Test Panel" id="my-custom-id">
          <div>Content</div>
        </DashboardPanel>
      )

      expect(getPanelById(container, 'my-custom-id')).toBeInTheDocument()
    })

    it('should render children content when expanded', () => {
      render(
        <DashboardPanel title="Test Panel" id="test-panel">
          <div>Child content here</div>
        </DashboardPanel>
      )

      expect(screen.getByText('Child content here')).toBeInTheDocument()
    })
  })

  describe('Default expansion state', () => {
    it('should be expanded by default', () => {
      render(
        <DashboardPanel title="Test Panel" id="test-panel">
          <div>Content</div>
        </DashboardPanel>
      )

      expect(screen.getByText('Content')).toBeInTheDocument()
      expect(screen.getByRole('button', { name: /test panel/i })).toHaveAttribute('aria-expanded', 'true')
    })

    it('should be collapsed when defaultExpanded is false', () => {
      render(
        <DashboardPanel title="Test Panel" id="test-panel" defaultExpanded={false}>
          <div>Content</div>
        </DashboardPanel>
      )

      expect(screen.queryByText('Content')).not.toBeInTheDocument()
      expect(screen.getByRole('button', { name: /test panel/i })).toHaveAttribute('aria-expanded', 'false')
    })
  })

  describe('Expand/collapse functionality', () => {
    it('should collapse when header is clicked', async () => {
      const user = userEvent.setup()
      render(
        <DashboardPanel title="Test Panel" id="test-panel">
          <div>Content</div>
        </DashboardPanel>
      )

      // Initially expanded
      expect(screen.getByText('Content')).toBeInTheDocument()

      // Click to collapse
      const toggleButton = screen.getByRole('button', { name: /test panel/i })
      await user.click(toggleButton)

      // Content should be hidden
      await waitFor(() => {
        expect(screen.queryByText('Content')).not.toBeInTheDocument()
      })
      expect(toggleButton).toHaveAttribute('aria-expanded', 'false')
    })

    it('should expand when collapsed header is clicked', async () => {
      const user = userEvent.setup()
      render(
        <DashboardPanel title="Test Panel" id="test-panel" defaultExpanded={false}>
          <div>Content</div>
        </DashboardPanel>
      )

      // Initially collapsed
      expect(screen.queryByText('Content')).not.toBeInTheDocument()

      // Click to expand
      const toggleButton = screen.getByRole('button', { name: /test panel/i })
      await user.click(toggleButton)

      // Content should be visible
      await waitFor(() => {
        expect(screen.getByText('Content')).toBeInTheDocument()
      })
      expect(toggleButton).toHaveAttribute('aria-expanded', 'true')
    })

    it('should toggle multiple times', async () => {
      const user = userEvent.setup()
      render(
        <DashboardPanel title="Test Panel" id="test-panel">
          <div>Content</div>
        </DashboardPanel>
      )

      const toggleButton = screen.getByRole('button', { name: /test panel/i })

      // Collapse
      await user.click(toggleButton)
      await waitFor(() => {
        expect(screen.queryByText('Content')).not.toBeInTheDocument()
      })

      // Expand
      await user.click(toggleButton)
      await waitFor(() => {
        expect(screen.getByText('Content')).toBeInTheDocument()
      })

      // Collapse again
      await user.click(toggleButton)
      await waitFor(() => {
        expect(screen.queryByText('Content')).not.toBeInTheDocument()
      })
    })
  })

  describe('Chevron icon rotation', () => {
    it('should have rotate-180 class when expanded', () => {
      const { container } = render(
        <DashboardPanel title="Test Panel" id="test-panel">
          <div>Content</div>
        </DashboardPanel>
      )

      const svg = container.querySelector('svg')
      expect(svg).toHaveClass('rotate-180')
    })

    it('should not have rotate-180 class when collapsed', () => {
      const { container } = render(
        <DashboardPanel title="Test Panel" id="test-panel" defaultExpanded={false}>
          <div>Content</div>
        </DashboardPanel>
      )

      const svg = container.querySelector('svg')
      expect(svg).not.toHaveClass('rotate-180')
    })
  })

  describe('Subtitle with interactive elements', () => {
    it('should allow clicking subtitle without toggling panel', async () => {
      const user = userEvent.setup()
      let subtitleClicked = false

      render(
        <DashboardPanel
          title="Test Panel"
          subtitle={
            <button onClick={(e) => { e.stopPropagation(); subtitleClicked = true }}>
              Click me
            </button>
          }
          id="test-panel"
        >
          <div>Content</div>
        </DashboardPanel>
      )

      // Panel should be expanded
      expect(screen.getByText('Content')).toBeInTheDocument()

      // Click the subtitle button
      await user.click(screen.getByText('Click me'))

      // Subtitle should have been clicked
      expect(subtitleClicked).toBe(true)

      // Panel should still be expanded (stopPropagation prevents toggle)
      expect(screen.getByText('Content')).toBeInTheDocument()
    })
  })
})

describe('TabButton component', () => {
  it('should render with children', () => {
    render(<TabButton active={false} onClick={() => {}}>Tab Label</TabButton>)

    expect(screen.getByText('Tab Label')).toBeInTheDocument()
  })

  it('should apply active styles when active', () => {
    render(<TabButton active={true} onClick={() => {}}>Active Tab</TabButton>)

    const button = screen.getByText('Active Tab')
    expect(button).toHaveClass('border-flux-blue')
    expect(button).toHaveClass('text-flux-blue')
  })

  it('should apply inactive styles when not active', () => {
    render(<TabButton active={false} onClick={() => {}}>Inactive Tab</TabButton>)

    const button = screen.getByText('Inactive Tab')
    expect(button).toHaveClass('border-transparent')
    expect(button).toHaveClass('text-gray-500')
  })

  it('should call onClick when clicked', async () => {
    const user = userEvent.setup()
    let clicked = false

    render(<TabButton active={false} onClick={() => { clicked = true }}>Click Me</TabButton>)

    await user.click(screen.getByText('Click Me'))

    expect(clicked).toBe(true)
  })
})
