// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { render, fireEvent } from '@testing-library/preact'
import { StatusChart } from './StatusChart'

describe('StatusChart', () => {
  // Helper to create mock events
  const createMockEvents = (count, type = 'Normal') => {
    const now = new Date()
    return Array.from({ length: count }, (_, i) => ({
      lastTimestamp: new Date(now.getTime() - i * 60000).toISOString(),
      type,
      message: `Event ${i}`,
      involvedObject: 'Test/resource',
      namespace: 'default'
    }))
  }

  // Helper to create mock resources
  const createMockResources = (count, status = 'Ready') => {
    const now = new Date()
    return Array.from({ length: count }, (_, i) => ({
      lastReconciled: new Date(now.getTime() - i * 60000).toISOString(),
      status,
      name: `resource-${i}`,
      kind: 'Test',
      namespace: 'default',
      message: `Message ${i}`
    }))
  }

  describe('Basic Rendering', () => {
    it('renders without crashing', () => {
      render(<StatusChart items={[]} loading={false} mode="events" />)
    })

    it('renders single loading bar when loading', () => {
      const { container } = render(
        <StatusChart items={[]} loading={true} mode="events" />
      )

      // Should have single loading bar with shimmer
      const loadingBar = container.querySelector('.loading-shimmer')
      expect(loadingBar).toBeTruthy()
      expect(loadingBar.classList.contains('w-full')).toBe(true)
    })

    it('renders gray bar when no items', () => {
      const { container } = render(
        <StatusChart items={[]} loading={false} mode="events" />
      )

      // Should have a gray bar (no data state)
      const grayBar = container.querySelector('.bg-gray-200')
      expect(grayBar).toBeTruthy()
    })

    it('renders status bars when items are provided', () => {
      const events = createMockEvents(5)
      const { container } = render(
        <StatusChart items={events} loading={false} mode="events" />
      )

      const bars = container.querySelectorAll('.relative.group')
      expect(bars.length).toBeGreaterThan(0)
    })
  })

  describe('Status Bar Rendering', () => {
    it('renders one bar per status type', () => {
      const normalEvents = createMockEvents(3, 'Normal')
      const warningEvents = createMockEvents(2, 'Warning')
      const mixedEvents = [...normalEvents, ...warningEvents]

      const { container } = render(
        <StatusChart items={mixedEvents} loading={false} mode="events" />
      )

      // Should have 2 bars (one for Normal, one for Warning)
      const bars = container.querySelectorAll('.relative.group')
      expect(bars).toHaveLength(2)
    })

    it('renders bars sorted by percentage (largest first)', () => {
      const normalEvents = createMockEvents(7, 'Normal')
      const warningEvents = createMockEvents(3, 'Warning')
      const mixedEvents = [...normalEvents, ...warningEvents]

      const { container } = render(
        <StatusChart items={mixedEvents} loading={false} mode="events" />
      )

      const bars = container.querySelectorAll('.relative.group')
      expect(bars).toHaveLength(2)

      // First bar should be larger (70% Normal)
      const firstBarStyle = bars[0].getAttribute('style')
      const secondBarStyle = bars[1].getAttribute('style')

      expect(firstBarStyle).toContain('70')
      expect(secondBarStyle).toContain('30')
    })

    it('renders single bar for single status type', () => {
      const resources = createMockResources(10, 'Ready')
      const { container } = render(
        <StatusChart items={resources} loading={false} mode="resources" />
      )

      // Should have 1 bar (all Ready)
      const bars = container.querySelectorAll('.relative.group')
      expect(bars).toHaveLength(1)
    })

    it('renders multiple bars for multiple status types', () => {
      const now = new Date()
      const resources = [
        { lastReconciled: now.toISOString(), status: 'Ready', name: 'r1', kind: 'Test', namespace: 'default', message: 'Ready' },
        { lastReconciled: now.toISOString(), status: 'Failed', name: 'r2', kind: 'Test', namespace: 'default', message: 'Failed' },
        { lastReconciled: now.toISOString(), status: 'Progressing', name: 'r3', kind: 'Test', namespace: 'default', message: 'Progressing' },
      ]

      const { container } = render(
        <StatusChart items={resources} loading={false} mode="resources" />
      )

      // Should have 3 bars (Ready, Failed, Progressing)
      const bars = container.querySelectorAll('.relative.group')
      expect(bars).toHaveLength(3)
    })
  })

  describe('Events Mode - Color Coding', () => {
    it('renders green bars for Normal events', () => {
      const events = createMockEvents(5, 'Normal')
      const { container } = render(
        <StatusChart items={events} loading={false} mode="events" />
      )

      // Check for green color class
      const greenBars = container.querySelectorAll('.bg-green-500')
      expect(greenBars.length).toBeGreaterThan(0)
    })

    it('renders red bars for Warning events', () => {
      const events = createMockEvents(5, 'Warning')
      const { container } = render(
        <StatusChart items={events} loading={false} mode="events" />
      )

      // Check for red color class
      const redBars = container.querySelectorAll('.bg-red-500')
      expect(redBars.length).toBeGreaterThan(0)
    })

    it('renders separate bars for Normal and Warning events', () => {
      const normalEvents = createMockEvents(3, 'Normal')
      const warningEvents = createMockEvents(2, 'Warning')
      const mixedEvents = [...normalEvents, ...warningEvents]

      const { container } = render(
        <StatusChart items={mixedEvents} loading={false} mode="events" />
      )

      // Should have both green and red bars
      const greenBars = container.querySelectorAll('.bg-green-500')
      const redBars = container.querySelectorAll('.bg-red-500')

      expect(greenBars.length).toBeGreaterThan(0)
      expect(redBars.length).toBeGreaterThan(0)
    })
  })

  describe('Resources Mode - Color Coding', () => {
    it('renders green bars for Ready status', () => {
      const resources = createMockResources(5, 'Ready')
      const { container } = render(
        <StatusChart items={resources} loading={false} mode="resources" />
      )

      // Check for green color class
      const greenBars = container.querySelectorAll('.bg-green-500')
      expect(greenBars.length).toBeGreaterThan(0)
    })

    it('renders red bars for Failed status', () => {
      const resources = createMockResources(5, 'Failed')
      const { container } = render(
        <StatusChart items={resources} loading={false} mode="resources" />
      )

      // Check for red color class
      const redBars = container.querySelectorAll('.bg-red-500')
      expect(redBars.length).toBeGreaterThan(0)
    })

    it('renders blue bars for Progressing status', () => {
      const resources = createMockResources(5, 'Progressing')
      const { container } = render(
        <StatusChart items={resources} loading={false} mode="resources" />
      )

      const blueBars = container.querySelectorAll('.bg-blue-500')
      expect(blueBars.length).toBeGreaterThan(0)
    })

    it('renders yellow bars for Suspended status', () => {
      const resources = createMockResources(5, 'Suspended')
      const { container } = render(
        <StatusChart items={resources} loading={false} mode="resources" />
      )

      const yellowBars = container.querySelectorAll('.bg-yellow-500')
      expect(yellowBars.length).toBeGreaterThan(0)
    })

    it('renders dark grey bars for Unknown status', () => {
      const resources = createMockResources(5, 'Unknown')
      const { container } = render(
        <StatusChart items={resources} loading={false} mode="resources" />
      )

      const greyBars = container.querySelectorAll('.bg-gray-600')
      expect(greyBars.length).toBeGreaterThan(0)
    })

    it('renders separate bars for each status type', () => {
      const now = new Date()
      const resources = [
        { lastReconciled: now.toISOString(), status: 'Ready', name: 'r1', kind: 'Test', namespace: 'default', message: 'Ready' },
        { lastReconciled: now.toISOString(), status: 'Failed', name: 'r2', kind: 'Test', namespace: 'default', message: 'Failed' },
        { lastReconciled: now.toISOString(), status: 'Progressing', name: 'r3', kind: 'Test', namespace: 'default', message: 'Progressing' },
        { lastReconciled: now.toISOString(), status: 'Suspended', name: 'r4', kind: 'Test', namespace: 'default', message: 'Suspended' },
      ]

      const { container } = render(
        <StatusChart items={resources} loading={false} mode="resources" />
      )

      // Should have 4 separate bars with different colors
      const greenBars = container.querySelectorAll('.bg-green-500')
      const redBars = container.querySelectorAll('.bg-red-500')
      const blueBars = container.querySelectorAll('.bg-blue-500')
      const yellowBars = container.querySelectorAll('.bg-yellow-500')

      expect(greenBars.length).toBeGreaterThan(0)
      expect(redBars.length).toBeGreaterThan(0)
      expect(blueBars.length).toBeGreaterThan(0)
      expect(yellowBars.length).toBeGreaterThan(0)
    })
  })

  describe('Tooltips', () => {
    it('does not show tooltip when loading', () => {
      const { container } = render(
        <StatusChart items={[]} loading={true} mode="events" />
      )

      // Tooltips should not be visible
      const tooltips = container.querySelectorAll('.absolute.bottom-full')
      expect(tooltips).toHaveLength(0)
    })

    it('does not show tooltip initially when no items', () => {
      const { container } = render(
        <StatusChart items={[]} loading={false} mode="events" />
      )

      // Tooltips should not be visible initially (only on hover)
      const tooltips = container.querySelectorAll('.absolute.bottom-full')
      expect(tooltips).toHaveLength(0)
    })

    it('shows tooltip on hover with status, count, and percentage', async () => {
      const events = createMockEvents(5, 'Normal')
      const { container } = render(
        <StatusChart items={events} loading={false} mode="events" />
      )

      const bars = container.querySelectorAll('.relative.group')
      expect(bars.length).toBeGreaterThan(0)

      // Simulate hover by triggering mouseenter
      const firstBar = bars[0]
      fireEvent.mouseEnter(firstBar)

      // Wait for tooltip to appear
      await vi.waitFor(() => {
        const tooltips = container.querySelectorAll('.absolute.bottom-full')
        expect(tooltips.length).toBeGreaterThan(0)

        // Check tooltip content
        const tooltip = tooltips[0]
        expect(tooltip.textContent).toContain('Normal')
        expect(tooltip.textContent).toContain('Count: 5')
        expect(tooltip.textContent).toContain('Percentage: 100.0%')
      })
    })

    it('shows tooltip with correct percentage for mixed events', async () => {
      const normalEvents = createMockEvents(7, 'Normal')
      const warningEvents = createMockEvents(3, 'Warning')
      const mixedEvents = [...normalEvents, ...warningEvents]

      const { container } = render(
        <StatusChart items={mixedEvents} loading={false} mode="events" />
      )

      const bars = container.querySelectorAll('.relative.group')
      expect(bars).toHaveLength(2)

      // Hover over first bar (should be Normal with 70%)
      fireEvent.mouseEnter(bars[0])

      await vi.waitFor(() => {
        const tooltip = container.querySelector('.absolute.bottom-full')
        expect(tooltip).toBeTruthy()
        expect(tooltip.textContent).toContain('Normal')
        expect(tooltip.textContent).toContain('Count: 7')
        expect(tooltip.textContent).toContain('70.0%')
      })
    })

    it('hides tooltip on mouse leave', async () => {
      const events = createMockEvents(5, 'Normal')
      const { container } = render(
        <StatusChart items={events} loading={false} mode="events" />
      )

      const bars = container.querySelectorAll('.relative.group')
      const firstBar = bars[0]

      // Hover
      fireEvent.mouseEnter(firstBar)

      // Wait for tooltip
      await vi.waitFor(() => {
        const tooltips = container.querySelectorAll('.absolute.bottom-full')
        expect(tooltips.length).toBeGreaterThan(0)
      })

      // Leave
      fireEvent.mouseLeave(firstBar)

      // Tooltip should be gone
      await vi.waitFor(() => {
        const tooltips = container.querySelectorAll('.absolute.bottom-full')
        expect(tooltips).toHaveLength(0)
      })
    })
  })

  describe('Animation', () => {
    it('applies horizontal fill animation to bars', () => {
      const events = createMockEvents(5)
      const { container } = render(
        <StatusChart items={events} loading={false} mode="events" />
      )

      // Check for animation style
      const animatedSegments = container.querySelectorAll('[style*="animation"]')
      expect(animatedSegments.length).toBeGreaterThan(0)
    })

    it('does not animate loading bar', () => {
      const { container } = render(
        <StatusChart items={[]} loading={true} mode="events" />
      )

      // Loading bar should not have fillRight animation
      const loadingBar = container.querySelector('.loading-shimmer')
      expect(loadingBar).toBeTruthy()
      const style = loadingBar.getAttribute('style')
      if (style) {
        expect(style).not.toContain('fillRight')
      }
      // If no style attribute, that's also fine (no animation)
    })

    it('renders gray background for all bars', () => {
      const events = createMockEvents(5)
      const { container } = render(
        <StatusChart items={events} loading={false} mode="events" />
      )

      // All bars should have gray background
      const graySegments = container.querySelectorAll('.bg-gray-200')
      expect(graySegments.length).toBeGreaterThan(0)
    })
  })

  describe('Percentage Calculation', () => {
    it('calculates correct percentages for events', () => {
      const normalEvents = createMockEvents(6, 'Normal')
      const warningEvents = createMockEvents(4, 'Warning')
      const mixedEvents = [...normalEvents, ...warningEvents]

      const { container } = render(
        <StatusChart items={mixedEvents} loading={false} mode="events" />
      )

      const bars = container.querySelectorAll('.relative.group')
      expect(bars).toHaveLength(2)

      // First bar should be 60% (6 out of 10)
      const firstBarStyle = bars[0].getAttribute('style')
      expect(firstBarStyle).toContain('60')

      // Second bar should be 40% (4 out of 10)
      const secondBarStyle = bars[1].getAttribute('style')
      expect(secondBarStyle).toContain('40')
    })

    it('calculates correct percentages for resources', () => {
      const now = new Date()
      const resources = [
        { lastReconciled: now.toISOString(), status: 'Ready', name: 'r1', kind: 'Test', namespace: 'default', message: 'Ready' },
        { lastReconciled: now.toISOString(), status: 'Ready', name: 'r2', kind: 'Test', namespace: 'default', message: 'Ready' },
        { lastReconciled: now.toISOString(), status: 'Ready', name: 'r3', kind: 'Test', namespace: 'default', message: 'Ready' },
        { lastReconciled: now.toISOString(), status: 'Failed', name: 'r4', kind: 'Test', namespace: 'default', message: 'Failed' },
      ]

      const { container } = render(
        <StatusChart items={resources} loading={false} mode="resources" />
      )

      const bars = container.querySelectorAll('.relative.group')
      expect(bars).toHaveLength(2)

      // First bar (Ready) should be 75% (3 out of 4)
      const firstBarStyle = bars[0].getAttribute('style')
      expect(firstBarStyle).toContain('75')

      // Second bar (Failed) should be 25% (1 out of 4)
      const secondBarStyle = bars[1].getAttribute('style')
      expect(secondBarStyle).toContain('25')
    })
  })

  describe('Layout', () => {
    it('renders horizontal chart with correct height', () => {
      const events = createMockEvents(5)
      const { container } = render(
        <StatusChart items={events} loading={false} mode="events" />
      )

      // Check that container has correct height (32px)
      const chartContainer = container.querySelector('[style*="height"]')
      expect(chartContainer).toBeTruthy()
      expect(chartContainer.style.height).toBe('32px')
    })

    it('renders bars in horizontal flex layout', () => {
      const events = createMockEvents(5)
      const { container } = render(
        <StatusChart items={events} loading={false} mode="events" />
      )

      // Container should use flex layout
      const chartContainer = container.querySelector('.flex')
      expect(chartContainer).toBeTruthy()
    })

    it('renders bars with proportional widths', () => {
      const normalEvents = createMockEvents(8, 'Normal')
      const warningEvents = createMockEvents(2, 'Warning')
      const mixedEvents = [...normalEvents, ...warningEvents]

      const { container } = render(
        <StatusChart items={mixedEvents} loading={false} mode="events" />
      )

      const bars = container.querySelectorAll('.relative.group')
      expect(bars).toHaveLength(2)

      // First bar should be wider (80%)
      const firstBarStyle = bars[0].getAttribute('style')
      expect(firstBarStyle).toContain('80')

      // Second bar should be narrower (20%)
      const secondBarStyle = bars[1].getAttribute('style')
      expect(secondBarStyle).toContain('20')
    })
  })

  describe('Header with Totals and Stats', () => {
    it('renders header with total count for events', () => {
      const events = createMockEvents(10)
      const { container } = render(
        <StatusChart items={events} loading={false} mode="events" />
      )

      // Should show total events count
      expect(container.textContent).toContain('Reconcile events:')
      expect(container.textContent).toContain('10')
    })

    it('renders header with total count for resources', () => {
      const resources = createMockResources(15)
      const { container } = render(
        <StatusChart items={resources} loading={false} mode="resources" />
      )

      // Should show total reconciliations count
      expect(container.textContent).toContain('Reconcilers:')
      expect(container.textContent).toContain('15')
    })

    it('shows loading state in header', () => {
      const { container } = render(
        <StatusChart items={[]} loading={true} mode="events" />
      )

      // Should show loading text
      expect(container.textContent).toContain('Loading...')
    })

    it('displays stats for events mode when multiple types exist', () => {
      const normalEvents = createMockEvents(8, 'Normal')
      const warningEvents = createMockEvents(3, 'Warning')
      const mixedEvents = [...normalEvents, ...warningEvents]

      const { container } = render(
        <StatusChart items={mixedEvents} loading={false} mode="events" />
      )

      // Should show Info and Warning counts when both types exist
      expect(container.textContent).toContain('Info:')
      expect(container.textContent).toContain('8')
      expect(container.textContent).toContain('Warning:')
      expect(container.textContent).toContain('3')
    })

    it('hides stats for events mode when single type', () => {
      const normalEvents = createMockEvents(10, 'Normal')

      const { container } = render(
        <StatusChart items={normalEvents} loading={false} mode="events" />
      )

      // Should NOT show stats when only one type exists
      expect(container.textContent).not.toContain('Info:')
    })

    it('displays stats for resources mode when multiple statuses exist', () => {
      const now = new Date()
      const resources = [
        { lastReconciled: now.toISOString(), status: 'Ready', name: 'r1', kind: 'Test', namespace: 'default', message: 'Ready' },
        { lastReconciled: now.toISOString(), status: 'Ready', name: 'r2', kind: 'Test', namespace: 'default', message: 'Ready' },
        { lastReconciled: now.toISOString(), status: 'Failed', name: 'r3', kind: 'Test', namespace: 'default', message: 'Failed' },
        { lastReconciled: now.toISOString(), status: 'Progressing', name: 'r4', kind: 'Test', namespace: 'default', message: 'Progressing' },
        { lastReconciled: now.toISOString(), status: 'Suspended', name: 'r5', kind: 'Test', namespace: 'default', message: 'Suspended' },
      ]

      const { container } = render(
        <StatusChart items={resources} loading={false} mode="resources" />
      )

      // Should show status counts when multiple statuses exist
      expect(container.textContent).toContain('Ready:')
      expect(container.textContent).toContain('2')
      expect(container.textContent).toContain('Failed:')
      expect(container.textContent).toContain('Progressing:')
      expect(container.textContent).toContain('Suspended:')
    })

    it('hides stats for resources mode when single status', () => {
      const resources = createMockResources(10, 'Ready')

      const { container } = render(
        <StatusChart items={resources} loading={false} mode="resources" />
      )

      // Should NOT show stats when only one status exists
      expect(container.textContent).not.toContain('Ready:')
    })

    it('shows zero count when no items', () => {
      const { container } = render(
        <StatusChart items={[]} loading={false} mode="events" />
      )

      // Should show 0 for total
      expect(container.textContent).toContain('Reconcile events:')
      expect(container.textContent).toContain('0')
    })
  })
})
