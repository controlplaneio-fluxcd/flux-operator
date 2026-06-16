// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/preact'
import userEvent from '@testing-library/user-event'
import { WorkloadLogsViewer } from './WorkloadLogsViewer'

// Mock the fetchWithMock function
vi.mock('../../../utils/fetch', () => ({
  fetchWithMock: vi.fn()
}))

import { fetchWithMock } from '../../../utils/fetch'

describe('WorkloadLogsViewer component', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    // Each line carries a leading timestamp (as the API returns); timestamps are
    // hidden by default so the viewer strips it, leaving the message text.
    fetchWithMock.mockResolvedValue({ pod: 'my-pod', container: 'app', logs: '2026-01-01T00:00:00Z line one\n2026-01-01T00:00:01Z line two\n' })
  })

  const defaultProps = {
    namespace: 'default',
    name: 'my-pod',
    containers: [{ name: 'app', isInit: false }],
    onClose: vi.fn()
  }

  it('fetches and renders logs on mount', async () => {
    render(<WorkloadLogsViewer {...defaultProps} />)

    expect(screen.getByTestId('logs-viewer')).toBeInTheDocument()
    await waitFor(() => {
      expect(screen.getByTestId('logs-content')).toHaveTextContent('line one')
    })

    expect(fetchWithMock).toHaveBeenCalledWith(
      expect.objectContaining({
        endpoint: expect.stringContaining('/api/v1/workload/logs?'),
        mockExport: 'getMockWorkloadLogs'
      })
    )
    const call = fetchWithMock.mock.calls[0][0]
    expect(call.endpoint).toContain('namespace=default')
    expect(call.endpoint).toContain('name=my-pod')
    expect(call.endpoint).toContain('container=app')
  })

  it('always shows the container selector, even for a single container', async () => {
    render(<WorkloadLogsViewer {...defaultProps} />)
    await waitFor(() => expect(screen.getByTestId('logs-content')).toBeInTheDocument())
    const select = screen.getByTestId('logs-container-select')
    expect(select).toBeInTheDocument()
    expect(select).toHaveValue('app')
  })

  it('shows the container selector for multiple containers and refetches on change', async () => {
    const user = userEvent.setup()
    const props = {
      ...defaultProps,
      containers: [{ name: 'app', isInit: false }, { name: 'sidecar', isInit: false }]
    }
    render(<WorkloadLogsViewer {...props} />)

    const select = await screen.findByTestId('logs-container-select')
    expect(select).toBeInTheDocument()

    await user.selectOptions(select, 'sidecar')
    await waitFor(() => {
      const lastCall = fetchWithMock.mock.calls[fetchWithMock.mock.calls.length - 1][0]
      expect(lastCall.endpoint).toContain('container=sidecar')
    })
  })

  it('shows an error message when the fetch fails', async () => {
    fetchWithMock.mockRejectedValue(new Error('Permission denied'))
    render(<WorkloadLogsViewer {...defaultProps} />)

    await waitFor(() => {
      expect(screen.getByTestId('logs-error')).toHaveTextContent('Permission denied')
    })
  })

  it('calls onClose when the close button is clicked', async () => {
    const onClose = vi.fn()
    const user = userEvent.setup()
    render(<WorkloadLogsViewer {...defaultProps} onClose={onClose} />)

    await user.click(screen.getByTestId('logs-close-button'))
    expect(onClose).toHaveBeenCalled()
  })

  it('renders each log entry as a separate row', async () => {
    render(<WorkloadLogsViewer {...defaultProps} />)
    await waitFor(() => {
      expect(screen.getAllByTestId('logs-line')).toHaveLength(2)
    })
    const rows = screen.getAllByTestId('logs-line')
    expect(rows[0]).toHaveTextContent('line one')
    expect(rows[1]).toHaveTextContent('line two')
  })

  it('requests 100 lines by default', async () => {
    render(<WorkloadLogsViewer {...defaultProps} />)
    await waitFor(() => expect(fetchWithMock).toHaveBeenCalled())
    expect(fetchWithMock.mock.calls[0][0].endpoint).toContain('tailLines=100')
  })

  it('requests the selected number of lines', async () => {
    const user = userEvent.setup()
    render(<WorkloadLogsViewer {...defaultProps} />)
    await waitFor(() => expect(screen.getByTestId('logs-content')).toBeInTheDocument())

    await user.selectOptions(screen.getByTestId('logs-lines-select'), '500')
    await waitFor(() => {
      const lastCall = fetchWithMock.mock.calls[fetchWithMock.mock.calls.length - 1][0]
      expect(lastCall.endpoint).toContain('tailLines=500')
    })
  })

  it('filters log entries by the contains text', async () => {
    const user = userEvent.setup()
    render(<WorkloadLogsViewer {...defaultProps} />)
    await waitFor(() => expect(screen.getAllByTestId('logs-line')).toHaveLength(2))

    await user.type(screen.getByTestId('logs-filter-input'), 'two')
    await waitFor(() => expect(screen.getAllByTestId('logs-line')).toHaveLength(1))
    expect(screen.getByTestId('logs-line')).toHaveTextContent('line two')
  })

  it('hides timestamps by default and shows them when toggled', async () => {
    const user = userEvent.setup()
    fetchWithMock.mockResolvedValue({ logs: '2026-06-16T00:00:00Z hello world\n' })
    render(<WorkloadLogsViewer {...defaultProps} />)
    await waitFor(() => expect(screen.getByTestId('logs-line')).toHaveTextContent('hello world'))

    // Timestamps are hidden by default: the leading timestamp is stripped.
    const toggle = screen.getByTestId('logs-timestamps-toggle')
    expect(toggle).toHaveAttribute('aria-pressed', 'false')
    expect(screen.getByTestId('logs-line')).not.toHaveTextContent('2026-06-16T00:00:00Z')

    // Toggle on to reveal the timestamps.
    await user.click(toggle)
    expect(toggle).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByTestId('logs-line')).toHaveTextContent('2026-06-16T00:00:00Z')
  })

  it('toggles the previous container instance and refetches', async () => {
    const user = userEvent.setup()
    render(<WorkloadLogsViewer {...defaultProps} />)
    await waitFor(() => expect(fetchWithMock).toHaveBeenCalled())
    expect(fetchWithMock.mock.calls[0][0].endpoint).toContain('previous=false')

    await user.click(screen.getByTestId('logs-previous-toggle'))
    await waitFor(() => {
      const lastCall = fetchWithMock.mock.calls[fetchWithMock.mock.calls.length - 1][0]
      expect(lastCall.endpoint).toContain('previous=true')
    })
  })

  it('downloads the logs as a <pod>.log file', async () => {
    const user = userEvent.setup()
    fetchWithMock.mockResolvedValue({ logs: 'line one\nline two\n' })

    let downloadName
    window.URL.createObjectURL = vi.fn(() => 'blob:mock')
    window.URL.revokeObjectURL = vi.fn()
    const clickSpy = vi.spyOn(window.HTMLAnchorElement.prototype, 'click').mockImplementation(function () {
      downloadName = this.download
    })

    render(<WorkloadLogsViewer {...defaultProps} />)
    await waitFor(() => expect(screen.getByTestId('logs-content')).toBeInTheDocument())

    await user.click(screen.getByTestId('logs-download-button'))
    expect(window.URL.createObjectURL).toHaveBeenCalled()
    expect(clickSpy).toHaveBeenCalled()
    expect(downloadName).toBe('my-pod.log')
    expect(window.URL.revokeObjectURL).toHaveBeenCalledWith('blob:mock')

    clickSpy.mockRestore()
  })

  it('toggles fullscreen mode', async () => {
    const user = userEvent.setup()
    render(<WorkloadLogsViewer {...defaultProps} />)
    const toggle = screen.getByTestId('logs-fullscreen-toggle')
    expect(toggle).toHaveAttribute('aria-pressed', 'false')

    await user.click(toggle)
    expect(toggle).toHaveAttribute('aria-pressed', 'true')
  })

  it('follows by default, sets up polling and can be disabled', async () => {
    const user = userEvent.setup()
    const setIntervalSpy = vi.spyOn(globalThis, 'setInterval')
    const polls = () => setIntervalSpy.mock.calls.filter(c => c[1] === 5000).length

    render(<WorkloadLogsViewer {...defaultProps} />)
    await waitFor(() => expect(screen.getByTestId('logs-content')).toBeInTheDocument())

    const toggle = screen.getByTestId('logs-follow-toggle')
    expect(toggle).toHaveAttribute('aria-pressed', 'true')
    await waitFor(() => expect(polls()).toBe(1))

    await user.click(toggle)
    expect(toggle).toHaveAttribute('aria-pressed', 'false')

    setIntervalSpy.mockRestore()
  })

  it('highlights the latest line when new logs arrive', async () => {
    const user = userEvent.setup()
    fetchWithMock.mockResolvedValue({ logs: 'line one\nline two\n' })
    render(<WorkloadLogsViewer {...defaultProps} />)
    await waitFor(() => expect(screen.getAllByTestId('logs-line')).toHaveLength(2))

    // No highlight on the initial load.
    expect(screen.getAllByTestId('logs-line').some(el => el.getAttribute('data-latest') === 'true')).toBe(false)

    // A new entry arrives on the next fetch.
    fetchWithMock.mockResolvedValue({ logs: 'line one\nline two\nline three\n' })
    await user.click(screen.getByTestId('logs-refresh-button'))
    await waitFor(() => {
      const rows = screen.getAllByTestId('logs-line')
      expect(rows).toHaveLength(3)
      expect(rows[rows.length - 1]).toHaveAttribute('data-latest', 'true')
    })
    expect(screen.getAllByTestId('logs-line')[0]).not.toHaveAttribute('data-latest')
  })
})
