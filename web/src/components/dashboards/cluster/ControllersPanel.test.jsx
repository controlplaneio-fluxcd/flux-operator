// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/preact'
import { ControllersPanel } from './ControllersPanel'

describe('ControllersPanel', () => {
  const mockComponents = [
    { name: 'source-controller', ready: true, status: 'Running' },
    { name: 'kustomize-controller', ready: true, status: 'Running' },
    { name: 'helm-controller', ready: false, status: 'CrashLoopBackOff' }
  ]

  const mockMetrics = [
    {
      pod: 'source-controller-xyz',
      cpu: 0.1,
      cpuRequests: 0.05,
      cpuLimits: 2.0,
      memory: 512 * 1024 * 1024, // 512 MiB
      memoryRequests: 256 * 1024 * 1024,
      memoryLimits: 1024 * 1024 * 1024 // 1 GiB
    },
    {
      pod: 'helm-controller-abc',
      cpu: 0.05,
      cpuRequests: 0.05,
      cpuLimits: 1.0,
      memory: 256 * 1024 * 1024,
      memoryRequests: 128 * 1024 * 1024,
      memoryLimits: 512 * 1024 * 1024
    }
  ]

  it('should render section title', () => {
    render(<ControllersPanel components={mockComponents} />)

    expect(screen.getByText('Flux Components')).toBeInTheDocument()
  })

  it('should render all components', () => {
    render(<ControllersPanel components={mockComponents} />)

    expect(screen.getByText('source-controller')).toBeInTheDocument()
    expect(screen.getByText('kustomize-controller')).toBeInTheDocument()
    expect(screen.getByText('helm-controller')).toBeInTheDocument()
  })

  it('should render ready badge for healthy components', () => {
    render(<ControllersPanel components={mockComponents} />)

    const readyBadges = screen.getAllByText('Ready')
    expect(readyBadges.length).toBe(2)
  })

  it('should render failing badge for unhealthy components', () => {
    render(<ControllersPanel components={mockComponents} />)

    expect(screen.getByText('Failing')).toBeInTheDocument()
  })

  it('should render progressing badge while a component is rolling out', () => {
    const components = [
      { name: 'source-controller', ready: true, status: 'Current Deployment is available. Replicas: 1' },
      { name: 'kustomize-controller', ready: false, status: 'InProgress Deployment not Available' }
    ]

    render(<ControllersPanel components={components} />)

    expect(screen.getByText('Ready')).toBeInTheDocument()
    expect(screen.getByText('Progressing')).toBeInTheDocument()
    expect(screen.queryByText('Failing')).not.toBeInTheDocument()
    // The header counts the rollout as progressing, not failing.
    expect(screen.getByText('1 progressing')).toBeInTheDocument()
    expect(screen.queryByText(/failing/)).not.toBeInTheDocument()
  })

  it('should count progressing and failing components separately', () => {
    const components = [
      { name: 'source-controller', ready: false, status: 'InProgress Deployment not Available' },
      { name: 'kustomize-controller', ready: false, status: 'InProgress Deployment not Available' },
      { name: 'helm-controller', ready: false, status: 'Failed Deployment has failed' }
    ]

    render(<ControllersPanel components={components} />)

    expect(screen.getByText('2 progressing')).toBeInTheDocument()
    expect(screen.getByText('1 failing')).toBeInTheDocument()
  })

  it('should render resource metrics when available', async () => {
    render(<ControllersPanel components={mockComponents} metrics={mockMetrics} />)

    // Expand row to see metrics
    const button = screen.getByText('source-controller').closest('button')
    await fireEvent.click(button)

    // Absolute values first, percentages of the real requests/limits after.
    expect(screen.getByText('100m')).toBeInTheDocument()
    expect(screen.getByText(/200% of request · 5% of limit/)).toBeInTheDocument()
    expect(screen.getByText('512 MiB')).toBeInTheDocument()
    expect(screen.getByText(/200% of request · 50% of limit/)).toBeInTheDocument()
  })

  it('should sum usage across all pods of a scaled component', async () => {
    const haMetrics = [
      {
        pod: 'source-controller-aaa',
        cpu: 0.1,
        cpuRequests: 0.1,
        cpuLimits: 1.0,
        memory: 128 * 1024 * 1024,
        memoryRequests: 128 * 1024 * 1024,
        memoryLimits: 512 * 1024 * 1024
      },
      {
        pod: 'source-controller-bbb',
        cpu: 0.3,
        cpuRequests: 0.1,
        cpuLimits: 1.0,
        memory: 128 * 1024 * 1024,
        memoryRequests: 128 * 1024 * 1024,
        memoryLimits: 512 * 1024 * 1024
      }
    ]

    render(<ControllersPanel components={mockComponents} metrics={haMetrics} />)

    const button = screen.getByText('source-controller').closest('button')
    await fireEvent.click(button)

    // 0.4 cores of 0.2 requests and 2.0 limits, 256 MiB of 256 MiB requests and 1 GiB limits.
    expect(screen.getByText('400m')).toBeInTheDocument()
    expect(screen.getByText(/200% of request · 20% of limit/)).toBeInTheDocument()
    expect(screen.getByText('256 MiB')).toBeInTheDocument()
    expect(screen.getByText(/100% of request · 25% of limit/)).toBeInTheDocument()
  })

  it('should gracefully handle missing metrics', async () => {
    render(<ControllersPanel components={mockComponents} metrics={[]} />)

    // Expand row
    const button = screen.getByText('source-controller').closest('button')
    await fireEvent.click(button)

    // Should still render components
    expect(screen.getByText('source-controller')).toBeInTheDocument()
    // Metrics section should be hidden when no metrics available
    expect(screen.queryByText('CPU')).not.toBeInTheDocument()
    expect(screen.queryByText('Memory')).not.toBeInTheDocument()
  })

  it('should sort components by name', () => {
    const unsortedComponents = [
      { name: 'z-controller', ready: true },
      { name: 'a-controller', ready: true }
    ]

    render(<ControllersPanel components={unsortedComponents} />)

    const components = screen.getAllByText(/-controller/)
    expect(components[0]).toHaveTextContent('a-controller')
    expect(components[1]).toHaveTextContent('z-controller')
  })

  it('should handle null/undefined components gracefully', () => {
    const { container } = render(<ControllersPanel components={null} />)

    // Should render empty container or nothing, but not crash
    expect(container).toBeInTheDocument()
  })

  it('should handle empty components array', () => {
    render(<ControllersPanel components={[]} />)

    expect(screen.getByText('Flux Components')).toBeInTheDocument()
    // No components to find
    expect(screen.queryByText(/-controller/)).not.toBeInTheDocument()
  })

  it('should render status message if available', async () => {
    const componentsWithMsg = [
      { name: 'helm-controller', ready: false, status: 'CrashLoopBackOff' }
    ]

    render(<ControllersPanel components={componentsWithMsg} />)

    // Expand row
    const button = screen.getByText('helm-controller').closest('button')
    await fireEvent.click(button)

    expect(screen.getByText('CrashLoopBackOff')).toBeInTheDocument()
  })

  it('should display the limit percentage without requests', async () => {
    const complexMetrics = [{
      pod: 'source-controller-xyz',
      cpu: 0.1,
      memory: 128 * 1024 * 1024,
      cpuLimits: 0.2,
      memoryLimits: 256 * 1024 * 1024
    }]

    render(<ControllersPanel components={mockComponents} metrics={complexMetrics} />)

    // Expand row
    const button = screen.getByText('source-controller').closest('button')
    await fireEvent.click(button)

    // Only the limit fragment is shown when requests are unset,
    // on both the CPU and the Memory rows.
    expect(screen.getAllByText(/^· 50% of limit$/)).toHaveLength(2)
    expect(screen.queryByText(/of request/)).not.toBeInTheDocument()
  })

  it('should collapse and expand the panel', async () => {
    render(<ControllersPanel components={mockComponents} />)

    // Panel should be expanded by default - table should be visible
    expect(screen.getByRole('table')).toBeInTheDocument()

    // Click the panel header to collapse
    const panelHeader = screen.getByText('Flux Components').closest('button')
    await fireEvent.click(panelHeader)

    // Table should no longer be visible
    expect(screen.queryByRole('table')).not.toBeInTheDocument()

    // Click again to expand
    await fireEvent.click(panelHeader)

    // Table should be visible again
    expect(screen.getByRole('table')).toBeInTheDocument()
  })

  it('should toggle row expansion on and off', async () => {
    render(<ControllersPanel components={mockComponents} />)

    // Click to expand row
    const button = screen.getByText('source-controller').closest('button')
    await fireEvent.click(button)

    // Should show expanded content with status message (Running only appears in expanded row)
    expect(screen.getByText('Running')).toBeInTheDocument()

    // Click again to collapse
    await fireEvent.click(button)

    // Expanded content should be gone
    expect(screen.queryByText('Running')).not.toBeInTheDocument()
  })

  it('should display version from image string with tag', async () => {
    const componentsWithImage = [
      {
        name: 'source-controller',
        ready: true,
        status: 'Running',
        image: 'ghcr.io/fluxcd/source-controller:v1.2.3'
      }
    ]

    render(<ControllersPanel components={componentsWithImage} />)

    expect(screen.getByText('v1.2.3')).toBeInTheDocument()
  })

  it('should display "latest" when image has no version tag', async () => {
    const componentsWithImage = [
      {
        name: 'source-controller',
        ready: true,
        status: 'Running',
        image: 'ghcr.io/fluxcd/source-controller'
      }
    ]

    render(<ControllersPanel components={componentsWithImage} />)

    expect(screen.getByText('latest')).toBeInTheDocument()
  })

  it('should display "unknown" when image is empty or null', async () => {
    const componentsWithoutImage = [
      {
        name: 'source-controller',
        ready: true,
        status: 'Running',
        image: ''
      }
    ]

    render(<ControllersPanel components={componentsWithoutImage} />)

    expect(screen.getByText('unknown')).toBeInTheDocument()
  })

  it('should handle image with digest after version', async () => {
    const componentsWithImage = [
      {
        name: 'source-controller',
        ready: true,
        status: 'Running',
        image: 'ghcr.io/fluxcd/source-controller:v1.2.3@sha256:abc123'
      }
    ]

    render(<ControllersPanel components={componentsWithImage} />)

    // Should show v1.2.3, not the sha256 part
    expect(screen.getByText('v1.2.3')).toBeInTheDocument()
  })

  it('should display image and digest separately in expanded row', async () => {
    const componentsWithImage = [
      {
        name: 'source-controller',
        ready: true,
        status: 'Running',
        image: 'ghcr.io/fluxcd/source-controller:v1.2.3@sha256:abc123'
      }
    ]

    render(<ControllersPanel components={componentsWithImage} />)

    // Expand row
    const button = screen.getByText('source-controller').closest('button')
    await fireEvent.click(button)

    // Image and digest should be split
    expect(screen.getByText('ghcr.io/fluxcd/source-controller:v1.2.3')).toBeInTheDocument()
    expect(screen.getByText('sha256:abc123')).toBeInTheDocument()
    expect(screen.getByText('Digest')).toBeInTheDocument()
  })

  it('should show failing count badge when components are failing', () => {
    render(<ControllersPanel components={mockComponents} />)

    // mockComponents has 1 failing component
    expect(screen.getByText('1 failing')).toBeInTheDocument()
  })

  it('should not show failing count badge when all components are ready', () => {
    const allReadyComponents = [
      { name: 'source-controller', ready: true, status: 'Running' },
      { name: 'kustomize-controller', ready: true, status: 'Running' }
    ]

    render(<ControllersPanel components={allReadyComponents} />)

    expect(screen.queryByText(/failing/)).not.toBeInTheDocument()
  })

  it('should display component count in header', () => {
    render(<ControllersPanel components={mockComponents} />)

    expect(screen.getByText('3 controllers deployed')).toBeInTheDocument()
  })

  it('should show absolute values only when no requests/limits are set', async () => {
    const metricsWithZeroLimits = [{
      pod: 'source-controller-xyz',
      cpu: 0.1,
      cpuRequests: 0,
      cpuLimits: 0,
      memory: 128 * 1024 * 1024,
      memoryRequests: 0,
      memoryLimits: 0
    }]

    const components = [
      { name: 'source-controller', ready: true, status: 'Running', image: 'test:v1' }
    ]

    render(<ControllersPanel components={components} metrics={metricsWithZeroLimits} />)

    // Expand row
    const button = screen.getByText('source-controller').closest('button')
    await fireEvent.click(button)

    // Absolute values without any percentage or progress bar.
    expect(screen.getByText('100m')).toBeInTheDocument()
    expect(screen.getByText('128 MiB')).toBeInTheDocument()
    expect(screen.queryByText(/of request/)).not.toBeInTheDocument()
    expect(screen.queryByText(/of limit/)).not.toBeInTheDocument()
  })

  it('should handle negative memory values gracefully', async () => {
    const metricsWithNegative = [{
      pod: 'source-controller-xyz',
      cpu: 0.1,
      cpuLimits: 1,
      memory: -100,
      memoryLimits: 1024 * 1024 * 1024
    }]

    const components = [
      { name: 'source-controller', ready: true, status: 'Running', image: 'test:v1' }
    ]

    render(<ControllersPanel components={components} metrics={metricsWithNegative} />)

    // Expand row
    const button = screen.getByText('source-controller').closest('button')
    await fireEvent.click(button)

    // Negative memory renders as 0 with a 0% limit percentage.
    expect(screen.getByText('0')).toBeInTheDocument()
    expect(screen.getByText(/^· 0% of limit$/)).toBeInTheDocument()
  })

  it('should handle metrics with no matching pod', async () => {
    const metricsNoMatch = [{
      pod: 'other-controller-xyz',
      cpu: 0.1,
      cpuLimits: 1,
      memory: 128 * 1024 * 1024,
      memoryLimits: 256 * 1024 * 1024
    }]

    const components = [
      { name: 'source-controller', ready: true, status: 'Running', image: 'test:v1' }
    ]

    render(<ControllersPanel components={components} metrics={metricsNoMatch} />)

    // Expand row
    const button = screen.getByText('source-controller').closest('button')
    await fireEvent.click(button)

    // Metrics section should be hidden since no matching metrics
    expect(screen.queryByText('CPU')).not.toBeInTheDocument()
  })

  it('should handle metrics with pod that does not start with component name', async () => {
    const metricsPartialMatch = [{
      pod: 'source-controller', // No suffix like -xyz
      cpu: 0.1,
      cpuLimits: 1,
      memory: 128 * 1024 * 1024,
      memoryLimits: 256 * 1024 * 1024
    }]

    const components = [
      { name: 'source-controller', ready: true, status: 'Running', image: 'test:v1' }
    ]

    render(<ControllersPanel components={components} metrics={metricsPartialMatch} />)

    // Expand row
    const button = screen.getByText('source-controller').closest('button')
    await fireEvent.click(button)

    // Exact match without hyphen suffix should not match
    expect(screen.queryByText('CPU')).not.toBeInTheDocument()
  })

  it('should handle metrics with undefined pod', async () => {
    const metricsUndefinedPod = [{
      cpu: 0.1,
      cpuLimits: 1,
      memory: 128 * 1024 * 1024,
      memoryLimits: 256 * 1024 * 1024
    }]

    const components = [
      { name: 'source-controller', ready: true, status: 'Running', image: 'test:v1' }
    ]

    render(<ControllersPanel components={components} metrics={metricsUndefinedPod} />)

    // Expand row
    const button = screen.getByText('source-controller').closest('button')
    await fireEvent.click(button)

    // Metrics section should be hidden since pod is undefined
    expect(screen.queryByText('CPU')).not.toBeInTheDocument()
  })
})
