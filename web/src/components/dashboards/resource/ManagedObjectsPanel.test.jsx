// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen } from '@testing-library/preact'
import userEvent from '@testing-library/user-event'
import { ManagedObjectsPanel } from './ManagedObjectsPanel'
import { fetchWithMock } from '../../../utils/fetch'
import { getPanelById } from '../common/panel.test'

// Mock useHashTab to use simple useState instead
vi.mock('../../../utils/hash', async () => {
  const { useState } = await import('preact/hooks')
  return {
    useHashTab: (panel, defaultTab) => useState(defaultTab)
  }
})

// Mock the workload status fetch the panel issues for the Graph/Workloads tabs
vi.mock('../../../utils/fetch', () => ({
  fetchWithMock: vi.fn(() => Promise.resolve({ workloads: [] }))
}))

// Mock the inventory list so this test asserts the panel's integration contract
// (tab visibility, switching, and the props handed to the list) rather than the
// list's DOM, which is owned and tested by InventoryTabContent.test.jsx.
vi.mock('./InventoryTabContent', () => ({
  InventoryTabContent: ({ inventory }) => (
    <div
      data-testid="inventory-tab-content"
      data-count={inventory?.length ?? 0}
    >
      InventoryTabContent
    </div>
  )
}))

describe('ManagedObjectsPanel component', () => {
  const mockKustomizationData = {
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
      wait: true,
      decryption: {
        provider: 'sops'
      }
    },
    status: {
      inventory: [
        { apiVersion: 'v1', kind: 'Namespace', name: 'production' },
        { apiVersion: 'v1', kind: 'ConfigMap', namespace: 'production', name: 'app-config' },
        { apiVersion: 'v1', kind: 'Secret', namespace: 'production', name: 'app-secret' },
        { apiVersion: 'apps/v1', kind: 'Deployment', namespace: 'production', name: 'app' },
        { apiVersion: 'kustomize.toolkit.fluxcd.io/v1', kind: 'Kustomization', namespace: 'production', name: 'backend' }
      ]
    }
  }

  const mockHelmReleaseData = {
    apiVersion: 'helm.toolkit.fluxcd.io/v2',
    kind: 'HelmRelease',
    metadata: {
      name: 'nginx',
      namespace: 'default'
    },
    spec: {
      interval: '5m',
      upgrade: {
        disableWait: false
      }
    },
    status: {
      inventory: [
        { apiVersion: 'v1', kind: 'Service', namespace: 'default', name: 'nginx' },
        { apiVersion: 'apps/v1', kind: 'Deployment', namespace: 'default', name: 'nginx' }
      ]
    }
  }

  const mockFluxInstanceData = {
    apiVersion: 'fluxcd.controlplane.io/v1',
    kind: 'FluxInstance',
    metadata: {
      name: 'flux',
      namespace: 'flux-system'
    },
    spec: {
      wait: true
    },
    status: {
      inventory: [
        { apiVersion: 'source.toolkit.fluxcd.io/v1', kind: 'GitRepository', namespace: 'flux-system', name: 'flux-system' },
        { apiVersion: 'helm.toolkit.fluxcd.io/v2', kind: 'HelmRelease', namespace: 'flux-system', name: 'cert-manager' }
      ]
    }
  }

  const mockOnNavigate = vi.fn()

  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('should render with only Overview tab when Kustomization has no inventory', () => {
    const noInventoryData = {
      ...mockKustomizationData,
      status: {}
    }

    const { container } = render(
      <ManagedObjectsPanel
        resourceData={noInventoryData}
        onNavigate={mockOnNavigate}
      />
    )

    // Panel should render
    expect(getPanelById(container, 'inventory-panel')).toBeInTheDocument()
    expect(screen.getByText('Managed Objects')).toBeInTheDocument()

    // Only Overview tab should be visible
    expect(screen.getByText('Overview')).toBeInTheDocument()
    expect(screen.queryByText('Inventory')).not.toBeInTheDocument()
    expect(screen.queryByText('Workloads')).not.toBeInTheDocument()

    // All counts should be zero
    const textContent = document.body.textContent
    expect(textContent).toContain('Total resources')
    expect(textContent).toContain('0')
  })

  it('should render with only Overview tab when Kustomization has empty inventory', () => {
    const emptyInventoryData = {
      ...mockKustomizationData,
      status: { inventory: [] }
    }

    const { container } = render(
      <ManagedObjectsPanel
        resourceData={emptyInventoryData}
        onNavigate={mockOnNavigate}
      />
    )

    // Panel should render
    expect(getPanelById(container, 'inventory-panel')).toBeInTheDocument()
    expect(screen.getByText('Managed Objects')).toBeInTheDocument()

    // Only Overview tab should be visible
    expect(screen.getByText('Overview')).toBeInTheDocument()
    expect(screen.queryByText('Inventory')).not.toBeInTheDocument()
    expect(screen.queryByText('Workloads')).not.toBeInTheDocument()

    // All counts should be zero
    const textContent = document.body.textContent
    expect(textContent).toContain('Total resources')
    expect(textContent).toContain('0')
  })

  it('should not render for non-inventory kinds like GitRepository', () => {
    const gitRepoData = {
      apiVersion: 'source.toolkit.fluxcd.io/v1',
      kind: 'GitRepository',
      metadata: {
        name: 'flux-system',
        namespace: 'flux-system'
      },
      spec: {
        interval: '1m',
        url: 'https://github.com/example/repo'
      },
      status: {}
    }

    const { container } = render(
      <ManagedObjectsPanel
        resourceData={gitRepoData}
        onNavigate={mockOnNavigate}
      />
    )

    expect(container.firstChild).toBeNull()
  })

  it('should render the managed objects section when inventory exists', () => {
    const { container } = render(
      <ManagedObjectsPanel
        resourceData={mockKustomizationData}
        onNavigate={mockOnNavigate}
      />
    )

    expect(getPanelById(container, 'inventory-panel')).toBeInTheDocument()
    expect(screen.getByText('Managed Objects')).toBeInTheDocument()
  })

  it('should display overview tab by default', () => {
    render(
      <ManagedObjectsPanel
        resourceData={mockKustomizationData}
        onNavigate={mockOnNavigate}
      />
    )

    // Overview tab should be active
    const overviewTab = screen.getByText('Overview').closest('button')
    expect(overviewTab).toHaveClass('border-flux-blue')

    // Check overview content is visible
    expect(screen.getByText('Garbage collection')).toBeInTheDocument()
    expect(screen.getByText('Health checking')).toBeInTheDocument()
    expect(screen.getByText('Secret decryption')).toBeInTheDocument()
    expect(screen.getByText('Total resources')).toBeInTheDocument()
  })

  it('should calculate total resources count correctly', () => {
    render(
      <ManagedObjectsPanel
        resourceData={mockKustomizationData}
        onNavigate={mockOnNavigate}
      />
    )

    const textContent = document.body.textContent
    expect(textContent).toContain('Total resources')
    expect(textContent).toContain('5')
  })

  it('should calculate flux resources count correctly', () => {
    render(
      <ManagedObjectsPanel
        resourceData={mockKustomizationData}
        onNavigate={mockOnNavigate}
      />
    )

    const textContent = document.body.textContent
    expect(textContent).toContain('Flux resources')
    expect(textContent).toContain('1')
  })

  it('should calculate workloads count correctly', () => {
    render(
      <ManagedObjectsPanel
        resourceData={mockKustomizationData}
        onNavigate={mockOnNavigate}
      />
    )

    const textContent = document.body.textContent
    expect(textContent).toContain('Kubernetes workloads')
    expect(textContent).toContain('1')
  })

  it('should calculate secrets count correctly', () => {
    render(
      <ManagedObjectsPanel
        resourceData={mockKustomizationData}
        onNavigate={mockOnNavigate}
      />
    )

    const textContent = document.body.textContent
    expect(textContent).toContain('Kubernetes secrets')
    expect(textContent).toContain('1')
  })

  it('should show garbage collection as enabled for Kustomization with prune=true', () => {
    render(
      <ManagedObjectsPanel
        resourceData={mockKustomizationData}
        onNavigate={mockOnNavigate}
      />
    )

    const textContent = document.body.textContent
    expect(textContent).toContain('Garbage collection')
    expect(textContent).toContain('Enabled')
  })

  it('should show garbage collection as disabled for Kustomization with prune=false', () => {
    const dataWithoutPrune = {
      ...mockKustomizationData,
      spec: {
        ...mockKustomizationData.spec,
        prune: false
      }
    }

    render(
      <ManagedObjectsPanel
        resourceData={dataWithoutPrune}
        onNavigate={mockOnNavigate}
      />
    )

    const textContent = document.body.textContent
    expect(textContent).toContain('Garbage collection')
    expect(textContent).toContain('Disabled')
  })

  it('should show garbage collection as enabled for HelmRelease', () => {
    render(
      <ManagedObjectsPanel
        resourceData={mockHelmReleaseData}
        onNavigate={mockOnNavigate}
      />
    )

    const textContent = document.body.textContent
    expect(textContent).toContain('Garbage collection')
    expect(textContent).toContain('Enabled')
  })

  it('should show health checking as enabled for Kustomization with wait=true', () => {
    render(
      <ManagedObjectsPanel
        resourceData={mockKustomizationData}
        onNavigate={mockOnNavigate}
      />
    )

    const textContent = document.body.textContent
    expect(textContent).toContain('Health checking')
    expect(textContent).toContain('Enabled')
  })

  it('should show health checking as disabled for Kustomization with wait=false', () => {
    const dataWithoutWait = {
      ...mockKustomizationData,
      spec: {
        ...mockKustomizationData.spec,
        wait: false
      }
    }

    render(
      <ManagedObjectsPanel
        resourceData={dataWithoutWait}
        onNavigate={mockOnNavigate}
      />
    )

    const textContent = document.body.textContent
    expect(textContent).toContain('Health checking')
    expect(textContent).toContain('Disabled')
  })

  it('should show secret decryption as enabled for Kustomization with decryption', () => {
    render(
      <ManagedObjectsPanel
        resourceData={mockKustomizationData}
        onNavigate={mockOnNavigate}
      />
    )

    const textContent = document.body.textContent
    expect(textContent).toContain('Secret decryption')
    expect(textContent).toContain('Enabled')
  })

  it('should show secret decryption as disabled for Kustomization without decryption', () => {
    const dataWithoutDecryption = {
      ...mockKustomizationData,
      spec: {
        interval: '10m',
        path: './apps',
        prune: true,
        wait: true
      }
    }

    render(
      <ManagedObjectsPanel
        resourceData={dataWithoutDecryption}
        onNavigate={mockOnNavigate}
      />
    )

    const textContent = document.body.textContent
    expect(textContent).toContain('Secret decryption')
    expect(textContent).toContain('Disabled')
  })

  it('should switch to inventory tab when clicked', async () => {
    const user = userEvent.setup()

    render(
      <ManagedObjectsPanel
        resourceData={mockKustomizationData}
        onNavigate={mockOnNavigate}
      />
    )

    // Click on Inventory tab
    const inventoryTab = screen.getByText('Inventory')
    await user.click(inventoryTab)

    // Tab is active and the inventory list renders with the resource's inventory
    expect(inventoryTab).toHaveClass('border-flux-blue')
    const list = screen.getByTestId('inventory-tab-content')
    expect(list).toBeInTheDocument()
    expect(list).toHaveAttribute('data-count', '5')
  })

  it('should toggle collapse/expand state', async () => {
    const user = userEvent.setup()

    render(
      <ManagedObjectsPanel
        resourceData={mockKustomizationData}
        onNavigate={mockOnNavigate}
      />
    )

    // Initially expanded, content should be visible
    expect(screen.getByText('Overview')).toBeInTheDocument()

    // Click to collapse
    const toggleButton = screen.getByRole('button', { name: /managed objects/i })
    await user.click(toggleButton)

    // Content should be hidden
    expect(screen.queryByText('Overview')).not.toBeInTheDocument()

    // Click to expand again
    await user.click(toggleButton)

    // Content should be visible again
    expect(screen.getByText('Overview')).toBeInTheDocument()
  })

  it('should switch back to overview tab', async () => {
    const user = userEvent.setup()

    render(
      <ManagedObjectsPanel
        resourceData={mockKustomizationData}
        onNavigate={mockOnNavigate}
      />
    )

    // Switch to inventory tab
    await user.click(screen.getByText('Inventory'))
    expect(screen.getByTestId('inventory-tab-content')).toBeInTheDocument()

    // Switch back to overview
    await user.click(screen.getByText('Overview'))

    // Check overview content is displayed again and the list is gone
    expect(screen.getByText('Garbage collection')).toBeInTheDocument()
    expect(screen.queryByTestId('inventory-tab-content')).not.toBeInTheDocument()
  })

  it('should handle FluxInstance with correct feature flags', () => {
    render(
      <ManagedObjectsPanel
        resourceData={mockFluxInstanceData}
        onNavigate={mockOnNavigate}
      />
    )

    const textContent = document.body.textContent

    // FluxInstance should have garbage collection enabled
    expect(textContent).toContain('Garbage collection')
    expect(textContent).toContain('Enabled')

    // FluxInstance with wait=true should have health checking enabled
    expect(textContent).toContain('Health checking')
    expect(textContent).toContain('Enabled')

    // FluxInstance doesn't support secret decryption
    expect(textContent).toContain('Secret decryption')
    expect(textContent).toContain('Disabled')
  })

  it('never shows a Workloads tab; workloads live in the Inventory tab', () => {
    render(
      <ManagedObjectsPanel
        resourceData={mockKustomizationData}
        onNavigate={mockOnNavigate}
      />
    )

    // The Inventory tab is the home for workloads (filterable by category);
    // there is no dedicated Workloads tab.
    expect(screen.getByText('Inventory')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Workloads' })).not.toBeInTheDocument()
  })

  it('fetches workload status at the panel level and propagates it to the Graph tab', async () => {
    const user = userEvent.setup()

    // The panel owns the fetch; return a real status for the inventory's Deployment
    fetchWithMock.mockResolvedValueOnce({
      workloads: [
        {
          kind: 'Deployment',
          namespace: 'production',
          name: 'app',
          status: 'Current',
          statusMessage: 'Deployment has minimum availability.'
        }
      ]
    })

    render(
      <ManagedObjectsPanel
        resourceData={mockKustomizationData}
        onNavigate={mockOnNavigate}
      />
    )

    await user.click(screen.getByText('Graph'))

    // The fetched status renders as a message + dot in the Graph's Workloads group,
    // replacing the "computing..." placeholder
    expect(await screen.findByText('Deployment has minimum availability.')).toBeInTheDocument()
    expect(screen.queryByTestId('workload-status-computing')).not.toBeInTheDocument()

    // The panel issued the batch POST with the resolved workload
    expect(fetchWithMock).toHaveBeenCalledWith(expect.objectContaining({
      endpoint: '/api/v1/workloads',
      method: 'POST',
      body: { workloads: [{ kind: 'Deployment', name: 'app', namespace: 'production' }] }
    }))

    // And the row links to the dedicated workload dashboard
    expect(document.querySelector('a[href="/workload/Deployment/production/app"]')).not.toBeNull()
  })

  it('should show health checking as disabled for HelmRelease with disableWait=true', () => {
    const helmReleaseWithDisabledWait = {
      ...mockHelmReleaseData,
      spec: {
        interval: '5m',
        upgrade: {
          disableWait: true
        }
      }
    }

    render(
      <ManagedObjectsPanel
        resourceData={helmReleaseWithDisabledWait}
        onNavigate={mockOnNavigate}
      />
    )

    const textContent = document.body.textContent
    expect(textContent).toContain('Health checking')
    expect(textContent).toContain('Disabled')
  })

  it('should show garbage collection as enabled for ResourceSet', () => {
    const resourceSetData = {
      apiVersion: 'fluxcd.controlplane.io/v1',
      kind: 'ResourceSet',
      metadata: {
        name: 'tenants',
        namespace: 'flux-system'
      },
      spec: {
        wait: false
      },
      status: {
        inventory: [
          { apiVersion: 'v1', kind: 'Namespace', name: 'tenant-1' }
        ]
      }
    }

    render(
      <ManagedObjectsPanel
        resourceData={resourceSetData}
        onNavigate={mockOnNavigate}
      />
    )

    const textContent = document.body.textContent
    expect(textContent).toContain('Garbage collection')
    expect(textContent).toContain('Enabled')
  })

  it('should show garbage collection as enabled for ArtifactGenerator', () => {
    const artifactGeneratorData = {
      apiVersion: 'fluxcd.controlplane.io/v1',
      kind: 'ArtifactGenerator',
      metadata: {
        name: 'generator',
        namespace: 'flux-system'
      },
      spec: {},
      status: {
        inventory: [
          { apiVersion: 'v1', kind: 'ConfigMap', namespace: 'flux-system', name: 'generated' }
        ]
      }
    }

    render(
      <ManagedObjectsPanel
        resourceData={artifactGeneratorData}
        onNavigate={mockOnNavigate}
      />
    )

    const textContent = document.body.textContent
    expect(textContent).toContain('Garbage collection')
    expect(textContent).toContain('Enabled')
  })

  it('should handle onNavigate being undefined', async () => {
    const user = userEvent.setup()

    render(
      <ManagedObjectsPanel
        resourceData={mockKustomizationData}
        onNavigate={undefined}
      />
    )

    // Panel renders and switches tabs without throwing when onNavigate is omitted
    await user.click(screen.getByText('Inventory'))
    expect(screen.getByTestId('inventory-tab-content')).toBeInTheDocument()
  })

  it('should show garbage collection as disabled for unknown kind without prune spec', () => {
    // Test the fallback case in pruningEnabled calculation
    // Using Kustomization without prune set (not true, not false, just undefined)
    const kustomizationWithoutPrune = {
      apiVersion: 'kustomize.toolkit.fluxcd.io/v1',
      kind: 'Kustomization',
      metadata: {
        name: 'test',
        namespace: 'flux-system'
      },
      spec: {
        interval: '10m',
        path: './test'
        // prune is undefined
      },
      status: {
        inventory: [
          { apiVersion: 'v1', kind: 'ConfigMap', namespace: 'flux-system', name: 'test' }
        ]
      }
    }

    render(
      <ManagedObjectsPanel
        resourceData={kustomizationWithoutPrune}
        onNavigate={mockOnNavigate}
      />
    )

    const textContent = document.body.textContent
    expect(textContent).toContain('Garbage collection')
    expect(textContent).toContain('Disabled')
  })

  describe('Graph tab', () => {
    it('should show Graph tab when inventory exists', () => {
      render(
        <ManagedObjectsPanel
          resourceData={mockKustomizationData}
          onNavigate={mockOnNavigate}
        />
      )

      expect(screen.getByText('Graph')).toBeInTheDocument()
    })

    it('should show Graph tab even when inventory is empty', () => {
      const noInventoryData = {
        ...mockKustomizationData,
        status: {}
      }

      render(
        <ManagedObjectsPanel
          resourceData={noInventoryData}
          onNavigate={mockOnNavigate}
        />
      )

      // Graph tab should always be visible for reconciler kinds
      expect(screen.getByText('Graph')).toBeInTheDocument()
    })

    it('should switch to Graph tab when clicked', async () => {
      const user = userEvent.setup()

      render(
        <ManagedObjectsPanel
          resourceData={mockKustomizationData}
          onNavigate={mockOnNavigate}
        />
      )

      // Click on Graph tab
      const graphTab = screen.getByText('Graph')
      await user.click(graphTab)

      // Check that graph tab is active
      expect(graphTab).toHaveClass('border-flux-blue')

      // Check that graph content is displayed (wait for lazy load)
      expect(await screen.findByTestId('graph-tab-content')).toBeInTheDocument()
    })

    it('should display graph with reconciler information', async () => {
      const user = userEvent.setup()

      render(
        <ManagedObjectsPanel
          resourceData={mockKustomizationData}
          onNavigate={mockOnNavigate}
        />
      )

      // Switch to Graph tab
      const graphTab = screen.getByText('Graph')
      await user.click(graphTab)

      // Check that current reconciler is shown (wait for lazy load, Kustomization may appear multiple times)
      const kustomizations = await screen.findAllByText('Kustomization')
      expect(kustomizations.length).toBeGreaterThanOrEqual(1)
    })

    it('should display inventory groups in graph', async () => {
      const user = userEvent.setup()

      render(
        <ManagedObjectsPanel
          resourceData={mockKustomizationData}
          onNavigate={mockOnNavigate}
        />
      )

      // Switch to Graph tab
      const graphTab = screen.getByText('Graph')
      await user.click(graphTab)

      // Check inventory groups are shown (wait for lazy load)
      // mockKustomizationData has 1 Flux resource (Kustomization), 1 workload (Deployment), and 3 other resources
      // Text is split across nodes so use regex
      expect(await screen.findByText(/Flux Resources \(1\)/)).toBeInTheDocument()
      expect(await screen.findByText(/Workloads \(1\)/)).toBeInTheDocument()
      expect(await screen.findByText(/Resources \(3\)/)).toBeInTheDocument()
    })
  })

  describe('inventoryError', () => {
    it('should not display error when inventoryError is not present', () => {
      render(
        <ManagedObjectsPanel
          resourceData={mockKustomizationData}
          onNavigate={mockOnNavigate}
        />
      )

      expect(screen.queryByTestId('inventory-error')).not.toBeInTheDocument()
    })

    it('should display error banner when inventoryError is present and hide overview content', () => {
      const dataWithError = {
        ...mockKustomizationData,
        status: {
          ...mockKustomizationData.status,
          inventoryError: 'Forbidden: User does not have permission to list resources'
        }
      }

      render(
        <ManagedObjectsPanel
          resourceData={dataWithError}
          onNavigate={mockOnNavigate}
        />
      )

      const errorBanner = screen.getByTestId('inventory-error')
      expect(errorBanner).toBeInTheDocument()
      expect(errorBanner).toHaveTextContent('Forbidden: User does not have permission to list resources')

      // Overview content should be hidden when there's an error
      expect(screen.queryByText('Garbage collection')).not.toBeInTheDocument()
      expect(screen.queryByText('Total resources')).not.toBeInTheDocument()
    })

    it('should display error banner alongside inventory data in inventory tab', async () => {
      const user = userEvent.setup()
      const dataWithErrorAndInventory = {
        ...mockKustomizationData,
        status: {
          ...mockKustomizationData.status,
          inventoryError: 'Partial inventory: some resources could not be listed'
        }
      }

      render(
        <ManagedObjectsPanel
          resourceData={dataWithErrorAndInventory}
          onNavigate={mockOnNavigate}
        />
      )

      // Error should be visible
      expect(screen.getByTestId('inventory-error')).toBeInTheDocument()

      // Overview content should be hidden
      expect(screen.queryByText('Garbage collection')).not.toBeInTheDocument()

      // Switch to inventory tab
      const inventoryTab = screen.getByText('Inventory')
      await user.click(inventoryTab)

      // Error should still be visible
      expect(screen.getByTestId('inventory-error')).toBeInTheDocument()

      // Inventory list should also be visible alongside the error
      expect(screen.getByTestId('inventory-tab-content')).toBeInTheDocument()
    })

    it('should display error when there is no inventory data', () => {
      const dataWithErrorNoInventory = {
        ...mockKustomizationData,
        status: {
          inventoryError: 'Failed to list resources: connection refused'
        }
      }

      render(
        <ManagedObjectsPanel
          resourceData={dataWithErrorNoInventory}
          onNavigate={mockOnNavigate}
        />
      )

      expect(screen.getByTestId('inventory-error')).toBeInTheDocument()
      expect(screen.getByText('Failed to list resources: connection refused')).toBeInTheDocument()

      // Inventory tab should not be visible
      expect(screen.queryByText('Inventory')).not.toBeInTheDocument()
    })
  })
})
