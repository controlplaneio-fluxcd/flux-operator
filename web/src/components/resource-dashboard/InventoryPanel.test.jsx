// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen } from '@testing-library/preact'
import userEvent from '@testing-library/user-event'
import { InventoryPanel } from './InventoryPanel'

describe('InventoryPanel component', () => {
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

    render(
      <InventoryPanel
        resourceData={noInventoryData}
        onNavigate={mockOnNavigate}
      />
    )

    // Panel should render
    expect(screen.getByTestId('inventory-view')).toBeInTheDocument()
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

    render(
      <InventoryPanel
        resourceData={emptyInventoryData}
        onNavigate={mockOnNavigate}
      />
    )

    // Panel should render
    expect(screen.getByTestId('inventory-view')).toBeInTheDocument()
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
      <InventoryPanel
        resourceData={gitRepoData}
        onNavigate={mockOnNavigate}
      />
    )

    expect(container.firstChild).toBeNull()
  })

  it('should render the managed objects section when inventory exists', () => {
    render(
      <InventoryPanel
        resourceData={mockKustomizationData}
        onNavigate={mockOnNavigate}
      />
    )

    expect(screen.getByTestId('inventory-view')).toBeInTheDocument()
    expect(screen.getByText('Managed Objects')).toBeInTheDocument()
  })

  it('should display overview tab by default', () => {
    render(
      <InventoryPanel
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
      <InventoryPanel
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
      <InventoryPanel
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
      <InventoryPanel
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
      <InventoryPanel
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
      <InventoryPanel
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
      <InventoryPanel
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
      <InventoryPanel
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
      <InventoryPanel
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
      <InventoryPanel
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
      <InventoryPanel
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
      <InventoryPanel
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
      <InventoryPanel
        resourceData={mockKustomizationData}
        onNavigate={mockOnNavigate}
      />
    )

    // Click on Inventory tab
    const inventoryTab = screen.getByText('Inventory')
    await user.click(inventoryTab)

    // Check that inventory tab is active
    expect(inventoryTab).toHaveClass('border-flux-blue')

    // Check that inventory table is displayed
    const table = document.querySelector('table')
    expect(table).toBeInTheDocument()

    // Check table headers
    const headers = document.querySelectorAll('th')
    expect(headers[0].textContent).toBe('Name')
    expect(headers[1].textContent).toBe('Namespace')
    expect(headers[2].textContent).toBe('Kind')
  })

  it('should display all inventory items in the table', async () => {
    const user = userEvent.setup()

    render(
      <InventoryPanel
        resourceData={mockKustomizationData}
        onNavigate={mockOnNavigate}
      />
    )

    // Switch to inventory tab
    const inventoryTab = screen.getByText('Inventory')
    await user.click(inventoryTab)

    // Check that all items are displayed
    const textContent = document.body.textContent
    expect(textContent).toContain('production')
    expect(textContent).toContain('app-config')
    expect(textContent).toContain('app-secret')
    expect(textContent).toContain('app')
    expect(textContent).toContain('backend')
  })

  it('should make Flux resources clickable in inventory', async () => {
    const user = userEvent.setup()

    render(
      <InventoryPanel
        resourceData={mockKustomizationData}
        onNavigate={mockOnNavigate}
      />
    )

    // Switch to inventory tab
    const inventoryTab = screen.getByText('Inventory')
    await user.click(inventoryTab)

    // Find the Kustomization button (Flux resource)
    const kustomizationButton = screen.getByText('backend').closest('button')
    expect(kustomizationButton).toBeInTheDocument()

    // Click it
    await user.click(kustomizationButton)

    // Check that onNavigate was called with the correct item
    expect(mockOnNavigate).toHaveBeenCalledWith({
      apiVersion: 'kustomize.toolkit.fluxcd.io/v1',
      kind: 'Kustomization',
      namespace: 'production',
      name: 'backend'
    })
  })

  it('should not make non-Flux resources clickable in inventory', async () => {
    const user = userEvent.setup()

    render(
      <InventoryPanel
        resourceData={mockKustomizationData}
        onNavigate={mockOnNavigate}
      />
    )

    // Switch to inventory tab
    const inventoryTab = screen.getByText('Inventory')
    await user.click(inventoryTab)

    // ConfigMap should not be in a button
    const configMapElement = screen.getByText('app-config')
    expect(configMapElement.tagName).toBe('SPAN')
    expect(configMapElement.closest('button')).toBeNull()
  })

  it('should toggle collapse/expand state', async () => {
    const user = userEvent.setup()

    render(
      <InventoryPanel
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

  it('should display namespace or dash for inventory items', async () => {
    const user = userEvent.setup()

    render(
      <InventoryPanel
        resourceData={mockKustomizationData}
        onNavigate={mockOnNavigate}
      />
    )

    // Switch to inventory tab
    const inventoryTab = screen.getByText('Inventory')
    await user.click(inventoryTab)

    // Check that namespace is displayed for namespaced resources
    const rows = document.querySelectorAll('tbody tr')
    expect(rows.length).toBe(5)

    // First row is Namespace (no namespace)
    const firstRowCells = rows[0].querySelectorAll('td')
    expect(firstRowCells[1].textContent).toBe('-')

    // Second row is ConfigMap (has namespace)
    const secondRowCells = rows[1].querySelectorAll('td')
    expect(secondRowCells[1].textContent).toBe('production')
  })

  it('should switch back to overview tab', async () => {
    const user = userEvent.setup()

    render(
      <InventoryPanel
        resourceData={mockKustomizationData}
        onNavigate={mockOnNavigate}
      />
    )

    // Switch to inventory tab
    const inventoryTab = screen.getByText('Inventory')
    await user.click(inventoryTab)

    expect(screen.getByText('Name')).toBeInTheDocument()

    // Switch back to overview
    const overviewTab = screen.getByText('Overview')
    await user.click(overviewTab)

    // Check overview content is displayed again
    expect(screen.getByText('Garbage collection')).toBeInTheDocument()
    expect(screen.queryByText('Name')).not.toBeInTheDocument()
  })

  it('should handle FluxInstance with correct feature flags', () => {
    render(
      <InventoryPanel
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

  it('should not call onNavigate when clicking non-Flux resources', async () => {
    const user = userEvent.setup()

    render(
      <InventoryPanel
        resourceData={mockHelmReleaseData}
        onNavigate={mockOnNavigate}
      />
    )

    // Switch to inventory tab
    const inventoryTab = screen.getByText('Inventory')
    await user.click(inventoryTab)

    // Check that there are no buttons in the table (all are non-Flux resources)
    const buttons = document.querySelectorAll('tbody button')
    expect(buttons.length).toBe(0)

    // Check that spans are used instead
    const spans = document.querySelectorAll('tbody td span')
    expect(spans.length).toBeGreaterThan(0)

    // Verify onNavigate was not called
    expect(mockOnNavigate).not.toHaveBeenCalled()
  })

  it('should sort inventory items correctly', async () => {
    const user = userEvent.setup()

    render(
      <InventoryPanel
        resourceData={mockKustomizationData}
        onNavigate={mockOnNavigate}
      />
    )

    // Switch to inventory tab
    const inventoryTab = screen.getByText('Inventory')
    await user.click(inventoryTab)

    // Get all rows in the table
    const rows = document.querySelectorAll('tbody tr')

    // Expected order based on mock data:
    // 1. Namespace (non-namespaced, kind: Namespace)
    // 2. ConfigMap (namespaced: production, kind: ConfigMap)
    // 3. Deployment (namespaced: production, kind: Deployment)
    // 4. Kustomization (namespaced: production, kind: Kustomization)
    // 5. Secret (namespaced: production, kind: Secret)

    // Check first row is Namespace (non-namespaced)
    expect(rows[0].querySelectorAll('td')[0].textContent).toBe('production')
    expect(rows[0].querySelectorAll('td')[1].textContent).toBe('-')
    expect(rows[0].querySelectorAll('td')[2].textContent).toBe('Namespace')

    // Check second row is ConfigMap (namespaced, production)
    expect(rows[1].querySelectorAll('td')[0].textContent).toBe('app-config')
    expect(rows[1].querySelectorAll('td')[1].textContent).toBe('production')
    expect(rows[1].querySelectorAll('td')[2].textContent).toBe('ConfigMap')

    // Check third row is Deployment (namespaced, production)
    expect(rows[2].querySelectorAll('td')[0].textContent).toBe('app')
    expect(rows[2].querySelectorAll('td')[1].textContent).toBe('production')
    expect(rows[2].querySelectorAll('td')[2].textContent).toBe('Deployment')

    // Check fourth row is Kustomization (namespaced, production)
    expect(rows[3].querySelectorAll('td')[0].textContent).toBe('backend')
    expect(rows[3].querySelectorAll('td')[1].textContent).toBe('production')
    expect(rows[3].querySelectorAll('td')[2].textContent).toBe('Kustomization')

    // Check fifth row is Secret (namespaced, production)
    expect(rows[4].querySelectorAll('td')[0].textContent).toBe('app-secret')
    expect(rows[4].querySelectorAll('td')[1].textContent).toBe('production')
    expect(rows[4].querySelectorAll('td')[2].textContent).toBe('Secret')
  })
})
