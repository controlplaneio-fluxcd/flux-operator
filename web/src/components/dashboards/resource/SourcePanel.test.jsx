// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/preact'
import userEvent from '@testing-library/user-event'
import { SourcePanel } from './SourcePanel'
import { fetchWithMock } from '../../../utils/fetch'
import { getPanelById } from '../common/panel.test'

// Mock the fetch utility
vi.mock('../../../utils/fetch', () => ({
  fetchWithMock: vi.fn()
}))

// Mock preact-iso
const mockRoute = vi.fn()
vi.mock('preact-iso', () => ({
  useLocation: () => ({
    route: mockRoute
  })
}))

describe('SourcePanel component', () => {
  const mockSourceRef = {
    kind: 'GitRepository',
    name: 'flux-system',
    namespace: 'flux-system',
    status: 'Ready',
    url: 'https://github.com/example/repo.git',
    message: "stored artifact for revision 'refs/heads/main@sha1:abc123'"
  }

  // Mock resourceData that contains sourceRef in status and namespace in metadata
  const mockResourceData = {
    kind: 'Kustomization',
    metadata: {
      name: 'test-kustomization',
      namespace: 'flux-system'
    },
    status: {
      sourceRef: mockSourceRef
    }
  }

  const mockSourceData = {
    apiVersion: 'source.toolkit.fluxcd.io/v1',
    kind: 'GitRepository',
    metadata: {
      name: 'flux-system',
      namespace: 'flux-system'
    },
    spec: {
      interval: '1m',
      url: 'https://github.com/example/repo.git',
      ref: {
        branch: 'main'
      }
    },
    status: {
      conditions: [
        {
          type: 'Ready',
          status: 'True',
          lastTransitionTime: '2025-01-15T10:00:00Z',
          reason: 'Succeeded',
          message: "stored artifact for revision 'refs/heads/main@sha1:abc123'"
        }
      ],
      artifact: {
        revision: 'refs/heads/main@sha1:abc123'
      }
    }
  }

  const mockEvents = {
    events: [
      {
        type: 'Normal',
        message: 'Artifact up to date with remote revision',
        lastTimestamp: '2025-01-15T10:00:00Z'
      },
      {
        type: 'Warning',
        message: 'Failed to fetch artifact',
        lastTimestamp: '2025-01-15T09:55:00Z'
      }
    ]
  }

  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('should render the source section initially', () => {
    const { container } = render(
      <SourcePanel
        resourceData={mockResourceData}
      />
    )

    expect(getPanelById(container, 'source-panel')).toBeInTheDocument()
    expect(screen.getByText('Source')).toBeInTheDocument()
  })

  it('should fetch source data when component mounts', async () => {
    fetchWithMock.mockResolvedValue(mockSourceData)

    render(
      <SourcePanel
        resourceData={mockResourceData}
      />
    )

    await waitFor(() => {
      expect(fetchWithMock).toHaveBeenCalledWith({
        endpoint: '/api/v1/resource?kind=GitRepository&name=flux-system&namespace=flux-system',
        mockPath: '../mock/resource',
        mockExport: 'getMockResource'
      })
    })
  })

  it('should show loading state while fetching source data', async () => {
    let resolvePromise
    const promise = new Promise((resolve) => { resolvePromise = resolve })
    fetchWithMock.mockReturnValue(promise)

    render(
      <SourcePanel
        resourceData={mockResourceData}
      />
    )

    // Should show loading spinner (component is expanded by default)
    expect(screen.getByText('Loading source...')).toBeInTheDocument()
    expect(document.querySelector('.animate-spin')).toBeInTheDocument()

    // Resolve the promise
    resolvePromise(mockSourceData)

    // Wait for loading to complete
    await waitFor(() => {
      expect(screen.queryByText('Loading source...')).not.toBeInTheDocument()
    })
  })

  it('should display overview tab content after loading', async () => {
    fetchWithMock.mockResolvedValue(mockSourceData)

    render(
      <SourcePanel
        resourceData={mockResourceData}
      />
    )

    await waitFor(() => {
      expect(screen.getByText('Overview')).toBeInTheDocument()
    })

    // Check overview content
    const textContent = document.body.textContent
    expect(textContent).toContain('Status')
    expect(textContent).toContain('Ready')
    expect(textContent).toContain('Reconciled by')
    expect(textContent).toContain('source-controller')
    expect(textContent).toContain('GitRepository/flux-system/flux-system')
    expect(textContent).toContain('URL')
    expect(textContent).toContain('https://github.com/example/repo.git')
  })

  it('should navigate to source resource when ID button is clicked', async () => {
    fetchWithMock.mockResolvedValue(mockSourceData)
    const user = userEvent.setup()

    render(
      <SourcePanel
        resourceData={mockResourceData}
      />
    )

    await waitFor(() => {
      expect(screen.getByText('GitRepository/flux-system/flux-system')).toBeInTheDocument()
    })

    const idButton = screen.getByText('GitRepository/flux-system/flux-system').closest('button')
    await user.click(idButton)

    expect(mockRoute).toHaveBeenCalledWith('/resource/GitRepository/flux-system/flux-system')
  })

  it('should display fetch every and fetched at when source data is available', async () => {
    fetchWithMock.mockResolvedValue(mockSourceData)

    render(
      <SourcePanel
        resourceData={mockResourceData}
      />
    )

    await waitFor(() => {
      const textContent = document.body.textContent
      expect(textContent).toContain('Fetch every')
      expect(textContent).toContain('1m')
      expect(textContent).toContain('Fetched at')
    })
  })

  it('should display origin URL and revision when available', async () => {
    const sourceRefWithOrigin = {
      ...mockSourceRef,
      originURL: 'https://github.com/original/repo.git',
      originRevision: 'v1.2.3'
    }

    const resourceDataWithOrigin = {
      ...mockResourceData,
      status: {
        sourceRef: sourceRefWithOrigin
      }
    }

    fetchWithMock.mockResolvedValue(mockSourceData)

    render(
      <SourcePanel resourceData={resourceDataWithOrigin} />
    )

    await waitFor(() => {
      const textContent = document.body.textContent
      expect(textContent).toContain('Origin URL')
      expect(textContent).toContain('https://github.com/original/repo.git')
      expect(textContent).toContain('Origin Revision')
      expect(textContent).toContain('v1.2.3')
    })
  })

  it('should not display origin URL and revision when empty', async () => {
    fetchWithMock.mockResolvedValue(mockSourceData)

    render(
      <SourcePanel
        resourceData={mockResourceData}
      />
    )

    await waitFor(() => {
      const textContent = document.body.textContent
      expect(textContent).not.toContain('Origin URL')
      expect(textContent).not.toContain('Origin Revision')
    })
  })

  it('should display fetch result message', async () => {
    fetchWithMock.mockResolvedValue(mockSourceData)

    render(
      <SourcePanel
        resourceData={mockResourceData}
      />
    )

    await waitFor(() => {
      const textContent = document.body.textContent
      expect(textContent).toContain('Fetch result')
      expect(textContent).toContain("stored artifact for revision 'refs/heads/main@sha1:abc123'")
    })
  })

  it('should switch to events tab and fetch events on demand', async () => {
    fetchWithMock.mockResolvedValueOnce(mockSourceData)
    const user = userEvent.setup()

    render(
      <SourcePanel
        resourceData={mockResourceData}
      />
    )

    // Wait for initial load
    await waitFor(() => {
      expect(screen.getByText('Events')).toBeInTheDocument()
    })

    // Events should not be fetched yet
    expect(fetchWithMock).toHaveBeenCalledTimes(1)

    // Click on Events tab
    fetchWithMock.mockResolvedValueOnce(mockEvents)
    const eventsTab = screen.getByText('Events')
    await user.click(eventsTab)

    // Now events should be fetched
    await waitFor(() => {
      expect(fetchWithMock).toHaveBeenCalledTimes(2)
      expect(fetchWithMock).toHaveBeenCalledWith({
        endpoint: '/api/v1/events?kind=GitRepository&name=flux-system&namespace=flux-system',
        mockPath: '../mock/events',
        mockExport: 'getMockEvents'
      })
    })
  })

  it('should display events after loading', async () => {
    fetchWithMock.mockResolvedValueOnce(mockSourceData)
    const user = userEvent.setup()

    render(
      <SourcePanel
        resourceData={mockResourceData}
      />
    )

    // Wait for initial load
    await waitFor(() => {
      expect(screen.getByText('Events')).toBeInTheDocument()
    })

    // Click on Events tab
    fetchWithMock.mockResolvedValueOnce(mockEvents)
    const eventsTab = screen.getByText('Events')
    await user.click(eventsTab)

    // Check events are displayed
    await waitFor(() => {
      expect(screen.getByText('Artifact up to date with remote revision')).toBeInTheDocument()
      expect(screen.getByText('Failed to fetch artifact')).toBeInTheDocument()
    })

    // Check event types are displayed
    const infoBadges = screen.getAllByText('Info')
    expect(infoBadges.length).toBeGreaterThan(0)

    const warningBadges = screen.getAllByText('Warning')
    expect(warningBadges.length).toBeGreaterThan(0)
  })

  it('should show "No events found" when events array is empty', async () => {
    fetchWithMock.mockResolvedValueOnce(mockSourceData)
    const user = userEvent.setup()

    render(
      <SourcePanel
        resourceData={mockResourceData}
      />
    )

    // Wait for initial load
    await waitFor(() => {
      expect(screen.getByText('Events')).toBeInTheDocument()
    })

    // Click on Events tab
    fetchWithMock.mockResolvedValueOnce({ events: [] })
    const eventsTab = screen.getByText('Events')
    await user.click(eventsTab)

    // Check "No events found" message
    await waitFor(() => {
      expect(screen.getByText('No events found')).toBeInTheDocument()
    })
  })

  it('should display specification tab with YAML', async () => {
    fetchWithMock.mockResolvedValue(mockSourceData)
    const user = userEvent.setup()

    render(
      <SourcePanel
        resourceData={mockResourceData}
      />
    )

    await waitFor(() => {
      expect(screen.getByText('Specification')).toBeInTheDocument()
    })

    // Click on Specification tab
    const specTab = screen.getByText('Specification')
    await user.click(specTab)

    // Check YAML content
    await waitFor(() => {
      const codeElement = document.querySelector('.language-yaml')
      expect(codeElement).toBeInTheDocument()
      expect(codeElement.innerHTML).toContain('apiVersion')
      expect(codeElement.innerHTML).toContain('source.toolkit.fluxcd.io/v1')
      expect(codeElement.innerHTML).toContain('GitRepository')
      expect(codeElement.innerHTML).toContain('interval')
      expect(codeElement.innerHTML).toContain('1m')
    })
  })

  it('should display status tab with YAML', async () => {
    fetchWithMock.mockResolvedValue(mockSourceData)
    const user = userEvent.setup()

    render(
      <SourcePanel
        resourceData={mockResourceData}
      />
    )

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Status' })).toBeInTheDocument()
    })

    // Click on Status tab
    const statusTab = screen.getByRole('button', { name: 'Status' })
    await user.click(statusTab)

    // Check YAML content
    await waitFor(() => {
      const textContent = document.body.textContent
      expect(textContent).toContain('status:')
      expect(textContent).toContain('conditions:')
    })
  })

  it('should toggle collapse/expand state', async () => {
    fetchWithMock.mockResolvedValue(mockSourceData)
    const user = userEvent.setup()

    render(
      <SourcePanel
        resourceData={mockResourceData}
      />
    )

    // Initially expanded, content should be visible
    await waitFor(() => {
      expect(screen.getByText('Overview')).toBeInTheDocument()
    })

    // Click to collapse
    const toggleButton = screen.getByRole('button', { name: /source/i })
    await user.click(toggleButton)

    // Content should be hidden
    await waitFor(() => {
      expect(screen.queryByText('Overview')).not.toBeInTheDocument()
    })

    // Click to expand again
    await user.click(toggleButton)

    // Content should be visible again
    await waitFor(() => {
      expect(screen.getByText('Overview')).toBeInTheDocument()
    })
  })

  it('should handle fetch error gracefully', async () => {
    const consoleErrorSpy = vi.spyOn(console, 'error').mockImplementation(() => {})
    fetchWithMock.mockRejectedValue(new Error('Network error'))

    render(
      <SourcePanel
        resourceData={mockResourceData}
      />
    )

    // Wait for fetch to complete
    await waitFor(() => {
      expect(fetchWithMock).toHaveBeenCalled()
    })

    // Component should still render, just without data
    await waitFor(() => {
      expect(screen.getByText('Overview')).toBeInTheDocument()
    })

    consoleErrorSpy.mockRestore()
  })

  it('should only show Overview and Events tabs when source data fails to load', async () => {
    const consoleErrorSpy = vi.spyOn(console, 'error').mockImplementation(() => {})
    fetchWithMock.mockRejectedValue(new Error('Network error'))

    render(
      <SourcePanel
        resourceData={mockResourceData}
      />
    )

    await waitFor(() => {
      expect(screen.getByText('Overview')).toBeInTheDocument()
      expect(screen.getByText('Events')).toBeInTheDocument()
    })

    // Specification and Status tabs should not be present
    expect(screen.queryByText('Specification')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Status' })).not.toBeInTheDocument()

    consoleErrorSpy.mockRestore()
  })

  it('should fetch events only once when switching tabs multiple times', async () => {
    fetchWithMock.mockResolvedValueOnce(mockSourceData)
    const user = userEvent.setup()

    render(
      <SourcePanel
        resourceData={mockResourceData}
      />
    )

    await waitFor(() => {
      expect(screen.getByText('Events')).toBeInTheDocument()
    })

    // Click on Events tab
    fetchWithMock.mockResolvedValueOnce(mockEvents)
    const eventsTab = screen.getByText('Events')
    await user.click(eventsTab)

    await waitFor(() => {
      expect(fetchWithMock).toHaveBeenCalledTimes(2)
    })

    // Switch to Overview
    const overviewTab = screen.getByText('Overview')
    await user.click(overviewTab)

    // Switch back to Events
    await user.click(eventsTab)

    // Should still only have been called twice (not fetched again)
    expect(fetchWithMock).toHaveBeenCalledTimes(2)
  })

  describe('Source data auto-refresh', () => {
    it('should show loading spinner on initial load', async () => {
      let resolvePromise
      const promise = new Promise((resolve) => { resolvePromise = resolve })
      fetchWithMock.mockReturnValue(promise)

      render(
        <SourcePanel
          resourceData={mockResourceData}
        />
      )

      // Should show loading spinner (component is expanded by default)
      expect(screen.getByText('Loading source...')).toBeInTheDocument()
      expect(document.querySelector('.animate-spin')).toBeInTheDocument()

      // Resolve the promise
      resolvePromise(mockSourceData)

      // Wait for loading to complete
      await waitFor(() => {
        expect(screen.queryByText('Loading source...')).not.toBeInTheDocument()
      })
    })

    it('should NOT show loading spinner during auto-refresh', async () => {
      // Initial render
      fetchWithMock.mockResolvedValueOnce(mockSourceData)
      const { rerender } = render(
        <SourcePanel
          resourceData={mockResourceData}
        />
      )

      // Wait for initial load to complete
      await waitFor(() => {
        expect(screen.getByText('Overview')).toBeInTheDocument()
        expect(screen.queryByText('Loading source...')).not.toBeInTheDocument()
      })

      // Simulate parent auto-refresh by changing resourceData
      const updatedResourceData = {
        ...mockResourceData,
        status: {
          ...mockResourceData.status,
          reconcilerRef: { status: 'Progressing' }
        }
      }

      fetchWithMock.mockResolvedValueOnce(mockSourceData)

      rerender(
        <SourcePanel
          resourceData={updatedResourceData}
        />
      )

      // Loading spinner should NOT appear during auto-refresh
      await waitFor(() => {
        expect(fetchWithMock).toHaveBeenCalledTimes(2)
      })

      // Should never show loading spinner
      expect(screen.queryByText('Loading source...')).not.toBeInTheDocument()
      expect(document.querySelector('.animate-spin')).not.toBeInTheDocument()
    })

    it('should preserve existing data when auto-refresh fails', async () => {
      const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {})

      // Initial render
      fetchWithMock.mockResolvedValueOnce(mockSourceData)
      const { rerender } = render(
        <SourcePanel
          resourceData={mockResourceData}
        />
      )

      // Wait for initial load
      await waitFor(() => {
        expect(screen.getByText('Overview')).toBeInTheDocument()
        const textContent = document.body.textContent
        expect(textContent).toContain('https://github.com/example/repo.git')
      })

      // Simulate parent auto-refresh with fetch error
      const updatedResourceData = {
        ...mockResourceData,
        status: {
          ...mockResourceData.status,
          reconcilerRef: { status: 'Progressing' }
        }
      }

      fetchWithMock.mockRejectedValueOnce(new Error('Network error'))

      rerender(
        <SourcePanel
          resourceData={updatedResourceData}
        />
      )

      // Should preserve existing source data
      await waitFor(() => {
        const textContent = document.body.textContent
        expect(textContent).toContain('https://github.com/example/repo.git')
      })

      // Should not show error or loading state
      expect(screen.queryByText('Loading source...')).not.toBeInTheDocument()

      consoleSpy.mockRestore()
    })
  })

  describe('Events auto-refresh', () => {
    it('should refetch events when resourceData changes if Events tab is open', async () => {
      const user = userEvent.setup()

      // Track call count to return different values for refresh
      let eventsCallCount = 0

      // Use implementation to handle race conditions between source and events refetch
      fetchWithMock.mockImplementation(({ endpoint }) => {
        if (endpoint.includes('/api/v1/resource?')) {
          return Promise.resolve(mockSourceData)
        }
        if (endpoint.includes('/api/v1/events?')) {
          eventsCallCount++
          if (eventsCallCount === 1) {
            return Promise.resolve(mockEvents)
          }
          // Second events call (refetch)
          return Promise.resolve({
            events: [
              {
                type: 'Normal',
                message: 'New event after refresh',
                lastTimestamp: '2025-01-15T10:05:00Z'
              }
            ]
          })
        }
        return Promise.resolve({})
      })

      const { rerender } = render(
        <SourcePanel
          resourceData={mockResourceData}
        />
      )

      // Wait for initial load
      await waitFor(() => {
        expect(screen.getByText('Events')).toBeInTheDocument()
      })

      // Click on Events tab
      const eventsTab = screen.getByText('Events')
      await user.click(eventsTab)

      // Wait for events to load
      await waitFor(() => {
        expect(screen.getByText('Artifact up to date with remote revision')).toBeInTheDocument()
      })

      // Simulate parent auto-refresh by changing resourceData
      const updatedResourceData = {
        ...mockResourceData,
        status: {
          ...mockResourceData.status,
          reconcilerRef: { status: 'Progressing' }
        }
      }

      rerender(
        <SourcePanel
          resourceData={updatedResourceData}
        />
      )

      // Should refetch events and show new content
      await waitFor(() => {
        expect(screen.getByText('New event after refresh')).toBeInTheDocument()
      })
    })

    it('should NOT refetch events when resourceData changes if Events tab is not open', async () => {
      // Initial render
      fetchWithMock.mockResolvedValueOnce(mockSourceData)
      const { rerender } = render(
        <SourcePanel
          resourceData={mockResourceData}
        />
      )

      // Wait for initial load (on Overview tab)
      await waitFor(() => {
        expect(screen.getByText('Overview')).toBeInTheDocument()
      })

      // Only source data should be fetched
      expect(fetchWithMock).toHaveBeenCalledTimes(1)

      // Simulate parent auto-refresh by changing resourceData
      const updatedResourceData = {
        ...mockResourceData,
        status: {
          ...mockResourceData.status,
          reconcilerRef: { status: 'Progressing' }
        }
      }

      fetchWithMock.mockResolvedValueOnce(mockSourceData)

      rerender(
        <SourcePanel
          resourceData={updatedResourceData}
        />
      )

      // Should only refetch source data, not events
      await waitFor(() => {
        expect(fetchWithMock).toHaveBeenCalledTimes(2) // source, source (refresh)
      })
    })

    it('should NOT refetch events on initial mount when Events tab is opened', async () => {
      const user = userEvent.setup()

      fetchWithMock.mockResolvedValueOnce(mockSourceData)
      render(
        <SourcePanel
          resourceData={mockResourceData}
        />
      )

      // Wait for initial load
      await waitFor(() => {
        expect(screen.getByText('Events')).toBeInTheDocument()
      })

      // Click on Events tab
      fetchWithMock.mockResolvedValueOnce(mockEvents)
      const eventsTab = screen.getByText('Events')
      await user.click(eventsTab)

      // Events should be fetched only once (not twice due to auto-refresh effect)
      await waitFor(() => {
        expect(fetchWithMock).toHaveBeenCalledTimes(2) // source, events (NOT events again)
      })
    })

    it('should preserve event data when refetch fails during auto-refresh', async () => {
      const user = userEvent.setup()
      const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {})

      // Initial render
      fetchWithMock.mockResolvedValueOnce(mockSourceData)
      const { rerender } = render(
        <SourcePanel
          resourceData={mockResourceData}
        />
      )

      // Wait for initial load
      await waitFor(() => {
        expect(screen.getByText('Events')).toBeInTheDocument()
      })

      // Click on Events tab
      fetchWithMock.mockResolvedValueOnce(mockEvents)
      const eventsTab = screen.getByText('Events')
      await user.click(eventsTab)

      // Wait for events to load
      await waitFor(() => {
        expect(screen.getByText('Artifact up to date with remote revision')).toBeInTheDocument()
      })

      // Simulate parent auto-refresh with events fetch error
      const updatedResourceData = {
        ...mockResourceData,
        status: {
          ...mockResourceData.status,
          reconcilerRef: { status: 'Progressing' }
        }
      }

      fetchWithMock.mockResolvedValueOnce(mockSourceData)
      fetchWithMock.mockRejectedValueOnce(new Error('Network error'))

      rerender(
        <SourcePanel
          resourceData={updatedResourceData}
        />
      )

      // Should preserve existing events
      await waitFor(() => {
        expect(screen.getByText('Artifact up to date with remote revision')).toBeInTheDocument()
      })

      consoleSpy.mockRestore()
    })
  })

  describe('Chart tab for HelmRelease', () => {
    const mockHelmRepoSourceRef = {
      kind: 'HelmRepository',
      name: 'tailscale-operator',
      namespace: 'tailscale',
      status: 'Ready',
      url: 'https://pkgs.tailscale.com/helmcharts',
      message: "stored artifact for digest 'sha256:abc123'"
    }

    const mockHelmRepoSourceData = {
      apiVersion: 'source.toolkit.fluxcd.io/v1',
      kind: 'HelmRepository',
      metadata: {
        name: 'tailscale-operator',
        namespace: 'tailscale'
      },
      spec: {
        interval: '1h',
        url: 'https://pkgs.tailscale.com/helmcharts'
      },
      status: {
        conditions: [
          {
            type: 'Ready',
            status: 'True',
            lastTransitionTime: '2025-01-15T10:00:00Z',
            message: "stored artifact for digest 'sha256:abc123'"
          }
        ]
      }
    }

    const mockHelmReleaseResourceData = {
      apiVersion: 'helm.toolkit.fluxcd.io/v2',
      kind: 'HelmRelease',
      metadata: {
        name: 'tailscale-operator',
        namespace: 'tailscale'
      },
      spec: {
        chart: {
          spec: {
            chart: 'tailscale-operator',
            version: '>=1.0.0',
            sourceRef: {
              kind: 'HelmRepository',
              name: 'tailscale-operator'
            }
          }
        },
        interval: '5m'
      },
      status: {
        helmChart: 'tailscale/tailscale-tailscale-operator',
        sourceRef: mockHelmRepoSourceRef,
        reconcilerRef: {
          status: 'Ready'
        }
      }
    }

    const mockHelmChartData = {
      apiVersion: 'source.toolkit.fluxcd.io/v1',
      kind: 'HelmChart',
      metadata: {
        name: 'tailscale-tailscale-operator',
        namespace: 'tailscale'
      },
      spec: {
        chart: 'tailscale-operator',
        interval: '24h0m0s',
        version: '*'
      },
      status: {
        reconcilerRef: {
          status: 'Ready',
          message: "pulled 'tailscale-operator' chart with version '1.90.6'"
        },
        conditions: [
          {
            type: 'Ready',
            status: 'True',
            lastTransitionTime: '2025-01-15T10:00:00Z',
            message: "pulled 'tailscale-operator' chart with version '1.90.6'"
          }
        ]
      }
    }

    it('should NOT show Chart tab when resourceData is not a HelmRelease', async () => {
      fetchWithMock.mockResolvedValue(mockSourceData)

      render(
        <SourcePanel
          resourceData={mockResourceData}
        />
      )

      await waitFor(() => {
        expect(screen.getByText('Overview')).toBeInTheDocument()
      })

      expect(screen.queryByText('Chart')).not.toBeInTheDocument()
    })

    it('should NOT show Chart tab when HelmRelease has no helmChart status', async () => {
      fetchWithMock.mockResolvedValue(mockHelmRepoSourceData)

      render(
        <SourcePanel
          resourceData={{
            kind: 'HelmRelease',
            status: {
              sourceRef: mockHelmRepoSourceRef
              // No helmChart field
            }
          }}
        />
      )

      await waitFor(() => {
        expect(screen.getByText('Overview')).toBeInTheDocument()
      })

      expect(screen.queryByText('Chart')).not.toBeInTheDocument()
    })

    it('should show Chart tab when resourceData is HelmRelease with helmChart status', async () => {
      fetchWithMock.mockResolvedValue(mockHelmRepoSourceData)

      render(
        <SourcePanel
          resourceData={mockHelmReleaseResourceData}
        />
      )

      await waitFor(() => {
        expect(screen.getByText('Overview')).toBeInTheDocument()
        expect(screen.getByText('Chart')).toBeInTheDocument()
      })
    })

    it('should fetch HelmChart data when Chart tab is clicked', async () => {
      fetchWithMock.mockResolvedValueOnce(mockHelmRepoSourceData)
      const user = userEvent.setup()

      render(
        <SourcePanel
          resourceData={mockHelmReleaseResourceData}
        />
      )

      await waitFor(() => {
        expect(screen.getByText('Chart')).toBeInTheDocument()
      })

      // HelmChart should not be fetched yet
      expect(fetchWithMock).toHaveBeenCalledTimes(1)

      // Click on Chart tab
      fetchWithMock.mockResolvedValueOnce(mockHelmChartData)
      const chartTab = screen.getByText('Chart')
      await user.click(chartTab)

      // Now HelmChart should be fetched
      await waitFor(() => {
        expect(fetchWithMock).toHaveBeenCalledTimes(2)
        expect(fetchWithMock).toHaveBeenCalledWith({
          endpoint: '/api/v1/resource?kind=HelmChart&name=tailscale-tailscale-operator&namespace=tailscale',
          mockPath: '../mock/resource',
          mockExport: 'getMockResource'
        })
      })
    })

    it('should display Chart tab content correctly', async () => {
      fetchWithMock.mockResolvedValueOnce(mockHelmRepoSourceData)
      const user = userEvent.setup()

      render(
        <SourcePanel
          resourceData={mockHelmReleaseResourceData}
        />
      )

      await waitFor(() => {
        expect(screen.getByText('Chart')).toBeInTheDocument()
      })

      // Click on Chart tab
      fetchWithMock.mockResolvedValueOnce(mockHelmChartData)
      const chartTab = screen.getByText('Chart')
      await user.click(chartTab)

      // Check chart content is displayed
      await waitFor(() => {
        const textContent = document.body.textContent
        expect(textContent).toContain('HelmChart/tailscale/tailscale-tailscale-operator')
        expect(textContent).toContain('Status')
        expect(textContent).toContain('Ready')
        expect(textContent).toContain('Semver')
        expect(textContent).toContain('>=1.0.0')
        expect(textContent).toContain('Fetch every')
        expect(textContent).toContain('24h0m0s')
        expect(textContent).toContain('Fetched at')
        expect(textContent).toContain('Fetch result')
        expect(textContent).toContain("pulled 'tailscale-operator' chart with version '1.90.6'")
      })
    })

    it('should show loading state while fetching HelmChart data', async () => {
      fetchWithMock.mockResolvedValueOnce(mockHelmRepoSourceData)
      const user = userEvent.setup()

      render(
        <SourcePanel
          resourceData={mockHelmReleaseResourceData}
        />
      )

      await waitFor(() => {
        expect(screen.getByText('Chart')).toBeInTheDocument()
      })

      // Setup pending promise
      let resolvePromise
      const promise = new Promise((resolve) => { resolvePromise = resolve })
      fetchWithMock.mockReturnValueOnce(promise)

      // Click on Chart tab
      const chartTab = screen.getByText('Chart')
      await user.click(chartTab)

      // Should show loading spinner
      expect(screen.getByText('Loading chart...')).toBeInTheDocument()

      // Resolve the promise
      resolvePromise(mockHelmChartData)

      // Wait for loading to complete
      await waitFor(() => {
        expect(screen.queryByText('Loading chart...')).not.toBeInTheDocument()
      })
    })

    it('should navigate to HelmChart resource when link is clicked', async () => {
      fetchWithMock.mockResolvedValueOnce(mockHelmRepoSourceData)
      const user = userEvent.setup()

      render(
        <SourcePanel
          resourceData={mockHelmReleaseResourceData}
        />
      )

      await waitFor(() => {
        expect(screen.getByText('Chart')).toBeInTheDocument()
      })

      // Click on Chart tab
      fetchWithMock.mockResolvedValueOnce(mockHelmChartData)
      const chartTab = screen.getByText('Chart')
      await user.click(chartTab)

      // Wait for chart content to load
      await waitFor(() => {
        expect(screen.getByText('HelmChart/tailscale/tailscale-tailscale-operator')).toBeInTheDocument()
      })

      // Click on the HelmChart link
      const chartLink = screen.getByText('HelmChart/tailscale/tailscale-tailscale-operator').closest('button')
      await user.click(chartLink)

      expect(mockRoute).toHaveBeenCalledWith('/resource/HelmChart/tailscale/tailscale-tailscale-operator')
    })

    it('should show "*" for semver when version is not specified', async () => {
      fetchWithMock.mockResolvedValueOnce(mockHelmRepoSourceData)
      const user = userEvent.setup()

      const resourceDataWithoutVersion = {
        ...mockHelmReleaseResourceData,
        spec: {
          ...mockHelmReleaseResourceData.spec,
          chart: {
            spec: {
              chart: 'tailscale-operator',
              // No version specified
              sourceRef: {
                kind: 'HelmRepository',
                name: 'tailscale-operator'
              }
            }
          }
        }
      }

      render(
        <SourcePanel
          resourceData={resourceDataWithoutVersion}
        />
      )

      await waitFor(() => {
        expect(screen.getByText('Chart')).toBeInTheDocument()
      })

      // Click on Chart tab
      fetchWithMock.mockResolvedValueOnce(mockHelmChartData)
      const chartTab = screen.getByText('Chart')
      await user.click(chartTab)

      // Check semver shows "*"
      await waitFor(() => {
        const textContent = document.body.textContent
        expect(textContent).toContain('Semver')
        expect(textContent).toContain('*')
      })
    })

    it('should handle HelmChart fetch error gracefully', async () => {
      const user = userEvent.setup()
      const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {})

      // Use implementation to handle all calls
      fetchWithMock.mockImplementation(({ endpoint }) => {
        if (endpoint.includes('/api/v1/resource?') && endpoint.includes('kind=HelmRepository')) {
          return Promise.resolve(mockHelmRepoSourceData)
        }
        if (endpoint.includes('/api/v1/resource?') && endpoint.includes('kind=HelmChart')) {
          return Promise.reject(new Error('Network error'))
        }
        return Promise.resolve({})
      })

      render(
        <SourcePanel
          resourceData={mockHelmReleaseResourceData}
        />
      )

      await waitFor(() => {
        expect(screen.getByText('Chart')).toBeInTheDocument()
      })

      // Click on Chart tab
      const chartTab = screen.getByText('Chart')
      await user.click(chartTab)

      // Should not crash, chart tab should still be visible after error
      await waitFor(() => {
        expect(consoleSpy).toHaveBeenCalled()
      })

      // Chart tab should still be visible
      expect(screen.getByText('Chart')).toBeInTheDocument()

      consoleSpy.mockRestore()
    })

    it('should fetch HelmChart data only once when switching tabs multiple times', async () => {
      fetchWithMock.mockResolvedValueOnce(mockHelmRepoSourceData)
      const user = userEvent.setup()

      render(
        <SourcePanel
          resourceData={mockHelmReleaseResourceData}
        />
      )

      await waitFor(() => {
        expect(screen.getByText('Chart')).toBeInTheDocument()
      })

      // Click on Chart tab
      fetchWithMock.mockResolvedValueOnce(mockHelmChartData)
      const chartTab = screen.getByText('Chart')
      await user.click(chartTab)

      await waitFor(() => {
        expect(fetchWithMock).toHaveBeenCalledTimes(2)
      })

      // Switch to Overview
      const overviewTab = screen.getByText('Overview')
      await user.click(overviewTab)

      // Switch back to Chart
      await user.click(chartTab)

      // Should still only have been called twice (not fetched again)
      expect(fetchWithMock).toHaveBeenCalledTimes(2)
    })

    it('should refetch HelmChart data when resourceData changes and Chart tab is open', async () => {
      fetchWithMock.mockResolvedValueOnce(mockHelmRepoSourceData)
      const user = userEvent.setup()

      const { rerender } = render(
        <SourcePanel
          resourceData={mockHelmReleaseResourceData}
        />
      )

      await waitFor(() => {
        expect(screen.getByText('Chart')).toBeInTheDocument()
      })

      // Click on Chart tab
      fetchWithMock.mockResolvedValueOnce(mockHelmChartData)
      const chartTab = screen.getByText('Chart')
      await user.click(chartTab)

      // Wait for chart to load
      await waitFor(() => {
        expect(screen.getByText("pulled 'tailscale-operator' chart with version '1.90.6'")).toBeInTheDocument()
      })

      // Simulate parent auto-refresh
      const updatedResourceData = {
        ...mockHelmReleaseResourceData,
        status: {
          ...mockHelmReleaseResourceData.status,
          reconcilerRef: {
            status: 'Progressing'
          }
        }
      }

      fetchWithMock.mockResolvedValueOnce(mockHelmRepoSourceData)
      fetchWithMock.mockResolvedValueOnce({
        ...mockHelmChartData,
        status: {
          ...mockHelmChartData.status,
          conditions: [
            {
              type: 'Ready',
              status: 'True',
              lastTransitionTime: '2025-01-15T10:05:00Z',
              message: "pulled 'tailscale-operator' chart with version '1.91.0'"
            }
          ]
        }
      })

      rerender(
        <SourcePanel
          resourceData={updatedResourceData}
        />
      )

      // Should refetch chart data
      await waitFor(() => {
        expect(screen.getByText("pulled 'tailscale-operator' chart with version '1.91.0'")).toBeInTheDocument()
      })
    })
  })
})
