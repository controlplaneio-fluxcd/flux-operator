// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, afterEach } from 'vitest'
import { render, screen, fireEvent, cleanup } from '@testing-library/preact'
import { ClusterSync } from './ClusterSync'

afterEach(cleanup)

describe('ClusterSync', () => {
  const baseProps = {
    sync: {
      id: 'flux-system/flux-cluster',
      source: 'oci://ghcr.io/stefanprodan/manifests/flux-cluster',
      path: './',
      interval: '10m0s',
      status: 'Applied revision: v2.4.0',
      ready: true
    }
  }

  describe('Basic Rendering', () => {
    it('should render "Cluster Sync" title', () => {
      render(<ClusterSync {...baseProps} />)
      expect(screen.getByText('Cluster Sync')).toBeInTheDocument()
    })

    it('should render sync id', () => {
      render(<ClusterSync {...baseProps} />)
      expect(screen.getByText(baseProps.sync.id)).toBeInTheDocument()
    })

    it('should render source and path', () => {
      render(<ClusterSync {...baseProps} />)
      expect(screen.getByText(baseProps.sync.source)).toBeInTheDocument()
      expect(screen.getByText(baseProps.sync.path)).toBeInTheDocument()
    })
  })

  describe('Interaction', () => {
    it('should toggle content visibility on click', async () => {
      render(<ClusterSync {...baseProps} />)

      // Content is visible by default
      expect(screen.getByText(baseProps.sync.source)).toBeInTheDocument()

      // Click to collapse
      const button = screen.getByRole('button')
      await fireEvent.click(button)
      expect(screen.queryByText(baseProps.sync.source)).not.toBeInTheDocument()

      // Click to expand again
      await fireEvent.click(button)
      expect(screen.getByText(baseProps.sync.source)).toBeInTheDocument()
    })
  })

  describe('Status Scenarios', () => {
    it('should render synced state correctly', () => {
      const { container } = render(<ClusterSync {...baseProps} />)

      // Check for success icon
      const successIcon = container.querySelector('.text-success')
      expect(successIcon).toBeInTheDocument()

      // Check for status message
      expect(screen.getByText(baseProps.sync.status)).toBeInTheDocument()

      // "failing" badge should not be present
      expect(screen.queryByText('failing')).not.toBeInTheDocument()
    })

    it('should render not synced state correctly', () => {
      const props = {
        sync: {
          ...baseProps.sync,
          ready: false,
          status: 'oci repository pull error'
        }
      }
      const { container } = render(<ClusterSync {...props} />)

      // Check for danger icon
      const dangerIcon = container.querySelector('.text-danger')
      expect(dangerIcon).toBeInTheDocument()

      // Check for status message
      expect(screen.getByText(props.sync.status)).toBeInTheDocument()

      // "failing" badge should be present
      expect(screen.getByText('failing')).toBeInTheDocument()
    })

    it('should render suspended state correctly', () => {
      const props = {
        sync: {
          ...baseProps.sync,
          ready: false,
          status: 'Suspended'
        }
      }
      const { container } = render(<ClusterSync {...props} />)

      // Check for blue (suspended) icon
      const suspendedIcon = container.querySelector('.text-blue-600')
      expect(suspendedIcon).toBeInTheDocument()

      // Check for status message
      expect(screen.getByText(props.sync.status)).toBeInTheDocument()

      // "failing" badge should not be present
      expect(screen.queryByText('failing')).not.toBeInTheDocument()
    })
  })

  describe('Edge Cases - Missing Data', () => {
    it('should handle missing id, source, path, and status', () => {
      const props = {
        sync: {
          id: null,
          source: null,
          path: null,
          status: null,
          ready: true
        }
      }
      const { container } = render(<ClusterSync {...props} />)

      // Should not crash and should render without the missing text
      expect(container.querySelector('.card')).toBeInTheDocument()
      expect(screen.queryByText(/null/)).not.toBeInTheDocument()
    })

    it('should treat undefined ready state as not synced', () => {
      const props = {
        sync: {
          ...baseProps.sync,
          ready: undefined,
          status: 'Some error'
        }
      }
      const { container } = render(<ClusterSync {...props} />)

      // Check for danger icon
      const dangerIcon = container.querySelector('.text-danger')
      expect(dangerIcon).toBeInTheDocument()

      // "failing" badge should be present
      expect(screen.getByText('failing')).toBeInTheDocument()
    })
  })
})
