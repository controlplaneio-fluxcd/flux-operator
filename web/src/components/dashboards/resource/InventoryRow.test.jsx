// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/preact'
import { InventoryRow } from './InventoryRow'

// Stub the detail view so the row test stays a unit test of the row itself.
vi.mock('./ObjectDetailsView', () => ({
  ObjectDetailsView: ({ isExpanded }) => (isExpanded ? <div data-testid="object-details">details</div> : null)
}))

const fluxItem = { apiVersion: 'kustomize.toolkit.fluxcd.io/v1', kind: 'Kustomization', namespace: 'flux-system', name: 'apps' }
const configItem = { apiVersion: 'v1', kind: 'ConfigMap', namespace: 'apps', name: 'podinfo-config' }
const clusterItem = { apiVersion: 'rbac.authorization.k8s.io/v1', kind: 'ClusterRole', namespace: '', name: 'podinfo-reader' }

describe('InventoryRow', () => {
  it('renders the kind as plain text (no chip alias)', () => {
    render(<InventoryRow item={configItem} />)
    // Kind shown verbatim (desktop + mobile), not a chip alias like "cm".
    expect(screen.getAllByText('ConfigMap').length).toBeGreaterThan(0)
  })

  it('makes the whole row a dashboard link for Flux/workload kinds', () => {
    render(<InventoryRow item={fluxItem} />)
    // The entire row is a single link to the dashboard; it never expands.
    expect(screen.getByRole('link')).toHaveAttribute('href', '/resource/Kustomization/flux-system/apps')
    expect(screen.queryByRole('button')).toBeNull()
    expect(screen.queryByTestId('object-details')).toBeNull()
  })

  it('renders no link for non-Flux/non-workload kinds', () => {
    render(<InventoryRow item={configItem} />)
    expect(screen.queryByRole('link')).toBeNull()
  })

  it('renders a cluster-scoped name without a namespace prefix', () => {
    render(<InventoryRow item={clusterItem} />)
    // Name present, no "ns/" prefix anywhere.
    expect(screen.getAllByText('podinfo-reader').length).toBeGreaterThan(0)
    expect(screen.queryByText('/', { exact: false })).toBeNull()
  })

  it('shows the computing placeholder until a status is provided', () => {
    render(<InventoryRow item={configItem} />)
    expect(screen.getAllByTestId('inventory-status-computing').length).toBeGreaterThan(0)
  })

  it('shows the live status badge from the injected status', () => {
    // formatWorkloadStatus maps kstatus "Current" to the display label "Ready".
    render(<InventoryRow item={configItem} status={{ status: 'Current', statusMessage: '' }} />)
    expect(screen.queryByTestId('inventory-status-computing')).toBeNull()
    expect(screen.getAllByText('Ready').length).toBeGreaterThan(0)
  })

  it('expands the detail view when a non-dashboard row is clicked', () => {
    render(<InventoryRow item={configItem} />)
    expect(screen.queryByTestId('object-details')).toBeNull()
    // The whole row is the click target for kinds without a dashboard.
    fireEvent.click(screen.getByRole('button'))
    expect(screen.getByTestId('object-details')).toBeInTheDocument()
  })
})
