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

    // 0.100 / 2.0 cores
    expect(screen.getByText(/0.100 \/ 2.0 cores/)).toBeInTheDocument()
    // 512 / 1024 MiB
    expect(screen.getByText(/512 \/ 1024 MiB/)).toBeInTheDocument()
  })

  it('should gracefully handle missing metrics', async () => {
    render(<ControllersPanel components={mockComponents} metrics={[]} />)

    // Expand row
    const button = screen.getByText('source-controller').closest('button')
    await fireEvent.click(button)

    // Should still render components
    expect(screen.getByText('source-controller')).toBeInTheDocument()
    // Should not show metrics (or show "Not available")
    expect(screen.getByText('Not available')).toBeInTheDocument()
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

    expect(screen.getByText(/0.100 \/ 0.2 cores/)).toBeInTheDocument()
  })
})
