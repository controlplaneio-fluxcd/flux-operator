// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/preact'
import { InventoryTabContent } from './InventoryTabContent'

describe('InventoryTabContent component', () => {
  const mockInventory = [
    { kind: 'Deployment', name: 'podinfo', namespace: 'apps', apiVersion: 'apps/v1' },
    { kind: 'Kustomization', name: 'apps', namespace: 'flux-system', apiVersion: 'kustomize.toolkit.fluxcd.io/v1' },
    { kind: 'ConfigMap', name: 'settings', namespace: 'apps', apiVersion: 'v1' },
    { kind: 'ClusterRole', name: 'reader', apiVersion: 'rbac.authorization.k8s.io/v1' }
  ]

  it('should render a row for each inventory item with name, namespace and kind', () => {
    render(<InventoryTabContent inventory={mockInventory} namespace="flux-system" />)

    const body = document.body.textContent
    expect(body).toContain('podinfo')
    expect(body).toContain('apps')
    expect(body).toContain('Deployment')
    expect(body).toContain('Kustomization')
    expect(body).toContain('ConfigMap')
    expect(body).toContain('ClusterRole')
  })

  it('should show a dash for items without a namespace', () => {
    render(<InventoryTabContent inventory={[{ kind: 'ClusterRole', name: 'reader' }]} namespace="flux-system" />)

    expect(screen.getByText('-')).toBeInTheDocument()
  })

  it('should sort cluster-scoped items first, then by namespace, kind and name', () => {
    render(<InventoryTabContent inventory={mockInventory} namespace="flux-system" />)

    const rows = document.querySelectorAll('tbody tr')
    const firstCells = Array.from(rows).map(r => r.querySelector('td').textContent)

    // Cluster-scoped (no namespace) sorts first
    expect(firstCells[0]).toBe('reader')
    // Then namespaced items: 'apps' namespace before 'flux-system',
    // within 'apps' ConfigMap before Deployment by kind
    expect(firstCells[1]).toBe('settings')
    expect(firstCells[2]).toBe('podinfo')
    expect(firstCells[3]).toBe('apps')
  })

  it('should sort multiple cluster-scoped items by kind then name', () => {
    const inventory = [
      { apiVersion: 'v1', kind: 'Namespace', name: 'production' },
      { apiVersion: 'v1', kind: 'Namespace', name: 'development' },
      { apiVersion: 'rbac.authorization.k8s.io/v1', kind: 'ClusterRole', name: 'admin' },
      { apiVersion: 'rbac.authorization.k8s.io/v1', kind: 'ClusterRole', name: 'reader' }
    ]

    render(<InventoryTabContent inventory={inventory} namespace="flux-system" />)

    const rows = document.querySelectorAll('tbody tr')

    // Sorted by kind first (ClusterRole before Namespace), then by name
    expect(rows[0].querySelectorAll('td')[0].textContent).toBe('admin')
    expect(rows[0].querySelectorAll('td')[2].textContent).toBe('ClusterRole')
    expect(rows[1].querySelectorAll('td')[0].textContent).toBe('reader')
    expect(rows[1].querySelectorAll('td')[2].textContent).toBe('ClusterRole')
    expect(rows[2].querySelectorAll('td')[0].textContent).toBe('development')
    expect(rows[2].querySelectorAll('td')[2].textContent).toBe('Namespace')
    expect(rows[3].querySelectorAll('td')[0].textContent).toBe('production')
    expect(rows[3].querySelectorAll('td')[2].textContent).toBe('Namespace')
  })

  it('should sort namespaced items by namespace then kind then name', () => {
    const inventory = [
      { apiVersion: 'v1', kind: 'ConfigMap', namespace: 'production', name: 'config-b' },
      { apiVersion: 'v1', kind: 'ConfigMap', namespace: 'production', name: 'config-a' },
      { apiVersion: 'v1', kind: 'ConfigMap', namespace: 'development', name: 'config-c' },
      { apiVersion: 'v1', kind: 'Secret', namespace: 'development', name: 'secret-a' }
    ]

    render(<InventoryTabContent inventory={inventory} namespace="flux-system" />)

    const rows = document.querySelectorAll('tbody tr')

    // Sorted by namespace (development first), then kind (ConfigMap before Secret), then name
    expect(rows[0].querySelectorAll('td')[0].textContent).toBe('config-c')
    expect(rows[0].querySelectorAll('td')[1].textContent).toBe('development')
    expect(rows[1].querySelectorAll('td')[0].textContent).toBe('secret-a')
    expect(rows[1].querySelectorAll('td')[1].textContent).toBe('development')
    expect(rows[2].querySelectorAll('td')[0].textContent).toBe('config-a')
    expect(rows[2].querySelectorAll('td')[1].textContent).toBe('production')
    expect(rows[3].querySelectorAll('td')[0].textContent).toBe('config-b')
    expect(rows[3].querySelectorAll('td')[1].textContent).toBe('production')
  })

  it('should render Flux resources as links and other kinds as plain text', () => {
    render(<InventoryTabContent inventory={mockInventory} namespace="flux-system" />)

    // Flux resources link to the resource dashboard
    expect(screen.getByRole('link', { name: 'apps' }))
      .toHaveAttribute('href', '/resource/Kustomization/flux-system/apps')

    // Workloads link to the workload dashboard
    expect(screen.getByRole('link', { name: 'podinfo' }))
      .toHaveAttribute('href', '/workload/Deployment/apps/podinfo')

    // A non-Flux, non-workload kind (ConfigMap) is plain text
    const configMap = screen.getByText('settings')
    expect(configMap.tagName).toBe('SPAN')
    expect(configMap.closest('a')).toBeNull()
  })

  it('should fall back to the provided namespace when an item has none for link routing', () => {
    render(
      <InventoryTabContent
        inventory={[{ kind: 'Kustomization', name: 'infra', apiVersion: 'kustomize.toolkit.fluxcd.io/v1' }]}
        namespace="flux-system"
      />
    )

    const link = screen.getByRole('link', { name: 'infra' })
    expect(link.getAttribute('href')).toContain('flux-system')
  })

  it('should render an empty table when inventory is undefined', () => {
    render(<InventoryTabContent namespace="flux-system" />)

    expect(document.querySelectorAll('tbody tr')).toHaveLength(0)
  })
})
