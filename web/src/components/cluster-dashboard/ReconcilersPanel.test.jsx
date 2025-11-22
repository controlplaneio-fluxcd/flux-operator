// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/preact'
import { ReconcilersPanel } from './ReconcilersPanel'
import { selectedResourceKind, selectedResourceStatus } from '../resource-browser/ResourceList'

// Mock preact-iso
const mockRoute = vi.fn()
vi.mock('preact-iso', () => ({
  useLocation: () => ({
    path: '/',
    query: {},
    route: mockRoute
  })
}))

describe('ReconcilersPanel', () => {
  const mockReconcilers = [
    {
      kind: 'Kustomization',
      apiVersion: 'kustomize.toolkit.fluxcd.io/v1',
      stats: { running: 5, failing: 2, suspended: 1 }
    },
    {
      kind: 'HelmRelease',
      apiVersion: 'helm.toolkit.fluxcd.io/v2',
      stats: { running: 3, failing: 0, suspended: 0 }
    },
    {
      kind: 'GitRepository',
      apiVersion: 'source.toolkit.fluxcd.io/v1',
      stats: { running: 4, failing: 1, suspended: 0 }
    },
    {
      kind: 'OCIRepository',
      apiVersion: 'source.toolkit.fluxcd.io/v1beta2',
      stats: { running: 2, failing: 0, suspended: 0 }
    },
    {
      kind: 'Alert',
      apiVersion: 'notification.toolkit.fluxcd.io/v1beta3',
      stats: { running: 1, failing: 0, suspended: 0 }
    },
    {
      kind: 'ImageUpdateAutomation',
      apiVersion: 'image.toolkit.fluxcd.io/v1beta2',
      stats: { running: 1, failing: 1, suspended: 0 }
    }
  ]

  beforeEach(() => {
    // Reset signals
    selectedResourceKind.value = ''
    selectedResourceStatus.value = ''

    // Reset mocks
    vi.clearAllMocks()
  })

  describe('Rendering', () => {
    it('should render component title', () => {
      render(<ReconcilersPanel reconcilers={mockReconcilers} />)

      expect(screen.getByText('Flux Reconcilers')).toBeInTheDocument()
    })

    it('should show total CRD count', () => {
      render(<ReconcilersPanel reconcilers={mockReconcilers} />)

      expect(screen.getByText(/6 CRDs/)).toBeInTheDocument()
    })

    it('should show total resource count', () => {
      render(<ReconcilersPanel reconcilers={mockReconcilers} />)

      // Total: 5+2+1 + 3 + 4+1 + 2 + 1 + 1+1 = 21
      expect(screen.getByText(/21 resources/)).toBeInTheDocument()
    })

    it('should show failing count when there are failures', () => {
      render(<ReconcilersPanel reconcilers={mockReconcilers} />)

      // Total failing: 2 + 1 + 1 = 4
      expect(screen.getByText('4 failing')).toBeInTheDocument()
    })

    it('should not show failing badge when no failures', () => {
      const healthyReconcilers = [
        {
          kind: 'GitRepository',
          apiVersion: 'source.toolkit.fluxcd.io/v1',
          stats: { running: 5, failing: 0, suspended: 0 }
        }
      ]

      render(<ReconcilersPanel reconcilers={healthyReconcilers} />)

      expect(screen.queryByText(/failing/)).not.toBeInTheDocument()
    })

    it('should render expand/collapse toggle', () => {
      render(<ReconcilersPanel reconcilers={mockReconcilers} />)

      const toggle = screen.getByRole('button', { name: /Flux Reconcilers/i })
      expect(toggle).toBeInTheDocument()
    })

    it('should render reconciler cards when expanded', () => {
      render(<ReconcilersPanel reconcilers={mockReconcilers} />)

      expect(screen.getByText('Kustomization')).toBeInTheDocument()
      expect(screen.getByText('HelmRelease')).toBeInTheDocument()
      expect(screen.getByText('GitRepository')).toBeInTheDocument()
      expect(screen.getByText('Alert')).toBeInTheDocument()
      expect(screen.getByText('ImageUpdateAutomation')).toBeInTheDocument()
    })
  })

  describe('Grouping by API Type', () => {
    it('should render Appliers group', () => {
      render(<ReconcilersPanel reconcilers={mockReconcilers} />)

      expect(screen.getByText('Appliers')).toBeInTheDocument()
      // Kustomization and HelmRelease should be in Appliers group
    })

    it('should render Sources group', () => {
      render(<ReconcilersPanel reconcilers={mockReconcilers} />)

      expect(screen.getByText('Sources')).toBeInTheDocument()
      // GitRepository and OCIRepository should be in Sources group
    })

    it('should render Notifications group', () => {
      render(<ReconcilersPanel reconcilers={mockReconcilers} />)

      expect(screen.getByText('Notifications')).toBeInTheDocument()
      // Alert should be in Notifications group
    })

    it('should render Image Automation group', () => {
      render(<ReconcilersPanel reconcilers={mockReconcilers} />)

      expect(screen.getByText('Image Automation')).toBeInTheDocument()
      // ImageUpdateAutomation should be in Image Automation group
    })
  })

  describe('ReconcilerCard Display', () => {
    it('should display reconciler kind and apiVersion', () => {
      render(<ReconcilersPanel reconcilers={mockReconcilers} />)

      expect(screen.getByText('Kustomization')).toBeInTheDocument()
      expect(screen.getByText('kustomize.toolkit.fluxcd.io/v1')).toBeInTheDocument()
    })

    it('should display total resource count for each reconciler', () => {
      render(<ReconcilersPanel reconcilers={mockReconcilers} />)

      // Find the Kustomization card and check its total (5+2+1 = 8)
      const kustomizationCard = screen.getByText('Kustomization').closest('button')
      expect(kustomizationCard).toHaveTextContent('8')
    })

    it('should show running badge when stats.running > 0', () => {
      render(<ReconcilersPanel reconcilers={mockReconcilers} />)

      expect(screen.getByText('5 running')).toBeInTheDocument() // Kustomization
      expect(screen.getByText('3 running')).toBeInTheDocument() // HelmRelease
    })

    it('should show failing badge when stats.failing > 0', () => {
      render(<ReconcilersPanel reconcilers={mockReconcilers} />)

      expect(screen.getByText('2 failing')).toBeInTheDocument() // Kustomization
      // GitRepository and ImageUpdateAutomation both have 1 failing
      expect(screen.getAllByText('1 failing')).toHaveLength(2)
    })

    it('should show suspended badge when stats.suspended > 0', () => {
      render(<ReconcilersPanel reconcilers={mockReconcilers} />)

      expect(screen.getByText('1 suspended')).toBeInTheDocument() // Kustomization
    })

    it('should apply danger border color when there are failures', () => {
      render(<ReconcilersPanel reconcilers={mockReconcilers} />)

      const kustomizationCard = screen.getByText('Kustomization').closest('button')
      expect(kustomizationCard).toHaveClass('border-danger')
    })

    it('should apply warning border color when suspended but no failures', () => {
      const reconcilers = [{
        kind: 'Kustomization',
        apiVersion: 'kustomize.toolkit.fluxcd.io/v1',
        stats: { running: 5, failing: 0, suspended: 1 }
      }]

      render(<ReconcilersPanel reconcilers={reconcilers} />)

      const card = screen.getByText('Kustomization').closest('button')
      expect(card).toHaveClass('border-warning')
    })

    it('should apply success border color when healthy', () => {
      const reconcilers = [{
        kind: 'HelmRelease',
        apiVersion: 'helm.toolkit.fluxcd.io/v2',
        stats: { running: 3, failing: 0, suspended: 0 }
      }]

      render(<ReconcilersPanel reconcilers={reconcilers} />)

      const card = screen.getByText('HelmRelease').closest('button')
      expect(card).toHaveClass('border-success')
    })

    it('should show error icon when there are failures', () => {
      render(<ReconcilersPanel reconcilers={mockReconcilers} />)

      const kustomizationCard = screen.getByText('Kustomization').closest('button')
      const svg = kustomizationCard.querySelector('svg')
      expect(svg).toBeInTheDocument()
      expect(svg).toHaveClass('text-danger')
    })
  })

  describe('Navigation - Card Click', () => {
    it('should navigate to search view when card clicked', () => {
      render(<ReconcilersPanel reconcilers={mockReconcilers} />)

      const gitRepoCard = screen.getByText('GitRepository').closest('button')
      fireEvent.click(gitRepoCard)

      expect(mockRoute).toHaveBeenCalledWith('/resources?kind=GitRepository')
    })

    it('should set kind filter when card clicked', () => {
      render(<ReconcilersPanel reconcilers={mockReconcilers} />)

      const gitRepoCard = screen.getByText('GitRepository').closest('button')
      fireEvent.click(gitRepoCard)

      expect(selectedResourceKind.value).toBe('GitRepository')
      expect(mockRoute).toHaveBeenCalledWith('/resources?kind=GitRepository')
    })

    it('should clear status filter when card clicked', () => {
      selectedResourceStatus.value = 'Failed' // Pre-set a status

      render(<ReconcilersPanel reconcilers={mockReconcilers} />)

      const gitRepoCard = screen.getByText('GitRepository').closest('button')
      fireEvent.click(gitRepoCard)

      expect(selectedResourceStatus.value).toBe('')
      expect(mockRoute).toHaveBeenCalledWith('/resources?kind=GitRepository')
    })
  })

  describe('Navigation - Status Badge Click', () => {
    it('should navigate to search view when running badge clicked', () => {
      render(<ReconcilersPanel reconcilers={mockReconcilers} />)

      const runningBadge = screen.getByText('5 running')
      fireEvent.click(runningBadge)

      expect(mockRoute).toHaveBeenCalledWith('/resources?kind=Kustomization&status=Ready')
    })

    it('should set kind and Ready status when running badge clicked', () => {
      render(<ReconcilersPanel reconcilers={mockReconcilers} />)

      const runningBadge = screen.getByText('5 running') // Kustomization
      fireEvent.click(runningBadge)

      expect(selectedResourceKind.value).toBe('Kustomization')
      expect(selectedResourceStatus.value).toBe('Ready')
      expect(mockRoute).toHaveBeenCalledWith('/resources?kind=Kustomization&status=Ready')
    })

    it('should set kind and Failed status when failing badge clicked', () => {
      render(<ReconcilersPanel reconcilers={mockReconcilers} />)

      const failingBadge = screen.getByText('2 failing') // Kustomization
      fireEvent.click(failingBadge)

      expect(selectedResourceKind.value).toBe('Kustomization')
      expect(selectedResourceStatus.value).toBe('Failed')
      expect(mockRoute).toHaveBeenCalledWith('/resources?kind=Kustomization&status=Failed')
    })

    it('should set kind and Suspended status when suspended badge clicked', () => {
      render(<ReconcilersPanel reconcilers={mockReconcilers} />)

      const suspendedBadge = screen.getByText('1 suspended') // Kustomization
      fireEvent.click(suspendedBadge)

      expect(selectedResourceKind.value).toBe('Kustomization')
      expect(selectedResourceStatus.value).toBe('Suspended')
      expect(mockRoute).toHaveBeenCalledWith('/resources?kind=Kustomization&status=Suspended')
    })

    it('should prevent card click when status badge clicked', () => {
      render(<ReconcilersPanel reconcilers={mockReconcilers} />)

      // Click the failing badge
      const failingBadge = screen.getByText('2 failing')
      fireEvent.click(failingBadge)

      // Status should be 'Failed', not '' (which would happen if card click also fired)
      expect(selectedResourceStatus.value).toBe('Failed')
      expect(mockRoute).toHaveBeenCalledWith('/resources?kind=Kustomization&status=Failed')
    })
  })

  describe('Expand/Collapse', () => {
    it('should collapse grid when toggle clicked', () => {
      render(<ReconcilersPanel reconcilers={mockReconcilers} />)

      // Initially expanded - cards should be visible
      expect(screen.getByText('Kustomization')).toBeInTheDocument()

      // Click toggle to collapse
      const toggle = screen.getByRole('button', { name: /Flux Reconcilers/i })
      fireEvent.click(toggle)

      // Cards should be hidden
      expect(screen.queryByText('Kustomization')).not.toBeInTheDocument()
    })

    it('should toggle grid visibility when clicked multiple times', () => {
      render(<ReconcilersPanel reconcilers={mockReconcilers} />)

      const toggle = screen.getByRole('button', { name: /Flux Reconcilers/i })

      // Get initial state
      const initiallyVisible = screen.queryByText('Kustomization') !== null

      // Click to toggle
      fireEvent.click(toggle)
      const afterFirstClick = screen.queryByText('Kustomization') !== null

      // State should have changed
      expect(afterFirstClick).not.toBe(initiallyVisible)

      // Click again to toggle back
      fireEvent.click(toggle)
      const afterSecondClick = screen.queryByText('Kustomization') !== null

      // State should be back to initial
      expect(afterSecondClick).toBe(initiallyVisible)
    })

    it('should rotate chevron icon when toggled', () => {
      render(<ReconcilersPanel reconcilers={mockReconcilers} />)

      const toggle = screen.getByRole('button', { name: /Flux Reconcilers/i })
      const chevron = toggle.querySelector('svg')

      // Get initial rotation state
      const initiallyRotated = chevron.classList.contains('rotate-180')

      // Toggle
      fireEvent.click(toggle)
      const rotatedAfterClick = chevron.classList.contains('rotate-180')

      // Rotation state should have changed
      expect(rotatedAfterClick).not.toBe(initiallyRotated)
    })
  })
})
