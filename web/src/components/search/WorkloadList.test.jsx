// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/preact'
import {
  WorkloadList,
  workloadsData,
  workloadsLoading,
  workloadsError,
  selectedWorkloadKind,
  selectedWorkloadName,
  selectedWorkloadNamespace,
  fetchWorkloadsStatus
} from './WorkloadList'
import { reportData } from '../../app'
import { fetchWithMock } from '../../utils/fetch'

// Mock the fetch utility
vi.mock('../../utils/fetch', () => ({
  fetchWithMock: vi.fn()
}))

// Mock routing utilities
vi.mock('../../utils/routing', () => ({
  useRestoreFiltersFromUrl: vi.fn(),
  useSyncFiltersToUrl: vi.fn(),
  getDashboardUrl: (kind, namespace, name) => `/workload/${kind}/${namespace}/${name}`
}))

// Mock preact-iso
vi.mock('preact-iso', () => ({
  useLocation: () => ({
    path: '/workloads',
    query: {},
    route: vi.fn()
  })
}))

// Mock WorkloadDetailsView. The compact row mounts it lazily on expand and waits
// for it to call `onReady` before it reveals the panel (and swaps the toggle
// spinner back to a chevron). To keep the disclosure controllable we do NOT call
// onReady automatically; instead the mock exposes a button the test clicks to
// simulate the detail fetch settling.
vi.mock('./WorkloadDetailsView', () => ({
  WorkloadDetailsView: ({ kind, name, namespace, isExpanded, onReady }) => (
    isExpanded ? (
      <div data-testid="workload-details-view">
        <span data-testid="workload-details-view-kind">{kind}</span>
        <span data-testid="workload-details-view-name">{name}</span>
        <span data-testid="workload-details-view-namespace">{namespace}</span>
        <button data-testid="settle-details" onClick={onReady}>settle</button>
      </div>
    ) : null
  )
}))

// Mock FilterForm to capture props (no statusSignal, flat kinds)
vi.mock('./FilterForm', () => ({
  FilterForm: ({ onClear, kindSignal, nameSignal, namespaceSignal, statusSignal, kinds }) => (
    <div data-testid="filter-form">
      <button onClick={onClear} data-testid="clear-filters">Clear</button>
      <span data-testid="kind-signal">{kindSignal.value}</span>
      <span data-testid="name-signal">{nameSignal.value}</span>
      <span data-testid="namespace-signal">{namespaceSignal.value}</span>
      <span data-testid="has-status-signal">{statusSignal ? 'yes' : 'no'}</span>
      <span data-testid="kinds-prop">{(kinds || []).join(',')}</span>
    </div>
  )
}))

describe('WorkloadList', () => {
  const mockWorkloads = [
    {
      kind: 'Deployment',
      name: 'podinfo',
      namespace: 'apps',
      apiVersion: 'apps/v1',
      reconcilerKind: 'Kustomization',
      reconcilerNamespace: 'flux-system',
      reconcilerName: 'apps',
      reconcilerStatus: 'Ready',
      lastReconciled: new Date('2025-01-15T10:00:00Z')
    },
    {
      kind: 'DaemonSet',
      name: 'node-exporter',
      namespace: 'monitoring',
      apiVersion: 'apps/v1',
      reconcilerKind: 'HelmRelease',
      reconcilerNamespace: 'monitoring',
      reconcilerName: 'metrics',
      reconcilerStatus: 'Failed',
      lastReconciled: new Date('2025-01-15T09:00:00Z')
    }
  ]

  beforeEach(() => {
    workloadsData.value = []
    workloadsLoading.value = false
    workloadsError.value = null
    selectedWorkloadKind.value = ''
    selectedWorkloadName.value = ''
    selectedWorkloadNamespace.value = ''

    reportData.value = {
      spec: {
        namespaces: ['flux-system', 'apps', 'monitoring']
      }
    }

    vi.clearAllMocks()
    fetchWithMock.mockResolvedValue({ workloads: [] })
  })

  describe('fetchWorkloadsStatus function', () => {
    it('should fetch workloads with no filters', async () => {
      fetchWithMock.mockResolvedValue({ workloads: mockWorkloads })

      await fetchWorkloadsStatus()

      expect(fetchWithMock).toHaveBeenCalledWith({
        endpoint: '/api/v1/workloads?',
        mockPath: '../mock/workloads',
        mockExport: 'getMockWorkloadsList'
      })
      expect(workloadsData.value).toEqual(mockWorkloads)
      expect(workloadsLoading.value).toBe(false)
    })

    it('should pass kind, name, and namespace query params', async () => {
      selectedWorkloadKind.value = 'Deployment'
      selectedWorkloadName.value = 'podinfo'
      selectedWorkloadNamespace.value = 'apps'
      fetchWithMock.mockResolvedValue({ workloads: mockWorkloads })

      await fetchWorkloadsStatus()

      expect(fetchWithMock).toHaveBeenCalledWith({
        endpoint: '/api/v1/workloads?kind=Deployment&name=podinfo&namespace=apps',
        mockPath: '../mock/workloads',
        mockExport: 'getMockWorkloadsList'
      })
    })

    it('should handle fetch errors', async () => {
      fetchWithMock.mockRejectedValue(new Error('Network error'))

      await fetchWorkloadsStatus()

      expect(workloadsError.value).toBe('Network error')
      expect(workloadsData.value).toEqual([])
      expect(workloadsLoading.value).toBe(false)
    })
  })

  describe('Component rendering', () => {
    it('should fetch workloads on mount', async () => {
      fetchWithMock.mockResolvedValue({ workloads: mockWorkloads })

      render(<WorkloadList />)

      await waitFor(() => {
        expect(fetchWithMock).toHaveBeenCalled()
      })
    })

    it('should display workload rows when data loads', async () => {
      fetchWithMock.mockResolvedValue({ workloads: mockWorkloads })

      render(<WorkloadList />)

      // The name renders twice per row (a mobile link and a desktop link).
      await waitFor(() => {
        expect(screen.getAllByText('podinfo').length).toBeGreaterThan(0)
      })
      expect(screen.getAllByText('node-exporter').length).toBeGreaterThan(0)
    })

    it('should show empty state when no workloads match filters', async () => {
      fetchWithMock.mockResolvedValue({ workloads: [] })

      render(<WorkloadList />)

      await waitFor(() => {
        expect(screen.getByText('No workloads found for the selected filters')).toBeInTheDocument()
      })
    })

    it('should show error state on fetch failure', async () => {
      fetchWithMock.mockRejectedValue(new Error('Failed to connect'))

      render(<WorkloadList />)

      await waitFor(() => {
        expect(screen.getByText(/Failed to load workloads: Failed to connect/)).toBeInTheDocument()
      })
    })

    it('should display workload count when loaded', async () => {
      fetchWithMock.mockResolvedValue({ workloads: mockWorkloads })

      render(<WorkloadList />)

      // The count renders both in the desktop page title and the FilterBar toolbar.
      await waitFor(() => {
        expect(screen.getAllByText('2 workloads').length).toBeGreaterThan(0)
      })
    })
  })

  describe('Compact row', () => {
    it('should render a neutral kind pill regardless of the reconciler status', async () => {
      fetchWithMock.mockResolvedValue({ workloads: mockWorkloads })

      render(<WorkloadList />)

      // Chips show the kubectl-style short name: Deployment -> deploy, DaemonSet -> ds.
      await waitFor(() => {
        expect(screen.getAllByText('deploy').length).toBeGreaterThan(0)
      })

      // Both the Ready (podinfo/deploy) and Failed (node-exporter/ds) rows carry a
      // neutral gray chip: a workload has no status of its own in the list, so the
      // chip is never tinted by the reconciler's status.
      const chips = [...screen.getAllByText('deploy'), ...screen.getAllByText('ds')]
      expect(chips.length).toBe(4) // two chips (mobile + desktop) per row
      chips.forEach((chip) => {
        expect(chip.className).toContain('bg-gray-100')
        expect(chip.className).not.toMatch(/bg-(red|green|blue|yellow)-/)
      })
    })

    it('should show a "Reconciled <time>" mobile line with no status word', async () => {
      fetchWithMock.mockResolvedValue({ workloads: mockWorkloads })

      render(<WorkloadList />)

      // The mobile second line states the reconcile time instead of a status word,
      // because the workload itself has no status in the list index.
      await waitFor(() => {
        expect(screen.getAllByText(/^Reconciled\b/).length).toBe(2)
      })

      // No bare status word is rendered for the workload (status lives only in the
      // reconciler dot/tooltip, not as visible text).
      expect(screen.queryByText('Ready')).not.toBeInTheDocument()
      expect(screen.queryByText('Failed')).not.toBeInTheDocument()
    })
  })

  describe('Reconciler reference', () => {
    it('should render the parent reconciler as a "managed by" line with a status dot', async () => {
      fetchWithMock.mockResolvedValue({ workloads: mockWorkloads })

      render(<WorkloadList />)

      await waitFor(() => {
        expect(screen.getAllByText('podinfo').length).toBeGreaterThan(0)
      })

      // Each row carries a muted desktop "managed by" label.
      expect(screen.getAllByText('managed by')).toHaveLength(2)

      // Reconciler ref is shown as a single kind/namespace/name path.
      expect(screen.getByText('Kustomization/flux-system/apps')).toBeInTheDocument()
      expect(screen.getByText('HelmRelease/monitoring/metrics')).toBeInTheDocument()

      // Reconciler status is conveyed via a colored dot + tooltip, never a message.
      const readyRef = screen.getByText('Kustomization/flux-system/apps').closest('span[title]')
      expect(readyRef.querySelector('span.bg-green-500')).toBeInTheDocument()
      expect(readyRef.getAttribute('title')).toContain('(Ready)')

      const failedRef = screen.getByText('HelmRelease/monitoring/metrics').closest('span[title]')
      expect(failedRef.querySelector('span.bg-red-500')).toBeInTheDocument()
      expect(failedRef.getAttribute('title')).toContain('(Failed)')
    })

    it('should link the workload name to the workload dashboard', async () => {
      fetchWithMock.mockResolvedValue({ workloads: [mockWorkloads[0]] })

      render(<WorkloadList />)

      await waitFor(() => {
        expect(screen.getAllByText('podinfo').length).toBeGreaterThan(0)
      })

      // Both the mobile and desktop name links point at the dashboard.
      screen.getAllByText('podinfo').forEach((nameSpan) => {
        expect(nameSpan.closest('a')).toHaveAttribute('href', '/workload/Deployment/apps/podinfo')
      })
    })

    it('should not render the workload reconciler message', async () => {
      const withMessage = [{ ...mockWorkloads[0], reconcilerMessage: 'should not appear' }]
      fetchWithMock.mockResolvedValue({ workloads: withMessage })

      render(<WorkloadList />)

      await waitFor(() => {
        expect(screen.getAllByText('podinfo').length).toBeGreaterThan(0)
      })

      expect(screen.queryByText('should not appear')).not.toBeInTheDocument()
    })
  })

  describe('Details panel', () => {
    it('should display a details toggle for every workload', async () => {
      fetchWithMock.mockResolvedValue({ workloads: mockWorkloads })

      render(<WorkloadList />)

      await waitFor(() => {
        expect(screen.getAllByLabelText('Toggle details')).toHaveLength(2)
      })
    })

    it('should keep the details view collapsed by default', async () => {
      fetchWithMock.mockResolvedValue({ workloads: mockWorkloads })

      render(<WorkloadList />)

      await waitFor(() => {
        expect(screen.getAllByText('podinfo').length).toBeGreaterThan(0)
      })

      // Lazily mounted: the panel is absent until the row is expanded.
      expect(screen.queryByTestId('workload-details-view')).not.toBeInTheDocument()
    })

    it('should mount WorkloadDetailsView with the workload identity when toggled', async () => {
      fetchWithMock.mockResolvedValue({ workloads: [mockWorkloads[0]] })

      render(<WorkloadList />)

      const toggle = await screen.findByLabelText('Toggle details')
      fireEvent.click(toggle)

      await waitFor(() => {
        expect(screen.getByTestId('workload-details-view')).toBeInTheDocument()
      })
      expect(screen.getByTestId('workload-details-view-kind')).toHaveTextContent('Deployment')
      expect(screen.getByTestId('workload-details-view-name')).toHaveTextContent('podinfo')
      expect(screen.getByTestId('workload-details-view-namespace')).toHaveTextContent('apps')
    })

    it('should spin the toggle while the detail fetch is in flight', async () => {
      fetchWithMock.mockResolvedValue({ workloads: [mockWorkloads[0]] })

      render(<WorkloadList />)

      const toggle = await screen.findByLabelText('Toggle details')

      // Collapsed: the toggle shows a static chevron, not a spinner.
      expect(toggle.querySelector('svg.animate-spin')).not.toBeInTheDocument()

      fireEvent.click(toggle)

      // While the lazily mounted panel is fetching (onReady not yet called), the
      // toggle shows a spinner and the panel is kept collapsed.
      await waitFor(() => {
        expect(toggle.querySelector('svg.animate-spin')).toBeInTheDocument()
      })
      const revealWrapper = screen.getByTestId('workload-details-view').closest('div.grid')
      expect(revealWrapper.className).toContain('grid-rows-[0fr]')
    })

    it('should reveal the panel once the detail view reports ready', async () => {
      fetchWithMock.mockResolvedValue({ workloads: [mockWorkloads[0]] })

      render(<WorkloadList />)

      const toggle = await screen.findByLabelText('Toggle details')
      fireEvent.click(toggle)

      // Spinner first, while the fetch is pending.
      await waitFor(() => {
        expect(toggle.querySelector('svg.animate-spin')).toBeInTheDocument()
      })

      // Simulate the detail fetch settling: the mock calls onReady.
      fireEvent.click(screen.getByTestId('settle-details'))

      // The spinner is swapped back to a chevron and the Reveal opens.
      await waitFor(() => {
        expect(toggle.querySelector('svg.animate-spin')).not.toBeInTheDocument()
      })
      const revealWrapper = screen.getByTestId('workload-details-view').closest('div.grid')
      expect(revealWrapper.className).toContain('grid-rows-[1fr]')
    })
  })

  describe('FilterBar wiring', () => {
    it('should render the filter toolbar with the workload count', async () => {
      fetchWithMock.mockResolvedValue({ workloads: mockWorkloads })

      render(<WorkloadList />)

      await waitFor(() => {
        expect(screen.getByTestId('filter-form')).toBeInTheDocument()
      })
      // FilterBar shows the count + label (mobile toolbar row).
      expect(screen.getAllByText('2 workloads').length).toBeGreaterThan(0)
    })

    it('should pass workload kinds and omit the status signal', async () => {
      fetchWithMock.mockResolvedValue({ workloads: [] })

      render(<WorkloadList />)

      expect(screen.getByTestId('has-status-signal')).toHaveTextContent('no')
      expect(screen.getByTestId('kinds-prop')).toHaveTextContent('CronJob,DaemonSet,Deployment,StatefulSet')
    })

    it('should not render a StatusChart', async () => {
      fetchWithMock.mockResolvedValue({ workloads: mockWorkloads })

      const { container } = render(<WorkloadList />)

      await waitFor(() => {
        expect(screen.getAllByText('podinfo').length).toBeGreaterThan(0)
      })

      // StatusChart renders a status timeline; none should be present
      expect(container.querySelector('[data-testid="status-chart"]')).not.toBeInTheDocument()
    })

    it('should clear all filters when clear button clicked', async () => {
      selectedWorkloadKind.value = 'Deployment'
      selectedWorkloadName.value = 'test'
      selectedWorkloadNamespace.value = 'apps'

      render(<WorkloadList />)

      fireEvent.click(screen.getByTestId('clear-filters'))

      expect(selectedWorkloadKind.value).toBe('')
      expect(selectedWorkloadName.value).toBe('')
      expect(selectedWorkloadNamespace.value).toBe('')
    })

    it('should re-fetch when a filter changes', async () => {
      fetchWithMock.mockResolvedValue({ workloads: [] })

      render(<WorkloadList />)

      await waitFor(() => {
        expect(fetchWithMock).toHaveBeenCalledTimes(1)
      })

      selectedWorkloadKind.value = 'StatefulSet'

      await waitFor(() => {
        expect(fetchWithMock).toHaveBeenCalledTimes(2)
      })
    })
  })
})
