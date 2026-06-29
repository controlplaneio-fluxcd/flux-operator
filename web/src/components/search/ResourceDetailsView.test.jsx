// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/preact'
import userEvent from '@testing-library/user-event'
import { ResourceDetailsView } from './ResourceDetailsView'
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

describe('ResourceDetailsView component', () => {
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
      <ResourceDetailsView
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
      <ResourceDetailsView
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
      <ResourceDetailsView
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

  it('should call onReady once the fetch settles successfully', async () => {
    fetchWithMock.mockResolvedValue(mockResourceData)
    const onReady = vi.fn()

    render(
      <ResourceDetailsView
        kind="Kustomization"
        name="apps"
        namespace="flux-system"
        isExpanded={true}
        onReady={onReady}
      />
    )

    await waitFor(() => {
      expect(onReady).toHaveBeenCalledTimes(1)
    })
  })

  it('should call onReady even when the fetch fails', async () => {
    fetchWithMock.mockRejectedValue(new Error('boom'))
    const onReady = vi.fn()

    render(
      <ResourceDetailsView
        kind="Kustomization"
        name="apps"
        namespace="flux-system"
        isExpanded={true}
        onReady={onReady}
      />
    )

    await waitFor(() => {
      expect(onReady).toHaveBeenCalledTimes(1)
    })
  })

  describe('Overview tab', () => {
    const mockResourceWithReconciler = {
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
        reconcilerRef: {
          status: 'Ready',
          message: 'Applied revision: main@sha1:abc123def456',
          lastReconciled: '2025-01-15T10:00:00Z',
          managedBy: 'Kustomization/flux-system/flux-system'
        },
        inventory: [
          { apiVersion: 'v1', kind: 'Namespace', name: 'production' }
        ]
      }
    }

    it('should default to the Overview tab and mark it active', async () => {
      fetchWithMock.mockResolvedValue(mockResourceWithReconciler)

      render(
        <ResourceDetailsView
          kind="Kustomization"
          name="apps"
          namespace="flux-system"
          isExpanded={true}
        />
      )

      // The Overview tab is rendered and selected by default, even though the
      // resource also has an Inventory tab available.
      const overviewTab = await screen.findByText('Overview')
      expect(overviewTab).toBeInTheDocument()
      // Active tab merges into the content panel by sharing its background.
      expect(overviewTab).toHaveClass('bg-gray-100')

      // Inventory tab is present but not active.
      const inventoryTab = screen.getByText('Inventory')
      expect(inventoryTab).not.toHaveClass('bg-gray-100')
    })

    it('should show the reconciler status badge', async () => {
      fetchWithMock.mockResolvedValue(mockResourceWithReconciler)

      render(
        <ResourceDetailsView
          kind="Kustomization"
          name="apps"
          namespace="flux-system"
          isExpanded={true}
        />
      )

      // The Overview badge is labelled with the resource kind and shows the
      // reconciler status with the corresponding badge styling.
      await waitFor(() => {
        expect(screen.getByText('Kustomization')).toBeInTheDocument()
      })
      const badge = screen.getByText('Ready')
      expect(badge).toBeInTheDocument()
      expect(badge).toHaveClass('bg-green-100')
    })

    it('should show the controller, interval and managed-by link', async () => {
      fetchWithMock.mockResolvedValue(mockResourceWithReconciler)

      render(
        <ResourceDetailsView
          kind="Kustomization"
          name="apps"
          namespace="flux-system"
          isExpanded={true}
        />
      )

      await waitFor(() => {
        expect(screen.getByText('Reconciled by')).toBeInTheDocument()
      })

      // Controller name derived from the kind.
      expect(screen.getByText('kustomize-controller')).toBeInTheDocument()

      // Reconcile interval field with the spec interval.
      expect(screen.getByText('Reconcile every')).toBeInTheDocument()
      expect(document.body.textContent).toContain('10m')

      // Managed-by renders a namespace/name link to the owning resource dashboard.
      expect(screen.getByText('Managed by')).toBeInTheDocument()
      const managedByLink = screen.getByRole('link', { name: 'flux-system/flux-system' })
      expect(managedByLink).toHaveAttribute('href', '/resource/Kustomization/flux-system/flux-system')
    })

    it('should show the last action timestamp and reconciler message', async () => {
      fetchWithMock.mockResolvedValue(mockResourceWithReconciler)

      render(
        <ResourceDetailsView
          kind="Kustomization"
          name="apps"
          namespace="flux-system"
          isExpanded={true}
        />
      )

      await waitFor(() => {
        expect(screen.getByText(/Last action/)).toBeInTheDocument()
      })

      expect(screen.getByText('Applied revision: main@sha1:abc123def456')).toBeInTheDocument()
    })

    it('should fall back to the status prop for the Overview badge', async () => {
      const resourceWithoutReconciler = {
        apiVersion: 'kustomize.toolkit.fluxcd.io/v1',
        kind: 'Kustomization',
        metadata: { name: 'apps', namespace: 'flux-system' },
        spec: { interval: '10m' },
        status: {}
      }

      fetchWithMock.mockResolvedValue(resourceWithoutReconciler)

      render(
        <ResourceDetailsView
          kind="Kustomization"
          name="apps"
          namespace="flux-system"
          isExpanded={true}
          status="Suspended"
        />
      )

      await waitFor(() => {
        expect(screen.getByText('Overview')).toBeInTheDocument()
      })

      // With no reconcilerRef status, the Overview badge uses the status prop.
      expect(screen.getByText('Suspended')).toBeInTheDocument()
    })
  })

  it('should display specification tab as highlighted YAML after loading', async () => {
    fetchWithMock.mockResolvedValue(mockResourceData)
    const user = userEvent.setup()

    render(
      <ResourceDetailsView
        kind="Kustomization"
        name="apps"
        namespace="flux-system"
        isExpanded={true}
      />
    )

    await waitFor(() => {
      expect(screen.getByText('Spec')).toBeInTheDocument()
    })

    // Overview is the default tab, switch to the Spec tab to see the YAML.
    const specTab = screen.getByText('Spec')
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
      <ResourceDetailsView
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
      <ResourceDetailsView
        kind="Kustomization"
        name="apps"
        namespace="flux-system"
        isExpanded={true}
      />
    )

    // Wait for inventory tab to appear
    const inventoryTab = await screen.findByText('Inventory')
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
      <ResourceDetailsView
        kind="GitRepository"
        name="flux-system"
        namespace="flux-system"
        isExpanded={true}
      />
    )

    await waitFor(() => {
      expect(screen.getByText('Spec')).toBeInTheDocument()
    })

    // Inventory tab should not be present
    expect(screen.queryByText('Inventory')).not.toBeInTheDocument()

    // Overview, Specification and Status tabs should be visible
    expect(screen.getByText('Overview')).toBeInTheDocument()
    expect(screen.getByText('Spec')).toBeInTheDocument()
    expect(screen.getByText('Status')).toBeInTheDocument()
  })

  it('should cache resource data and not refetch on re-expand', async () => {
    fetchWithMock.mockResolvedValue(mockResourceData)

    const { rerender } = render(
      <ResourceDetailsView
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
      <ResourceDetailsView
        kind="Kustomization"
        name="apps"
        namespace="flux-system"
        isExpanded={false}
      />
    )

    // Re-expand
    rerender(
      <ResourceDetailsView
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
      expect(screen.getByText('Spec')).toBeInTheDocument()
      expect(screen.getByText('Status')).toBeInTheDocument()
    })
  })

  it('should show error state when fetch fails', async () => {
    const errorMessage = 'Network connection failed'
    fetchWithMock.mockRejectedValue(new Error(errorMessage))

    render(
      <ResourceDetailsView
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
    const user = userEvent.setup()

    render(
      <ResourceDetailsView
        kind="Test"
        name="test"
        namespace="default"
        isExpanded={true}
      />
    )

    await waitFor(() => {
      expect(screen.getByText('Spec')).toBeInTheDocument()
    })

    // Switch to the Spec tab and confirm the empty spec renders as YAML.
    await user.click(screen.getByText('Spec'))

    await waitFor(() => {
      const codeElement = document.querySelector('.language-yaml')
      expect(codeElement).toBeInTheDocument()
    })
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
      <ResourceDetailsView
        kind="Kustomization"
        name="infrastructure"
        namespace="flux-system"
        isExpanded={true}
      />
    )

    // Wait for inventory tab to appear
    const inventoryTab = await screen.findByText('Inventory')
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
      <ResourceDetailsView
        kind="Kustomization"
        name="cluster-resources"
        namespace="flux-system"
        isExpanded={true}
      />
    )

    // Wait for inventory tab to appear
    const inventoryTab = await screen.findByText('Inventory')
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
      <ResourceDetailsView
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
      <ResourceDetailsView
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

    // Source tab should be visible along with Spec and Status
    expect(screen.getByText('Source')).toBeInTheDocument()
    expect(screen.getByText('Spec')).toBeInTheDocument()
    expect(screen.getByText('Status')).toBeInTheDocument()
  })

  it('should show an Inventory tab for Kustomization even when empty', async () => {
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
    const user = userEvent.setup()

    render(
      <ResourceDetailsView
        kind="Kustomization"
        name="flux-system"
        namespace="flux-system"
        isExpanded={true}
      />
    )

    // Inventory tab is present (kinds with inventory always expose it), even
    // though the inventory is empty. Overview remains the default tab.
    const inventoryTab = await screen.findByText('Inventory')
    expect(inventoryTab).toBeInTheDocument()

    // Switching to it shows the empty inventory message.
    await user.click(inventoryTab)
    expect(screen.getByText('Empty inventory, no managed objects')).toBeInTheDocument()

    // Source tab should also be visible
    expect(screen.getByText('Source')).toBeInTheDocument()
  })

  it('should show an Inventory tab for ResourceSet even when empty', async () => {
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
    const user = userEvent.setup()

    render(
      <ResourceDetailsView
        kind="ResourceSet"
        name="preview-envs"
        namespace="flux-system"
        isExpanded={true}
      />
    )

    // Inventory tab is present even though the inventory is empty.
    const inventoryTab = await screen.findByText('Inventory')
    expect(inventoryTab).toBeInTheDocument()

    // Switching to it shows the empty inventory message.
    await user.click(inventoryTab)
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
      <ResourceDetailsView
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

      // Resource link shows namespace/name; the kind labels the status field.
      expect(textContent).toContain('cert-manager/cert-manager')
      expect(textContent).toContain('OCIRepository')

      // Check URL
      expect(textContent).toContain('URL')
      expect(textContent).toContain('oci://quay.io/jetstack/charts/cert-manager')

      // Check Origin URL
      expect(textContent).toContain('Origin URL')
      expect(textContent).toContain('https://github.com/cert-manager/cert-manager')

      // Check Status (now displayed as badge)
      expect(textContent).toContain('Status')
      expect(textContent).toContain('Ready')

      // Check Fetch result
      expect(textContent).toContain('Fetch result')
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
      <ResourceDetailsView
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
      // Resource link shows namespace/name; the kind labels the status field.
      expect(textContent).toContain('flux-system/flux-system')
      expect(textContent).toContain('GitRepository')
      expect(textContent).toContain('URL')
      expect(textContent).toContain('Fetch result')

      // Origin URL should not appear when empty
      const hasOriginURL = textContent.includes('Origin URL')
      expect(hasOriginURL).toBe(false)
    })
  })

  it('should not show Source tab when sourceRef is not present', async () => {
    fetchWithMock.mockResolvedValue(mockResourceDataNoInventory)

    render(
      <ResourceDetailsView
        kind="GitRepository"
        name="flux-system"
        namespace="flux-system"
        isExpanded={true}
      />
    )

    await waitFor(() => {
      expect(screen.getByText('Spec')).toBeInTheDocument()
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
        <ResourceDetailsView
          kind="Kustomization"
          name="apps"
          namespace="flux-system"
          isExpanded={true}
        />
      )

      // Wait for inventory tab to appear and click it
      const inventoryTab = await screen.findByText('Inventory')
      await user.click(inventoryTab)

      // Find the GitRepository link
      const gitRepoLink = await screen.findByRole('link', { name: /GitRepository\/flux-system\/podinfo/ })
      expect(gitRepoLink).toBeInTheDocument()

      // Verify correct href for navigation to resource dashboard
      expect(gitRepoLink).toHaveAttribute('href', '/resource/GitRepository/flux-system/podinfo')
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
        <ResourceDetailsView
          kind="Kustomization"
          name="apps"
          namespace="flux-system"
          isExpanded={true}
        />
      )

      // Wait for inventory tab and click it
      const inventoryTab = await screen.findByText('Inventory')
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
        <ResourceDetailsView
          kind="Kustomization"
          name="apps"
          namespace="flux-system"
          isExpanded={true}
        />
      )

      // Wait for inventory tab and click it
      const inventoryTab = await screen.findByText('Inventory')
      await user.click(inventoryTab)

      // Find the FluxInstance link
      const fluxLink = await screen.findByRole('link', { name: /FluxInstance\/flux/ })

      // Verify correct href for navigation to resource dashboard (with empty namespace)
      expect(fluxLink).toHaveAttribute('href', '/resource/FluxInstance//flux')
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
        <ResourceDetailsView
          kind="Kustomization"
          name="apps"
          namespace="flux-system"
          isExpanded={true}
        />
      )

      // Wait for inventory tab and click it
      const inventoryTab = await screen.findByText('Inventory')
      await user.click(inventoryTab)

      // Find the link and check for icon
      const kustomizationLink = await screen.findByRole('link', { name: /Kustomization\/apps\/backend/ })
      const svg = kustomizationLink.querySelector('svg')

      expect(svg).toBeInTheDocument()
      expect(svg).toHaveAttribute('viewBox', '0 0 24 24')
    })
  })
})
