// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, afterEach } from 'vitest'
import { render, screen, fireEvent, cleanup } from '@testing-library/preact'
import { ComponentList } from './ComponentList'

afterEach(cleanup)

describe('ComponentList', () => {
  const baseProps = {
    components: [
      {
        name: 'source-controller',
        image: 'ghcr.io/fluxcd/source-controller:v1.2.3@sha256:abc',
        ready: true,
        status: 'Running'
      },
      {
        name: 'kustomize-controller',
        image: 'ghcr.io/fluxcd/kustomize-controller:v1.2.4@sha256:def',
        ready: false,
        status: 'CrashLoopBackOff'
      }
    ],
    metrics: [
      {
        pod: 'source-controller-a1b2c3d4-xyz12',
        cpu: 0.5,
        cpuLimit: 2.0,
        memory: 512 * 1024 * 1024, // 512 MiB
        memoryLimit: 1024 * 1024 * 1024 // 1024 MiB
      }
      // No metrics for kustomize-controller
    ]
  }

  describe('Basic Rendering', () => {
    it('should render the main title', () => {
      render(<ComponentList {...baseProps} />)
      expect(screen.getByText('Flux Components')).toBeInTheDocument()
    })

    it('should display the correct total component count', () => {
      render(<ComponentList {...baseProps} />)
      expect(screen.getByText('2 controllers deployed')).toBeInTheDocument()
    })

    it('should display the correct failing component count', () => {
      render(<ComponentList {...baseProps} />)
      expect(screen.getByText('1 failing')).toBeInTheDocument()
    })

    it('should hide the failing badge when all components are ready', () => {
      const props = {
        ...baseProps,
        components: [baseProps.components[0]]
      }
      render(<ComponentList {...props} />)
      expect(screen.queryByText(/failing/)).not.toBeInTheDocument()
    })
  })

  describe('Interaction', () => {
    it('should toggle the table visibility on header click', async () => {
      render(<ComponentList {...baseProps} />)
      const headerButton = screen.getByText('Flux Components').closest('button')

      // Table is visible by default
      expect(screen.getByRole('table')).toBeInTheDocument()

      // Click to collapse
      await fireEvent.click(headerButton)
      expect(screen.queryByRole('table')).not.toBeInTheDocument()

      // Click to expand
      await fireEvent.click(headerButton)
      expect(screen.getByRole('table')).toBeInTheDocument()
    })
  })

  describe('ComponentRow', () => {
    it('should render component name, version, and status', () => {
      render(<ComponentList {...baseProps} />)

      // Ready component
      expect(screen.getByText('source-controller')).toBeInTheDocument()
      expect(screen.getByText('v1.2.3')).toBeInTheDocument()
      expect(screen.getByText('Ready')).toBeInTheDocument()

      // Failing component
      expect(screen.getByText('kustomize-controller')).toBeInTheDocument()
      expect(screen.getByText('v1.2.4')).toBeInTheDocument()
      expect(screen.getByText('Failing')).toBeInTheDocument()
    })

    it('should toggle row details on click', async () => {
      render(<ComponentList {...baseProps} />)
      const sourceControllerRowButton = screen.getByText('source-controller').closest('button')

      // Details are hidden by default
      expect(screen.queryByText(baseProps.components[0].image)).not.toBeInTheDocument()

      // Click to expand
      await fireEvent.click(sourceControllerRowButton)
      expect(screen.getByText(baseProps.components[0].image)).toBeInTheDocument()

      // Click to collapse
      await fireEvent.click(sourceControllerRowButton)
      expect(screen.queryByText(baseProps.components[0].image)).not.toBeInTheDocument()
    })
  })

  describe('Metrics Display', () => {
    it('should display metrics when a match is found', async () => {
      render(<ComponentList {...baseProps} />)
      const sourceControllerRowButton = screen.getByText('source-controller').closest('button')
      await fireEvent.click(sourceControllerRowButton)

      expect(screen.getByText(/0.500 \/ 2.0 cores/)).toBeInTheDocument()
      expect(screen.getByText(/512 \/ 1024 MiB/)).toBeInTheDocument()
      expect(screen.getByText(/\(25%\)/)).toBeInTheDocument() // CPU
      expect(screen.getByText(/\(50%\)/)).toBeInTheDocument() // Memory
    })

    it('should not display metrics when no match is found', async () => {
      render(<ComponentList {...baseProps} />)
      const kustomizeControllerRowButton = screen.getByText('kustomize-controller').closest('button')
      await fireEvent.click(kustomizeControllerRowButton)

      // Expanded section should be present
      expect(screen.getByText(baseProps.components[1].image)).toBeInTheDocument()

      // But no metrics
      expect(screen.queryByText(/cores/)).not.toBeInTheDocument()
      expect(screen.queryByText(/MiB/)).not.toBeInTheDocument()
    })
  })

  describe('Edge Cases', () => {
    it('should handle an empty components array', () => {
      render(<ComponentList components={[]} metrics={[]} />)
      expect(screen.getByText('0 controllers deployed')).toBeInTheDocument()
      expect(screen.queryByRole('table')).not.toBeInTheDocument()
    })

    it('should handle null or empty metrics array', async () => {
      const props = {
        ...baseProps,
        metrics: null
      }
      render(<ComponentList {...props} />)
      const sourceControllerRowButton = screen.getByText('source-controller').closest('button')
      await fireEvent.click(sourceControllerRowButton)

      expect(screen.getByText(props.components[0].image)).toBeInTheDocument()
      expect(screen.queryByText(/cores/)).not.toBeInTheDocument()
    })

    it('should handle malformed image string without a version', () => {
      const props = {
        components: [{
          name: 'test-controller',
          image: 'test-controller',
          ready: true,
          status: 'Running'
        }],
        metrics: []
      }
      render(<ComponentList {...props} />)
      expect(screen.getByText('latest')).toBeInTheDocument()
    })
  })
})
