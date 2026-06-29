// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent, waitFor, within } from '@testing-library/preact'
import { InventoryTabContent } from './InventoryTabContent'
import { fetchWithMock } from '../../../utils/fetch'

// The batch status fetch is stubbed so the tab test is deterministic and offline.
vi.mock('../../../utils/fetch', () => ({
  fetchWithMock: vi.fn(() => Promise.resolve({ objects: [] }))
}))

// Stub the detail view so expansion is observable without a second fetch. The stub
// echoes the object name so we can assert which row is expanded.
vi.mock('./ObjectDetailsView', () => ({
  ObjectDetailsView: ({ isExpanded, name }) => (isExpanded ? <div data-testid={`details-${name}`}>details</div> : null)
}))

const inventory = [
  { kind: 'Deployment', name: 'podinfo', namespace: 'apps', apiVersion: 'apps/v1' },
  { kind: 'Kustomization', name: 'apps', namespace: 'flux-system', apiVersion: 'kustomize.toolkit.fluxcd.io/v1' },
  { kind: 'ConfigMap', name: 'settings', namespace: 'apps', apiVersion: 'v1' },
  { kind: 'ClusterRole', name: 'reader', namespace: '', apiVersion: 'rbac.authorization.k8s.io/v1' },
]

const rows = (container) => container.querySelectorAll('.card > div')
// A non-dashboard row (e.g. ConfigMap) is itself the toggle button; the whole row
// is the click target. Returns that row's button.
const rowToggle = (name) => {
  const row = screen.getAllByText(name)[0].closest('.border-b')
  return within(row).getByRole('button')
}

describe('InventoryTabContent', () => {
  beforeEach(() => vi.clearAllMocks())

  it('renders a row for each item with kind shown as plain text', () => {
    render(<InventoryTabContent inventory={inventory} />)
    const body = document.body.textContent
    expect(body).toContain('podinfo')
    expect(body).toContain('Kustomization')
    expect(body).toContain('ConfigMap')
    expect(body).toContain('ClusterRole')
  })

  it('sorts cluster-scoped first, then by namespace, kind, name', () => {
    const { container } = render(<InventoryTabContent inventory={inventory} />)
    const r = rows(container)
    expect(r[0].textContent).toContain('reader')      // cluster-scoped
    expect(r[1].textContent).toContain('settings')    // apps/ConfigMap
    expect(r[2].textContent).toContain('podinfo')     // apps/Deployment
    expect(r[3].textContent).toContain('apps')        // flux-system/Kustomization
  })

  it('shows an error pill (not perpetual "computing…") for objects the user cannot read', async () => {
    fetchWithMock.mockResolvedValue({
      objects: [{ apiVersion: 'v1', kind: 'ConfigMap', namespace: 'apps', name: 'settings', error: 'Forbidden' }]
    })
    render(<InventoryTabContent inventory={[{ kind: 'ConfigMap', name: 'settings', namespace: 'apps', apiVersion: 'v1' }]} />)
    await waitFor(() => expect(screen.getAllByText('Forbidden').length).toBeGreaterThan(0))
    expect(screen.queryByTestId('inventory-status-computing')).toBeNull()
  })

  it('fetches the batch status for the whole inventory on mount', () => {
    render(<InventoryTabContent inventory={inventory} />)
    expect(fetchWithMock).toHaveBeenCalledTimes(1)
    const call = fetchWithMock.mock.calls[0][0]
    expect(call.endpoint).toBe('/api/v1/inventory/objects')
    expect(call.method).toBe('POST')
    expect(call.body.objects).toHaveLength(4)
  })

  it('filters by the Flux category', () => {
    const { container } = render(<InventoryTabContent inventory={inventory} />)
    fireEvent.click(screen.getByTestId('inventory-cat-flux'))
    const r = rows(container)
    expect(r).toHaveLength(1)
    expect(r[0].textContent).toContain('Kustomization')
  })

  it('filters by the Workloads category', () => {
    const { container } = render(<InventoryTabContent inventory={inventory} />)
    fireEvent.click(screen.getByTestId('inventory-cat-workloads'))
    const r = rows(container)
    expect(r).toHaveLength(1)
    expect(r[0].textContent).toContain('Deployment')
  })

  it('filters by the Other category (neither Flux nor workload)', () => {
    const { container } = render(<InventoryTabContent inventory={inventory} />)
    fireEvent.click(screen.getByTestId('inventory-cat-other'))
    const r = rows(container)
    expect(r).toHaveLength(2) // ConfigMap + ClusterRole
    expect(container.textContent).toContain('ConfigMap')
    expect(container.textContent).toContain('ClusterRole')
    expect(container.textContent).not.toContain('Kustomization')
  })

  it('searches across fields and composes with the category', () => {
    const { container } = render(<InventoryTabContent inventory={inventory} />)
    // Search alone: "podinfo" matches the Deployment by name.
    fireEvent.input(screen.getByTestId('inventory-search'), { target: { value: 'podinfo' } })
    expect(rows(container)).toHaveLength(1)
    // Compose with a category that excludes it → empty.
    fireEvent.click(screen.getByTestId('inventory-cat-flux'))
    expect(screen.getByTestId('inventory-empty')).toBeInTheDocument()
  })

  it('clears both filters with the clear button', () => {
    const { container } = render(<InventoryTabContent inventory={inventory} />)
    fireEvent.click(screen.getByTestId('inventory-cat-flux'))
    fireEvent.input(screen.getByTestId('inventory-search'), { target: { value: 'nomatch' } })
    expect(screen.getByTestId('inventory-empty')).toBeInTheDocument()
    fireEvent.click(screen.getByTestId('inventory-clear'))
    expect(rows(container)).toHaveLength(4)
    expect(screen.getByTestId('inventory-search')).toHaveValue('')
  })

  it('shows the empty state when filters exclude everything', () => {
    render(<InventoryTabContent inventory={inventory} />)
    fireEvent.input(screen.getByTestId('inventory-search'), { target: { value: 'zzz-no-match' } })
    expect(screen.getByTestId('inventory-empty')).toHaveTextContent('No objects match the filters')
  })

  it('keeps an expanded row mounted across an inventory prop change (stable keys)', () => {
    const { rerender } = render(<InventoryTabContent inventory={inventory} />)
    fireEvent.click(rowToggle('settings'))
    expect(screen.getByTestId('details-settings')).toBeInTheDocument()

    // New array (poll), same items plus an added one — settings stays expanded.
    const added = [...inventory, { kind: 'Service', name: 'svc', namespace: 'apps', apiVersion: 'v1' }]
    rerender(<InventoryTabContent inventory={added} />)
    expect(screen.getByTestId('details-settings')).toBeInTheDocument()
  })

  it('unmounts a row (and its detail) when its item is pruned', () => {
    const { rerender } = render(<InventoryTabContent inventory={inventory} />)
    fireEvent.click(rowToggle('settings'))
    expect(screen.getByTestId('details-settings')).toBeInTheDocument()

    // settings removed from the inventory → its row and open detail disappear.
    const pruned = inventory.filter(i => i.name !== 'settings')
    rerender(<InventoryTabContent inventory={pruned} />)
    expect(screen.queryByTestId('details-settings')).toBeNull()
    expect(screen.queryByText('settings')).toBeNull()
  })

  it('renders the empty state for an undefined inventory', () => {
    render(<InventoryTabContent />)
    expect(screen.getByTestId('inventory-empty')).toBeInTheDocument()
  })
})
