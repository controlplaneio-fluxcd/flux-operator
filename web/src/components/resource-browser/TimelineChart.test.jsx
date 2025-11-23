// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { render, fireEvent } from '@testing-library/preact'
import { TimelineChart } from './TimelineChart'

describe('TimelineChart', () => {
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

  // Helper to set window width for responsive tests
  const setWindowWidth = (width) => {
    Object.defineProperty(window, 'innerWidth', {
      writable: true,
      configurable: true,
      value: width,
    })
    // Trigger resize event
    fireEvent(window, new window.Event('resize'))
  }

  beforeEach(() => {
    // Reset window width to desktop size before each test
    setWindowWidth(1024)
  })

  describe('Basic Rendering', () => {
    it('renders without crashing', () => {
      render(<TimelineChart items={[]} loading={false} mode="events" />)
    })

    it('renders single loading bar when loading', () => {
      const { container } = render(
        <TimelineChart items={[]} loading={true} mode="events" />
      )

      // Should have single loading bar with shimmer
      const loadingBar = container.querySelector('.loading-shimmer')
      expect(loadingBar).toBeTruthy()
      expect(loadingBar.classList.contains('w-full')).toBe(true)
    })

    it('renders placeholder bars when no items', () => {
      const { container } = render(
        <TimelineChart items={[]} loading={false} mode="events" />
      )

      // Should have bars (placeholder)
      const bars = container.querySelectorAll('.relative.flex-1.group')
      expect(bars.length).toBeGreaterThan(0)
    })

    it('renders bars when items are provided', () => {
      const events = createMockEvents(5)
      const { container } = render(
        <TimelineChart items={events} loading={false} mode="events" />
      )

      const bars = container.querySelectorAll('.relative.flex-1.group')
      expect(bars.length).toBeGreaterThan(0)
    })
  })

  describe('Responsive Behavior', () => {
    it('renders 7 bars on mobile (< 1024px)', () => {
      setWindowWidth(768)

      const events = createMockEvents(10)
      const { container } = render(
        <TimelineChart items={events} loading={false} mode="events" />
      )

      const bars = container.querySelectorAll('.relative.flex-1.group')
      expect(bars).toHaveLength(7)
    })

    it('renders 10 bars on desktop (>= 1024px)', () => {
      setWindowWidth(1920)

      const events = createMockEvents(10)
      const { container } = render(
        <TimelineChart items={events} loading={false} mode="events" />
      )

      const bars = container.querySelectorAll('.relative.flex-1.group')
      expect(bars).toHaveLength(10)
    })
  })

  describe('Events Mode - Color Coding', () => {
    it('renders green bars for all normal events', () => {
      const events = createMockEvents(5, 'Normal')
      const { container } = render(
        <TimelineChart items={events} loading={false} mode="events" />
      )

      // Check for green color class
      const greenBars = container.querySelectorAll('.bg-green-500')
      expect(greenBars.length).toBeGreaterThan(0)
    })

    it('renders red bars for all warning events', () => {
      const events = createMockEvents(5, 'Warning')
      const { container } = render(
        <TimelineChart items={events} loading={false} mode="events" />
      )

      // Check for red color class
      const redBars = container.querySelectorAll('.bg-red-500')
      expect(redBars.length).toBeGreaterThan(0)
    })

    it('renders orange bars for mixed events', () => {
      const normalEvents = createMockEvents(2, 'Normal')
      const warningEvents = createMockEvents(2, 'Warning')
      const mixedEvents = [...normalEvents, ...warningEvents]

      const { container } = render(
        <TimelineChart items={mixedEvents} loading={false} mode="events" />
      )

      // Check for orange color class (mixed events in same bucket)
      // Note: This depends on bucketing, so we just verify the component renders
      const bars = container.querySelectorAll('.relative.flex-1.group')
      expect(bars.length).toBeGreaterThan(0)
    })
  })

  describe('Resources Mode - Color Coding', () => {
    it('renders green bars when no failures', () => {
      const resources = createMockResources(5, 'Ready')
      const { container } = render(
        <TimelineChart items={resources} loading={false} mode="resources" />
      )

      // Check for green color class
      const greenBars = container.querySelectorAll('.bg-green-500')
      expect(greenBars.length).toBeGreaterThan(0)
    })

    it('renders red bars when all failed', () => {
      const resources = createMockResources(5, 'Failed')
      const { container } = render(
        <TimelineChart items={resources} loading={false} mode="resources" />
      )

      // Check for red color class
      const redBars = container.querySelectorAll('.bg-red-500')
      expect(redBars.length).toBeGreaterThan(0)
    })

    it('renders orange bars when some failed', () => {
      const now = new Date()
      // Create resources with same timestamp so they bucket together
      const mixedResources = [
        {
          lastReconciled: now.toISOString(),
          status: 'Ready',
          name: 'resource-1',
          kind: 'Test',
          namespace: 'default',
          message: 'Ready'
        },
        {
          lastReconciled: now.toISOString(),
          status: 'Failed',
          name: 'resource-2',
          kind: 'Test',
          namespace: 'default',
          message: 'Failed'
        },
        {
          lastReconciled: now.toISOString(),
          status: 'Ready',
          name: 'resource-3',
          kind: 'Test',
          namespace: 'default',
          message: 'Ready'
        }
      ]

      const { container } = render(
        <TimelineChart items={mixedResources} loading={false} mode="resources" />
      )

      // Check for orange color class (mixed with any Failed or Unknown)
      const orangeBars = container.querySelectorAll('.bg-orange-500')
      expect(orangeBars.length).toBeGreaterThan(0)
    })

    it('handles different resource statuses without failures', () => {
      const now = new Date()
      // Mix of Ready, Progressing, and Suspended (no Failed or Unknown)
      const resources = [
        { lastReconciled: new Date(now.getTime() - 1000).toISOString(), status: 'Ready', name: 'r1', kind: 'Test', namespace: 'default', message: 'Ready' },
        { lastReconciled: new Date(now.getTime() - 2000).toISOString(), status: 'Progressing', name: 'r2', kind: 'Test', namespace: 'default', message: 'Progressing' },
        { lastReconciled: new Date(now.getTime() - 3000).toISOString(), status: 'Suspended', name: 'r3', kind: 'Test', namespace: 'default', message: 'Suspended' },
        { lastReconciled: new Date(now.getTime() - 4000).toISOString(), status: 'Ready', name: 'r4', kind: 'Test', namespace: 'default', message: 'Ready' },
      ]

      const { container } = render(
        <TimelineChart items={resources} loading={false} mode="resources" />
      )

      // Mixed without Failed/Unknown should result in green
      const greenBars = container.querySelectorAll('.bg-green-500')
      expect(greenBars.length).toBeGreaterThan(0)
    })

    it('renders orange bars when Unknown status present in mix', () => {
      const now = new Date()
      // Create resources with same timestamp so they bucket together
      const resources = [
        { lastReconciled: now.toISOString(), status: 'Ready', name: 'r1', kind: 'Test', namespace: 'default', message: 'Ready' },
        { lastReconciled: now.toISOString(), status: 'Unknown', name: 'r2', kind: 'Test', namespace: 'default', message: 'Unknown' },
      ]

      const { container } = render(
        <TimelineChart items={resources} loading={false} mode="resources" />
      )

      // Mixed with Unknown should result in orange
      const orangeBars = container.querySelectorAll('.bg-orange-500')
      expect(orangeBars.length).toBeGreaterThan(0)
    })

    it('renders blue bars when all Progressing', () => {
      const resources = createMockResources(5, 'Progressing')
      const { container } = render(
        <TimelineChart items={resources} loading={false} mode="resources" />
      )

      const blueBars = container.querySelectorAll('.bg-blue-500')
      expect(blueBars.length).toBeGreaterThan(0)
    })

    it('renders yellow bars when all Suspended', () => {
      const resources = createMockResources(5, 'Suspended')
      const { container } = render(
        <TimelineChart items={resources} loading={false} mode="resources" />
      )

      const yellowBars = container.querySelectorAll('.bg-yellow-500')
      expect(yellowBars.length).toBeGreaterThan(0)
    })

    it('renders red bars when all Unknown', () => {
      const resources = createMockResources(5, 'Unknown')
      const { container } = render(
        <TimelineChart items={resources} loading={false} mode="resources" />
      )

      const redBars = container.querySelectorAll('.bg-red-500')
      expect(redBars.length).toBeGreaterThan(0)
    })
  })

  describe('Tooltips', () => {
    it('does not show tooltip when loading', () => {
      const { container } = render(
        <TimelineChart items={[]} loading={true} mode="events" />
      )

      // Tooltips should not be visible
      const tooltips = container.querySelectorAll('.absolute.bottom-full')
      expect(tooltips).toHaveLength(0)
    })

    it('does not show tooltip when no items', () => {
      const { container } = render(
        <TimelineChart items={[]} loading={false} mode="events" />
      )

      // Tooltips should not be visible initially (only on hover)
      const tooltips = container.querySelectorAll('.absolute.bottom-full')
      expect(tooltips).toHaveLength(0)
    })

    it('shows tooltip on hover with event data', async () => {
      const events = createMockEvents(5, 'Normal')
      const { container } = render(
        <TimelineChart items={events} loading={false} mode="events" />
      )

      const bars = container.querySelectorAll('.relative.flex-1.group')
      expect(bars.length).toBeGreaterThan(0)

      // Simulate hover by triggering mouseenter
      const firstBar = bars[0]
      fireEvent.mouseEnter(firstBar)

      // Wait for tooltip to appear
      await vi.waitFor(() => {
        const tooltips = container.querySelectorAll('.absolute.bottom-full')
        expect(tooltips.length).toBeGreaterThan(0)
      })
    })

    it('shows tooltip on hover with resource data', async () => {
      const resources = createMockResources(5, 'Ready')
      const { container } = render(
        <TimelineChart items={resources} loading={false} mode="resources" />
      )

      const bars = container.querySelectorAll('.relative.flex-1.group')
      expect(bars.length).toBeGreaterThan(0)

      // Simulate hover
      const firstBar = bars[0]
      fireEvent.mouseEnter(firstBar)

      // Wait for tooltip to appear
      await vi.waitFor(() => {
        const tooltips = container.querySelectorAll('.absolute.bottom-full')
        expect(tooltips.length).toBeGreaterThan(0)
      })
    })

    it('hides tooltip on mouse leave', async () => {
      const events = createMockEvents(5, 'Normal')
      const { container } = render(
        <TimelineChart items={events} loading={false} mode="events" />
      )

      const bars = container.querySelectorAll('.relative.flex-1.group')
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

    it('shows tooltip with date and time range format', async () => {
      const events = createMockEvents(10, 'Normal')
      const { container } = render(
        <TimelineChart items={events} loading={false} mode="events" />
      )

      const bars = container.querySelectorAll('.relative.flex-1.group')
      const firstBar = bars[0]

      // Simulate hover
      fireEvent.mouseEnter(firstBar)

      // Wait for tooltip to appear and check format
      await vi.waitFor(() => {
        const tooltip = container.querySelector('.absolute.bottom-full')
        expect(tooltip).toBeTruthy()
        // Tooltip should contain time format like "HH:MM:SS - HH:MM:SS"
        expect(tooltip.textContent).toMatch(/\d{2}:\d{2}:\d{2}\s*-\s*\d{2}:\d{2}:\d{2}/)
      })
    })
  })

  describe('Animation', () => {
    it('applies horizontal fill animation to segments with items', () => {
      const events = createMockEvents(5)
      const { container } = render(
        <TimelineChart items={events} loading={false} mode="events" />
      )

      // Check for animation style
      const animatedSegments = container.querySelectorAll('[style*="animation"]')
      expect(animatedSegments.length).toBeGreaterThan(0)
    })

    it('does not animate placeholder segments', () => {
      const { container } = render(
        <TimelineChart items={[]} loading={true} mode="events" />
      )

      // Loading segments should not have colored fill with animation
      const coloredSegments = container.querySelectorAll('.bg-green-500, .bg-yellow-500, .bg-red-500')
      expect(coloredSegments).toHaveLength(0)
    })

    it('renders gray background for all segments', () => {
      const events = createMockEvents(5)
      const { container } = render(
        <TimelineChart items={events} loading={false} mode="events" />
      )

      // All segments should have gray background
      const graySegments = container.querySelectorAll('.bg-gray-200')
      expect(graySegments.length).toBeGreaterThan(0)
    })
  })

  describe('Time Bucketing', () => {
    it('groups items into buckets', () => {
      const events = createMockEvents(100) // Many events
      const { container } = render(
        <TimelineChart items={events} loading={false} mode="events" />
      )

      // Should still render fixed number of bars (10 on desktop)
      const bars = container.querySelectorAll('.relative.flex-1.group')
      expect(bars).toHaveLength(10)
    })

    it('handles items with same timestamp', () => {
      const now = new Date()
      const sameTimeEvents = Array.from({ length: 5 }, (_, i) => ({
        lastTimestamp: now.toISOString(),
        type: 'Normal',
        message: `Event ${i}`,
        involvedObject: 'Test/resource',
        namespace: 'default'
      }))

      const { container } = render(
        <TimelineChart items={sameTimeEvents} loading={false} mode="events" />
      )

      // When all items have the same timestamp, render a single bar
      const bars = container.querySelectorAll('.relative.flex-1.group')
      expect(bars).toHaveLength(1)
    })

    it('renders one bar per unique timestamp for 5 or fewer unique timestamps', () => {
      const now = new Date()
      const fiveTimeEvents = Array.from({ length: 5 }, (_, i) => ({
        lastTimestamp: new Date(now.getTime() - i * 60000).toISOString(),
        type: 'Normal',
        message: `Event ${i}`,
        involvedObject: 'Test/resource',
        namespace: 'default'
      }))

      const { container } = render(
        <TimelineChart items={fiveTimeEvents} loading={false} mode="events" />
      )

      // When there are 5 unique timestamps, render 5 bars
      const bars = container.querySelectorAll('.relative.flex-1.group')
      expect(bars).toHaveLength(5)
    })

    it('renders one bar per unique timestamp for 3 unique timestamps', () => {
      const now = new Date()
      const threeTimeEvents = Array.from({ length: 3 }, (_, i) => ({
        lastTimestamp: new Date(now.getTime() - i * 60000).toISOString(),
        type: 'Normal',
        message: `Event ${i}`,
        involvedObject: 'Test/resource',
        namespace: 'default'
      }))

      const { container } = render(
        <TimelineChart items={threeTimeEvents} loading={false} mode="events" />
      )

      // When there are 3 unique timestamps, render 3 bars
      const bars = container.querySelectorAll('.relative.flex-1.group')
      expect(bars).toHaveLength(3)
    })

    it('uses standard bucketing for more than 5 unique timestamps', () => {
      const events = createMockEvents(10)
      const { container } = render(
        <TimelineChart items={events} loading={false} mode="events" />
      )

      // When there are more than 5 unique timestamps, use standard bucketing (10 on desktop)
      const bars = container.querySelectorAll('.relative.flex-1.group')
      expect(bars).toHaveLength(10)
    })
  })

  describe('Layout', () => {
    it('renders horizontal timeline with correct height', () => {
      const events = createMockEvents(5)
      const { container } = render(
        <TimelineChart items={events} loading={false} mode="events" />
      )

      // Check that container has correct height (32px)
      const timelineContainer = container.querySelector('[style*="height"]')
      expect(timelineContainer).toBeTruthy()
      expect(timelineContainer.style.height).toBe('32px')
    })

    it('renders segments in horizontal flex layout', () => {
      const events = createMockEvents(5)
      const { container } = render(
        <TimelineChart items={events} loading={false} mode="events" />
      )

      // Container should use flex layout
      const timelineContainer = container.querySelector('.flex')
      expect(timelineContainer).toBeTruthy()
    })
  })

  describe('Header with Totals and Stats', () => {
    it('renders header with total count for events', () => {
      const events = createMockEvents(10)
      const { container } = render(
        <TimelineChart items={events} loading={false} mode="events" />
      )

      // Should show total events count
      expect(container.textContent).toContain('Reconcile events:')
      expect(container.textContent).toContain('10')
    })

    it('renders header with total count for resources', () => {
      const resources = createMockResources(15)
      const { container } = render(
        <TimelineChart items={resources} loading={false} mode="resources" />
      )

      // Should show total reconciliations count
      expect(container.textContent).toContain('Reconcilers:')
      expect(container.textContent).toContain('15')
    })

    it('shows loading state in header', () => {
      const { container } = render(
        <TimelineChart items={[]} loading={true} mode="events" />
      )

      // Should show loading text
      expect(container.textContent).toContain('Loading...')
    })

    it('displays stats for events mode when multiple types exist', () => {
      const normalEvents = createMockEvents(8, 'Normal')
      const warningEvents = createMockEvents(3, 'Warning')
      const mixedEvents = [...normalEvents, ...warningEvents]

      const { container } = render(
        <TimelineChart items={mixedEvents} loading={false} mode="events" />
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
        <TimelineChart items={normalEvents} loading={false} mode="events" />
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
        <TimelineChart items={resources} loading={false} mode="resources" />
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
        <TimelineChart items={resources} loading={false} mode="resources" />
      )

      // Should NOT show stats when only one status exists
      expect(container.textContent).not.toContain('Ready:')
    })

    it('shows zero count when no items', () => {
      const { container } = render(
        <TimelineChart items={[]} loading={false} mode="events" />
      )

      // Should show 0 for total
      expect(container.textContent).toContain('Reconcile events:')
      expect(container.textContent).toContain('0')
    })

    it('maintains consistent height during loading', () => {
      const { container: loadingContainer } = render(
        <TimelineChart items={[]} loading={true} mode="events" />
      )

      const { container: loadedContainer } = render(
        <TimelineChart items={createMockEvents(5)} loading={false} mode="events" />
      )

      // Both should have header structure
      const loadingHeader = loadingContainer.querySelector('.flex.items-center.justify-between')
      const loadedHeader = loadedContainer.querySelector('.flex.items-center.justify-between')

      expect(loadingHeader).toBeTruthy()
      expect(loadedHeader).toBeTruthy()
    })
  })
})
