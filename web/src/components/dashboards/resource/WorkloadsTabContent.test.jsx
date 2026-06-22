// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/preact'
import { WorkloadsTabContent } from './WorkloadsTabContent'

describe('WorkloadsTabContent component', () => {
  const mockWorkloadItems = [
    { kind: 'Deployment', name: 'podinfo', namespace: 'default' },
    { kind: 'StatefulSet', name: 'redis', namespace: 'default' }
  ]

  const mockStatuses = {
    'Deployment/default/podinfo': {
      status: 'Current',
      statusMessage: 'Deployment has minimum availability.'
    },
    'StatefulSet/default/redis': {
      status: 'InProgress',
      statusMessage: 'Waiting for replicas to be ready'
    }
  }

  it('should render a row for each workload with kind and namespace/name', () => {
    render(
      <WorkloadsTabContent
        workloadItems={mockWorkloadItems}
        namespace="default"
        workloadStatuses={mockStatuses}
      />
    )

    const textContent = document.body.textContent
    expect(textContent).toContain('Deployment')
    expect(textContent).toContain('default/podinfo')
    expect(textContent).toContain('StatefulSet')
    expect(textContent).toContain('default/redis')
  })

  it('should display status badges with correct colors from provided statuses', () => {
    render(
      <WorkloadsTabContent
        workloadItems={mockWorkloadItems}
        namespace="default"
        workloadStatuses={mockStatuses}
      />
    )

    const readyBadge = screen.getByText('Ready')
    expect(readyBadge).toBeInTheDocument()
    expect(readyBadge.className).toContain('bg-green-100')

    const progressingBadge = screen.getByText('Progressing')
    expect(progressingBadge).toBeInTheDocument()
    expect(progressingBadge.className).toContain('bg-blue-100')
  })

  it('should display the workload status message', () => {
    render(
      <WorkloadsTabContent
        workloadItems={[mockWorkloadItems[0]]}
        namespace="default"
        workloadStatuses={mockStatuses}
      />
    )

    expect(screen.getByText('Deployment has minimum availability.')).toBeInTheDocument()
  })

  it('should show a computing placeholder for workloads with no status yet', () => {
    // No statuses provided: every row shows the placeholder, keeping its shape
    render(
      <WorkloadsTabContent
        workloadItems={mockWorkloadItems}
        namespace="default"
        workloadStatuses={{}}
      />
    )

    expect(screen.getAllByTestId('workload-status-computing')).toHaveLength(2)
    expect(screen.queryByText('Ready')).not.toBeInTheDocument()
  })

  it('should show a real badge for known workloads and a placeholder for unknown ones', () => {
    const partial = { 'Deployment/default/podinfo': { status: 'Current', statusMessage: 'ok' } }

    render(
      <WorkloadsTabContent
        workloadItems={mockWorkloadItems}
        namespace="default"
        workloadStatuses={partial}
      />
    )

    // podinfo has a status, redis does not
    expect(screen.getByText('Ready')).toBeInTheDocument()
    expect(screen.getAllByTestId('workload-status-computing')).toHaveLength(1)
  })

  it('should link each row to the dedicated workload dashboard', () => {
    render(
      <WorkloadsTabContent
        workloadItems={mockWorkloadItems}
        namespace="default"
        workloadStatuses={mockStatuses}
      />
    )

    const hrefs = Array.from(document.querySelectorAll('a[href]')).map(a => a.getAttribute('href'))
    expect(hrefs).toContain('/workload/Deployment/default/podinfo')
    expect(hrefs).toContain('/workload/StatefulSet/default/redis')
  })

  it('should use fallback namespace in both the label and the link', () => {
    const itemsWithoutNamespace = [{ kind: 'Deployment', name: 'podinfo' }]
    const statuses = { 'Deployment/custom-namespace/podinfo': { status: 'Current', statusMessage: 'ok' } }

    render(
      <WorkloadsTabContent
        workloadItems={itemsWithoutNamespace}
        namespace="custom-namespace"
        workloadStatuses={statuses}
      />
    )

    expect(document.body.textContent).toContain('custom-namespace/podinfo')
    const link = document.querySelector('a[href]')
    expect(link.getAttribute('href')).toBe('/workload/Deployment/custom-namespace/podinfo')
  })

  it('should not render an expandable drawer (no buttons, images, or action bar)', () => {
    render(
      <WorkloadsTabContent
        workloadItems={[mockWorkloadItems[0]]}
        namespace="default"
        workloadStatuses={mockStatuses}
      />
    )

    expect(screen.queryAllByRole('button')).toHaveLength(0)
    expect(screen.queryByTestId('workload-action-bar')).not.toBeInTheDocument()
    expect(screen.queryByTestId('delete-pod-button')).not.toBeInTheDocument()
    expect(screen.queryByText('Images')).not.toBeInTheDocument()
    expect(screen.queryByText('Pods')).not.toBeInTheDocument()
  })

  it('should render nothing when there are no workloads', () => {
    const { container } = render(
      <WorkloadsTabContent
        workloadItems={[]}
        namespace="default"
        workloadStatuses={{}}
      />
    )

    expect(container.querySelectorAll('a[href]')).toHaveLength(0)
  })
})
