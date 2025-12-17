// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/preact'
import { HistoryTimeline } from './HistoryTimeline'

describe('HistoryTimeline component', () => {
  describe('Empty history', () => {
    it('should display "No history available" when history is empty', () => {
      render(<HistoryTimeline history={[]} kind="FluxInstance" />)
      expect(screen.getByText('No history available')).toBeInTheDocument()
    })

    it('should display "No history available" when history is null', () => {
      render(<HistoryTimeline history={null} kind="FluxInstance" />)
      expect(screen.getByText('No history available')).toBeInTheDocument()
    })

    it('should display "No history available" when history is undefined', () => {
      render(<HistoryTimeline history={undefined} kind="FluxInstance" />)
      expect(screen.getByText('No history available')).toBeInTheDocument()
    })
  })

  describe('FluxInstance history', () => {
    const fluxHistory = [
      {
        digest: 'sha256:bb1f3f3f4f4e5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2b3c4d5e6f7a8b9c0d',
        firstReconciled: '2025-11-06T21:30:00Z',
        lastReconciled: '2025-11-18T11:10:00Z',
        lastReconciledDuration: '100.123456ms',
        lastReconciledStatus: 'ReconciliationSucceeded',
        metadata: {
          flux: 'v2.7.3'
        },
        totalReconciliations: 300
      },
      {
        digest: 'sha256:aa1b2c3d4e5f60718293a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2b3c4d5',
        firstReconciled: '2025-11-05T20:15:30Z',
        lastReconciled: '2025-11-06T21:25:45Z',
        lastReconciledDuration: '200.654321ms',
        lastReconciledStatus: 'ReconciliationFailed',
        metadata: {
          flux: 'v2.7.2'
        },
        totalReconciliations: 50
      }
    ]

    it('should render FluxInstance history entries', () => {
      render(<HistoryTimeline history={fluxHistory} kind="FluxInstance" />)

      expect(screen.getByText('ReconciliationSucceeded')).toBeInTheDocument()
      expect(screen.getByText('ReconciliationFailed')).toBeInTheDocument()
    })

    it('should display Flux version for FluxInstance', () => {
      render(<HistoryTimeline history={fluxHistory} kind="FluxInstance" />)

      expect(screen.getAllByText(/Flux:/).length).toBeGreaterThan(0)
      expect(screen.getByText('v2.7.3')).toBeInTheDocument()
      expect(screen.getByText('v2.7.2')).toBeInTheDocument()
    })

    it('should display duration and reconciliation count', () => {
      render(<HistoryTimeline history={fluxHistory} kind="FluxInstance" />)

      expect(screen.getAllByText(/Duration:/).length).toBeGreaterThan(0)
      expect(screen.getByText('100.123456ms')).toBeInTheDocument()
      expect(screen.getAllByText(/Reconciliations:/).length).toBeGreaterThan(0)
      expect(screen.getByText('300')).toBeInTheDocument()
      expect(screen.getByText('50')).toBeInTheDocument()
    })

    it('should display truncated digest', () => {
      render(<HistoryTimeline history={fluxHistory} kind="FluxInstance" />)

      expect(screen.getAllByText(/Digest:/).length).toBeGreaterThan(0)
      expect(screen.getByText('bb1f3f3f4f4e')).toBeInTheDocument()
    })

    it('should show green badge for succeeded status', () => {
      render(<HistoryTimeline history={fluxHistory} kind="FluxInstance" />)

      const successBadge = screen.getByText('ReconciliationSucceeded')
      expect(successBadge).toHaveClass('bg-green-100')
    })

    it('should show red badge for failed status', () => {
      render(<HistoryTimeline history={fluxHistory} kind="FluxInstance" />)

      const failedBadge = screen.getByText('ReconciliationFailed')
      expect(failedBadge).toHaveClass('bg-red-100')
    })
  })

  describe('ResourceSet history', () => {
    const resourceSetHistory = [
      {
        digest: 'sha256:5ffcfb1437cd080bdb2666161275b38461bb115c75117f6f40a5eb07347b989b',
        firstReconciled: '2025-11-17T23:00:11Z',
        lastReconciled: '2025-11-18T11:39:39Z',
        lastReconciledDuration: '49.805625ms',
        lastReconciledStatus: 'ReconciliationSucceeded',
        metadata: {
          inputs: '1',
          resources: '4'
        },
        totalReconciliations: 14
      }
    ]

    it('should render ResourceSet history entries', () => {
      render(<HistoryTimeline history={resourceSetHistory} kind="ResourceSet" />)

      expect(screen.getByText('ReconciliationSucceeded')).toBeInTheDocument()
    })

    it('should display resources count for ResourceSet', () => {
      render(<HistoryTimeline history={resourceSetHistory} kind="ResourceSet" />)

      expect(screen.getByText(/Resources:/)).toBeInTheDocument()
      expect(screen.getByText('4')).toBeInTheDocument()
    })

    it('should display reconciliations count', () => {
      render(<HistoryTimeline history={resourceSetHistory} kind="ResourceSet" />)

      expect(screen.getByText(/Reconciliations:/)).toBeInTheDocument()
      expect(screen.getByText('14')).toBeInTheDocument()
    })
  })

  describe('Kustomization history', () => {
    const kustomizationHistory = [
      {
        digest: 'sha256:bbe7aa022b513c7ceb4cf38e9fee0cec579c96fee9bf15afcdeff34bf4eed934',
        firstReconciled: '2025-11-06T21:36:41Z',
        lastReconciled: '2025-11-18T11:09:21Z',
        lastReconciledDuration: '76.890708ms',
        lastReconciledStatus: 'ReconciliationSucceeded',
        metadata: {
          revision: 'refs/heads/main@sha1:d676e33990dc2865d67c022d26dea93d5e3236ff'
        },
        totalReconciliations: 279
      },
      {
        digest: 'sha256:bbe7aa022b513c7ceb4cf38e9fee0cec579c96fee9bf15afcdeff34bf4eed934',
        firstReconciled: '2025-11-11T13:27:39Z',
        lastReconciled: '2025-11-11T14:15:40Z',
        lastReconciledDuration: '6m0.068181246s',
        lastReconciledStatus: 'HealthCheckFailed',
        metadata: {
          revision: 'refs/heads/main@sha1:d676e33990dc2865d67c022d26dea93d5e3236ff'
        },
        totalReconciliations: 7
      }
    ]

    it('should render Kustomization history entries', () => {
      render(<HistoryTimeline history={kustomizationHistory} kind="Kustomization" />)

      expect(screen.getByText('ReconciliationSucceeded')).toBeInTheDocument()
      expect(screen.getByText('HealthCheckFailed')).toBeInTheDocument()
    })

    it('should display revision for Kustomization', () => {
      render(<HistoryTimeline history={kustomizationHistory} kind="Kustomization" />)

      expect(screen.getAllByText(/Revision:/).length).toBeGreaterThan(0)
      expect(screen.getAllByText(/refs\/heads\/main@sha1:d676e33990dc2865d67c022d26dea93d5e3236ff/).length).toBe(2)
    })

    it('should show red badge for HealthCheckFailed status', () => {
      render(<HistoryTimeline history={kustomizationHistory} kind="Kustomization" />)

      const failedBadge = screen.getByText('HealthCheckFailed')
      expect(failedBadge).toHaveClass('bg-red-100')
    })
  })

  describe('HelmRelease history', () => {
    const helmReleaseHistory = [
      {
        appVersion: 'v1.90.9',
        chartName: 'tailscale-operator',
        chartVersion: '1.90.9',
        configDigest: 'sha256:ec864259c2bedeada53e194919f2416a1d6e742b4d5beb3555037ecce7c634d1',
        digest: 'sha256:f00bb59fbdb28284e525bd4d85aea707e728b89653f2acfa6d98cef3b93e28d5',
        firstDeployed: '2025-11-01T23:31:01Z',
        lastDeployed: '2025-11-26T19:26:23Z',
        name: 'tailscale-operator',
        namespace: 'tailscale',
        status: 'deployed',
        version: 3
      },
      {
        appVersion: 'v1.90.8',
        chartName: 'tailscale-operator',
        chartVersion: '1.90.8',
        configDigest: 'sha256:ec864259c2bedeada53e194919f2416a1d6e742b4d5beb3555037ecce7c634d1',
        digest: 'sha256:0d1e61ba399cbc6b8df9e625cb70009ec90a444d2293c280616884cee2dac132',
        firstDeployed: '2025-11-01T23:31:01Z',
        lastDeployed: '2025-11-19T22:22:04Z',
        name: 'tailscale-operator',
        namespace: 'tailscale',
        status: 'superseded',
        version: 2
      }
    ]

    it('should render HelmRelease history entries', () => {
      render(<HistoryTimeline history={helmReleaseHistory} kind="HelmRelease" />)

      expect(screen.getByText('Deployed')).toBeInTheDocument()
      expect(screen.getByText('Superseded')).toBeInTheDocument()
    })

    it('should display chart version for HelmRelease', () => {
      render(<HistoryTimeline history={helmReleaseHistory} kind="HelmRelease" />)

      expect(screen.getAllByText(/Chart Version:/).length).toBeGreaterThan(0)
      expect(screen.getByText('1.90.9')).toBeInTheDocument()
      expect(screen.getByText('1.90.8')).toBeInTheDocument()
    })

    it('should display app version for HelmRelease', () => {
      render(<HistoryTimeline history={helmReleaseHistory} kind="HelmRelease" />)

      expect(screen.getAllByText(/App Version:/).length).toBeGreaterThan(0)
      expect(screen.getByText('v1.90.9')).toBeInTheDocument()
      expect(screen.getByText('v1.90.8')).toBeInTheDocument()
    })

    it('should display version count instead of reconciliations', () => {
      render(<HistoryTimeline history={helmReleaseHistory} kind="HelmRelease" />)

      expect(screen.getAllByText(/Version:/).length).toBeGreaterThan(0)
      expect(screen.getByText('3')).toBeInTheDocument()
      expect(screen.getByText('2')).toBeInTheDocument()
      expect(screen.queryByText(/Reconciliations:/)).not.toBeInTheDocument()
    })

    it('should NOT display digest for HelmRelease', () => {
      render(<HistoryTimeline history={helmReleaseHistory} kind="HelmRelease" />)

      expect(screen.queryByText(/Digest:/)).not.toBeInTheDocument()
    })

    it('should show green badge for deployed status', () => {
      render(<HistoryTimeline history={helmReleaseHistory} kind="HelmRelease" />)

      const deployedBadge = screen.getByText('Deployed')
      expect(deployedBadge).toHaveClass('bg-green-100')
    })

    it('should show yellow badge for superseded status', () => {
      render(<HistoryTimeline history={helmReleaseHistory} kind="HelmRelease" />)

      const supersededBadge = screen.getByText('Superseded')
      expect(supersededBadge).toHaveClass('bg-yellow-100')
    })
  })

  describe('Time formatting', () => {
    it('should display single time when first and last are the same', () => {
      const history = [
        {
          digest: 'sha256:test',
          firstReconciled: '2025-11-06T21:35:43Z',
          lastReconciled: '2025-11-06T21:35:43Z',
          lastReconciledDuration: '5.222284961s',
          lastReconciledStatus: 'ReconciliationSucceeded',
          metadata: {},
          totalReconciliations: 1
        }
      ]

      render(<HistoryTimeline history={history} kind="FluxInstance" />)

      // Should have only one timestamp, not two with arrow
      const timestamps = screen.queryAllByText(/→/)
      expect(timestamps.length).toBe(0)
    })

    it('should display time range when first and last differ', () => {
      const history = [
        {
          digest: 'sha256:test',
          firstReconciled: '2025-11-06T21:30:00Z',
          lastReconciled: '2025-11-18T11:10:00Z',
          lastReconciledDuration: '100ms',
          lastReconciledStatus: 'ReconciliationSucceeded',
          metadata: {},
          totalReconciliations: 10
        }
      ]

      render(<HistoryTimeline history={history} kind="FluxInstance" />)

      // Should have arrow indicating time range
      expect(screen.getByText(/→/)).toBeInTheDocument()
    })
  })

  describe('Timeline markers', () => {
    it('should render timeline start marker (gray dot) at the end', () => {
      const history = [
        {
          digest: 'sha256:test',
          firstReconciled: '2025-11-06T21:30:00Z',
          lastReconciled: '2025-11-18T11:10:00Z',
          lastReconciledDuration: '100ms',
          lastReconciledStatus: 'ReconciliationSucceeded',
          metadata: { flux: 'v2.7.3' },
          totalReconciliations: 10
        }
      ]

      const { container } = render(<HistoryTimeline history={history} kind="FluxInstance" />)

      // Check for gray dot (timeline start marker)
      const grayDots = container.querySelectorAll('.bg-gray-300')
      expect(grayDots.length).toBeGreaterThan(0)
    })
  })

  describe('Status badge colors with contains check', () => {
    it('should show green for any status containing "succe"', () => {
      const history = [
        {
          digest: 'sha256:test',
          firstReconciled: '2025-11-06T21:30:00Z',
          lastReconciled: '2025-11-18T11:10:00Z',
          lastReconciledDuration: '100ms',
          lastReconciledStatus: 'CustomSucceeded',
          metadata: {},
          totalReconciliations: 10
        }
      ]

      render(<HistoryTimeline history={history} kind="FluxInstance" />)

      const badge = screen.getByText('CustomSucceeded')
      expect(badge).toHaveClass('bg-green-100')
    })

    it('should show red for any status containing "failed"', () => {
      const history = [
        {
          digest: 'sha256:test',
          firstReconciled: '2025-11-06T21:30:00Z',
          lastReconciled: '2025-11-18T11:10:00Z',
          lastReconciledDuration: '100ms',
          lastReconciledStatus: 'BuildFailed',
          metadata: {},
          totalReconciliations: 10
        }
      ]

      render(<HistoryTimeline history={history} kind="FluxInstance" />)

      const badge = screen.getByText('BuildFailed')
      expect(badge).toHaveClass('bg-red-100')
    })

    it('should show yellow for any other status', () => {
      const history = [
        {
          digest: 'sha256:test',
          firstReconciled: '2025-11-06T21:30:00Z',
          lastReconciled: '2025-11-18T11:10:00Z',
          lastReconciledDuration: '100ms',
          lastReconciledStatus: 'Progressing',
          metadata: {},
          totalReconciliations: 10
        }
      ]

      render(<HistoryTimeline history={history} kind="FluxInstance" />)

      const badge = screen.getByText('Progressing')
      expect(badge).toHaveClass('bg-yellow-100')
    })
  })

  describe('Digest truncation', () => {
    it('should truncate digest to 12 characters', () => {
      const history = [
        {
          digest: 'sha256:bb1f3f3f4f4e5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2b3c4d5e6f7a8b9c0d',
          firstReconciled: '2025-11-06T21:30:00Z',
          lastReconciled: '2025-11-18T11:10:00Z',
          lastReconciledDuration: '100ms',
          lastReconciledStatus: 'ReconciliationSucceeded',
          metadata: {},
          totalReconciliations: 10
        }
      ]

      render(<HistoryTimeline history={history} kind="FluxInstance" />)

      // Should show first 12 characters after sha256:
      expect(screen.getByText('bb1f3f3f4f4e')).toBeInTheDocument()
      // Should NOT show full digest
      expect(screen.queryByText('bb1f3f3f4f4e5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2b3c4d5e6f7a8b9c0d')).not.toBeInTheDocument()
    })
  })
})
