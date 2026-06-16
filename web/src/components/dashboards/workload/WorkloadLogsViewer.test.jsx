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
    expect(select).toHaveValue('app::false')
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

    await user.selectOptions(select, 'sidecar::false')
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

  it('excludes log entries when the filter is prefixed with "!"', async () => {
    const user = userEvent.setup()
    render(<WorkloadLogsViewer {...defaultProps} />)
    await waitFor(() => expect(screen.getAllByTestId('logs-line')).toHaveLength(2))

    await user.type(screen.getByTestId('logs-filter-input'), '!two')
    await waitFor(() => expect(screen.getAllByTestId('logs-line')).toHaveLength(1))
    expect(screen.getByTestId('logs-line')).toHaveTextContent('line one')
  })

  it('always shows the timestamp as a separator pill, separate from the message row', async () => {
    fetchWithMock.mockResolvedValue({ logs: '2026-06-16T00:00:00Z hello world\n' })
    render(<WorkloadLogsViewer {...defaultProps} />)
    await waitFor(() => expect(screen.getByTestId('logs-line')).toHaveTextContent('hello world'))

    // The timestamp lives in the pill; the message row carries only the message.
    expect(screen.getByTestId('logs-timestamp')).toHaveTextContent('2026-06-16T00:00:00Z')
    expect(screen.getByTestId('logs-line')).toHaveTextContent('hello world')
    expect(screen.getByTestId('logs-line')).not.toHaveTextContent('2026-06-16T00:00:00Z')

    // There is no timestamps toggle anymore.
    expect(screen.queryByTestId('logs-timestamps-toggle')).not.toBeInTheDocument()
  })

  it('offers a "(previous)" entry only for restarted containers and refetches with previous=true', async () => {
    const user = userEvent.setup()
    const props = {
      ...defaultProps,
      containers: [
        { name: 'app', isInit: false, restartCount: 2 },
        { name: 'sidecar', isInit: false, restartCount: 0 }
      ]
    }
    render(<WorkloadLogsViewer {...props} />)
    await waitFor(() => expect(fetchWithMock).toHaveBeenCalled())
    expect(fetchWithMock.mock.calls[0][0].endpoint).toContain('previous=false')

    const select = screen.getByTestId('logs-container-select')
    const values = [...select.querySelectorAll('option')].map(o => o.value)
    // Restarted container gets a previous entry; the non-restarted one does not.
    expect(values).toContain('app::false')
    expect(values).toContain('app::true')
    expect(values).toContain('sidecar::false')
    expect(values).not.toContain('sidecar::true')

    await user.selectOptions(select, 'app::true')
    await waitFor(() => {
      const lastCall = fetchWithMock.mock.calls[fetchWithMock.mock.calls.length - 1][0]
      expect(lastCall.endpoint).toContain('container=app')
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

  it('always shows the line count in the footer, with a loader while fetching', async () => {
    let resolveFetch
    fetchWithMock.mockReturnValue(new Promise((resolve) => { resolveFetch = resolve }))
    render(<WorkloadLogsViewer {...defaultProps} />)

    // The footer always shows the line count; the loader shows alongside it
    // while the fetch is pending.
    expect(screen.getByTestId('logs-footer')).toHaveTextContent('0 lines')
    expect(screen.getByTestId('logs-loader')).toBeInTheDocument()

    resolveFetch({ logs: 'line one\nline two\n' })
    await waitFor(() => expect(screen.queryByTestId('logs-loader')).not.toBeInTheDocument())
    expect(screen.getByTestId('logs-footer')).toHaveTextContent('2 lines')
  })

  it('renders JSON lines as highlighted code blocks when the JSON toggle is on, leaving plain lines intact', async () => {
    const user = userEvent.setup()
    fetchWithMock.mockResolvedValue({
      logs: '2026-01-01T00:00:00Z {"level":"info","msg":"hello"}\n2026-01-01T00:00:01Z plain text line\n'
    })
    render(<WorkloadLogsViewer {...defaultProps} />)
    await waitFor(() => expect(screen.getAllByTestId('logs-line')).toHaveLength(2))

    const toggle = screen.getByTestId('logs-json-toggle')
    expect(toggle).toHaveAttribute('aria-pressed', 'false')

    // Off: the JSON line is shown compact, exactly as received, as a plain row.
    expect(screen.getAllByTestId('logs-line')[0].tagName).toBe('DIV')
    expect(screen.getAllByTestId('logs-line')[0].textContent).toBe('{"level":"info","msg":"hello"}')

    // On: the JSON line becomes an indented, syntax-highlighted code block. The
    // plain line also renders as a code block (so all lines share the same font
    // and size) but with its text unchanged and no syntax highlighting.
    await user.click(toggle)
    expect(toggle).toHaveAttribute('aria-pressed', 'true')
    const rows = screen.getAllByTestId('logs-line')
    expect(rows[0].tagName).toBe('PRE')
    expect(rows[0].querySelector('code')).toHaveClass('language-json')
    expect(rows[0].querySelector('.token')).not.toBeNull()
    expect(rows[0].textContent).toBe('{\n  "level": "info",\n  "msg": "hello"\n}')
    expect(rows[1].tagName).toBe('PRE')
    expect(rows[1].querySelector('code')).toHaveClass('language-json')
    expect(rows[1].querySelector('.token')).toBeNull()
    expect(rows[1].textContent).toBe('plain text line')
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

  it('highlights the latest timestamp pill when new logs arrive', async () => {
    const user = userEvent.setup()
    fetchWithMock.mockResolvedValue({ logs: '2026-01-01T00:00:00Z line one\n2026-01-01T00:00:01Z line two\n' })
    render(<WorkloadLogsViewer {...defaultProps} />)
    await waitFor(() => expect(screen.getAllByTestId('logs-line')).toHaveLength(2))

    // No highlight on the initial load.
    expect(screen.getAllByTestId('logs-timestamp').some(el => el.getAttribute('data-latest') === 'true')).toBe(false)

    // A new entry arrives on the next fetch (triggered by changing the line count).
    fetchWithMock.mockResolvedValue({ logs: '2026-01-01T00:00:00Z line one\n2026-01-01T00:00:01Z line two\n2026-01-01T00:00:02Z line three\n' })
    await user.selectOptions(screen.getByTestId('logs-lines-select'), '500')
    await waitFor(() => {
      const pills = screen.getAllByTestId('logs-timestamp')
      expect(pills).toHaveLength(3)
      expect(pills[pills.length - 1]).toHaveAttribute('data-latest', 'true')
    })
    expect(screen.getAllByTestId('logs-timestamp')[0]).not.toHaveAttribute('data-latest')
  })
})
