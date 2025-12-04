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
      cpuLimit: 2.0,
      memory: 512 * 1024 * 1024, // 512 MiB
      memoryLimit: 1024 * 1024 * 1024 // 1 GiB
    },
    {
      pod: 'helm-controller-abc',
      cpu: 0.05,
      cpuLimit: 1.0,
      memory: 256 * 1024 * 1024,
      memoryLimit: 512 * 1024 * 1024
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

  it('should render resource metrics when available', async () => {
    render(<ControllersPanel components={mockComponents} metrics={mockMetrics} />)

    // Expand row to see metrics
    const button = screen.getByText('source-controller').closest('button')
    await fireEvent.click(button)

    // CPU: 0.100/2.0 cores (5%)
    expect(screen.getByText(/0\.100\/2\.0 cores \(5%\)/)).toBeInTheDocument()
    // Memory: 512/1024 MiB (50%)
    expect(screen.getByText(/512\/1024 MiB \(50%\)/)).toBeInTheDocument()
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

  it('should display resource requests/limits if available', async () => {
    // Note: The component implementation might need to be updated to show this if it's not already
    // For now, we check if it doesn't crash with complex metrics
    const complexMetrics = [{
      pod: 'source-controller-xyz',
      cpu: 0.1,
      memory: 128 * 1024 * 1024,
      cpuLimit: 0.2,
      memoryLimit: 256 * 1024 * 1024
    }]

    render(<ControllersPanel components={mockComponents} metrics={complexMetrics} />)

    // Expand row
    const button = screen.getByText('source-controller').closest('button')
    await fireEvent.click(button)

    expect(screen.getByText(/0\.100\/0\.2 cores \(50%\)/)).toBeInTheDocument()
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

  it('should handle metrics with zero limits gracefully', async () => {
    const metricsWithZeroLimits = [{
      pod: 'source-controller-xyz',
      cpu: 0.1,
      cpuLimit: 0,
      memory: 128 * 1024 * 1024,
      memoryLimit: 0
    }]

    const components = [
      { name: 'source-controller', ready: true, status: 'Running', image: 'test:v1' }
    ]

    render(<ControllersPanel components={components} metrics={metricsWithZeroLimits} />)

    // Expand row
    const button = screen.getByText('source-controller').closest('button')
    await fireEvent.click(button)

    // Should show 0% when limit is 0 (avoid division by zero)
    expect(screen.getByText(/0\.100\/0\.0 cores \(0%\)/)).toBeInTheDocument()
    expect(screen.getByText(/128\/0 MiB \(0%\)/)).toBeInTheDocument()
  })

  it('should handle negative memory values gracefully', async () => {
    const metricsWithNegative = [{
      pod: 'source-controller-xyz',
      cpu: 0.1,
      cpuLimit: 1,
      memory: -100,
      memoryLimit: 1024 * 1024 * 1024
    }]

    const components = [
      { name: 'source-controller', ready: true, status: 'Running', image: 'test:v1' }
    ]

    render(<ControllersPanel components={components} metrics={metricsWithNegative} />)

    // Expand row
    const button = screen.getByText('source-controller').closest('button')
    await fireEvent.click(button)

    // Should show 0 for negative memory (formatMemory handles this)
    // Percentage is clamped to 0% for negative values
    expect(screen.getByText(/0\/1024 MiB \(0%\)/)).toBeInTheDocument()
  })

  it('should handle metrics with no matching pod', async () => {
    const metricsNoMatch = [{
      pod: 'other-controller-xyz',
      cpu: 0.1,
      cpuLimit: 1,
      memory: 128 * 1024 * 1024,
      memoryLimit: 256 * 1024 * 1024
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
      cpuLimit: 1,
      memory: 128 * 1024 * 1024,
      memoryLimit: 256 * 1024 * 1024
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
      cpuLimit: 1,
      memory: 128 * 1024 * 1024,
      memoryLimit: 256 * 1024 * 1024
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
