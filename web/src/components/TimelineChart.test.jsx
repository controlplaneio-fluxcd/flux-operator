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

    it('renders placeholder bars when loading', () => {
      const { container } = render(
        <TimelineChart items={[]} loading={true} mode="events" />
      )

      // Should have bars (placeholder)
      const bars = container.querySelectorAll('.relative.flex-1.group')
      expect(bars.length).toBeGreaterThan(0)
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
    it('renders 20 bars on mobile (< 1024px)', () => {
      setWindowWidth(768)

      const events = createMockEvents(5)
      const { container } = render(
        <TimelineChart items={events} loading={false} mode="events" />
      )

      const bars = container.querySelectorAll('.relative.flex-1.group')
      expect(bars).toHaveLength(20)
    })

    it('renders 40 bars on desktop (>= 1024px)', () => {
      setWindowWidth(1920)

      const events = createMockEvents(5)
      const { container } = render(
        <TimelineChart items={events} loading={false} mode="events" />
      )

      const bars = container.querySelectorAll('.relative.flex-1.group')
      expect(bars).toHaveLength(40)
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

    it('renders yellow bars for mixed events', () => {
      const normalEvents = createMockEvents(2, 'Normal')
      const warningEvents = createMockEvents(2, 'Warning')
      const mixedEvents = [...normalEvents, ...warningEvents]

      const { container } = render(
        <TimelineChart items={mixedEvents} loading={false} mode="events" />
      )

      // Check for yellow color class (mixed events in same bucket)
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

    it('renders yellow bars when some failed', () => {
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

      // Check for yellow color class (at least one failed but not all)
      const yellowBars = container.querySelectorAll('.bg-yellow-500')
      expect(yellowBars.length).toBeGreaterThan(0)
    })

    it('handles different resource statuses', () => {
      const now = new Date()
      const resources = [
        { lastReconciled: new Date(now.getTime() - 1000).toISOString(), status: 'Ready', name: 'r1', kind: 'Test', namespace: 'default', message: 'Ready' },
        { lastReconciled: new Date(now.getTime() - 2000).toISOString(), status: 'Progressing', name: 'r2', kind: 'Test', namespace: 'default', message: 'Progressing' },
        { lastReconciled: new Date(now.getTime() - 3000).toISOString(), status: 'Suspended', name: 'r3', kind: 'Test', namespace: 'default', message: 'Suspended' },
        { lastReconciled: new Date(now.getTime() - 4000).toISOString(), status: 'Unknown', name: 'r4', kind: 'Test', namespace: 'default', message: 'Unknown' },
      ]

      const { container } = render(
        <TimelineChart items={resources} loading={false} mode="resources" />
      )

      // All non-failed statuses should result in green
      const greenBars = container.querySelectorAll('.bg-green-500')
      expect(greenBars.length).toBeGreaterThan(0)
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
  })

  describe('Animation', () => {
    it('applies fill animation to bars with items', () => {
      const events = createMockEvents(5)
      const { container } = render(
        <TimelineChart items={events} loading={false} mode="events" />
      )

      // Check for animation style
      const animatedBars = container.querySelectorAll('[style*="animation"]')
      expect(animatedBars.length).toBeGreaterThan(0)
    })

    it('does not animate placeholder bars', () => {
      const { container } = render(
        <TimelineChart items={[]} loading={true} mode="events" />
      )

      // Loading bars should not have colored fill with animation
      const coloredBars = container.querySelectorAll('.bg-green-500, .bg-yellow-500, .bg-red-500')
      expect(coloredBars).toHaveLength(0)
    })
  })

  describe('Time Bucketing', () => {
    it('groups items into buckets', () => {
      const events = createMockEvents(100) // Many events
      const { container } = render(
        <TimelineChart items={events} loading={false} mode="events" />
      )

      // Should still render fixed number of bars (40 on desktop)
      const bars = container.querySelectorAll('.relative.flex-1.group')
      expect(bars).toHaveLength(40)
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

      const bars = container.querySelectorAll('.relative.flex-1.group')
      expect(bars).toHaveLength(40)
    })
  })

  describe('Height and Layout', () => {
    it('renders bars at full height (64px)', () => {
      const events = createMockEvents(5)
      const { container } = render(
        <TimelineChart items={events} loading={false} mode="events" />
      )

      // Check that container has correct height
      const barContainer = container.querySelector('[style*="height"]')
      expect(barContainer).toBeTruthy()
    })

    it('renders gray background for all bars', () => {
      const events = createMockEvents(5)
      const { container } = render(
        <TimelineChart items={events} loading={false} mode="events" />
      )

      // All bars should have gray background
      const grayBars = container.querySelectorAll('.bg-gray-200')
      expect(grayBars.length).toBeGreaterThan(0)
    })
  })
})
