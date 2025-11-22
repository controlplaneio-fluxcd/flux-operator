// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/preact'
import userEvent from '@testing-library/user-event'
import { SourcePanel } from './SourcePanel'
import { fetchWithMock } from '../../utils/fetch'

// Mock the fetch utility
vi.mock('../../utils/fetch', () => ({
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

  it('should render the source section in collapsed state initially', () => {
    render(
      <SourcePanel
        sourceRef={mockSourceRef}
        namespace="flux-system"
      />
    )

    expect(screen.getByTestId('source-view')).toBeInTheDocument()
    expect(screen.getByText('Source')).toBeInTheDocument()
  })

  it('should fetch source data when component mounts', async () => {
    fetchWithMock.mockResolvedValue(mockSourceData)

    render(
      <SourcePanel
        sourceRef={mockSourceRef}
        namespace="flux-system"
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
        sourceRef={mockSourceRef}
        namespace="flux-system"
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
        sourceRef={mockSourceRef}
        namespace="flux-system"
      />
    )

    await waitFor(() => {
      expect(screen.getByText('Overview')).toBeInTheDocument()
    })

    // Check overview content
    const textContent = document.body.textContent
    expect(textContent).toContain('Status:')
    expect(textContent).toContain('Ready')
    expect(textContent).toContain('Managed by:')
    expect(textContent).toContain('source-controller')
    expect(textContent).toContain('ID:')
    expect(textContent).toContain('GitRepository/flux-system/flux-system')
    expect(textContent).toContain('URL:')
    expect(textContent).toContain('https://github.com/example/repo.git')
  })

  it('should navigate to source resource when ID button is clicked', async () => {
    fetchWithMock.mockResolvedValue(mockSourceData)
    const user = userEvent.setup()

    render(
      <SourcePanel
        sourceRef={mockSourceRef}
        namespace="flux-system"
      />
    )

    await waitFor(() => {
      expect(screen.getByText('ID:')).toBeInTheDocument()
    })

    const idButton = screen.getByText('GitRepository/flux-system/flux-system').closest('button')
    await user.click(idButton)

    expect(mockRoute).toHaveBeenCalledWith('/resource/GitRepository/flux-system/flux-system')
  })

  it('should display fetch every and fetched at when source data is available', async () => {
    fetchWithMock.mockResolvedValue(mockSourceData)

    render(
      <SourcePanel
        sourceRef={mockSourceRef}
        namespace="flux-system"
      />
    )

    await waitFor(() => {
      const textContent = document.body.textContent
      expect(textContent).toContain('Fetch every:')
      expect(textContent).toContain('1m')
      expect(textContent).toContain('Fetched at:')
    })
  })

  it('should display origin URL and revision when available', async () => {
    const sourceRefWithOrigin = {
      ...mockSourceRef,
      originURL: 'https://github.com/original/repo.git',
      originRevision: 'v1.2.3'
    }

    fetchWithMock.mockResolvedValue(mockSourceData)

    render(
      <SourcePanel
        sourceRef={sourceRefWithOrigin}
        namespace="flux-system"
      />
    )

    await waitFor(() => {
      const textContent = document.body.textContent
      expect(textContent).toContain('Origin URL:')
      expect(textContent).toContain('https://github.com/original/repo.git')
      expect(textContent).toContain('Origin Revision:')
      expect(textContent).toContain('v1.2.3')
    })
  })

  it('should not display origin URL and revision when empty', async () => {
    fetchWithMock.mockResolvedValue(mockSourceData)

    render(
      <SourcePanel
        sourceRef={mockSourceRef}
        namespace="flux-system"
      />
    )

    await waitFor(() => {
      const textContent = document.body.textContent
      expect(textContent).not.toContain('Origin URL:')
      expect(textContent).not.toContain('Origin Revision:')
    })
  })

  it('should display fetch result message', async () => {
    fetchWithMock.mockResolvedValue(mockSourceData)

    render(
      <SourcePanel
        sourceRef={mockSourceRef}
        namespace="flux-system"
      />
    )

    await waitFor(() => {
      const textContent = document.body.textContent
      expect(textContent).toContain('Fetch result:')
      expect(textContent).toContain("stored artifact for revision 'refs/heads/main@sha1:abc123'")
    })
  })

  it('should switch to events tab and fetch events on demand', async () => {
    fetchWithMock.mockResolvedValueOnce(mockSourceData)
    const user = userEvent.setup()

    render(
      <SourcePanel
        sourceRef={mockSourceRef}
        namespace="flux-system"
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
        sourceRef={mockSourceRef}
        namespace="flux-system"
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
        sourceRef={mockSourceRef}
        namespace="flux-system"
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
        sourceRef={mockSourceRef}
        namespace="flux-system"
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
        sourceRef={mockSourceRef}
        namespace="flux-system"
      />
    )

    await waitFor(() => {
      expect(screen.getByText('Status')).toBeInTheDocument()
    })

    // Click on Status tab
    const statusTab = screen.getByText('Status')
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
        sourceRef={mockSourceRef}
        namespace="flux-system"
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
        sourceRef={mockSourceRef}
        namespace="flux-system"
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

  it('should use fallback namespace when sourceRef namespace is missing', async () => {
    const sourceRefWithoutNamespace = {
      kind: 'GitRepository',
      name: 'flux-system',
      status: 'Ready'
    }

    fetchWithMock.mockResolvedValue(mockSourceData)

    render(
      <SourcePanel
        sourceRef={sourceRefWithoutNamespace}
        namespace="default"
      />
    )

    await waitFor(() => {
      expect(fetchWithMock).toHaveBeenCalledWith({
        endpoint: '/api/v1/resource?kind=GitRepository&name=flux-system&namespace=default',
        mockPath: '../mock/resource',
        mockExport: 'getMockResource'
      })
    })
  })

  it('should only show Overview and Events tabs when source data fails to load', async () => {
    const consoleErrorSpy = vi.spyOn(console, 'error').mockImplementation(() => {})
    fetchWithMock.mockRejectedValue(new Error('Network error'))

    render(
      <SourcePanel
        sourceRef={mockSourceRef}
        namespace="flux-system"
      />
    )

    await waitFor(() => {
      expect(screen.getByText('Overview')).toBeInTheDocument()
      expect(screen.getByText('Events')).toBeInTheDocument()
    })

    // Specification and Status tabs should not be present
    expect(screen.queryByText('Specification')).not.toBeInTheDocument()
    expect(screen.queryByText('Status')).not.toBeInTheDocument()

    consoleErrorSpy.mockRestore()
  })

  it('should fetch events only once when switching tabs multiple times', async () => {
    fetchWithMock.mockResolvedValueOnce(mockSourceData)
    const user = userEvent.setup()

    render(
      <SourcePanel
        sourceRef={mockSourceRef}
        namespace="flux-system"
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
})
