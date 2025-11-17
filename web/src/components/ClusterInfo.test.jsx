// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/preact'
import { ClusterInfo } from './ClusterInfo'

describe('ClusterInfo', () => {
  const baseProps = {
    cluster: {
      serverVersion: 'v1.28.0',
      platform: 'aws',
      nodes: 3
    },
    distribution: {
      version: 'v2.4.0'
    },
    operator: {
      version: 'v0.1.0'
    },
    components: [
      { name: 'source-controller' },
      { name: 'kustomize-controller' }
    ],
    metrics: []
  }

  describe('Basic Rendering', () => {
    it('should render "Cluster Info" title', () => {
      render(<ClusterInfo {...baseProps} />)

      expect(screen.getByText('Cluster Info')).toBeInTheDocument()
    })

    it('should render Kubernetes version in header', () => {
      render(<ClusterInfo {...baseProps} />)

      expect(screen.getByText(/Kubernetes v1.28.0/)).toBeInTheDocument()
    })

    it('should render node count with plural', () => {
      render(<ClusterInfo {...baseProps} />)

      expect(screen.getByText(/3 nodes/)).toBeInTheDocument()
    })

    it('should render node count with singular for 1 node', () => {
      const props = {
        ...baseProps,
        cluster: { ...baseProps.cluster, nodes: 1 }
      }

      render(<ClusterInfo {...props} />)

      expect(screen.getByText(/1 node/)).toBeInTheDocument()
    })

    it('should render toggle button', () => {
      render(<ClusterInfo {...baseProps} />)

      const button = screen.getByRole('button')
      expect(button).toBeInTheDocument()
    })
  })

  describe('Version Information', () => {
    it('should render Flux Operator version', () => {
      render(<ClusterInfo {...baseProps} />)

      expect(screen.getByText('Flux Operator:')).toBeInTheDocument()
      expect(screen.getByText('v0.1.0')).toBeInTheDocument()
    })

    it('should render Flux Distribution version', () => {
      render(<ClusterInfo {...baseProps} />)

      expect(screen.getByText('Flux Distribution:')).toBeInTheDocument()
      expect(screen.getByText('v2.4.0')).toBeInTheDocument()
    })

    it('should render Platform', () => {
      render(<ClusterInfo {...baseProps} />)

      expect(screen.getByText('Platform:')).toBeInTheDocument()
      expect(screen.getByText('aws')).toBeInTheDocument()
    })

    it('should render Controller Pods count', () => {
      render(<ClusterInfo {...baseProps} />)

      expect(screen.getByText('Controller Pods:')).toBeInTheDocument()
      expect(screen.getByText('2')).toBeInTheDocument()
    })
  })

  describe('Resource Metrics', () => {
    it('should render CPU usage when metrics available', () => {
      const props = {
        ...baseProps,
        metrics: [
          { cpu: 0.5, cpuLimit: 1.0, memory: 512 * 1024 ** 3, memoryLimit: 1024 * 1024 ** 3 },
          { cpu: 0.3, cpuLimit: 1.0, memory: 256 * 1024 ** 3, memoryLimit: 512 * 1024 ** 3 }
        ]
      }

      render(<ClusterInfo {...props} />)

      expect(screen.getByText('Flux CPU Usage')).toBeInTheDocument()
      expect(screen.getByText(/0.80\/2.00 cores/)).toBeInTheDocument()
    })

    it('should render Memory usage when metrics available', () => {
      const props = {
        ...baseProps,
        metrics: [
          { cpu: 0.5, cpuLimit: 1.0, memory: 512 * 1024 ** 3, memoryLimit: 1024 * 1024 ** 3 },
          { cpu: 0.3, cpuLimit: 1.0, memory: 256 * 1024 ** 3, memoryLimit: 512 * 1024 ** 3 }
        ]
      }

      render(<ClusterInfo {...props} />)

      expect(screen.getByText('Flux Memory Usage')).toBeInTheDocument()
      expect(screen.getByText(/768.00\/1536.00 GiB/)).toBeInTheDocument()
    })

    it('should not render metrics section when metrics is empty', () => {
      render(<ClusterInfo {...baseProps} />)

      expect(screen.queryByText('Flux CPU Usage')).not.toBeInTheDocument()
      expect(screen.queryByText('Flux Memory Usage')).not.toBeInTheDocument()
    })

    it('should not render metrics section when metrics is null', () => {
      const props = {
        ...baseProps,
        metrics: null
      }

      render(<ClusterInfo {...props} />)

      expect(screen.queryByText('Flux CPU Usage')).not.toBeInTheDocument()
    })

    it('should calculate percentage correctly', () => {
      const props = {
        ...baseProps,
        metrics: [
          { cpu: 0.4, cpuLimit: 1.0, memory: 500 * 1024 ** 3, memoryLimit: 1000 * 1024 ** 3 }
        ]
      }

      render(<ClusterInfo {...props} />)

      expect(screen.getByText(/40%/)).toBeInTheDocument() // CPU percentage
      expect(screen.getByText(/50%/)).toBeInTheDocument() // Memory percentage
    })

    it('should use green progress bar for usage < 70%', () => {
      const props = {
        ...baseProps,
        metrics: [
          { cpu: 0.5, cpuLimit: 1.0, memory: 500 * 1024 ** 3, memoryLimit: 1000 * 1024 ** 3 }
        ]
      }

      const { container } = render(<ClusterInfo {...props} />)

      const progressBars = container.querySelectorAll('.bg-green-500')
      expect(progressBars.length).toBeGreaterThan(0)
    })

    it('should use yellow progress bar for usage 70-84%', () => {
      const props = {
        ...baseProps,
        metrics: [
          { cpu: 0.75, cpuLimit: 1.0, memory: 750 * 1024 ** 3, memoryLimit: 1000 * 1024 ** 3 }
        ]
      }

      const { container } = render(<ClusterInfo {...props} />)

      const progressBars = container.querySelectorAll('.bg-yellow-500')
      expect(progressBars.length).toBeGreaterThan(0)
    })

    it('should use red progress bar for usage >= 85%', () => {
      const props = {
        ...baseProps,
        metrics: [
          { cpu: 0.9, cpuLimit: 1.0, memory: 900 * 1024 ** 3, memoryLimit: 1000 * 1024 ** 3 }
        ]
      }

      const { container } = render(<ClusterInfo {...props} />)

      const progressBars = container.querySelectorAll('.bg-red-500')
      expect(progressBars.length).toBeGreaterThan(0)
    })
  })

  describe('Edge Cases - Missing Data', () => {
    it('should show "Unknown" for missing Kubernetes version', () => {
      const props = {
        ...baseProps,
        cluster: { ...baseProps.cluster, serverVersion: null }
      }

      render(<ClusterInfo {...props} />)

      expect(screen.getByText(/Kubernetes Unknown/)).toBeInTheDocument()
    })

    it('should show "Unknown" for empty string Kubernetes version', () => {
      const props = {
        ...baseProps,
        cluster: { ...baseProps.cluster, serverVersion: '' }
      }

      render(<ClusterInfo {...props} />)

      expect(screen.getByText(/Kubernetes Unknown/)).toBeInTheDocument()
    })

    it('should show "Unknown" for missing platform', () => {
      const props = {
        ...baseProps,
        cluster: { ...baseProps.cluster, platform: null }
      }

      render(<ClusterInfo {...props} />)

      expect(screen.getByText('Platform:')).toBeInTheDocument()
      expect(screen.getByText('Unknown')).toBeInTheDocument()
    })

    it('should show "Unknown" for empty string platform', () => {
      const props = {
        ...baseProps,
        cluster: { ...baseProps.cluster, platform: '' }
      }

      render(<ClusterInfo {...props} />)

      const unknownTexts = screen.getAllByText('Unknown')
      expect(unknownTexts.length).toBeGreaterThan(0)
    })

    it('should show "0 nodes" for missing node count', () => {
      const props = {
        ...baseProps,
        cluster: { ...baseProps.cluster, nodes: null }
      }

      render(<ClusterInfo {...props} />)

      expect(screen.getByText(/0 nodes/)).toBeInTheDocument()
    })

    it('should show "Unknown" for missing operator version', () => {
      const props = {
        ...baseProps,
        operator: { version: null }
      }

      render(<ClusterInfo {...props} />)

      expect(screen.getByText('Flux Operator:')).toBeInTheDocument()
      expect(screen.getByText('Unknown')).toBeInTheDocument()
    })

    it('should show "Unknown" for empty string operator version', () => {
      const props = {
        ...baseProps,
        operator: { version: '' }
      }

      render(<ClusterInfo {...props} />)

      const unknownTexts = screen.getAllByText('Unknown')
      expect(unknownTexts.length).toBeGreaterThan(0)
    })

    it('should show "Unknown" for missing distribution version', () => {
      const props = {
        ...baseProps,
        distribution: { version: null }
      }

      render(<ClusterInfo {...props} />)

      expect(screen.getByText('Flux Distribution:')).toBeInTheDocument()
      expect(screen.getByText('Unknown')).toBeInTheDocument()
    })

    it('should show "0" for missing components', () => {
      const props = {
        ...baseProps,
        components: null
      }

      render(<ClusterInfo {...props} />)

      expect(screen.getByText('Controller Pods:')).toBeInTheDocument()
      expect(screen.getByText('0')).toBeInTheDocument()
    })

    it('should handle metrics with missing cpu/memory values', () => {
      const props = {
        ...baseProps,
        metrics: [
          { cpu: null, cpuLimit: 1.0, memory: null, memoryLimit: 1000 * 1024 ** 3 }
        ]
      }

      render(<ClusterInfo {...props} />)

      // Should render with 0 for missing values
      expect(screen.getByText('Flux CPU Usage')).toBeInTheDocument()
      expect(screen.getByText(/0.00\/1.00 cores/)).toBeInTheDocument()
    })

    it('should handle zero cpuLimit without division by zero', () => {
      const props = {
        ...baseProps,
        metrics: [
          { cpu: 0.5, cpuLimit: 0, memory: 500 * 1024 ** 3, memoryLimit: 1000 * 1024 ** 3 }
        ]
      }

      render(<ClusterInfo {...props} />)

      // Should show 0% when limit is 0 - check for the CPU usage specifically
      expect(screen.getByText(/0\.50\/0\.00 cores \(0%\)/)).toBeInTheDocument()
    })
  })

  describe('Layout and Styling', () => {
    it('should render card container', () => {
      const { container } = render(<ClusterInfo {...baseProps} />)

      const card = container.querySelector('.card')
      expect(card).toBeInTheDocument()
    })

    it('should render button with hover styling', () => {
      render(<ClusterInfo {...baseProps} />)

      const button = screen.getByRole('button')
      expect(button).toHaveClass('hover:bg-gray-50')
    })

    it('should render chevron with transition', () => {
      render(<ClusterInfo {...baseProps} />)

      const button = screen.getByRole('button')
      const chevron = button.querySelector('svg')

      expect(chevron).toHaveClass('transition-transform')
    })
  })

  describe('Interaction', () => {
    it('should toggle content visibility on click', async () => {
      const {rerender} = render(<ClusterInfo {...baseProps} />);

      // Content is visible by default
      expect(screen.getByText('Flux Operator:')).toBeInTheDocument();

      // Click to collapse
      const button = screen.getByText('Cluster Info').closest('button');
      await fireEvent.click(button);
      rerender(<ClusterInfo {...baseProps} />);


      expect(screen.queryByText('Flux Operator:')).not.toBeInTheDocument();

      // Click to expand again
      await fireEvent.click(button);
      rerender(<ClusterInfo {...baseProps} />);

      expect(screen.getByText('Flux Operator:')).toBeInTheDocument();
    });
  });
})
