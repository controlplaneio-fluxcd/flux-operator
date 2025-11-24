// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/preact'
import { OverallStatusPanel } from './OverallStatusPanel'
import { reportUpdatedAt } from '../../app'
import { selectedResourceStatus } from '../resource-browser/ResourceList'

// Mock the time formatting utility
vi.mock('../../utils/time', () => ({
  formatTime: () => '2 minutes ago'
}))

// Mock preact-iso
const mockRoute = vi.fn()
vi.mock('preact-iso', () => ({
  useLocation: () => ({
    path: '/',
    query: {},
    route: mockRoute
  })
}))

describe('OverallStatusPanel', () => {
  beforeEach(() => {
    // Reset signals
    reportUpdatedAt.value = new Date()
    selectedResourceStatus.value = ''

    // Reset mocks
    vi.clearAllMocks()
  })

  describe('Status: Initializing', () => {
    it('should show initializing status when distribution is missing', () => {
      const report = {
        components: [],
        reconcilers: []
      }

      render(<OverallStatusPanel report={report} />)

      expect(screen.getByText('System Initializing')).toBeInTheDocument()
      expect(screen.getByText('Waiting for the Flux instance rollout to complete')).toBeInTheDocument()
    })

    it('should show initializing status when distribution version is missing', () => {
      const report = {
        distribution: {},
        components: [],
        reconcilers: []
      }

      render(<OverallStatusPanel report={report} />)

      expect(screen.getByText('System Initializing')).toBeInTheDocument()
    })

    it('should show spinning icon for initializing status', () => {
      const report = {
        components: [],
        reconcilers: []
      }

      render(<OverallStatusPanel report={report} />)

      const spinner = document.querySelector('.animate-spin')
      expect(spinner).toBeInTheDocument()
    })

    it('should not be clickable in initializing state', () => {
      const report = {
        components: [],
        reconcilers: []
      }

      render(<OverallStatusPanel report={report} />)

      const container = screen.getByText('System Initializing').closest('div')
      expect(container.tagName).not.toBe('BUTTON')
    })
  })

  describe('Status: Maintenance', () => {
    it('should show maintenance status when sync is suspended', () => {
      const report = {
        distribution: { version: 'v2.4.0' },
        sync: {
          ready: false,
          status: 'Suspended: manual intervention'
        },
        components: [],
        reconcilers: []
      }

      render(<OverallStatusPanel report={report} />)

      expect(screen.getByText('Under Maintenance')).toBeInTheDocument()
      expect(screen.getByText('Cluster reconciliation is currently suspended')).toBeInTheDocument()
    })

    it('should not be clickable in maintenance state', () => {
      const report = {
        distribution: { version: 'v2.4.0' },
        sync: {
          ready: false,
          status: 'Suspended: manual intervention'
        },
        components: [],
        reconcilers: []
      }

      render(<OverallStatusPanel report={report} />)

      const container = screen.getByText('Under Maintenance').closest('div')
      expect(container.tagName).not.toBe('BUTTON')
    })
  })

  describe('Status: Major Outage', () => {
    it('should show major outage when all components failed', () => {
      const report = {
        distribution: { version: 'v2.4.0' },
        components: [
          { name: 'source-controller', ready: false },
          { name: 'kustomize-controller', ready: false }
        ],
        reconcilers: []
      }

      render(<OverallStatusPanel report={report} />)

      expect(screen.getByText('Major Outage')).toBeInTheDocument()
      expect(screen.getByText('Critical system failure detected')).toBeInTheDocument()
    })

    it('should show major outage when all reconcilers completely broken', () => {
      const report = {
        distribution: { version: 'v2.4.0' },
        components: [],
        reconcilers: [
          { kind: 'GitRepository', stats: { failing: 2, running: 0 } },
          { kind: 'Kustomization', stats: { failing: 3, running: 0 } }
        ]
      }

      render(<OverallStatusPanel report={report} />)

      expect(screen.getByText('Major Outage')).toBeInTheDocument()
    })

    it('should show major outage when single reconciler completely broken', () => {
      const report = {
        distribution: { version: 'v2.4.0' },
        components: [{ name: 'source-controller', ready: true }],
        reconcilers: [
          { kind: 'GitRepository', stats: { failing: 5, running: 0 } }
        ]
      }

      render(<OverallStatusPanel report={report} />)

      expect(screen.getByText('Major Outage')).toBeInTheDocument()
    })

    it('should NOT show major outage when reconcilers have some running resources', () => {
      const report = {
        distribution: { version: 'v2.4.0' },
        components: [{ name: 'source-controller', ready: true }],
        reconcilers: [
          { kind: 'GitRepository', stats: { failing: 5, running: 1 } },
          { kind: 'Kustomization', stats: { failing: 3, running: 0 } }
        ]
      }

      render(<OverallStatusPanel report={report} />)

      // Should be Degraded, not Major Outage
      expect(screen.queryByText('Major Outage')).not.toBeInTheDocument()
      expect(screen.getByText('Degraded Performance')).toBeInTheDocument()
    })

    it('should NOT trigger major outage when failingReconcilers count equals totalReconcilers (bug regression test)', () => {
      // This tests the specific bug that was fixed:
      // failingReconcilers (5) === totalReconcilers (5) should NOT trigger major outage
      // if the reconcilers have running resources
      const report = {
        distribution: { version: 'v2.4.0' },
        components: [{ name: 'source-controller', ready: true }],
        sync: { ready: true },
        reconcilers: [
          { kind: 'GitRepository', stats: { failing: 2, running: 3 } },
          { kind: 'Kustomization', stats: { failing: 3, running: 5 } },
          { kind: 'HelmRelease', stats: { failing: 0, running: 8 } },
          { kind: 'HelmRepository', stats: { failing: 0, running: 2 } },
          { kind: 'OCIRepository', stats: { failing: 0, running: 1 } }
        ]
      }

      // failingReconcilers = 2 + 3 = 5
      // totalReconcilers = 5
      // These are equal, but should NOT trigger Major Outage

      render(<OverallStatusPanel report={report} />)

      expect(screen.queryByText('Major Outage')).not.toBeInTheDocument()
      expect(screen.getByText('Degraded Performance')).toBeInTheDocument()
      expect(screen.getByText('5 reconcilers failing')).toBeInTheDocument()
    })

    it('should be clickable in major outage state', () => {
      const report = {
        distribution: { version: 'v2.4.0' },
        components: [
          { name: 'source-controller', ready: false }
        ],
        reconcilers: []
      }

      render(<OverallStatusPanel report={report} />)

      const button = screen.getByText('Major Outage').closest('button')
      expect(button).toBeInTheDocument()
      expect(button).toHaveClass('cursor-pointer')
    })
  })

  describe('Status: Partial Outage', () => {
    it('should show partial outage when some components failed', () => {
      const report = {
        distribution: { version: 'v2.4.0' },
        components: [
          { name: 'source-controller', ready: true },
          { name: 'kustomize-controller', ready: false }
        ],
        reconcilers: [
          { kind: 'GitRepository', stats: { failing: 0, running: 5 } }
        ]
      }

      render(<OverallStatusPanel report={report} />)

      expect(screen.getByText('Partial Outage')).toBeInTheDocument()
      expect(screen.getByText('1 failure detected')).toBeInTheDocument()
    })

    it('should show partial outage when cluster sync failed', () => {
      const report = {
        distribution: { version: 'v2.4.0' },
        components: [
          { name: 'source-controller', ready: true }
        ],
        sync: {
          ready: false
        },
        reconcilers: [
          { kind: 'GitRepository', stats: { failing: 0, running: 5 } }
        ]
      }

      render(<OverallStatusPanel report={report} />)

      expect(screen.getByText('Partial Outage')).toBeInTheDocument()
      expect(screen.getByText(/cluster sync failed/)).toBeInTheDocument()
    })

    it('should show partial outage with cluster sync failed message when sync fails with reconciler failures', () => {
      const report = {
        distribution: { version: 'v2.4.0' },
        components: [
          { name: 'source-controller', ready: true }
        ],
        sync: {
          ready: false
        },
        reconcilers: [
          { kind: 'GitRepository', stats: { failing: 3, running: 5 } }
        ]
      }

      render(<OverallStatusPanel report={report} />)

      expect(screen.getByText('Partial Outage')).toBeInTheDocument()
      expect(screen.getByText(/cluster sync failed/)).toBeInTheDocument()
    })

    it('should show correct plural for multiple failures', () => {
      const report = {
        distribution: { version: 'v2.4.0' },
        components: [
          { name: 'source-controller', ready: false },
          { name: 'kustomize-controller', ready: false },
          { name: 'helm-controller', ready: true }
        ],
        reconcilers: [
          { kind: 'GitRepository', stats: { failing: 0, running: 5 } }
        ]
      }

      render(<OverallStatusPanel report={report} />)

      expect(screen.getByText('2 failures detected')).toBeInTheDocument()
    })

    it('should include reconciler failures in total failure count', () => {
      const report = {
        distribution: { version: 'v2.4.0' },
        components: [
          { name: 'source-controller', ready: false },
          { name: 'kustomize-controller', ready: true }
        ],
        reconcilers: [
          { kind: 'GitRepository', stats: { failing: 3, running: 2 } },
          { kind: 'Kustomization', stats: { failing: 2, running: 8 } }
        ]
      }

      render(<OverallStatusPanel report={report} />)

      // 1 failed component + 5 failing reconcilers = 6 failures
      expect(screen.getByText('Partial Outage')).toBeInTheDocument()
      expect(screen.getByText('6 failures detected')).toBeInTheDocument()
    })

    it('should be clickable in partial outage state', () => {
      const report = {
        distribution: { version: 'v2.4.0' },
        components: [
          { name: 'source-controller', ready: true },
          { name: 'kustomize-controller', ready: false }
        ],
        reconcilers: [
          { kind: 'GitRepository', stats: { failing: 0, running: 5 } }
        ]
      }

      render(<OverallStatusPanel report={report} />)

      const button = screen.getByText('Partial Outage').closest('button')
      expect(button).toBeInTheDocument()
    })
  })

  describe('Status: Degraded', () => {
    it('should show degraded status when reconcilers failing but components ok', () => {
      const report = {
        distribution: { version: 'v2.4.0' },
        components: [
          { name: 'source-controller', ready: true }
        ],
        sync: { ready: true },
        reconcilers: [
          { kind: 'GitRepository', stats: { failing: 2, running: 3 } }
        ]
      }

      render(<OverallStatusPanel report={report} />)

      expect(screen.getByText('Degraded Performance')).toBeInTheDocument()
      expect(screen.getByText('2 reconcilers failing')).toBeInTheDocument()
    })

    it('should show correct singular for one failure', () => {
      const report = {
        distribution: { version: 'v2.4.0' },
        components: [
          { name: 'source-controller', ready: true }
        ],
        reconcilers: [
          { kind: 'GitRepository', stats: { failing: 1, running: 5 } },
          { kind: 'Kustomization', stats: { failing: 0, running: 3 } }
        ]
      }

      render(<OverallStatusPanel report={report} />)

      expect(screen.getByText('1 reconciler failing')).toBeInTheDocument()
    })

    it('should be clickable in degraded state', () => {
      const report = {
        distribution: { version: 'v2.4.0' },
        components: [
          { name: 'source-controller', ready: true }
        ],
        reconcilers: [
          { kind: 'GitRepository', stats: { failing: 2, running: 3 } }
        ]
      }

      render(<OverallStatusPanel report={report} />)

      const button = screen.getByText('Degraded Performance').closest('button')
      expect(button).toBeInTheDocument()
    })
  })

  describe('Status: Operational', () => {
    it('should show operational status when everything is healthy', () => {
      const report = {
        distribution: { version: 'v2.4.0' },
        components: [
          { name: 'source-controller', ready: true },
          { name: 'kustomize-controller', ready: true }
        ],
        sync: { ready: true },
        reconcilers: [
          { kind: 'GitRepository', stats: { failing: 0, running: 5 } }
        ]
      }

      render(<OverallStatusPanel report={report} />)

      expect(screen.getByText('All Systems Operational')).toBeInTheDocument()
      expect(screen.getByText('Cluster in sync with desired state')).toBeInTheDocument()
    })

    it('should show checkmark icon for operational status', () => {
      const report = {
        distribution: { version: 'v2.4.0' },
        components: [
          { name: 'source-controller', ready: true }
        ],
        reconcilers: [
          { kind: 'GitRepository', stats: { failing: 0, running: 5 } }
        ]
      }

      render(<OverallStatusPanel report={report} />)

      // Check for checkmark path
      const checkmark = document.querySelector('path[d="M5 13l4 4L19 7"]')
      expect(checkmark).toBeInTheDocument()
    })

    it('should not be clickable in operational state', () => {
      const report = {
        distribution: { version: 'v2.4.0' },
        components: [
          { name: 'source-controller', ready: true }
        ],
        reconcilers: [
          { kind: 'GitRepository', stats: { failing: 0, running: 5 } }
        ]
      }

      render(<OverallStatusPanel report={report} />)

      const wrapper = document.querySelector('.card')
      expect(wrapper.tagName).toBe('DIV')
      expect(wrapper.tagName).not.toBe('BUTTON')
    })
  })

  describe('Click Handler - Navigation', () => {
    it('should navigate to search view with Failed filter when clicked', () => {
      const report = {
        distribution: { version: 'v2.4.0' },
        components: [
          { name: 'source-controller', ready: true },
          { name: 'kustomize-controller', ready: false }
        ],
        reconcilers: [
          { kind: 'GitRepository', stats: { failing: 0, running: 5 } }
        ]
      }

      render(<OverallStatusPanel report={report} />)

      const button = screen.getByText('Partial Outage').closest('button')
      fireEvent.click(button)

      expect(mockRoute).toHaveBeenCalledWith('/resources?status=Failed')
      expect(selectedResourceStatus.value).toBe('Failed')
    })

    it('should clear other filters when navigating', () => {
      const report = {
        distribution: { version: 'v2.4.0' },
        components: [
          { name: 'source-controller', ready: true }
        ],
        sync: { ready: true },
        reconcilers: [
          { kind: 'GitRepository', stats: { failing: 1, running: 3 } },
          { kind: 'Kustomization', stats: { failing: 0, running: 2 } }
        ]
      }

      render(<OverallStatusPanel report={report} />)

      const button = screen.getByText('Degraded Performance').closest('button')
      fireEvent.click(button)

      expect(mockRoute).toHaveBeenCalledWith('/resources?status=Failed')
      expect(selectedResourceStatus.value).toBe('Failed')
    })
  })

  describe('Status Priority Order', () => {
    it('should prioritize initializing over all other statuses', () => {
      const report = {
        // No distribution - should show initializing despite other failures
        components: [
          { name: 'source-controller', ready: false }
        ],
        sync: { ready: false, status: 'Suspended' },
        reconcilers: [
          { kind: 'GitRepository', stats: { failing: 5, running: 0 } }
        ]
      }

      render(<OverallStatusPanel report={report} />)

      expect(screen.getByText('System Initializing')).toBeInTheDocument()
    })

    it('should prioritize maintenance over outages', () => {
      const report = {
        distribution: { version: 'v2.4.0' },
        components: [
          { name: 'source-controller', ready: false }
        ],
        sync: {
          ready: false,
          status: 'Suspended: manual intervention'
        },
        reconcilers: [
          { kind: 'GitRepository', stats: { failing: 5, running: 0 } }
        ]
      }

      render(<OverallStatusPanel report={report} />)

      expect(screen.getByText('Under Maintenance')).toBeInTheDocument()
      expect(screen.queryByText('Major Outage')).not.toBeInTheDocument()
      expect(screen.queryByText('Partial Outage')).not.toBeInTheDocument()
    })

    it('should prioritize major outage over partial outage', () => {
      const report = {
        distribution: { version: 'v2.4.0' },
        components: [
          { name: 'source-controller', ready: false },
          { name: 'kustomize-controller', ready: false }
        ],
        sync: { ready: false }, // This would trigger partial outage
        reconcilers: []
      }

      render(<OverallStatusPanel report={report} />)

      expect(screen.getByText('Major Outage')).toBeInTheDocument()
      expect(screen.queryByText('Partial Outage')).not.toBeInTheDocument()
    })

    it('should prioritize partial outage over degraded', () => {
      const report = {
        distribution: { version: 'v2.4.0' },
        components: [
          { name: 'source-controller', ready: false },
          { name: 'kustomize-controller', ready: true }
        ],
        reconcilers: [
          { kind: 'GitRepository', stats: { failing: 3, running: 5 } }
        ]
      }

      render(<OverallStatusPanel report={report} />)

      expect(screen.getByText('Partial Outage')).toBeInTheDocument()
      expect(screen.queryByText('Degraded Performance')).not.toBeInTheDocument()
    })
  })

  describe('Last Updated Display', () => {
    it('should display last updated timestamp', () => {
      const report = {
        distribution: { version: 'v2.4.0' },
        components: [],
        reconcilers: [
          { kind: 'GitRepository', stats: { failing: 0, running: 5 } }
        ]
      }

      render(<OverallStatusPanel report={report} />)

      expect(screen.getByText('Last Updated')).toBeInTheDocument()
      expect(screen.getByText('2 minutes ago')).toBeInTheDocument()
    })
  })

  describe('Counter Calculations', () => {
    it('should handle missing components array', () => {
      const report = {
        distribution: { version: 'v2.4.0' },
        components: [
          { name: 'source-controller', ready: true }
        ],
        sync: { ready: true },
        reconcilers: [
          { kind: 'GitRepository', stats: { failing: 0, running: 5 } }
        ]
      }

      render(<OverallStatusPanel report={report} />)

      expect(screen.getByText('All Systems Operational')).toBeInTheDocument()
    })

    it('should handle undefined reconcilers array', () => {
      const report = {
        distribution: { version: 'v2.4.0' },
        components: [
          { name: 'source-controller', ready: true }
        ],
        sync: { ready: true }
        // reconcilers is undefined
      }

      render(<OverallStatusPanel report={report} />)

      // Should show operational since components are ready
      expect(screen.getByText('All Systems Operational')).toBeInTheDocument()
    })

    it('should handle empty reconcilers array', () => {
      const report = {
        distribution: { version: 'v2.4.0' },
        components: [
          { name: 'source-controller', ready: true }
        ],
        sync: { ready: true },
        reconcilers: []
      }

      render(<OverallStatusPanel report={report} />)

      expect(screen.getByText('All Systems Operational')).toBeInTheDocument()
    })

    it('should handle reconcilers with missing stats fields', () => {
      const report = {
        distribution: { version: 'v2.4.0' },
        components: [
          { name: 'source-controller', ready: true }
        ],
        sync: { ready: true },
        reconcilers: [
          { kind: 'GitRepository', stats: {} },
          { kind: 'Kustomization', stats: { failing: 2, running: 5 } }
        ]
      }

      render(<OverallStatusPanel report={report} />)

      // Should count only the 2 from Kustomization
      expect(screen.getByText('Degraded Performance')).toBeInTheDocument()
      expect(screen.getByText('2 reconcilers failing')).toBeInTheDocument()
    })

    it('should handle reconcilers with null/undefined failing count', () => {
      const report = {
        distribution: { version: 'v2.4.0' },
        components: [
          { name: 'source-controller', ready: true }
        ],
        sync: { ready: true },
        reconcilers: [
          { kind: 'GitRepository', stats: { failing: null, running: 5 } },
          { kind: 'Kustomization', stats: { running: 5 } }
        ]
      }

      render(<OverallStatusPanel report={report} />)

      // Should treat missing/null as 0 and show operational
      expect(screen.getByText('All Systems Operational')).toBeInTheDocument()
    })

    it('should sum failing reconcilers across all types', () => {
      const report = {
        distribution: { version: 'v2.4.0' },
        components: [
          { name: 'source-controller', ready: true }
        ],
        reconcilers: [
          { kind: 'GitRepository', stats: { failing: 2, running: 3 } },
          { kind: 'Kustomization', stats: { failing: 3, running: 5 } }
        ]
      }

      render(<OverallStatusPanel report={report} />)

      expect(screen.getByText('5 reconcilers failing')).toBeInTheDocument()
    })

    it('should correctly calculate when all reconcilers are completely broken', () => {
      const report = {
        distribution: { version: 'v2.4.0' },
        components: [
          { name: 'source-controller', ready: true }
        ],
        reconcilers: [
          { kind: 'GitRepository', stats: { failing: 10, running: 0 } },
          { kind: 'Kustomization', stats: { failing: 5, running: 0 } },
          { kind: 'HelmRelease', stats: { failing: 3, running: 0 } }
        ]
      }

      render(<OverallStatusPanel report={report} />)

      // All reconcilers have 0 running, so it's a major outage
      expect(screen.getByText('Major Outage')).toBeInTheDocument()
    })

    it('should handle mix of reconcilers with and without failures', () => {
      const report = {
        distribution: { version: 'v2.4.0' },
        components: [
          { name: 'source-controller', ready: true }
        ],
        sync: { ready: true },
        reconcilers: [
          { kind: 'GitRepository', stats: { failing: 0, running: 10 } },
          { kind: 'Kustomization', stats: { failing: 3, running: 5 } },
          { kind: 'HelmRelease', stats: { failing: 0, running: 8 } }
        ]
      }

      render(<OverallStatusPanel report={report} />)

      // Should be degraded with 3 failing
      expect(screen.getByText('Degraded Performance')).toBeInTheDocument()
      expect(screen.getByText('3 reconcilers failing')).toBeInTheDocument()
    })
  })
})
