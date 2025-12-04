// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/preact'
import { ReconcilersPanel } from './ReconcilersPanel'
import { selectedResourceKind, selectedResourceStatus } from '../../search/ResourceList'
import { fluxCRDs } from '../../../utils/constants'

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

      // Shows count of installed CRDs (6 in mockReconcilers)
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
      const errorIcon = kustomizationCard.querySelector('svg.text-danger')
      expect(errorIcon).toBeInTheDocument()
      expect(errorIcon).toHaveClass('w-6', 'h-6')
    })

    it('should render documentation link for known CRDs', () => {
      render(<ReconcilersPanel reconcilers={mockReconcilers} />)

      const kustomizationCard = screen.getByText('Kustomization').closest('button')
      const docLink = kustomizationCard.querySelector('a[target="_blank"]')
      expect(docLink).toBeInTheDocument()
      expect(docLink).toHaveAttribute('href', 'https://toolkit.fluxcd.io/components/kustomize/kustomizations/')
      expect(docLink).toHaveAttribute('title', 'Kustomization documentation')
    })

    it('should open documentation link in new tab', () => {
      render(<ReconcilersPanel reconcilers={mockReconcilers} />)

      const gitRepoCard = screen.getByText('GitRepository').closest('button')
      const docLink = gitRepoCard.querySelector('a[target="_blank"]')
      expect(docLink).toHaveAttribute('rel', 'noopener noreferrer')
    })

    it('should not navigate card when documentation link clicked', () => {
      render(<ReconcilersPanel reconcilers={mockReconcilers} />)

      const helmReleaseCard = screen.getByText('HelmRelease').closest('button')
      const docLink = helmReleaseCard.querySelector('a[target="_blank"]')
      fireEvent.click(docLink)

      // Card navigation should not be triggered
      expect(mockRoute).not.toHaveBeenCalled()
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

  describe('Sorting', () => {
    it('should sort notifications alphabetically by kind', () => {
      const reconcilers = [
        {
          kind: 'Receiver',
          apiVersion: 'notification.toolkit.fluxcd.io/v1beta3',
          stats: { running: 1, failing: 0, suspended: 0 }
        },
        {
          kind: 'Alert',
          apiVersion: 'notification.toolkit.fluxcd.io/v1beta3',
          stats: { running: 2, failing: 0, suspended: 0 }
        },
        {
          kind: 'Provider',
          apiVersion: 'notification.toolkit.fluxcd.io/v1beta3',
          stats: { running: 3, failing: 0, suspended: 0 }
        }
      ]

      render(<ReconcilersPanel reconcilers={reconcilers} />)

      // Get all cards in the Notifications group
      const notificationsGroup = screen.getByText('Notifications').closest('div')
      const cards = notificationsGroup.querySelectorAll('button')

      // Should be sorted alphabetically: Alert, Provider, Receiver
      expect(cards[0]).toHaveTextContent('Alert')
      expect(cards[1]).toHaveTextContent('Provider')
      expect(cards[2]).toHaveTextContent('Receiver')
    })

    it('should sort image automation alphabetically by kind', () => {
      const reconcilers = [
        {
          kind: 'ImageUpdateAutomation',
          apiVersion: 'image.toolkit.fluxcd.io/v1beta2',
          stats: { running: 1, failing: 0, suspended: 0 }
        },
        {
          kind: 'ImagePolicy',
          apiVersion: 'image.toolkit.fluxcd.io/v1beta2',
          stats: { running: 2, failing: 0, suspended: 0 }
        },
        {
          kind: 'ImageRepository',
          apiVersion: 'image.toolkit.fluxcd.io/v1beta2',
          stats: { running: 3, failing: 0, suspended: 0 }
        }
      ]

      render(<ReconcilersPanel reconcilers={reconcilers} />)

      // Get all cards in the Image Automation group
      const imageGroup = screen.getByText('Image Automation').closest('div')
      const cards = imageGroup.querySelectorAll('button')

      // Should be sorted alphabetically: ImagePolicy, ImageRepository, ImageUpdateAutomation
      expect(cards[0]).toHaveTextContent('ImagePolicy')
      expect(cards[1]).toHaveTextContent('ImageRepository')
      expect(cards[2]).toHaveTextContent('ImageUpdateAutomation')
    })

    it('should display sources in fluxCRDs array order', () => {
      render(<ReconcilersPanel reconcilers={mockReconcilers} />)

      // Get all cards in the Sources group (can be button or a elements)
      const sourcesGroup = screen.getByText('Sources').closest('div')
      const cards = sourcesGroup.querySelectorAll('button, a.card')

      // Should follow fluxCRDs array order for Sources group
      const sourceCrds = fluxCRDs.filter(crd => crd.group === 'Sources')
      sourceCrds.forEach((crd, index) => {
        expect(cards[index]).toHaveTextContent(crd.kind)
      })
    })

    it('should display appliers in fluxCRDs array order', () => {
      render(<ReconcilersPanel reconcilers={mockReconcilers} />)

      // Get all cards in the Appliers group (can be button or a elements)
      const appliersGroup = screen.getByText('Appliers').closest('div')
      const cards = appliersGroup.querySelectorAll('button, a.card')

      // Should follow fluxCRDs array order for Appliers group
      const applierCrds = fluxCRDs.filter(crd => crd.group === 'Appliers')
      applierCrds.forEach((crd, index) => {
        expect(cards[index]).toHaveTextContent(crd.kind)
      })
    })

    it('should place ResourceSetInputProvider in Sources group', () => {
      render(<ReconcilersPanel reconcilers={mockReconcilers} />)

      // ResourceSetInputProvider should be in Sources group based on fluxCRDs definition
      const sourcesGroup = screen.getByText('Sources').closest('div')
      expect(sourcesGroup).toHaveTextContent('ResourceSetInputProvider')
    })
  })

  describe('All CRDs Always Shown', () => {
    it('should always render all groups from fluxCRDs', () => {
      // Even with only one reconciler with stats, all CRDs should be shown
      const reconcilers = [
        {
          kind: 'GitRepository',
          apiVersion: 'source.toolkit.fluxcd.io/v1',
          stats: { running: 1, failing: 0, suspended: 0 }
        }
      ]

      render(<ReconcilersPanel reconcilers={reconcilers} />)

      // All groups should be rendered since all CRDs are always shown
      expect(screen.getByText('Sources')).toBeInTheDocument()
      expect(screen.getByText('Appliers')).toBeInTheDocument()
      expect(screen.getByText('Notifications')).toBeInTheDocument()
      expect(screen.getByText('Image Automation')).toBeInTheDocument()
    })

    it('should show CRDs with zero stats when no data from API', () => {
      render(<ReconcilersPanel reconcilers={[]} />)

      // All CRDs should still be shown with 0 counts
      expect(screen.getByText('FluxInstance')).toBeInTheDocument()
      expect(screen.getByText('Kustomization')).toBeInTheDocument()
      expect(screen.getByText('GitRepository')).toBeInTheDocument()
    })

    it('should apply gray border for CRDs that are not installed', () => {
      render(<ReconcilersPanel reconcilers={[]} />)

      // CRDs not installed should have gray border (rendered as <a> links)
      const fluxInstanceCard = screen.getByText('FluxInstance').closest('a')
      expect(fluxInstanceCard).toHaveClass('border-gray-300')
    })

    it('should show "not installed" badge for CRDs that are not installed', () => {
      render(<ReconcilersPanel reconcilers={[]} />)

      // All CRDs should show "not installed" badge
      const notInstalledBadges = screen.getAllByText('not installed')
      expect(notInstalledBadges.length).toBe(fluxCRDs.length)
    })

    it('should render not installed CRDs as links to documentation', () => {
      render(<ReconcilersPanel reconcilers={[]} />)

      // Not installed CRDs should be rendered as <a> elements linking to docs
      const fluxInstanceCard = screen.getByText('FluxInstance').closest('a')
      expect(fluxInstanceCard).toHaveAttribute('href', 'https://fluxcd.control-plane.io/operator/fluxinstance/')
      expect(fluxInstanceCard).toHaveAttribute('target', '_blank')
      expect(fluxInstanceCard).toHaveAttribute('rel', 'noopener noreferrer')
    })

    it('should not show "not installed" badge for installed CRDs', () => {
      const reconcilers = [
        {
          kind: 'Kustomization',
          apiVersion: 'kustomize.toolkit.fluxcd.io/v1',
          stats: { running: 5, failing: 0, suspended: 0 }
        }
      ]

      render(<ReconcilersPanel reconcilers={reconcilers} />)

      // Kustomization card should not have "not installed" badge (rendered as button)
      const kustomizationCard = screen.getByText('Kustomization').closest('button')
      expect(kustomizationCard).not.toHaveTextContent('not installed')

      // But other cards should still have it (rendered as links)
      const fluxInstanceCard = screen.getByText('FluxInstance').closest('a')
      expect(fluxInstanceCard).toHaveTextContent('not installed')
    })

    it('should show docs icon for not installed CRDs', () => {
      render(<ReconcilersPanel reconcilers={[]} />)

      // Not installed CRDs should show docs icon (as a span, not clickable link since whole card is link)
      const fluxInstanceCard = screen.getByText('FluxInstance').closest('a')
      const docIcon = fluxInstanceCard.querySelector('span.text-blue-500 svg')
      expect(docIcon).toBeInTheDocument()
    })

    it('should apply success border for installed CRDs with zero resources', () => {
      // CRD is installed (in API response) but has 0 running/failing/suspended
      const reconcilers = [
        {
          kind: 'FluxInstance',
          apiVersion: 'fluxcd.controlplane.io/v1',
          stats: { running: 0, failing: 0, suspended: 0 }
        }
      ]

      render(<ReconcilersPanel reconcilers={reconcilers} />)

      // Installed CRD with zero stats should have success border (rendered as link to docs)
      const fluxInstanceCard = screen.getByText('FluxInstance').closest('a')
      expect(fluxInstanceCard).toHaveClass('border-success')
    })

    it('should show "no resources" badge for installed CRDs with zero resources', () => {
      const reconcilers = [
        {
          kind: 'FluxInstance',
          apiVersion: 'fluxcd.controlplane.io/v1',
          stats: { running: 0, failing: 0, suspended: 0 }
        }
      ]

      render(<ReconcilersPanel reconcilers={reconcilers} />)

      // Installed CRD with zero stats should show "no resources" badge (rendered as link)
      const fluxInstanceCard = screen.getByText('FluxInstance').closest('a')
      expect(fluxInstanceCard).toHaveTextContent('no resources')
      expect(fluxInstanceCard).not.toHaveTextContent('not installed')
    })

    it('should render installed CRDs with no resources as links to documentation', () => {
      const reconcilers = [
        {
          kind: 'FluxInstance',
          apiVersion: 'fluxcd.controlplane.io/v1',
          stats: { running: 0, failing: 0, suspended: 0 }
        }
      ]

      render(<ReconcilersPanel reconcilers={reconcilers} />)

      // Installed CRD with zero resources should be rendered as <a> linking to docs
      const fluxInstanceCard = screen.getByText('FluxInstance').closest('a')
      expect(fluxInstanceCard).toHaveAttribute('href', 'https://fluxcd.control-plane.io/operator/fluxinstance/')
      expect(fluxInstanceCard).toHaveAttribute('target', '_blank')
    })

    it('should not show "no resources" badge for CRDs with resources', () => {
      const reconcilers = [
        {
          kind: 'Kustomization',
          apiVersion: 'kustomize.toolkit.fluxcd.io/v1',
          stats: { running: 5, failing: 0, suspended: 0 }
        }
      ]

      render(<ReconcilersPanel reconcilers={reconcilers} />)

      const kustomizationCard = screen.getByText('Kustomization').closest('button')
      expect(kustomizationCard).not.toHaveTextContent('no resources')
      expect(kustomizationCard).toHaveTextContent('5 running')
    })
  })

  describe('Edge Cases', () => {
    it('should handle undefined stats gracefully', () => {
      const reconcilers = [
        {
          kind: 'GitRepository',
          apiVersion: 'source.toolkit.fluxcd.io/v1',
          stats: { running: undefined, failing: undefined, suspended: undefined }
        }
      ]

      render(<ReconcilersPanel reconcilers={reconcilers} />)

      // Should render with total of 0 (as link since no resources)
      const card = screen.getByText('GitRepository').closest('a')
      expect(card).toHaveTextContent('0')
    })

    it('should handle empty reconcilers array', () => {
      render(<ReconcilersPanel reconcilers={[]} />)

      expect(screen.getByText('Flux Reconcilers')).toBeInTheDocument()
      // Shows 0 CRDs when none are installed, and 0 resources
      expect(screen.getByText(/0 CRDs/)).toBeInTheDocument()
      expect(screen.getByText(/0 resources/)).toBeInTheDocument()
    })

    it('should URL encode kind when navigating', () => {
      render(<ReconcilersPanel reconcilers={mockReconcilers} />)

      // Click on a card (use a known CRD kind)
      const card = screen.getByText('Kustomization').closest('button')
      fireEvent.click(card)

      expect(mockRoute).toHaveBeenCalledWith('/resources?kind=Kustomization')
    })
  })
})
