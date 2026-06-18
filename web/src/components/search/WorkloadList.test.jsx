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

    it('should display workload cards when data loads', async () => {
      fetchWithMock.mockResolvedValue({ workloads: mockWorkloads })

      render(<WorkloadList />)

      await waitFor(() => {
        expect(screen.getByText('podinfo')).toBeInTheDocument()
      })
      expect(screen.getByText('node-exporter')).toBeInTheDocument()
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

      await waitFor(() => {
        expect(screen.getByText('2 workloads')).toBeInTheDocument()
      })
    })
  })

  describe('Reconciler status', () => {
    it('should render the parent reconciler ref as a "Managed by" line with a status dot', async () => {
      fetchWithMock.mockResolvedValue({ workloads: mockWorkloads })

      render(<WorkloadList />)

      await waitFor(() => {
        expect(screen.getByText('podinfo')).toBeInTheDocument()
      })

      // Each card carries a muted "Managed by" label.
      expect(screen.getAllByText('Managed by')).toHaveLength(2)

      // Reconciler ref is shown as a single kind/namespace/name path.
      expect(screen.getByText('Kustomization/flux-system/apps')).toBeInTheDocument()
      expect(screen.getByText('HelmRelease/monitoring/metrics')).toBeInTheDocument()

      // Reconciler status is conveyed via a colored dot + tooltip, never a message.
      const readyLine = screen.getByText('Kustomization/flux-system/apps').closest('div')
      expect(readyLine.querySelector('span.bg-green-500')).toBeInTheDocument()
      expect(readyLine.getAttribute('title')).toContain('(Ready)')

      const failedLine = screen.getByText('HelmRelease/monitoring/metrics').closest('div')
      expect(failedLine.querySelector('span.bg-red-500')).toBeInTheDocument()
      expect(failedLine.getAttribute('title')).toContain('(Failed)')
    })

    it('should link the workload name to the workload dashboard', async () => {
      fetchWithMock.mockResolvedValue({ workloads: [mockWorkloads[0]] })

      render(<WorkloadList />)

      await waitFor(() => {
        expect(screen.getByText('podinfo')).toBeInTheDocument()
      })

      const link = screen.getByText('podinfo').closest('a')
      expect(link).toHaveAttribute('href', '/workload/Deployment/apps/podinfo')
    })

    it('should not render the workload reconciler message', async () => {
      const withMessage = [{ ...mockWorkloads[0], reconcilerMessage: 'should not appear' }]
      fetchWithMock.mockResolvedValue({ workloads: withMessage })

      render(<WorkloadList />)

      await waitFor(() => {
        expect(screen.getByText('podinfo')).toBeInTheDocument()
      })

      expect(screen.queryByText('should not appear')).not.toBeInTheDocument()
    })
  })

  describe('FilterForm wiring', () => {
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
        expect(screen.getByText('podinfo')).toBeInTheDocument()
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
