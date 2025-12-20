// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/preact'
import userEvent from '@testing-library/user-event'
import { GraphTabContent, buildGraphData } from './GraphTabContent'

describe('buildGraphData function', () => {
  it('should extract source from resourceData with all fields', () => {
    const resourceData = {
      kind: 'Kustomization',
      metadata: { name: 'test-ks', namespace: 'flux-system' },
      status: {
        reconcilerRef: { status: 'Ready' },
        sourceRef: {
          kind: 'GitRepository',
          name: 'flux-system',
          namespace: 'flux-system',
          status: 'Ready',
          url: 'https://github.com/example/repo'
        },
        inventory: []
      }
    }

    const result = buildGraphData(resourceData)

    expect(result.sources).toHaveLength(1)
    expect(result.sources[0]).toEqual({
      kind: 'GitRepository',
      name: 'flux-system',
      namespace: 'flux-system',
      status: 'Ready',
      isClickable: true,
      url: 'https://github.com/example/repo',
      accentBorder: false
    })
  })

  it('should handle missing source', () => {
    const resourceData = {
      kind: 'Kustomization',
      metadata: { name: 'test-ks', namespace: 'flux-system' },
      status: {
        reconcilerRef: { status: 'Ready' },
        inventory: []
      }
    }

    const result = buildGraphData(resourceData)

    expect(result.sources).toHaveLength(0)
  })

  it('should extract reconciler with revision from resourceData', () => {
    const resourceData = {
      kind: 'Kustomization',
      metadata: { name: 'test-ks', namespace: 'flux-system' },
      status: {
        reconcilerRef: { status: 'Ready' },
        lastAttemptedRevision: 'main@sha1:abc123',
        inventory: []
      }
    }

    const result = buildGraphData(resourceData)

    expect(result.reconciler).toEqual({
      kind: 'Kustomization',
      name: 'test-ks',
      namespace: 'flux-system',
      status: 'Ready',
      revision: 'main@sha1:abc123'
    })
  })

  it('should use lastAppliedRevision when lastAttemptedRevision is not present', () => {
    const resourceData = {
      kind: 'Kustomization',
      metadata: { name: 'test-ks', namespace: 'flux-system' },
      status: {
        reconcilerRef: { status: 'Ready' },
        lastAppliedRevision: 'main@sha1:def456',
        inventory: []
      }
    }

    const result = buildGraphData(resourceData)

    expect(result.reconciler.revision).toBe('main@sha1:def456')
  })

  it('should group Flux resources separately as array', () => {
    const resourceData = {
      kind: 'Kustomization',
      metadata: { name: 'test-ks', namespace: 'flux-system' },
      status: {
        reconcilerRef: { status: 'Ready' },
        inventory: [
          { kind: 'Kustomization', name: 'child-ks', namespace: 'default' },
          { kind: 'HelmRelease', name: 'nginx', namespace: 'default' },
          { kind: 'GitRepository', name: 'apps', namespace: 'flux-system' }
        ]
      }
    }

    const result = buildGraphData(resourceData)

    expect(result.inventory.flux).toHaveLength(3)
    expect(result.inventory.flux[0]).toEqual({ kind: 'Kustomization', name: 'child-ks', namespace: 'default' })
    expect(result.inventory.flux[1]).toEqual({ kind: 'HelmRelease', name: 'nginx', namespace: 'default' })
    expect(result.inventory.flux[2]).toEqual({ kind: 'GitRepository', name: 'apps', namespace: 'flux-system' })
  })

  it('should group workloads as array of items', () => {
    const resourceData = {
      kind: 'Kustomization',
      metadata: { name: 'test-ks', namespace: 'flux-system' },
      status: {
        reconcilerRef: { status: 'Ready' },
        inventory: [
          { kind: 'Deployment', name: 'app1', namespace: 'default' },
          { kind: 'Deployment', name: 'app2', namespace: 'default' },
          { kind: 'StatefulSet', name: 'db', namespace: 'default' }
        ]
      }
    }

    const result = buildGraphData(resourceData)

    expect(result.inventory.workloads).toHaveLength(3)
    expect(result.inventory.workloads[0]).toEqual({ kind: 'Deployment', name: 'app1', namespace: 'default' })
    expect(result.inventory.workloads[1]).toEqual({ kind: 'Deployment', name: 'app2', namespace: 'default' })
    expect(result.inventory.workloads[2]).toEqual({ kind: 'StatefulSet', name: 'db', namespace: 'default' })
  })

  it('should group other resources by kind with counts', () => {
    const resourceData = {
      kind: 'Kustomization',
      metadata: { name: 'test-ks', namespace: 'flux-system' },
      status: {
        reconcilerRef: { status: 'Ready' },
        inventory: [
          { kind: 'ConfigMap', name: 'cm1', namespace: 'default' },
          { kind: 'ConfigMap', name: 'cm2', namespace: 'default' },
          { kind: 'Secret', name: 'secret1', namespace: 'default' },
          { kind: 'Service', name: 'svc1', namespace: 'default' }
        ]
      }
    }

    const result = buildGraphData(resourceData)

    expect(result.inventory.resources).toEqual({
      ConfigMap: 2,
      Secret: 1,
      Service: 1
    })
  })

  it('should handle empty inventory', () => {
    const resourceData = {
      kind: 'Kustomization',
      metadata: { name: 'test-ks', namespace: 'flux-system' },
      status: {
        reconcilerRef: { status: 'Ready' },
        inventory: []
      }
    }

    const result = buildGraphData(resourceData)

    expect(result.inventory.flux).toHaveLength(0)
    expect(result.inventory.workloads).toHaveLength(0)
    expect(result.inventory.resources).toEqual({})
  })

  it('should handle missing inventory', () => {
    const resourceData = {
      kind: 'Kustomization',
      metadata: { name: 'test-ks', namespace: 'flux-system' },
      status: {
        reconcilerRef: { status: 'Ready' }
      }
    }

    const result = buildGraphData(resourceData)

    expect(result.inventory.flux).toHaveLength(0)
    expect(result.inventory.workloads).toHaveLength(0)
    expect(result.inventory.resources).toEqual({})
  })

  it('should create Distro source for FluxInstance', () => {
    const resourceData = {
      kind: 'FluxInstance',
      metadata: { name: 'flux', namespace: 'flux-system' },
      spec: {
        distribution: {
          registry: 'ghcr.io/fluxcd',
          version: 'v2.4.0'
        }
      },
      status: {
        reconcilerRef: { status: 'Ready' },
        inventory: []
      }
    }

    const result = buildGraphData(resourceData)

    expect(result.sources).toHaveLength(1)
    expect(result.sources[0]).toEqual({
      kind: 'Distro',
      name: 'Flux v2.4.0',
      namespace: null,
      status: 'Ready',
      isClickable: false,
      url: 'ghcr.io/fluxcd',
      accentBorder: true
    })
  })

  it('should handle FluxInstance without version', () => {
    const resourceData = {
      kind: 'FluxInstance',
      metadata: { name: 'flux', namespace: 'flux-system' },
      spec: {
        distribution: {
          registry: 'ghcr.io/fluxcd'
        }
      },
      status: {
        reconcilerRef: { status: 'Ready' },
        inventory: []
      }
    }

    const result = buildGraphData(resourceData)

    expect(result.sources).toHaveLength(1)
    expect(result.sources[0].name).toBe('Flux')
    expect(result.sources[0].url).toBe('ghcr.io/fluxcd')
  })

  it('should create multiple sources for ArtifactGenerator', () => {
    const resourceData = {
      kind: 'ArtifactGenerator',
      metadata: { name: 'platform', namespace: 'flux-system' },
      spec: {
        sources: [
          { kind: 'GitRepository', name: 'platform', namespace: 'flux-system' },
          { kind: 'OCIRepository', name: 'modules' }
        ]
      },
      status: {
        reconcilerRef: { status: 'Ready' },
        inventory: []
      }
    }

    const result = buildGraphData(resourceData)

    expect(result.sources).toHaveLength(2)
    expect(result.sources[0]).toEqual({
      kind: 'GitRepository',
      name: 'platform',
      namespace: 'flux-system',
      status: 'Unknown',
      isClickable: true,
      url: null,
      accentBorder: true
    })
    expect(result.sources[1]).toEqual({
      kind: 'OCIRepository',
      name: 'modules',
      namespace: 'flux-system', // defaults to ArtifactGenerator namespace
      status: 'Unknown',
      isClickable: true,
      url: null,
      accentBorder: true
    })
  })

  it('should handle ArtifactGenerator with empty sources array', () => {
    const resourceData = {
      kind: 'ArtifactGenerator',
      metadata: { name: 'test', namespace: 'flux-system' },
      spec: {
        sources: []
      },
      status: {
        reconcilerRef: { status: 'Ready' },
        inventory: []
      }
    }

    const result = buildGraphData(resourceData)

    expect(result.sources).toHaveLength(0)
  })

  it('should extract upstream from originURL', () => {
    const resourceData = {
      kind: 'Kustomization',
      metadata: { name: 'test-ks', namespace: 'flux-system' },
      status: {
        reconcilerRef: { status: 'Ready' },
        sourceRef: {
          kind: 'GitRepository',
          name: 'flux-system',
          namespace: 'flux-system',
          status: 'Ready',
          url: 'oci://ghcr.io/org/repo',
          originURL: 'https://github.com/org/my-repo.git'
        },
        inventory: []
      }
    }

    const result = buildGraphData(resourceData)

    expect(result.upstream).toEqual({
      kind: 'Upstream',
      name: 'my-repo',
      url: 'https://github.com/org/my-repo.git',
      isClickable: true,
      accentBorder: true
    })
  })

  it('should not have upstream when originURL is missing', () => {
    const resourceData = {
      kind: 'Kustomization',
      metadata: { name: 'test-ks', namespace: 'flux-system' },
      status: {
        reconcilerRef: { status: 'Ready' },
        sourceRef: {
          kind: 'GitRepository',
          name: 'flux-system',
          namespace: 'flux-system',
          status: 'Ready',
          url: 'https://github.com/example/repo'
        },
        inventory: []
      }
    }

    const result = buildGraphData(resourceData)

    expect(result.upstream).toBeNull()
  })

  it('should handle originURL with trailing slash', () => {
    const resourceData = {
      kind: 'Kustomization',
      metadata: { name: 'test-ks', namespace: 'flux-system' },
      status: {
        reconcilerRef: { status: 'Ready' },
        sourceRef: {
          kind: 'GitRepository',
          name: 'flux-system',
          namespace: 'flux-system',
          status: 'Ready',
          url: 'oci://ghcr.io/org/repo',
          originURL: 'https://github.com/org/my-repo/'
        },
        inventory: []
      }
    }

    const result = buildGraphData(resourceData)

    expect(result.upstream.name).toBe('my-repo')
  })

  it('should not make upstream clickable for non-https URLs', () => {
    const resourceData = {
      kind: 'Kustomization',
      metadata: { name: 'test-ks', namespace: 'flux-system' },
      status: {
        reconcilerRef: { status: 'Ready' },
        sourceRef: {
          kind: 'GitRepository',
          name: 'flux-system',
          namespace: 'flux-system',
          status: 'Ready',
          url: 'oci://ghcr.io/org/repo',
          originURL: 'ssh://git@github.com/org/my-repo.git'
        },
        inventory: []
      }
    }

    const result = buildGraphData(resourceData)

    expect(result.upstream.isClickable).toBe(false)
  })

  it('should extract HelmChart when source is HelmRepository', () => {
    const resourceData = {
      kind: 'HelmRelease',
      metadata: { name: 'nginx', namespace: 'default' },
      spec: {
        chart: {
          spec: {
            chart: 'nginx',
            version: '>=1.0.0'
          }
        }
      },
      status: {
        reconcilerRef: { status: 'Ready' },
        sourceRef: {
          kind: 'HelmRepository',
          name: 'bitnami',
          namespace: 'flux-system',
          status: 'Ready',
          url: 'https://charts.bitnami.com/bitnami'
        },
        helmChart: 'default/default-nginx',
        inventory: []
      }
    }

    const result = buildGraphData(resourceData)

    expect(result.helmChart).toEqual({
      kind: 'HelmChart',
      name: 'default-nginx',
      namespace: 'default',
      version: 'semver >=1.0.0',
      isClickable: true
    })
  })

  it('should not extract HelmChart when source is not HelmRepository', () => {
    const resourceData = {
      kind: 'HelmRelease',
      metadata: { name: 'cert-manager', namespace: 'cert-manager' },
      status: {
        reconcilerRef: { status: 'Ready' },
        sourceRef: {
          kind: 'OCIRepository',
          name: 'cert-manager',
          namespace: 'cert-manager',
          status: 'Ready'
        },
        inventory: []
      }
    }

    const result = buildGraphData(resourceData)

    expect(result.helmChart).toBeNull()
  })

  it('should handle HelmChart without version in spec', () => {
    const resourceData = {
      kind: 'HelmRelease',
      metadata: { name: 'nginx', namespace: 'default' },
      spec: {
        chart: {
          spec: {
            chart: 'nginx'
          }
        }
      },
      status: {
        reconcilerRef: { status: 'Ready' },
        sourceRef: {
          kind: 'HelmRepository',
          name: 'bitnami',
          namespace: 'flux-system',
          status: 'Ready'
        },
        helmChart: 'default/default-nginx',
        inventory: []
      }
    }

    const result = buildGraphData(resourceData)

    expect(result.helmChart.version).toBe('semver *')
  })
})

describe('GraphTabContent component', () => {
  const mockResourceData = {
    kind: 'Kustomization',
    metadata: { name: 'cluster-infra', namespace: 'flux-system' },
    status: {
      reconcilerRef: { status: 'Ready' },
      sourceRef: {
        kind: 'GitRepository',
        name: 'flux-system',
        namespace: 'flux-system',
        status: 'Ready',
        url: 'https://github.com/example/repo'
      },
      lastAttemptedRevision: 'main@sha1:abc123',
      inventory: [
        { kind: 'Kustomization', name: 'monitoring', namespace: 'flux-system' },
        { kind: 'HelmRelease', name: 'nginx', namespace: 'default' },
        { kind: 'Deployment', name: 'app1', namespace: 'default' },
        { kind: 'Deployment', name: 'app2', namespace: 'default' },
        { kind: 'Service', name: 'svc1', namespace: 'default' },
        { kind: 'ConfigMap', name: 'config', namespace: 'default' }
      ]
    }
  }

  it('should render the graph with source, reconciler, and inventory', () => {
    render(
      <GraphTabContent
        resourceData={mockResourceData}
        namespace="flux-system"
      />
    )

    // Check source is rendered (CSS uppercase transforms visually, but DOM has original case)
    expect(screen.getByText(/GitRepository/)).toBeInTheDocument()
    expect(screen.getByText('flux-system/flux-system')).toBeInTheDocument()

    // Check reconciler is rendered (Kustomization appears multiple times - reconciler and flux items)
    expect(screen.getAllByText(/Kustomization/).length).toBeGreaterThanOrEqual(1)

    // Check inventory groups are rendered (with arrows for clickable titles)
    // Text is split across nodes so use regex
    expect(screen.getByText(/Flux Resources \(2\)/)).toBeInTheDocument()
    expect(screen.getByText(/Workloads \(2\)/)).toBeInTheDocument()
    expect(screen.getByText(/^Resources \(2\)/)).toBeInTheDocument()
  })

  it('should render source URL as subtext', () => {
    render(
      <GraphTabContent
        resourceData={mockResourceData}
        namespace="flux-system"
      />
    )

    expect(screen.getByText('https://github.com/example/repo')).toBeInTheDocument()
  })

  it('should render reconciler revision as subtext', () => {
    render(
      <GraphTabContent
        resourceData={mockResourceData}
        namespace="flux-system"
      />
    )

    expect(screen.getByText('main@sha1:abc123')).toBeInTheDocument()
  })

  it('should render without source when not present', () => {
    const resourceDataNoSource = {
      ...mockResourceData,
      status: {
        ...mockResourceData.status,
        sourceRef: undefined
      }
    }

    render(
      <GraphTabContent
        resourceData={resourceDataNoSource}
        namespace="flux-system"
      />
    )

    // Reconciler should still be rendered (Kustomization appears multiple times)
    expect(screen.getAllByText(/Kustomization/).length).toBeGreaterThanOrEqual(1)

    // Source should not be present
    expect(screen.queryByText(/GitRepository/)).not.toBeInTheDocument()
  })

  it('should call onNavigate when clicking Flux resource item', async () => {
    const user = userEvent.setup()
    const onNavigate = vi.fn()

    render(
      <GraphTabContent
        resourceData={mockResourceData}
        namespace="flux-system"
        onNavigate={onNavigate}
      />
    )

    // Find and click the monitoring Kustomization item
    const monitoringLink = screen.getByText('monitoring →')
    await user.click(monitoringLink)

    expect(onNavigate).toHaveBeenCalledWith({
      kind: 'Kustomization',
      name: 'monitoring',
      namespace: 'flux-system'
    })
  })

  it('should call onNavigate when clicking source node', async () => {
    const user = userEvent.setup()
    const onNavigate = vi.fn()

    render(
      <GraphTabContent
        resourceData={mockResourceData}
        namespace="flux-system"
        onNavigate={onNavigate}
      />
    )

    // Find and click the source card
    const sourceCard = screen.getByText('flux-system/flux-system').closest('[role="button"]')
    await user.click(sourceCard)

    expect(onNavigate).toHaveBeenCalledWith({
      kind: 'GitRepository',
      name: 'flux-system',
      namespace: 'flux-system'
    })
  })

  it('should call setActiveTab when clicking Workloads title', async () => {
    const user = userEvent.setup()
    const setActiveTab = vi.fn()

    render(
      <GraphTabContent
        resourceData={mockResourceData}
        namespace="flux-system"
        setActiveTab={setActiveTab}
      />
    )

    // Find and click the Workloads title
    const workloadsTitle = screen.getByText('Workloads (2) →')
    await user.click(workloadsTitle)

    expect(setActiveTab).toHaveBeenCalledWith('workloads')
  })

  it('should call setActiveTab when clicking Resources title', async () => {
    const user = userEvent.setup()
    const setActiveTab = vi.fn()

    render(
      <GraphTabContent
        resourceData={mockResourceData}
        namespace="flux-system"
        setActiveTab={setActiveTab}
      />
    )

    // Find and click the Resources title
    const resourcesTitle = screen.getByText('Resources (2) →')
    await user.click(resourcesTitle)

    expect(setActiveTab).toHaveBeenCalledWith('inventory')
  })

  it('should not render empty inventory groups', () => {
    const resourceDataNoWorkloads = {
      kind: 'Kustomization',
      metadata: { name: 'test-ks', namespace: 'flux-system' },
      status: {
        reconcilerRef: { status: 'Ready' },
        inventory: [
          { kind: 'ConfigMap', name: 'config', namespace: 'default' }
        ]
      }
    }

    render(
      <GraphTabContent
        resourceData={resourceDataNoWorkloads}
        namespace="flux-system"
      />
    )

    // Flux Resources and Workloads groups should not be rendered
    expect(screen.queryByText(/Flux Resources \(/)).not.toBeInTheDocument()
    expect(screen.queryByText(/Workloads \(/)).not.toBeInTheDocument()

    // Resources group should be rendered (no arrow without setActiveTab)
    expect(screen.getByText(/Resources \(1\)/)).toBeInTheDocument()
  })

  it('should not render Resources group when no resources exist', () => {
    const resourceDataEmpty = {
      kind: 'Kustomization',
      metadata: { name: 'test-ks', namespace: 'flux-system' },
      status: {
        reconcilerRef: { status: 'Ready' },
        inventory: []
      }
    }

    render(
      <GraphTabContent
        resourceData={resourceDataEmpty}
        namespace="flux-system"
      />
    )

    // Resources group should not be rendered when empty
    expect(screen.queryByText(/Resources \(/)).not.toBeInTheDocument()
    expect(screen.queryByText('No resources')).not.toBeInTheDocument()
  })

  it('should render FluxInstance with Distro source', () => {
    const fluxInstanceData = {
      kind: 'FluxInstance',
      metadata: { name: 'flux', namespace: 'flux-system' },
      spec: {
        distribution: {
          registry: 'ghcr.io/fluxcd',
          version: 'v2.4.0'
        }
      },
      status: {
        reconcilerRef: { status: 'Ready' },
        inventory: []
      }
    }

    render(
      <GraphTabContent
        resourceData={fluxInstanceData}
        namespace="flux-system"
      />
    )

    // Check Distro source is rendered (CSS uppercase transforms visually)
    expect(screen.getByText('Distro')).toBeInTheDocument()
    expect(screen.getByText('Flux v2.4.0')).toBeInTheDocument()
    expect(screen.getByText('ghcr.io/fluxcd')).toBeInTheDocument()
  })

  it('should have test id for graph content', () => {
    render(
      <GraphTabContent
        resourceData={mockResourceData}
        namespace="flux-system"
      />
    )

    expect(screen.getByTestId('graph-tab-content')).toBeInTheDocument()
  })

  it('should display workload items individually', () => {
    render(
      <GraphTabContent
        resourceData={mockResourceData}
        namespace="flux-system"
      />
    )

    // Check workload items are rendered individually
    expect(screen.getByText('app1')).toBeInTheDocument()
    expect(screen.getByText('app2')).toBeInTheDocument()
  })

  it('should display resource counts alphabetically', () => {
    const resourceDataWithMultipleKinds = {
      kind: 'Kustomization',
      metadata: { name: 'test-ks', namespace: 'flux-system' },
      status: {
        reconcilerRef: { status: 'Ready' },
        inventory: [
          { kind: 'Service', name: 'svc1', namespace: 'default' },
          { kind: 'ConfigMap', name: 'cm1', namespace: 'default' },
          { kind: 'Secret', name: 'secret1', namespace: 'default' }
        ]
      }
    }

    render(
      <GraphTabContent
        resourceData={resourceDataWithMultipleKinds}
        namespace="flux-system"
      />
    )

    // All resource kinds should be visible
    expect(screen.getByText('ConfigMap')).toBeInTheDocument()
    expect(screen.getByText('Secret')).toBeInTheDocument()
    expect(screen.getByText('Service')).toBeInTheDocument()
  })

  it('should render ArtifactGenerator with multiple sources', () => {
    const artifactGeneratorData = {
      kind: 'ArtifactGenerator',
      metadata: { name: 'platform', namespace: 'flux-system' },
      spec: {
        sources: [
          { kind: 'GitRepository', name: 'platform', namespace: 'flux-system' },
          { kind: 'OCIRepository', name: 'modules' }
        ]
      },
      status: {
        reconcilerRef: { status: 'Ready' },
        inventory: []
      }
    }

    render(
      <GraphTabContent
        resourceData={artifactGeneratorData}
        namespace="flux-system"
      />
    )

    // Check both sources are rendered (desktop + mobile views)
    expect(screen.getAllByText('GitRepository').length).toBeGreaterThanOrEqual(1)
    expect(screen.getAllByText('flux-system/platform').length).toBeGreaterThanOrEqual(1)
    expect(screen.getAllByText('OCIRepository').length).toBeGreaterThanOrEqual(1)
    expect(screen.getAllByText('flux-system/modules').length).toBeGreaterThanOrEqual(1)

    // Check reconciler is rendered (desktop + mobile views)
    expect(screen.getAllByText('ArtifactGenerator').length).toBeGreaterThanOrEqual(1)
  })

  it('should call onNavigate when clicking ArtifactGenerator source', async () => {
    const user = userEvent.setup()
    const onNavigate = vi.fn()

    const artifactGeneratorData = {
      kind: 'ArtifactGenerator',
      metadata: { name: 'platform', namespace: 'flux-system' },
      spec: {
        sources: [
          { kind: 'GitRepository', name: 'platform', namespace: 'flux-system' }
        ]
      },
      status: {
        reconcilerRef: { status: 'Ready' },
        inventory: []
      }
    }

    render(
      <GraphTabContent
        resourceData={artifactGeneratorData}
        namespace="flux-system"
        onNavigate={onNavigate}
      />
    )

    // Find and click the source card (get first one from desktop view)
    const sourceCards = screen.getAllByText('flux-system/platform')
    const sourceCard = sourceCards[0].closest('[role="button"]')
    await user.click(sourceCard)

    expect(onNavigate).toHaveBeenCalledWith({
      kind: 'GitRepository',
      name: 'platform',
      namespace: 'flux-system'
    })
  })

  it('should render upstream node when originURL is present', () => {
    const resourceDataWithUpstream = {
      kind: 'Kustomization',
      metadata: { name: 'cluster-infra', namespace: 'flux-system' },
      status: {
        reconcilerRef: { status: 'Ready' },
        sourceRef: {
          kind: 'OCIRepository',
          name: 'platform',
          namespace: 'flux-system',
          status: 'Ready',
          url: 'oci://ghcr.io/org/platform',
          originURL: 'https://github.com/org/platform-repo.git'
        },
        inventory: []
      }
    }

    render(
      <GraphTabContent
        resourceData={resourceDataWithUpstream}
        namespace="flux-system"
      />
    )

    // Check upstream node is rendered
    expect(screen.getByText('Upstream')).toBeInTheDocument()
    expect(screen.getByText('platform-repo')).toBeInTheDocument()
    expect(screen.getByText('https://github.com/org/platform-repo.git')).toBeInTheDocument()

    // Check source node is also rendered
    expect(screen.getByText('OCIRepository')).toBeInTheDocument()
  })

  it('should show namespace in Flux Resources when items have different namespaces', () => {
    const resourceDataMixedNs = {
      kind: 'Kustomization',
      metadata: { name: 'cluster-infra', namespace: 'flux-system' },
      status: {
        reconcilerRef: { status: 'Ready' },
        inventory: [
          { kind: 'Kustomization', name: 'monitoring', namespace: 'flux-system' },
          { kind: 'HelmRelease', name: 'nginx', namespace: 'web-apps' }
        ]
      }
    }

    render(
      <GraphTabContent
        resourceData={resourceDataMixedNs}
        namespace="flux-system"
      />
    )

    // Both namespaces should be visible since they differ
    expect(screen.getByText('flux-system')).toBeInTheDocument()
    expect(screen.getByText('web-apps')).toBeInTheDocument()
  })

  it('should not show namespace in Flux Resources when all items have same namespace', () => {
    const resourceDataSameNs = {
      kind: 'Kustomization',
      metadata: { name: 'cluster-infra', namespace: 'flux-system' },
      status: {
        reconcilerRef: { status: 'Ready' },
        inventory: [
          { kind: 'Kustomization', name: 'monitoring', namespace: 'flux-system' },
          { kind: 'HelmRelease', name: 'nginx', namespace: 'flux-system' }
        ]
      }
    }

    render(
      <GraphTabContent
        resourceData={resourceDataSameNs}
        namespace="flux-system"
      />
    )

    // The items should just show names, not namespaces
    expect(screen.getByText('monitoring →')).toBeInTheDocument()
    expect(screen.getByText('nginx →')).toBeInTheDocument()

    // The namespace text should NOT appear as a separate line under items
    // (it only appears in the reconciler card title which has namespace/name format)
    const fluxResourcesPanel = screen.getByText('Flux Resources (2)').closest('div')
    // Within the Flux Resources panel, there should be no standalone namespace text
    expect(fluxResourcesPanel.querySelector('.text-gray-400')).toBeNull()
  })

  it('should render HelmChart node between source and reconciler', () => {
    const helmReleaseData = {
      kind: 'HelmRelease',
      metadata: { name: 'nginx', namespace: 'default' },
      spec: {
        chart: {
          spec: {
            chart: 'nginx',
            version: '>=1.0.0'
          }
        }
      },
      status: {
        reconcilerRef: { status: 'Ready' },
        sourceRef: {
          kind: 'HelmRepository',
          name: 'bitnami',
          namespace: 'flux-system',
          status: 'Ready',
          url: 'https://charts.bitnami.com/bitnami'
        },
        helmChart: 'default/default-nginx',
        inventory: []
      }
    }

    render(
      <GraphTabContent
        resourceData={helmReleaseData}
        namespace="default"
      />
    )

    // Check HelmChart node is rendered
    expect(screen.getByText('HelmChart')).toBeInTheDocument()
    expect(screen.getByText('default/default-nginx')).toBeInTheDocument()
    expect(screen.getByText('semver >=1.0.0')).toBeInTheDocument()

    // Check source and reconciler are also rendered
    expect(screen.getByText('HelmRepository')).toBeInTheDocument()
    expect(screen.getByText('HelmRelease')).toBeInTheDocument()
  })

  it('should call onNavigate when clicking HelmChart node', async () => {
    const user = userEvent.setup()
    const onNavigate = vi.fn()

    const helmReleaseData = {
      kind: 'HelmRelease',
      metadata: { name: 'nginx', namespace: 'default' },
      spec: {
        chart: {
          spec: {
            chart: 'nginx',
            version: '*'
          }
        }
      },
      status: {
        reconcilerRef: { status: 'Ready' },
        sourceRef: {
          kind: 'HelmRepository',
          name: 'bitnami',
          namespace: 'flux-system',
          status: 'Ready'
        },
        helmChart: 'default/default-nginx',
        inventory: []
      }
    }

    render(
      <GraphTabContent
        resourceData={helmReleaseData}
        namespace="default"
        onNavigate={onNavigate}
      />
    )

    // Find and click the HelmChart card
    const helmChartCard = screen.getByText('default/default-nginx').closest('[role="button"]')
    await user.click(helmChartCard)

    expect(onNavigate).toHaveBeenCalledWith({
      kind: 'HelmChart',
      name: 'default-nginx',
      namespace: 'default'
    })
  })

  it('should not render HelmChart when source is not HelmRepository', () => {
    const helmReleaseWithOCI = {
      kind: 'HelmRelease',
      metadata: { name: 'cert-manager', namespace: 'cert-manager' },
      status: {
        reconcilerRef: { status: 'Ready' },
        sourceRef: {
          kind: 'OCIRepository',
          name: 'cert-manager',
          namespace: 'cert-manager',
          status: 'Ready'
        },
        inventory: []
      }
    }

    render(
      <GraphTabContent
        resourceData={helmReleaseWithOCI}
        namespace="cert-manager"
      />
    )

    // HelmChart should not be rendered
    expect(screen.queryByText('HelmChart')).not.toBeInTheDocument()

    // Source and reconciler should be rendered
    expect(screen.getByText('OCIRepository')).toBeInTheDocument()
    expect(screen.getByText('HelmRelease')).toBeInTheDocument()
  })
})
