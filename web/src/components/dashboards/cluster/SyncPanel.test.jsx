// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, afterEach, vi } from 'vitest'
import { render, screen, fireEvent, cleanup } from '@testing-library/preact'
import { SyncPanel } from './SyncPanel'

// Mock preact-iso
const mockRoute = vi.fn()
vi.mock('preact-iso', () => ({
  useLocation: () => ({
    route: mockRoute
  })
}))

afterEach(() => {
  cleanup()
  vi.clearAllMocks()
})

describe('SyncPanel', () => {
  const baseProps = {
    sync: {
      id: 'flux-system/flux-cluster',
      source: 'oci://ghcr.io/stefanprodan/manifests/flux-cluster',
      path: './',
      interval: '10m0s',
      status: 'Applied revision: v2.4.0',
      ready: true
    },
    namespace: 'flux-system'
  }

  describe('Basic Rendering', () => {
    it('should render "Cluster Sync" title', () => {
      render(<SyncPanel {...baseProps} />)
      expect(screen.getByText('Cluster Sync')).toBeInTheDocument()
    })

    it('should render sync id as Kustomization link', () => {
      render(<SyncPanel {...baseProps} />)
      // syncName is extracted from sync.id by splitting and taking the last part
      const syncName = baseProps.sync.id.split('/').pop()
      expect(screen.getByText(`Kustomization/${baseProps.namespace}/${syncName}`)).toBeInTheDocument()
    })

    it('should render source and path', () => {
      render(<SyncPanel {...baseProps} />)
      expect(screen.getByText(baseProps.sync.source)).toBeInTheDocument()
      expect(screen.getByText(baseProps.sync.path)).toBeInTheDocument()
    })
  })

  describe('Interaction', () => {
    it('should toggle content visibility on click', async () => {
      render(<SyncPanel {...baseProps} />)

      // Content is visible by default
      expect(screen.getByText(baseProps.sync.source)).toBeInTheDocument()

      // Click to collapse - find the button by the Cluster Sync text
      const button = screen.getByText('Cluster Sync').closest('button')
      await fireEvent.click(button)
      expect(screen.queryByText(baseProps.sync.source)).not.toBeInTheDocument()

      // Click to expand again
      await fireEvent.click(button)
      expect(screen.getByText(baseProps.sync.source)).toBeInTheDocument()
    })
  })

  describe('Status Scenarios', () => {
    it('should render synced state correctly', () => {
      const { container } = render(<SyncPanel {...baseProps} />)

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
      const { container } = render(<SyncPanel {...props} />)

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
      const { container } = render(<SyncPanel {...props} />)

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
      const { container } = render(<SyncPanel {...props} />)

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
      const { container } = render(<SyncPanel {...props} />)

      // Check for danger icon
      const dangerIcon = container.querySelector('.text-danger')
      expect(dangerIcon).toBeInTheDocument()

      // "failing" badge should be present
      expect(screen.getByText('failing')).toBeInTheDocument()
    })
  })

  describe('Navigation', () => {
    it('should have correct href on Kustomization link', () => {
      render(<SyncPanel {...baseProps} />)

      // Find the Kustomization link (the one with the full path)
      const link = screen.getByText('Kustomization/flux-system/flux-cluster').closest('a')
      expect(link).toHaveAttribute('href', '/resource/Kustomization/flux-system/flux-cluster')
    })

    it('should not toggle panel when clicking Kustomization link', async () => {
      render(<SyncPanel {...baseProps} />)

      // Content is visible by default
      expect(screen.getByText(baseProps.sync.source)).toBeInTheDocument()

      // Click the Kustomization link
      const link = screen.getByText('Kustomization/flux-system/flux-cluster')
      await fireEvent.click(link)

      // Content should still be visible (click should not propagate to toggle)
      expect(screen.getByText(baseProps.sync.source)).toBeInTheDocument()
    })

    it('should handle special characters in sync id with correct href encoding', () => {
      const props = {
        ...baseProps,
        sync: {
          ...baseProps.sync,
          id: 'flux-system/my-app@v1.0'
        }
      }

      render(<SyncPanel {...props} />)

      const link = screen.getByText('Kustomization/flux-system/my-app@v1.0').closest('a')
      expect(link).toHaveAttribute('href', '/resource/Kustomization/flux-system/my-app%40v1.0')
    })
  })

  describe('Sync ID Parsing', () => {
    it('should extract sync name from id with namespace prefix', () => {
      render(<SyncPanel {...baseProps} />)

      // Should show "flux-cluster" extracted from "flux-system/flux-cluster"
      expect(screen.getByText('Kustomization/flux-system/flux-cluster')).toBeInTheDocument()
    })

    it('should handle id without namespace prefix', () => {
      const props = {
        ...baseProps,
        sync: {
          ...baseProps.sync,
          id: 'simple-sync'
        }
      }

      render(<SyncPanel {...props} />)

      expect(screen.getByText('Kustomization/flux-system/simple-sync')).toBeInTheDocument()
    })

    it('should handle empty sync id', () => {
      const props = {
        ...baseProps,
        sync: {
          ...baseProps.sync,
          id: ''
        }
      }

      render(<SyncPanel {...props} />)

      // Should render empty sync name
      expect(screen.getByText('Kustomization/flux-system/')).toBeInTheDocument()
    })

    it('should handle undefined sync id', () => {
      const props = {
        ...baseProps,
        sync: {
          ...baseProps.sync,
          id: undefined
        }
      }

      render(<SyncPanel {...props} />)

      // Should render empty sync name
      expect(screen.getByText('Kustomization/flux-system/')).toBeInTheDocument()
    })
  })

  describe('Display Layout', () => {
    it('should render sync name in subtitle', () => {
      render(<SyncPanel {...baseProps} />)

      // The subtitle shows just the sync name
      const subtitleSpan = screen.getByText('flux-cluster')
      expect(subtitleSpan).toBeInTheDocument()
      expect(subtitleSpan.tagName).toBe('SPAN')
    })

    it('should render full Kustomization path in body as link', () => {
      render(<SyncPanel {...baseProps} />)

      // The body shows the full Kustomization path with external link icon
      const linkText = screen.getByText('Kustomization/flux-system/flux-cluster')
      expect(linkText).toBeInTheDocument()
      // The link text should be inside an anchor
      expect(linkText.closest('a')).toBeInTheDocument()
    })
  })

  describe('Suspended Status Variations', () => {
    it('should recognize status starting with "Suspended" as suspended', () => {
      const props = {
        ...baseProps,
        sync: {
          ...baseProps.sync,
          ready: false,
          status: 'Suspended: manually paused'
        }
      }
      const { container } = render(<SyncPanel {...props} />)

      // Should show suspended icon (blue), not danger icon
      const suspendedIcon = container.querySelector('.text-blue-600')
      expect(suspendedIcon).toBeInTheDocument()

      // "failing" badge should not be present
      expect(screen.queryByText('failing')).not.toBeInTheDocument()
    })

    it('should not treat "NotSuspended" as suspended', () => {
      const props = {
        ...baseProps,
        sync: {
          ...baseProps.sync,
          ready: false,
          status: 'NotSuspended but failing'
        }
      }
      const { container } = render(<SyncPanel {...props} />)

      // Should show danger icon, not suspended
      const dangerIcon = container.querySelector('.text-danger')
      expect(dangerIcon).toBeInTheDocument()

      // "failing" badge should be present
      expect(screen.getByText('failing')).toBeInTheDocument()
    })
  })
})
