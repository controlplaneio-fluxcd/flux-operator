// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/preact'
import userEvent from '@testing-library/user-event'
import { InputsPanel } from './InputsPanel'
import { getPanelById } from '../common/panel.test'

// Mock fetchWithMock
vi.mock('../../../utils/fetch', () => ({
  fetchWithMock: vi.fn()
}))

// Mock useHashTab to use simple useState instead
vi.mock('../../../utils/hash', async () => {
  const { useState } = await import('preact/hooks')
  return {
    useHashTab: (panel, defaultTab) => useState(defaultTab)
  }
})

import { fetchWithMock } from '../../../utils/fetch'

describe('InputsPanel component', () => {
  const mockResourceSet = {
    apiVersion: 'fluxcd.controlplane.io/v1',
    kind: 'ResourceSet',
    metadata: {
      name: 'flux-status-server',
      namespace: 'flux-system'
    },
    spec: {
      inputStrategy: {
        name: 'Flatten'
      },
      inputs: [
        { name: 'cluster', value: 'production' },
        { name: 'region', value: 'us-east-1' }
      ],
      inputsFrom: [
        {
          kind: 'ResourceSetInputProvider',
          name: 'flux-status-server'
        }
      ]
    },
    status: {
      inputProviderRefs: [
        {
          type: 'OCIArtifactTag',
          name: 'flux-status-server',
          namespace: 'flux-system'
        }
      ]
    }
  }

  const mockResourceSetNoInputs = {
    apiVersion: 'fluxcd.controlplane.io/v1',
    kind: 'ResourceSet',
    metadata: {
      name: 'empty-resourceset',
      namespace: 'default'
    },
    spec: {},
    status: {}
  }

  const mockResourceSetInlineOnly = {
    apiVersion: 'fluxcd.controlplane.io/v1',
    kind: 'ResourceSet',
    metadata: {
      name: 'inline-only',
      namespace: 'default'
    },
    spec: {
      inputs: [
        { env: 'staging' }
      ]
    },
    status: {}
  }

  const mockResourceSetPermuteStrategy = {
    apiVersion: 'fluxcd.controlplane.io/v1',
    kind: 'ResourceSet',
    metadata: {
      name: 'permute-resourceset',
      namespace: 'default'
    },
    spec: {
      inputStrategy: {
        name: 'Permute'
      },
      inputs: []
    },
    status: {
      inputProviderRefs: [
        { type: 'GitHubPullRequest', name: 'github-prs', namespace: 'default' },
        { type: 'OCIArtifactTag', name: 'oci-tags', namespace: 'default' }
      ]
    }
  }

  const mockProviderResponse = {
    apiVersion: 'fluxcd.controlplane.io/v1',
    kind: 'ResourceSetInputProvider',
    metadata: {
      name: 'flux-status-server',
      namespace: 'flux-system'
    },
    spec: {
      type: 'OCIArtifactTag',
      url: 'oci://ghcr.io/example/app'
    },
    status: {
      reconcilerRef: {
        lastReconciled: '2025-01-15T10:00:00Z'
      },
      exportedInputs: [
        { tag: '1.0.0', digest: 'sha256:abc123' },
        { tag: '1.0.1', digest: 'sha256:def456' }
      ]
    }
  }

  beforeEach(() => {
    vi.clearAllMocks()
    fetchWithMock.mockResolvedValue(mockProviderResponse)
  })

  describe('Component rendering', () => {
    it('should render the inputs panel with header', () => {
      const { container } = render(<InputsPanel resourceData={mockResourceSet} namespace="flux-system" />)

      expect(getPanelById(container, 'inputs-panel')).toBeInTheDocument()
      expect(screen.getByText('Inputs')).toBeInTheDocument()
    })

    it('should show Overview tab by default', () => {
      render(<InputsPanel resourceData={mockResourceSet} namespace="flux-system" />)

      expect(screen.getByText('Overview')).toBeInTheDocument()
      expect(screen.getByText('Strategy')).toBeInTheDocument()
    })
  })

  describe('Overview tab - Strategy badge', () => {
    it('should display Flatten strategy as blue badge by default', () => {
      render(<InputsPanel resourceData={mockResourceSetNoInputs} namespace="default" />)

      expect(screen.getByText('Strategy')).toBeInTheDocument()
      const badge = screen.getByText('Flatten')
      expect(badge).toBeInTheDocument()
      expect(badge).toHaveClass('bg-blue-100')
    })

    it('should display custom strategy as blue badge', () => {
      render(<InputsPanel resourceData={mockResourceSetPermuteStrategy} namespace="default" />)

      const badge = screen.getByText('Permute')
      expect(badge).toBeInTheDocument()
      expect(badge).toHaveClass('bg-blue-100')
    })
  })

  describe('Overview tab - Provider badges', () => {
    it('should display provider type badges', () => {
      render(<InputsPanel resourceData={mockResourceSet} namespace="flux-system" />)

      expect(screen.getByText('Providers')).toBeInTheDocument()
      const badge = screen.getByText('OCIArtifactTag')
      expect(badge).toBeInTheDocument()
      expect(badge).toHaveClass('bg-green-100')
    })

    it('should display multiple provider type badges', () => {
      render(<InputsPanel resourceData={mockResourceSetPermuteStrategy} namespace="default" />)

      expect(screen.getByText('GitHubPullRequest')).toBeInTheDocument()
      expect(screen.getByText('OCIArtifactTag')).toBeInTheDocument()
    })

    it('should display None badge when no providers', () => {
      render(<InputsPanel resourceData={mockResourceSetInlineOnly} namespace="default" />)

      const noneBadge = screen.getByText('None')
      expect(noneBadge).toBeInTheDocument()
      expect(noneBadge).toHaveClass('bg-gray-100')
    })
  })

  describe('Overview tab - Counts', () => {
    it('should display inline inputs count', () => {
      render(<InputsPanel resourceData={mockResourceSet} namespace="flux-system" />)

      expect(screen.getByText('Inline inputs')).toBeInTheDocument()
      expect(screen.getByText('2')).toBeInTheDocument()
    })

    it('should display 0 inline inputs when none exist', () => {
      render(<InputsPanel resourceData={mockResourceSetNoInputs} namespace="default" />)

      expect(screen.getByText('Inline inputs')).toBeInTheDocument()
      // Both inline and external are 0, so we check for at least one 0
      const zeros = screen.getAllByText('0')
      expect(zeros.length).toBeGreaterThan(0)
    })

    it('should display external inputs count', () => {
      render(<InputsPanel resourceData={mockResourceSet} namespace="flux-system" />)

      expect(screen.getByText('External inputs')).toBeInTheDocument()
      expect(screen.getByText('1')).toBeInTheDocument()
    })

    it('should display 0 external inputs when none exist', () => {
      render(<InputsPanel resourceData={mockResourceSetInlineOnly} namespace="default" />)

      expect(screen.getByText('External inputs')).toBeInTheDocument()
      const counts = screen.getAllByText('0')
      expect(counts.length).toBeGreaterThan(0)
    })
  })

  describe('Values tab', () => {
    it('should switch to Values tab when clicked', async () => {
      const user = userEvent.setup()
      render(<InputsPanel resourceData={mockResourceSetInlineOnly} namespace="default" />)

      const valuesTab = screen.getByText('Values')
      await user.click(valuesTab)

      await waitFor(() => {
        expect(screen.getByText('Inline inputs')).toBeInTheDocument()
      })
    })

    it('should display inline inputs in YAML format', async () => {
      const user = userEvent.setup()
      render(<InputsPanel resourceData={mockResourceSetInlineOnly} namespace="default" />)

      await user.click(screen.getByText('Values'))

      await waitFor(() => {
        const panel = getPanelById(document.body, 'inputs-panel')
        expect(panel.textContent).toContain('staging')
      })
    })

    it('should display "No inputs available" when empty', async () => {
      const user = userEvent.setup()
      render(<InputsPanel resourceData={mockResourceSetNoInputs} namespace="default" />)

      await user.click(screen.getByText('Values'))

      await waitFor(() => {
        expect(screen.getByText('No inputs available')).toBeInTheDocument()
      })
    })

    it('should switch back to Overview tab', async () => {
      const user = userEvent.setup()
      render(<InputsPanel resourceData={mockResourceSetInlineOnly} namespace="default" />)

      // Switch to Values
      await user.click(screen.getByText('Values'))
      await waitFor(() => {
        const panel = getPanelById(document.body, 'inputs-panel')
        expect(panel.textContent).toContain('staging')
      })

      // Switch back to Overview
      await user.click(screen.getByText('Overview'))
      await waitFor(() => {
        expect(screen.getByText('Strategy')).toBeInTheDocument()
      })
    })
  })

  describe('Values tab - External inputs loading', () => {
    it('should show loading state when fetching external inputs', async () => {
      // Delay the mock response
      fetchWithMock.mockImplementation(() => new Promise(resolve => setTimeout(() => resolve(mockProviderResponse), 100)))

      const user = userEvent.setup()
      render(<InputsPanel resourceData={mockResourceSet} namespace="flux-system" />)

      await user.click(screen.getByText('Values'))

      // Should show loading state
      expect(screen.getByText('Loading inputs...')).toBeInTheDocument()
    })

    it('should fetch and display external inputs', async () => {
      const user = userEvent.setup()
      render(<InputsPanel resourceData={mockResourceSet} namespace="flux-system" />)

      await user.click(screen.getByText('Values'))

      await waitFor(() => {
        // Provider name should be a link
        expect(screen.getByRole('link', { name: /flux-status-server/i })).toBeInTheDocument()
      })
    })

    it('should display provider URL', async () => {
      const user = userEvent.setup()
      render(<InputsPanel resourceData={mockResourceSet} namespace="flux-system" />)

      await user.click(screen.getByText('Values'))

      await waitFor(() => {
        expect(screen.getByText('oci://ghcr.io/example/app')).toBeInTheDocument()
      })
    })

    it('should display provider type badge in Values tab', async () => {
      const user = userEvent.setup()
      render(<InputsPanel resourceData={mockResourceSet} namespace="flux-system" />)

      await user.click(screen.getByText('Values'))

      await waitFor(() => {
        // There should be OCIArtifactTag badge in the external input header
        const badges = screen.getAllByText('OCIArtifactTag')
        expect(badges.length).toBeGreaterThan(0)
      })
    })

    it('should handle fetch error gracefully', async () => {
      fetchWithMock.mockRejectedValue(new Error('Network error'))

      const user = userEvent.setup()
      render(<InputsPanel resourceData={mockResourceSet} namespace="flux-system" />)

      await user.click(screen.getByText('Values'))

      await waitFor(() => {
        expect(screen.getByText(/Failed to load/)).toBeInTheDocument()
      })
    })

    it('should not re-fetch when switching tabs after initial load', async () => {
      const user = userEvent.setup()
      render(<InputsPanel resourceData={mockResourceSet} namespace="flux-system" />)

      // First click on Values
      await user.click(screen.getByText('Values'))
      await waitFor(() => {
        expect(fetchWithMock).toHaveBeenCalledTimes(1)
      })

      // Switch to Overview
      await user.click(screen.getByText('Overview'))

      // Switch back to Values
      await user.click(screen.getByText('Values'))

      // Should not fetch again
      expect(fetchWithMock).toHaveBeenCalledTimes(1)
    })
  })

  describe('Edge cases', () => {
    it('should handle null resourceData gracefully', () => {
      const { container } = render(<InputsPanel resourceData={null} namespace="default" />)

      expect(getPanelById(container, 'inputs-panel')).toBeInTheDocument()
      expect(screen.getByText('Inputs')).toBeInTheDocument()
    })

    it('should handle undefined resourceData gracefully', () => {
      const { container } = render(<InputsPanel resourceData={undefined} namespace="default" />)

      expect(getPanelById(container, 'inputs-panel')).toBeInTheDocument()
    })

    it('should handle missing spec gracefully', () => {
      const mockNoSpec = {
        apiVersion: 'fluxcd.controlplane.io/v1',
        kind: 'ResourceSet',
        metadata: {
          name: 'test',
          namespace: 'default'
        },
        status: {}
      }

      render(<InputsPanel resourceData={mockNoSpec} namespace="default" />)

      expect(screen.getByText('Inputs')).toBeInTheDocument()
      expect(screen.getByText('Flatten')).toBeInTheDocument() // Default strategy
      // Both inline and external are 0
      const zeros = screen.getAllByText('0')
      expect(zeros.length).toBeGreaterThan(0)
    })

    it('should handle missing status gracefully', () => {
      const mockNoStatus = {
        apiVersion: 'fluxcd.controlplane.io/v1',
        kind: 'ResourceSet',
        metadata: {
          name: 'test',
          namespace: 'default'
        },
        spec: {
          inputs: [{ foo: 'bar' }]
        }
      }

      render(<InputsPanel resourceData={mockNoStatus} namespace="default" />)

      expect(screen.getByText('1')).toBeInTheDocument() // Inline inputs count
      expect(screen.getByText('None')).toBeInTheDocument() // No providers
    })

    it('should handle missing inputProviderRefs gracefully', () => {
      const mockNoProviderRefs = {
        apiVersion: 'fluxcd.controlplane.io/v1',
        kind: 'ResourceSet',
        metadata: {
          name: 'test',
          namespace: 'default'
        },
        spec: {},
        status: {
          conditions: []
        }
      }

      render(<InputsPanel resourceData={mockNoProviderRefs} namespace="default" />)

      expect(screen.getByText('None')).toBeInTheDocument()
    })
  })

  describe('Mobile display', () => {
    it('should show Info label on mobile (sm:hidden)', () => {
      render(<InputsPanel resourceData={mockResourceSet} namespace="flux-system" />)

      // The Info span exists but is hidden on larger screens
      const infoSpan = screen.getByText('Info')
      expect(infoSpan).toHaveClass('sm:hidden')
    })

    it('should show Overview label on desktop (hidden sm:inline)', () => {
      render(<InputsPanel resourceData={mockResourceSet} namespace="flux-system" />)

      const overviewSpan = screen.getByText('Overview')
      expect(overviewSpan).toHaveClass('hidden')
      expect(overviewSpan).toHaveClass('sm:inline')
    })
  })

  describe('Two-column layout', () => {
    it('should render Strategy and Providers in left column', () => {
      render(<InputsPanel resourceData={mockResourceSet} namespace="flux-system" />)

      expect(screen.getByText('Strategy')).toBeInTheDocument()
      expect(screen.getByText('Providers')).toBeInTheDocument()
    })

    it('should render counts in right column', () => {
      render(<InputsPanel resourceData={mockResourceSet} namespace="flux-system" />)

      expect(screen.getByText('Inline inputs')).toBeInTheDocument()
      expect(screen.getByText('External inputs')).toBeInTheDocument()
    })
  })
})
