// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/preact'
import { ObjectDetailsView } from './ObjectDetailsView'
import { fetchWithMock } from '../../../utils/fetch'

vi.mock('../../../utils/fetch', () => ({
  fetchWithMock: vi.fn()
}))

// A Flux object (linkable, has spec + status).
const fluxResult = {
  apiVersion: 'kustomize.toolkit.fluxcd.io/v1',
  kind: 'Kustomization',
  namespace: 'flux-system',
  name: 'apps',
  status: 'Ready',
  statusMessage: 'Applied revision: main@sha1:abc',
  object: {
    apiVersion: 'kustomize.toolkit.fluxcd.io/v1',
    kind: 'Kustomization',
    metadata: { name: 'apps', namespace: 'flux-system', creationTimestamp: '2026-01-10T08:00:00Z' },
    spec: { interval: '10m', prune: true },
    status: { conditions: [{ type: 'Ready', status: 'True', message: 'Applied revision: main@sha1:abc' }] }
  }
}

// A ConfigMap (not linkable, no spec → Spec falls back to body, no status tab).
const configMapResult = {
  apiVersion: 'v1',
  kind: 'ConfigMap',
  namespace: 'apps',
  name: 'podinfo-config',
  status: 'Current',
  object: {
    apiVersion: 'v1',
    kind: 'ConfigMap',
    metadata: { name: 'podinfo-config', namespace: 'apps' },
    data: { 'app.properties': 'color=blue' }
  }
}

const fluxProps = { apiVersion: 'kustomize.toolkit.fluxcd.io/v1', kind: 'Kustomization', namespace: 'flux-system', name: 'apps' }

const resp = (result) => ({ objects: [result] })

describe('ObjectDetailsView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders nothing when collapsed', () => {
    const { container } = render(
      <ObjectDetailsView apiVersion="v1" kind="ConfigMap" namespace="apps" name="x" isExpanded={false} />
    )
    expect(container.firstChild).toBeNull()
    expect(fetchWithMock).not.toHaveBeenCalled()
  })

  it('lazily fetches the object as a single-item POST on expand', async () => {
    fetchWithMock.mockResolvedValue(resp(fluxResult))
    render(
      <ObjectDetailsView apiVersion="kustomize.toolkit.fluxcd.io/v1" kind="Kustomization" namespace="flux-system" name="apps" isExpanded />
    )
    await waitFor(() => expect(fetchWithMock).toHaveBeenCalledTimes(1))
    const call = fetchWithMock.mock.calls[0][0]
    expect(call.endpoint).toBe('/api/v1/inventory/objects')
    expect(call.method).toBe('POST')
    expect(call.body).toEqual({ objects: [{ apiVersion: 'kustomize.toolkit.fluxcd.io/v1', kind: 'Kustomization', namespace: 'flux-system', name: 'apps' }] })
  })

  it('renders the whole manifest as a single YAML block, no tabs', async () => {
    fetchWithMock.mockResolvedValue(resp(fluxResult))
    render(
      <ObjectDetailsView apiVersion="kustomize.toolkit.fluxcd.io/v1" kind="Kustomization" namespace="flux-system" name="apps" isExpanded />
    )
    await waitFor(() => expect(screen.getByTestId('object-yaml')).toBeInTheDocument())
    // The full manifest is shown — spec and status together — with no Spec/Status tabs.
    const yaml = screen.getByTestId('object-yaml').textContent
    expect(yaml).toContain('kind: Kustomization')
    expect(yaml).toContain('interval: 10m')
    expect(yaml).toContain('Applied revision: main@sha1:abc')
    expect(screen.queryByRole('button', { name: 'Spec' })).toBeNull()
    expect(screen.queryByRole('button', { name: 'Status' })).toBeNull()
  })

  it('renders the manifest for a spec-less object', async () => {
    fetchWithMock.mockResolvedValue(resp(configMapResult))
    render(
      <ObjectDetailsView apiVersion="v1" kind="ConfigMap" namespace="apps" name="podinfo-config" isExpanded />
    )
    await waitFor(() => expect(screen.getByTestId('object-yaml')).toBeInTheDocument())
    const yaml = screen.getByTestId('object-yaml').textContent
    expect(yaml).toContain('kind: ConfigMap')
    expect(yaml).toContain('color=blue')
    expect(screen.queryByRole('link')).toBeNull()
  })

  it('shows the not-found message for a pruned object', async () => {
    fetchWithMock.mockResolvedValue(resp({
      apiVersion: 'v1', kind: 'ConfigMap', namespace: 'apps', name: 'gone', error: 'NotFound'
    }))
    render(
      <ObjectDetailsView apiVersion="v1" kind="ConfigMap" namespace="apps" name="gone" isExpanded />
    )
    await waitFor(() => expect(screen.getByText('Object no longer exists in the cluster.')).toBeInTheDocument())
  })

  it('shows the forbidden message when access is denied', async () => {
    fetchWithMock.mockResolvedValue(resp({
      apiVersion: 'v1', kind: 'Secret', namespace: 'apps', name: 'creds', error: 'Forbidden'
    }))
    render(
      <ObjectDetailsView apiVersion="v1" kind="Secret" namespace="apps" name="creds" isExpanded />
    )
    await waitFor(() => expect(screen.getByText('You do not have permission to view this object.')).toBeInTheDocument())
  })

  it('shows an error state when the fetch rejects', async () => {
    fetchWithMock.mockRejectedValue(new Error('boom'))
    render(
      <ObjectDetailsView apiVersion="v1" kind="ConfigMap" namespace="apps" name="x" isExpanded />
    )
    await waitFor(() => expect(screen.getByText(/Failed to load details: boom/)).toBeInTheDocument())
  })

  it('refetches in the background on poll and swaps content without a spinner', async () => {
    fetchWithMock.mockResolvedValue(resp(fluxResult))
    const { rerender } = render(<ObjectDetailsView {...fluxProps} isExpanded refreshKey={1} />)
    await waitFor(() => expect(screen.getByTestId('object-yaml').textContent).toContain('interval: 10m'))

    // Next poll tick returns an updated manifest.
    const updated = { ...fluxResult, object: { ...fluxResult.object, spec: { interval: '15m', prune: true } } }
    fetchWithMock.mockResolvedValue(resp(updated))
    rerender(<ObjectDetailsView {...fluxProps} isExpanded refreshKey={2} />)

    // No loading spinner during a background refetch; last-good content stays.
    expect(screen.queryByText('Loading details…')).toBeNull()
    expect(screen.getByTestId('object-yaml').textContent).toContain('interval: 10m')

    // Content swaps once the refetch resolves.
    await waitFor(() => expect(screen.getByTestId('object-yaml').textContent).toContain('interval: 15m'))
  })

  it('shows the not-found message if the object is deleted while open', async () => {
    fetchWithMock.mockResolvedValue(resp(fluxResult))
    const { rerender } = render(<ObjectDetailsView {...fluxProps} isExpanded refreshKey={1} />)
    await waitFor(() => expect(screen.getByTestId('object-yaml')).toBeInTheDocument())

    // The object is pruned: the poll refetch reports NotFound.
    fetchWithMock.mockResolvedValue(resp({ ...fluxProps, error: 'NotFound' }))
    rerender(<ObjectDetailsView {...fluxProps} isExpanded refreshKey={2} />)
    await waitFor(() => expect(screen.getByText('Object no longer exists in the cluster.')).toBeInTheDocument())
  })

  it('does not refetch on poll while the panel is collapsed', async () => {
    fetchWithMock.mockResolvedValue(resp(fluxResult))
    const { rerender } = render(<ObjectDetailsView {...fluxProps} isExpanded={false} refreshKey={1} />)
    rerender(<ObjectDetailsView {...fluxProps} isExpanded={false} refreshKey={2} />)
    expect(fetchWithMock).not.toHaveBeenCalled()
  })

  it('calls onReady once the fetch settles', async () => {
    fetchWithMock.mockResolvedValue(resp(configMapResult))
    const onReady = vi.fn()
    render(
      <ObjectDetailsView apiVersion="v1" kind="ConfigMap" namespace="apps" name="podinfo-config" isExpanded onReady={onReady} />
    )
    await waitFor(() => expect(onReady).toHaveBeenCalledTimes(1))
  })
})
