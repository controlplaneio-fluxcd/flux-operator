// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/preact'
import userEvent from '@testing-library/user-event'
import { ResourceView } from './ResourceView'
import { fetchWithMock } from '../../utils/fetch'

// Mock the fetch utility
vi.mock('../../utils/fetch', () => ({
  fetchWithMock: vi.fn()
}))

// Mock preact-iso
const mockRoute = vi.fn()
vi.mock('preact-iso', () => ({
  useLocation: () => ({
    path: '/resources',
    query: {},
    route: mockRoute
  })
}))

describe('ResourceView component', () => {
  const mockResourceData = {
    apiVersion: 'kustomize.toolkit.fluxcd.io/v1',
    kind: 'Kustomization',
    metadata: {
      name: 'apps',
      namespace: 'flux-system'
    },
    spec: {
      interval: '10m',
      path: './apps',
      prune: true,
      sourceRef: {
        kind: 'GitRepository',
        name: 'flux-system'
      }
    },
    status: {
      inventory: [
        { apiVersion: 'v1', kind: 'Namespace', name: 'production' },
        { apiVersion: 'v1', kind: 'ConfigMap', namespace: 'production', name: 'app-config' },
        { apiVersion: 'apps/v1', kind: 'Deployment', namespace: 'production', name: 'app' }
      ]
    }
  }

  const mockResourceDataNoInventory = {
    apiVersion: 'source.toolkit.fluxcd.io/v1',
    kind: 'GitRepository',
    metadata: {
      name: 'flux-system',
      namespace: 'flux-system'
    },
    spec: {
      interval: '1m',
      url: 'https://github.com/example/repo',
      ref: {
        branch: 'main'
      }
    },
    status: {}
  }

  beforeEach(() => {
    vi.clearAllMocks()
    mockRoute.mockClear()
  })

  it('should render nothing when isExpanded is false', () => {
    const { container } = render(
      <ResourceView
        kind="Kustomization"
        name="apps"
        namespace="flux-system"
        isExpanded={false}
      />
    )

    expect(container.firstChild).toBeNull()
  })

  it('should fetch resource data when expanded for the first time', async () => {
    fetchWithMock.mockResolvedValue(mockResourceData)

    render(
      <ResourceView
        kind="Kustomization"
        name="apps"
        namespace="flux-system"
        isExpanded={true}
      />
    )

    await waitFor(() => {
      expect(fetchWithMock).toHaveBeenCalledWith({
        endpoint: '/api/v1/resource?kind=Kustomization&name=apps&namespace=flux-system',
        mockPath: '../mock/resource',
        mockExport: 'getMockResource'
      })
    })
  })

  it('should show loading state while fetching', async () => {
    let resolvePromise
    const promise = new Promise((resolve) => { resolvePromise = resolve })
    fetchWithMock.mockReturnValue(promise)

    render(
      <ResourceView
        kind="Kustomization"
        name="apps"
        namespace="flux-system"
        isExpanded={true}
      />
    )

    // Should show loading spinner
    expect(screen.getByText('Loading details...')).toBeInTheDocument()
    expect(document.querySelector('.animate-spin')).toBeInTheDocument()

    // Resolve the promise
    resolvePromise(mockResourceData)

    // Wait for loading to complete
    await waitFor(() => {
      expect(screen.queryByText('Loading details...')).not.toBeInTheDocument()
    })
  })

  it('should display specification tab as highlighted YAML after loading', async () => {
    fetchWithMock.mockResolvedValue(mockResourceData)
    const user = userEvent.setup()

    render(
      <ResourceView
        kind="Kustomization"
        name="apps"
        namespace="flux-system"
        isExpanded={true}
      />
    )

    await waitFor(() => {
      expect(screen.getByText('Specification')).toBeInTheDocument()
    })

    // Click on Specification tab (Inventory is default when present)
    const specTab = screen.getByText('Specification')
    await user.click(specTab)

    // Check that specification content is present
    await waitFor(() => {
      const codeElement = document.querySelector('.language-yaml')
      expect(codeElement).toBeInTheDocument()

      // Check for apiVersion, kind, metadata, and spec fields in the rendered YAML
      expect(codeElement.innerHTML).toContain('apiVersion')
      expect(codeElement.innerHTML).toContain('kustomize.toolkit.fluxcd.io/v1')
      expect(codeElement.innerHTML).toContain('kind')
      expect(codeElement.innerHTML).toContain('Kustomization')
      expect(codeElement.innerHTML).toContain('metadata')
      expect(codeElement.innerHTML).toContain('interval')
      expect(codeElement.innerHTML).toContain('10m')
      expect(codeElement.innerHTML).toContain('path')
      expect(codeElement.innerHTML).toContain('./apps')
    })
  })

  it('should display status tab with YAML content', async () => {
    fetchWithMock.mockResolvedValue(mockResourceData)
    const user = userEvent.setup()

    render(
      <ResourceView
        kind="Kustomization"
        name="apps"
        namespace="flux-system"
        isExpanded={true}
      />
    )

    await waitFor(() => {
      expect(screen.getByText('Status')).toBeInTheDocument()
    })

    // Click on Status tab
    const statusTab = screen.getByText('Status')
    await user.click(statusTab)

    // Check that status tab displays YAML with correct fields
    await waitFor(() => {
      const textContent = document.body.textContent
      expect(textContent).toContain('apiVersion: kustomize.toolkit.fluxcd.io/v1')
      expect(textContent).toContain('kind: Kustomization')
      expect(textContent).toContain('name: apps')
      expect(textContent).toContain('namespace: flux-system')
      expect(textContent).toContain('status:')
    })

    // Verify inventory is not in the status YAML
    const textContent = document.body.textContent
    // The inventory should not appear in the Status tab YAML (it's removed)
    expect(textContent).not.toContain('inventory:')
  })

  it('should display inventory tab grouped by API version when present', async () => {
    fetchWithMock.mockResolvedValue(mockResourceData)
    const user = userEvent.setup()

    render(
      <ResourceView
        kind="Kustomization"
        name="apps"
        namespace="flux-system"
        isExpanded={true}
      />
    )

    // Wait for inventory tab to appear
    const inventoryTab = await screen.findByText(/Inventory \(3\)/)
    expect(inventoryTab).toBeInTheDocument()

    // Click on Inventory tab
    await user.click(inventoryTab)

    // Wait for inventory content to appear and check for API version groups
    await waitFor(() => {
      const textContent = document.body.textContent
      expect(textContent).toContain('v1')
      expect(textContent).toContain('apps/v1')
      expect(textContent).toContain('production')
      expect(textContent).toContain('app-config')
      expect(textContent).toContain('Namespace')
      expect(textContent).toContain('ConfigMap')
      expect(textContent).toContain('Deployment')
    })
  })

  it('should not show inventory tab if resource has no inventory', async () => {
    fetchWithMock.mockResolvedValue(mockResourceDataNoInventory)

    render(
      <ResourceView
        kind="GitRepository"
        name="flux-system"
        namespace="flux-system"
        isExpanded={true}
      />
    )

    await waitFor(() => {
      expect(screen.getByText('Specification')).toBeInTheDocument()
    })

    // Inventory tab should not be present
    expect(screen.queryByText(/Inventory/)).not.toBeInTheDocument()

    // Only Specification and Status tabs should be visible
    expect(screen.getByText('Specification')).toBeInTheDocument()
    expect(screen.getByText('Status')).toBeInTheDocument()
  })

  it('should cache resource data and not refetch on re-expand', async () => {
    fetchWithMock.mockResolvedValue(mockResourceData)

    const { rerender } = render(
      <ResourceView
        kind="Kustomization"
        name="apps"
        namespace="flux-system"
        isExpanded={true}
      />
    )

    // Wait for initial fetch
    await waitFor(() => {
      expect(fetchWithMock).toHaveBeenCalledTimes(1)
    })

    // Collapse
    rerender(
      <ResourceView
        kind="Kustomization"
        name="apps"
        namespace="flux-system"
        isExpanded={false}
      />
    )

    // Re-expand
    rerender(
      <ResourceView
        kind="Kustomization"
        name="apps"
        namespace="flux-system"
        isExpanded={true}
      />
    )

    // Should still only have been called once (cached)
    expect(fetchWithMock).toHaveBeenCalledTimes(1)

    // Data should still be displayed with tabs
    await waitFor(() => {
      expect(screen.getByText('Specification')).toBeInTheDocument()
      expect(screen.getByText('Status')).toBeInTheDocument()
    })
  })

  it('should show error state when fetch fails', async () => {
    const errorMessage = 'Network connection failed'
    fetchWithMock.mockRejectedValue(new Error(errorMessage))

    render(
      <ResourceView
        kind="Kustomization"
        name="apps"
        namespace="flux-system"
        isExpanded={true}
      />
    )

    await waitFor(() => {
      expect(screen.getByText(`Failed to load details: ${errorMessage}`)).toBeInTheDocument()
    })

    // Should show error styling
    const errorContainer = screen.getByText(`Failed to load details: ${errorMessage}`).closest('div')
    expect(errorContainer).toHaveClass('bg-red-50')
  })

  it('should handle empty spec gracefully', async () => {
    const resourceWithEmptySpec = {
      apiVersion: 'v1',
      kind: 'Test',
      metadata: {
        name: 'test',
        namespace: 'default'
      },
      spec: {},
      status: {}
    }

    fetchWithMock.mockResolvedValue(resourceWithEmptySpec)

    render(
      <ResourceView
        kind="Test"
        name="test"
        namespace="default"
        isExpanded={true}
      />
    )

    await waitFor(() => {
      expect(screen.getByText('Specification')).toBeInTheDocument()
    })

    // Should show specification with empty spec in YAML
    const codeElement = document.querySelector('.language-yaml')
    expect(codeElement).toBeInTheDocument()
  })

  it('should sort inventory by apiVersion with priorities', async () => {
    const resourceWithMixedApiVersions = {
      apiVersion: 'kustomize.toolkit.fluxcd.io/v1',
      kind: 'Kustomization',
      metadata: {
        name: 'infrastructure',
        namespace: 'flux-system'
      },
      spec: { interval: '10m' },
      status: {
        inventory: [
          { apiVersion: 'custom.io/v1', kind: 'Custom', name: 'custom1' },
          { apiVersion: 'apiextensions.k8s.io/v1', kind: 'CustomResourceDefinition', name: 'crd1' },
          { apiVersion: 'apps/v1', kind: 'Deployment', namespace: 'default', name: 'app' },
          { apiVersion: 'v1', kind: 'Namespace', name: 'test' },
          { apiVersion: 'another.io/v1', kind: 'Another', name: 'another1' }
        ]
      }
    }

    fetchWithMock.mockResolvedValue(resourceWithMixedApiVersions)
    const user = userEvent.setup()

    render(
      <ResourceView
        kind="Kustomization"
        name="infrastructure"
        namespace="flux-system"
        isExpanded={true}
      />
    )

    // Wait for inventory tab to appear
    const inventoryTab = await screen.findByText(/Inventory \(5\)/)
    expect(inventoryTab).toBeInTheDocument()

    // Click on Inventory tab
    await user.click(inventoryTab)

    // Get all API version headers in order after clicking
    await waitFor(() => {
      const apiVersionElements = screen.getAllByText(/^(apiextensions\.k8s\.io\/v1|v1|apps\/v1|custom\.io\/v1|another\.io\/v1)$/)

      // apiextensions.k8s.io/v1 should come first, then v1, then others alphabetically
      expect(apiVersionElements[0].textContent).toBe('apiextensions.k8s.io/v1')
      expect(apiVersionElements[1].textContent).toBe('v1')
      // The rest should be alphabetically sorted (another.io/v1 before apps/v1 before custom.io/v1)
    })
  })

  it('should handle inventory items without namespace (cluster-scoped)', async () => {
    const resourceWithClusterScoped = {
      apiVersion: 'kustomize.toolkit.fluxcd.io/v1',
      kind: 'Kustomization',
      metadata: {
        name: 'cluster-resources',
        namespace: 'flux-system'
      },
      spec: { interval: '10m' },
      status: {
        inventory: [
          { apiVersion: 'v1', kind: 'Namespace', name: 'test-namespace' },
          { apiVersion: 'v1', kind: 'ConfigMap', namespace: 'default', name: 'config' }
        ]
      }
    }

    fetchWithMock.mockResolvedValue(resourceWithClusterScoped)
    const user = userEvent.setup()

    render(
      <ResourceView
        kind="Kustomization"
        name="cluster-resources"
        namespace="flux-system"
        isExpanded={true}
      />
    )

    // Wait for inventory tab to appear
    const inventoryTab = await screen.findByText(/Inventory \(2\)/)
    expect(inventoryTab).toBeInTheDocument()

    // Click on Inventory tab
    await user.click(inventoryTab)

    // Wait for inventory content to appear
    await waitFor(() => {
      expect(screen.getByText('test-namespace')).toBeInTheDocument()
    })

    // Cluster-scoped resource should not show namespace
    const namespaceItem = screen.getByText('test-namespace').closest('div')
    expect(namespaceItem.textContent).toContain('Namespace/')
    expect(namespaceItem.textContent).toContain('test-namespace')

    // Namespaced resource should show namespace
    const configItem = screen.getByText('config').closest('div')
    expect(configItem.textContent).toContain('ConfigMap/')
    expect(configItem.textContent).toContain('default/')
    expect(configItem.textContent).toContain('config')
  })

  it('should not fetch when expanded is false initially', () => {
    fetchWithMock.mockResolvedValue(mockResourceData)

    render(
      <ResourceView
        kind="Kustomization"
        name="apps"
        namespace="flux-system"
        isExpanded={false}
      />
    )

    expect(fetchWithMock).not.toHaveBeenCalled()
  })

  it('should display Source tab when sourceRef is present in status', async () => {
    const resourceWithSourceRef = {
      apiVersion: 'kustomize.toolkit.fluxcd.io/v1',
      kind: 'Kustomization',
      metadata: {
        name: 'flux-system',
        namespace: 'flux-system'
      },
      spec: {
        interval: '10m',
        path: './clusters/homelab',
        sourceRef: {
          kind: 'GitRepository',
          name: 'flux-system'
        }
      },
      status: {
        sourceRef: {
          kind: 'GitRepository',
          message: "stored artifact for revision 'refs/heads/main@sha1:abc123'",
          name: 'flux-system',
          namespace: 'flux-system',
          originRevision: '',
          originURL: '',
          status: 'Ready',
          url: 'https://github.com/example/repo.git'
        }
      }
    }

    fetchWithMock.mockResolvedValue(resourceWithSourceRef)

    render(
      <ResourceView
        kind="Kustomization"
        name="flux-system"
        namespace="flux-system"
        isExpanded={true}
      />
    )

    // Wait for Source tab to appear
    await waitFor(() => {
      expect(screen.getByText('Source')).toBeInTheDocument()
    })

    // Source tab should be visible along with Specification and Status
    expect(screen.getByText('Source')).toBeInTheDocument()
    expect(screen.getByText('Specification')).toBeInTheDocument()
    expect(screen.getByText('Status')).toBeInTheDocument()
  })

  it('should show Inventory tab as default for Kustomization even when empty', async () => {
    const resourceWithSourceRef = {
      apiVersion: 'kustomize.toolkit.fluxcd.io/v1',
      kind: 'Kustomization',
      metadata: {
        name: 'flux-system',
        namespace: 'flux-system'
      },
      spec: {
        interval: '10m',
        sourceRef: {
          kind: 'GitRepository',
          name: 'flux-system'
        }
      },
      status: {
        sourceRef: {
          kind: 'GitRepository',
          message: "stored artifact for revision 'refs/heads/main@sha1:abc123'",
          name: 'flux-system',
          namespace: 'flux-system',
          originRevision: '',
          originURL: '',
          status: 'Ready',
          url: 'https://github.com/example/repo.git'
        }
      }
    }

    fetchWithMock.mockResolvedValue(resourceWithSourceRef)

    render(
      <ResourceView
        kind="Kustomization"
        name="flux-system"
        namespace="flux-system"
        isExpanded={true}
      />
    )

    // Wait for Inventory tab to appear and be active (even though empty)
    await waitFor(() => {
      const inventoryTab = screen.getByText('Inventory (0)')
      expect(inventoryTab).toBeInTheDocument()
      expect(inventoryTab).toHaveClass('border-flux-blue')
    })

    // Should show empty inventory message
    expect(screen.getByText('Empty inventory, no managed objects')).toBeInTheDocument()

    // Source tab should also be visible
    expect(screen.getByText('Source')).toBeInTheDocument()
  })

  it('should show Inventory tab as default for ResourceSet even when empty', async () => {
    const resourceSet = {
      apiVersion: 'fluxcd.controlplane.io/v1',
      kind: 'ResourceSet',
      metadata: {
        name: 'preview-envs',
        namespace: 'flux-system'
      },
      spec: {
        interval: '1h'
      },
      status: {
        conditions: [
          {
            type: 'Ready',
            status: 'True',
            lastTransitionTime: '2025-01-15T10:00:00Z',
            reason: 'ReconciliationSucceeded',
            message: 'Applied revision: main@sha1:abc123'
          }
        ]
      }
    }

    fetchWithMock.mockResolvedValue(resourceSet)

    render(
      <ResourceView
        kind="ResourceSet"
        name="preview-envs"
        namespace="flux-system"
        isExpanded={true}
      />
    )

    // Wait for Inventory tab to appear and be active (even though empty)
    await waitFor(() => {
      const inventoryTab = screen.getByText('Inventory (0)')
      expect(inventoryTab).toBeInTheDocument()
      expect(inventoryTab).toHaveClass('border-flux-blue')
    })

    // Should show empty inventory message
    expect(screen.getByText('Empty inventory, no managed objects')).toBeInTheDocument()
  })

  it('should display sourceRef data correctly in Source tab', async () => {
    const resourceWithSourceRef = {
      apiVersion: 'helm.toolkit.fluxcd.io/v2',
      kind: 'HelmRelease',
      metadata: {
        name: 'cert-manager',
        namespace: 'cert-manager'
      },
      spec: {
        interval: '24h',
        chartRef: {
          kind: 'OCIRepository',
          name: 'cert-manager'
        }
      },
      status: {
        sourceRef: {
          kind: 'OCIRepository',
          message: "stored artifact for digest 'v1.19.1@sha256:abc123'",
          name: 'cert-manager',
          namespace: 'cert-manager',
          originRevision: '',
          originURL: 'https://github.com/cert-manager/cert-manager',
          status: 'Ready',
          url: 'oci://quay.io/jetstack/charts/cert-manager'
        }
      }
    }

    fetchWithMock.mockResolvedValue(resourceWithSourceRef)
    const user = userEvent.setup()

    render(
      <ResourceView
        kind="HelmRelease"
        name="cert-manager"
        namespace="cert-manager"
        isExpanded={true}
      />
    )

    // Wait for Source tab and click it
    const sourceTab = await screen.findByText('Source')
    await user.click(sourceTab)

    // Check that source data is displayed with correct format
    await waitFor(() => {
      const textContent = document.body.textContent

      // Check ID format: kind/namespace/name
      expect(textContent).toContain('ID:')
      expect(textContent).toContain('OCIRepository/cert-manager/cert-manager')

      // Check URL
      expect(textContent).toContain('URL:')
      expect(textContent).toContain('oci://quay.io/jetstack/charts/cert-manager')

      // Check Origin URL
      expect(textContent).toContain('Origin URL:')
      expect(textContent).toContain('https://github.com/cert-manager/cert-manager')

      // Check Status
      expect(textContent).toContain('Status:')
      expect(textContent).toContain('Ready')

      // Check Message
      expect(textContent).toContain('Message:')
      expect(textContent).toContain("stored artifact for digest 'v1.19.1@sha256:abc123'")
    })
  })

  it('should not show Origin URL when it is empty', async () => {
    const resourceWithoutOriginURL = {
      apiVersion: 'kustomize.toolkit.fluxcd.io/v1',
      kind: 'Kustomization',
      metadata: {
        name: 'apps',
        namespace: 'flux-system'
      },
      spec: {
        interval: '10m'
      },
      status: {
        sourceRef: {
          kind: 'GitRepository',
          message: "stored artifact for revision 'refs/heads/main@sha1:abc123'",
          name: 'flux-system',
          namespace: 'flux-system',
          originRevision: '',
          originURL: '',
          status: 'Ready',
          url: 'https://github.com/example/repo.git'
        }
      }
    }

    fetchWithMock.mockResolvedValue(resourceWithoutOriginURL)
    const user = userEvent.setup()

    render(
      <ResourceView
        kind="Kustomization"
        name="apps"
        namespace="flux-system"
        isExpanded={true}
      />
    )

    // Wait for Source tab and click it
    const sourceTab = await screen.findByText('Source')
    await user.click(sourceTab)

    // Check that Origin URL is not displayed when empty
    await waitFor(() => {
      const textContent = document.body.textContent
      expect(textContent).toContain('ID:')
      expect(textContent).toContain('URL:')
      expect(textContent).toContain('Status:')
      expect(textContent).toContain('Message:')

      // Origin URL should not appear when empty
      const hasOriginURL = textContent.includes('Origin URL:')
      expect(hasOriginURL).toBe(false)
    })
  })

  it('should not show Source tab when sourceRef is not present', async () => {
    fetchWithMock.mockResolvedValue(mockResourceDataNoInventory)

    render(
      <ResourceView
        kind="GitRepository"
        name="flux-system"
        namespace="flux-system"
        isExpanded={true}
      />
    )

    await waitFor(() => {
      expect(screen.getByText('Specification')).toBeInTheDocument()
    })

    // Source tab should not be present when sourceRef is missing
    expect(screen.queryByText('Source')).not.toBeInTheDocument()
  })

  describe('Inventory navigation', () => {
    it('should make Flux resource inventory items clickable', async () => {
      const resourceWithFluxInventory = {
        apiVersion: 'kustomize.toolkit.fluxcd.io/v1',
        kind: 'Kustomization',
        metadata: {
          name: 'apps',
          namespace: 'flux-system'
        },
        spec: { interval: '10m' },
        status: {
          inventory: [
            { apiVersion: 'source.toolkit.fluxcd.io/v1', kind: 'GitRepository', namespace: 'flux-system', name: 'podinfo' },
            { apiVersion: 'helm.toolkit.fluxcd.io/v2', kind: 'HelmRelease', namespace: 'default', name: 'nginx' }
          ]
        }
      }

      fetchWithMock.mockResolvedValue(resourceWithFluxInventory)
      const user = userEvent.setup()

      render(
        <ResourceView
          kind="Kustomization"
          name="apps"
          namespace="flux-system"
          isExpanded={true}
        />
      )

      // Wait for inventory tab to appear and click it
      const inventoryTab = await screen.findByText(/Inventory \(2\)/)
      await user.click(inventoryTab)

      // Find the GitRepository button
      const gitRepoButton = await screen.findByRole('button', { name: /GitRepository\/flux-system\/podinfo/ })
      expect(gitRepoButton).toBeInTheDocument()

      // Click the GitRepository inventory item
      await user.click(gitRepoButton)

      // Verify navigation to resource dashboard
      expect(mockRoute).toHaveBeenCalledWith('/resource/GitRepository/flux-system/podinfo')
    })

    it('should not make non-Flux resource inventory items clickable', async () => {
      const resourceWithMixedInventory = {
        apiVersion: 'kustomize.toolkit.fluxcd.io/v1',
        kind: 'Kustomization',
        metadata: {
          name: 'apps',
          namespace: 'flux-system'
        },
        spec: { interval: '10m' },
        status: {
          inventory: [
            { apiVersion: 'v1', kind: 'ConfigMap', namespace: 'default', name: 'app-config' },
            { apiVersion: 'apps/v1', kind: 'Deployment', namespace: 'default', name: 'app' }
          ]
        }
      }

      fetchWithMock.mockResolvedValue(resourceWithMixedInventory)
      const user = userEvent.setup()

      render(
        <ResourceView
          kind="Kustomization"
          name="apps"
          namespace="flux-system"
          isExpanded={true}
        />
      )

      // Wait for inventory tab and click it
      const inventoryTab = await screen.findByText(/Inventory \(2\)/)
      await user.click(inventoryTab)

      // ConfigMap and Deployment should not be buttons
      await waitFor(() => {
        expect(screen.getByText('app-config')).toBeInTheDocument()
      })

      // Verify there are no clickable buttons for non-Flux resources
      const buttons = screen.queryAllByRole('button')
      // Should only have the tab buttons, not inventory item buttons
      const inventoryButtons = buttons.filter(btn =>
        btn.textContent.includes('ConfigMap') || btn.textContent.includes('Deployment')
      )
      expect(inventoryButtons).toHaveLength(0)
    })

    it('should handle inventory items without namespace', async () => {
      const resourceWithClusterScopedFlux = {
        apiVersion: 'kustomize.toolkit.fluxcd.io/v1',
        kind: 'Kustomization',
        metadata: {
          name: 'apps',
          namespace: 'flux-system'
        },
        spec: { interval: '10m' },
        status: {
          inventory: [
            { apiVersion: 'fluxcd.controlplane.io/v1', kind: 'FluxInstance', name: 'flux' }
          ]
        }
      }

      fetchWithMock.mockResolvedValue(resourceWithClusterScopedFlux)
      const user = userEvent.setup()

      render(
        <ResourceView
          kind="Kustomization"
          name="apps"
          namespace="flux-system"
          isExpanded={true}
        />
      )

      // Wait for inventory tab and click it
      const inventoryTab = await screen.findByText(/Inventory \(1\)/)
      await user.click(inventoryTab)

      // Find and click the FluxInstance button
      const fluxButton = await screen.findByRole('button', { name: /FluxInstance\/flux/ })
      await user.click(fluxButton)

      // Verify navigation to resource dashboard (with empty namespace)
      expect(mockRoute).toHaveBeenCalledWith('/resource/FluxInstance//flux')
    })

    it('should display navigation icon for clickable Flux resources', async () => {
      const resourceWithFluxInventory = {
        apiVersion: 'kustomize.toolkit.fluxcd.io/v1',
        kind: 'Kustomization',
        metadata: {
          name: 'apps',
          namespace: 'flux-system'
        },
        spec: { interval: '10m' },
        status: {
          inventory: [
            { apiVersion: 'kustomize.toolkit.fluxcd.io/v1', kind: 'Kustomization', namespace: 'apps', name: 'backend' }
          ]
        }
      }

      fetchWithMock.mockResolvedValue(resourceWithFluxInventory)
      const user = userEvent.setup()

      render(
        <ResourceView
          kind="Kustomization"
          name="apps"
          namespace="flux-system"
          isExpanded={true}
        />
      )

      // Wait for inventory tab and click it
      const inventoryTab = await screen.findByText(/Inventory \(1\)/)
      await user.click(inventoryTab)

      // Find the button and check for icon
      const kustomizationButton = await screen.findByRole('button', { name: /Kustomization\/apps\/backend/ })
      const svg = kustomizationButton.querySelector('svg')

      expect(svg).toBeInTheDocument()
      expect(svg).toHaveAttribute('viewBox', '0 0 24 24')
    })
  })
})
