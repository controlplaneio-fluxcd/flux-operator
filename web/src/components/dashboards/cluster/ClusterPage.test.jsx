// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen } from '@testing-library/preact'
import { ClusterPage } from './ClusterPage'

// Mock all child components
vi.mock('./OverallStatusPanel', () => ({
  OverallStatusPanel: ({ report }) => <div data-testid="cluster-status">ClusterStatus: {report.distribution.version}</div>
}))

vi.mock('./InfoPanel', () => ({
  InfoPanel: ({ cluster, operator }) => <div data-testid="cluster-info">ClusterInfo: {cluster.name} - {operator.version}</div>
}))

vi.mock('./SyncPanel', () => ({
  SyncPanel: ({ sync }) => <div data-testid="cluster-sync">ClusterSync: {sync.interval}</div>
}))

vi.mock('./ControllersPanel', () => ({
  ControllersPanel: ({ components }) => <div data-testid="component-list">ComponentList: {components.length} components</div>
}))

vi.mock('./ReconcilersPanel', () => ({
  ReconcilersPanel: ({ reconcilers }) => <div data-testid="reconciler-list">ReconcilerList: {reconcilers.length} reconcilers</div>
}))

vi.mock('../../layout/Footer', () => ({
  Footer: () => <div data-testid="footer">Footer</div>
}))

describe('ClusterPage', () => {
  const baseSpec = {
    distribution: {
      version: 'v2.4.0',
      ready: true
    },
    cluster: {
      name: 'production'
    }
  }

  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('Required Components', () => {
    it('should always render ClusterStatus', () => {
      render(<ClusterPage spec={baseSpec} />)

      expect(screen.getByTestId('cluster-status')).toBeInTheDocument()
      expect(screen.getByText(/ClusterStatus: v2.4.0/)).toBeInTheDocument()
    })

    it('should always render Footer', () => {
      render(<ClusterPage spec={baseSpec} />)

      expect(screen.getByTestId('footer')).toBeInTheDocument()
    })
  })

  describe('Conditional Components - ClusterInfo', () => {
    it('should render ClusterInfo when operator exists', () => {
      const spec = {
        ...baseSpec,
        operator: {
          version: 'v1.0.0'
        },
        components: [],
        metrics: []
      }

      render(<ClusterPage spec={spec} />)

      expect(screen.getByTestId('cluster-info')).toBeInTheDocument()
      expect(screen.getByText(/ClusterInfo: production - v1.0.0/)).toBeInTheDocument()
    })

    it('should not render ClusterInfo when operator is missing', () => {
      render(<ClusterPage spec={baseSpec} />)

      expect(screen.queryByTestId('cluster-info')).not.toBeInTheDocument()
    })

    it('should pass correct props to ClusterInfo', () => {
      const spec = {
        ...baseSpec,
        operator: {
          version: 'v1.0.0'
        },
        distribution: {
          version: 'v2.4.0'
        },
        components: [{ name: 'source-controller' }],
        metrics: [{ pod: 'source-controller-abc' }]
      }

      render(<ClusterPage spec={spec} />)

      const clusterInfo = screen.getByTestId('cluster-info')
      expect(clusterInfo).toBeInTheDocument()
    })
  })

  describe('Conditional Components - ClusterSync', () => {
    it('should render ClusterSync when sync exists', () => {
      const spec = {
        ...baseSpec,
        sync: {
          interval: '5m'
        }
      }

      render(<ClusterPage spec={spec} />)

      expect(screen.getByTestId('cluster-sync')).toBeInTheDocument()
      expect(screen.getByText(/ClusterSync: 5m/)).toBeInTheDocument()
    })

    it('should not render ClusterSync when sync is missing', () => {
      render(<ClusterPage spec={baseSpec} />)

      expect(screen.queryByTestId('cluster-sync')).not.toBeInTheDocument()
    })
  })

  describe('Conditional Components - ComponentList', () => {
    it('should render ComponentList when components exist', () => {
      const spec = {
        ...baseSpec,
        components: [
          { name: 'source-controller' },
          { name: 'kustomize-controller' }
        ],
        metrics: []
      }

      render(<ClusterPage spec={spec} />)

      expect(screen.getByTestId('component-list')).toBeInTheDocument()
      expect(screen.getByText(/ComponentList: 2 components/)).toBeInTheDocument()
    })

    it('should not render ComponentList when components is null', () => {
      const spec = {
        ...baseSpec,
        components: null
      }

      render(<ClusterPage spec={spec} />)

      expect(screen.queryByTestId('component-list')).not.toBeInTheDocument()
    })

    it('should not render ComponentList when components array is empty', () => {
      const spec = {
        ...baseSpec,
        components: []
      }

      render(<ClusterPage spec={spec} />)

      expect(screen.queryByTestId('component-list')).not.toBeInTheDocument()
    })
  })

  describe('Conditional Components - ReconcilerList', () => {
    it('should render ReconcilerList when reconcilers exist', () => {
      const spec = {
        ...baseSpec,
        reconcilers: [
          { kind: 'GitRepository' },
          { kind: 'Kustomization' },
          { kind: 'HelmRelease' }
        ]
      }

      render(<ClusterPage spec={spec} />)

      expect(screen.getByTestId('reconciler-list')).toBeInTheDocument()
      expect(screen.getByText(/ReconcilerList: 3 reconcilers/)).toBeInTheDocument()
    })

    it('should not render ReconcilerList when reconcilers is null', () => {
      const spec = {
        ...baseSpec,
        reconcilers: null
      }

      render(<ClusterPage spec={spec} />)

      expect(screen.queryByTestId('reconciler-list')).not.toBeInTheDocument()
    })

    it('should not render ReconcilerList when reconcilers array is empty', () => {
      const spec = {
        ...baseSpec,
        reconcilers: []
      }

      render(<ClusterPage spec={spec} />)

      expect(screen.queryByTestId('reconciler-list')).not.toBeInTheDocument()
    })
  })

  describe('Full Dashboard Rendering', () => {
    it('should render all components when all data is present', () => {
      const fullSpec = {
        distribution: {
          version: 'v2.4.0',
          ready: true
        },
        cluster: {
          name: 'production'
        },
        operator: {
          version: 'v1.0.0'
        },
        sync: {
          interval: '5m'
        },
        components: [
          { name: 'source-controller' }
        ],
        reconcilers: [
          { kind: 'GitRepository' }
        ],
        metrics: []
      }

      render(<ClusterPage spec={fullSpec} />)

      expect(screen.getByTestId('cluster-status')).toBeInTheDocument()
      expect(screen.getByTestId('cluster-info')).toBeInTheDocument()
      expect(screen.getByTestId('cluster-sync')).toBeInTheDocument()
      expect(screen.getByTestId('component-list')).toBeInTheDocument()
      expect(screen.getByTestId('reconciler-list')).toBeInTheDocument()
      expect(screen.getByTestId('footer')).toBeInTheDocument()
    })

    it('should render minimal dashboard with only required components', () => {
      render(<ClusterPage spec={baseSpec} />)

      // Only ClusterStatus and Footer should render
      expect(screen.getByTestId('cluster-status')).toBeInTheDocument()
      expect(screen.getByTestId('footer')).toBeInTheDocument()

      // All optional components should not render
      expect(screen.queryByTestId('cluster-info')).not.toBeInTheDocument()
      expect(screen.queryByTestId('cluster-sync')).not.toBeInTheDocument()
      expect(screen.queryByTestId('component-list')).not.toBeInTheDocument()
      expect(screen.queryByTestId('reconciler-list')).not.toBeInTheDocument()
    })
  })

  describe('Layout', () => {
    it('should render main container with proper styling', () => {
      render(<ClusterPage spec={baseSpec} />)

      const main = document.querySelector('main')
      expect(main).toBeInTheDocument()
      expect(main).toHaveClass('max-w-7xl')
      expect(main).toHaveClass('mx-auto')
    })

    it('should render components in space-y-6 container', () => {
      render(<ClusterPage spec={baseSpec} />)

      const spacer = document.querySelector('.space-y-6')
      expect(spacer).toBeInTheDocument()
    })
  })
})
