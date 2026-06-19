// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor, act, fireEvent } from '@testing-library/preact'
import userEvent from '@testing-library/user-event'
import { WorkloadLogsViewer } from './WorkloadLogsViewer'

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

  afterEach(() => {
    // Restore real timers so a leaked fake-timer interval can't bleed into the next test.
    vi.useRealTimers()
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

  it('always shows the container selector, defaulting to "All containers"', async () => {
    render(<WorkloadLogsViewer {...defaultProps} />)
    await waitFor(() => expect(screen.getByTestId('logs-content')).toBeInTheDocument())
    const select = screen.getByTestId('logs-container-select')
    expect(select).toBeInTheDocument()
    // "All containers" is the default even for a single container; the container
    // itself stays individually selectable.
    expect(select).toHaveValue('all')
    const values = [...select.querySelectorAll('option')].map(o => o.value)
    expect(values).toEqual(['all', 'app::false'])
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

  it('defaults to "All containers" for multiple regular containers and streams them all', async () => {
    const props = {
      ...defaultProps,
      containers: [{ name: 'app', isInit: false }, { name: 'sidecar', isInit: false }]
    }
    render(<WorkloadLogsViewer {...props} />)
    await waitFor(() => expect(screen.getByTestId('logs-content')).toBeInTheDocument())

    // The selector defaults to the "All containers" option.
    const select = screen.getByTestId('logs-container-select')
    expect(select).toHaveValue('all')
    const values = [...select.querySelectorAll('option')].map(o => o.value)
    expect(values).toContain('all')
    expect(values).toContain('app::false')
    expect(values).toContain('sidecar::false')

    // The initial request streams every regular container via repeated params.
    const endpoint = fetchWithMock.mock.calls[0][0].endpoint
    const containers = [...new URLSearchParams(endpoint.split('?')[1]).getAll('container')]
    expect(containers).toEqual(['app', 'sidecar'])
    expect(endpoint).toContain('previous=false')
  })

  it('excludes init containers from "All containers" but keeps them selectable', async () => {
    const user = userEvent.setup()
    const props = {
      ...defaultProps,
      containers: [
        { name: 'setup', isInit: true },
        { name: 'app', isInit: false },
        { name: 'sidecar', isInit: false }
      ]
    }
    render(<WorkloadLogsViewer {...props} />)
    await waitFor(() => expect(screen.getByTestId('logs-content')).toBeInTheDocument())

    // "All containers" streams only the regular containers, not the init one.
    const containers = [...new URLSearchParams(fetchWithMock.mock.calls[0][0].endpoint.split('?')[1]).getAll('container')]
    expect(containers).toEqual(['app', 'sidecar'])

    // The init container is still individually selectable and fetched on its own.
    const select = screen.getByTestId('logs-container-select')
    const values = [...select.querySelectorAll('option')].map(o => o.value)
    expect(values).toContain('setup::false')

    await user.selectOptions(select, 'setup::false')
    await waitFor(() => {
      const last = fetchWithMock.mock.calls.at(-1)[0].endpoint
      const c = [...new URLSearchParams(last.split('?')[1]).getAll('container')]
      expect(c).toEqual(['setup'])
    })
  })

  it('switches from "All containers" to a single container', async () => {
    const user = userEvent.setup()
    const props = {
      ...defaultProps,
      containers: [{ name: 'app', isInit: false }, { name: 'sidecar', isInit: false }]
    }
    render(<WorkloadLogsViewer {...props} />)
    await waitFor(() => expect(screen.getByTestId('logs-content')).toBeInTheDocument())

    await user.selectOptions(screen.getByTestId('logs-container-select'), 'sidecar::false')
    await waitFor(() => {
      const last = fetchWithMock.mock.calls.at(-1)[0].endpoint
      const c = [...new URLSearchParams(last.split('?')[1]).getAll('container')]
      expect(c).toEqual(['sidecar'])
    })
  })

  it('defaults to "All containers" for a single regular container, streaming only it', async () => {
    const props = {
      ...defaultProps,
      containers: [{ name: 'setup', isInit: true }, { name: 'app', isInit: false }]
    }
    render(<WorkloadLogsViewer {...props} />)
    await waitFor(() => expect(screen.getByTestId('logs-content')).toBeInTheDocument())

    // "All containers" is offered and is the default even with one regular
    // container; the init container is not part of it.
    const select = screen.getByTestId('logs-container-select')
    expect(select).toHaveValue('all')
    const containers = [...new URLSearchParams(fetchWithMock.mock.calls[0][0].endpoint.split('?')[1]).getAll('container')]
    expect(containers).toEqual(['app'])
  })

  it('titles the modal "Log Viewer" over the workload kind/namespace/name', async () => {
    const props = { ...defaultProps, kind: 'Deployment', workloadName: 'podinfo' }
    render(<WorkloadLogsViewer {...props} />)
    await waitFor(() => expect(screen.getByTestId('logs-content')).toBeInTheDocument())

    expect(screen.getByTestId('logs-viewer')).toHaveTextContent('Log Viewer')
    expect(screen.getByTestId('logs-title')).toHaveTextContent('Deployment/default/podinfo')
  })

  it('lists the workload pods and invokes onSelectPod when the pod changes', async () => {
    const user = userEvent.setup()
    const onSelectPod = vi.fn()
    const props = {
      ...defaultProps,
      name: 'pod-a',
      pods: [{ name: 'pod-a' }, { name: 'pod-b' }],
      onSelectPod
    }
    render(<WorkloadLogsViewer {...props} />)
    await waitFor(() => expect(screen.getByTestId('logs-content')).toBeInTheDocument())

    const podSelect = screen.getByTestId('logs-pod-select')
    expect(podSelect).toHaveValue('pod-a')
    expect([...podSelect.querySelectorAll('option')].map(o => o.value)).toEqual(['pod-a', 'pod-b'])

    await user.selectOptions(podSelect, 'pod-b')
    expect(onSelectPod).toHaveBeenCalledWith('pod-b')
  })

  it('resets the container selection to "All containers" when the pod changes', async () => {
    const user = userEvent.setup()
    const props = {
      ...defaultProps,
      name: 'pod-a',
      containers: [{ name: 'app', isInit: false }, { name: 'sidecar', isInit: false }],
      pods: [{ name: 'pod-a' }, { name: 'pod-b' }],
      onSelectPod: vi.fn()
    }
    const { rerender } = render(<WorkloadLogsViewer {...props} />)
    await waitFor(() => expect(screen.getByTestId('logs-content')).toBeInTheDocument())

    // Pick a specific container, away from the "All containers" default.
    await user.selectOptions(screen.getByTestId('logs-container-select'), 'sidecar::false')
    expect(screen.getByTestId('logs-container-select')).toHaveValue('sidecar::false')

    // The parent switches the pod (the viewer is controlled): it re-renders with
    // the new pod's name and containers, and the container dropdown snaps back to
    // "All containers".
    rerender(<WorkloadLogsViewer {...props} name="pod-b" containers={[{ name: 'web', isInit: false }]} />)
    await waitFor(() => expect(screen.getByTestId('logs-container-select')).toHaveValue('all'))
  })

  it('appends and dedupes across merged streams while following "All containers"', async () => {
    vi.useFakeTimers()
    try {
      const flush = async () => { await act(async () => { await Promise.resolve(); await Promise.resolve() }) }
      const props = {
        ...defaultProps,
        containers: [{ name: 'app', isInit: false }, { name: 'sidecar', isInit: false }]
      }
      // Initial load: lines from both containers, already interleaved by the backend.
      fetchWithMock.mockResolvedValueOnce({ logs: '2026-01-01T00:00:00Z app one\n2026-01-01T00:00:01Z side one\n' })
      // Follow poll: the last line is re-sent (sinceTime is second-granular) plus a new one.
      fetchWithMock.mockResolvedValue({ logs: '2026-01-01T00:00:01Z side one\n2026-01-01T00:00:02Z app two\n' })

      render(<WorkloadLogsViewer {...props} />)
      await flush()
      expect(screen.getAllByTestId('logs-line').map(r => r.textContent)).toEqual(['app one', 'side one'])

      // The initial load streams every regular container and asks for the tail.
      const first = fetchWithMock.mock.calls[0][0].endpoint
      expect([...new URLSearchParams(first.split('?')[1]).getAll('container')]).toEqual(['app', 'sidecar'])
      expect(first).not.toContain('sinceTime')

      await act(async () => { vi.advanceTimersByTime(5000) })
      await flush()

      // The overlapping "side one" is deduped across the merged result; only the
      // genuinely new "app two" is appended.
      expect(screen.getAllByTestId('logs-line').map(r => r.textContent)).toEqual(['app one', 'side one', 'app two'])

      // The follow poll narrows by sinceTime yet still streams all containers.
      const poll = fetchWithMock.mock.calls.at(-1)[0].endpoint
      expect(poll).toContain('sinceTime=2026-01-01T00%3A00%3A01Z')
      expect([...new URLSearchParams(poll.split('?')[1]).getAll('container')]).toEqual(['app', 'sidecar'])
    } finally {
      vi.useRealTimers()
    }
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

    // The timestamp is a separator pill, not a toggleable column.
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

  it('shows a per-level count summary in the footer, with a loader while fetching', async () => {
    let resolveFetch
    fetchWithMock.mockReturnValue(new Promise((resolve) => { resolveFetch = resolve }))
    render(<WorkloadLogsViewer {...defaultProps} />)

    // The loader shows while the fetch is pending.
    expect(screen.getByTestId('logs-loader')).toBeInTheDocument()

    // Two plain lines default to info; the footer summary counts them.
    resolveFetch({ logs: 'line one\nline two\n' })
    await waitFor(() => expect(screen.queryByTestId('logs-loader')).not.toBeInTheDocument())
    expect(screen.getByTestId('logs-level-summary')).toHaveTextContent('Info 2')
  })

  it('colors the timestamp pill by the detected log level', async () => {
    fetchWithMock.mockResolvedValue({
      logs: '2026-01-01T00:00:00Z {"level":"error","msg":"boom"}\n2026-01-01T00:00:01Z just some text\n'
    })
    render(<WorkloadLogsViewer {...defaultProps} />)
    await waitFor(() => expect(screen.getAllByTestId('logs-timestamp')).toHaveLength(2))

    const pills = screen.getAllByTestId('logs-timestamp')
    expect(pills[0]).toHaveAttribute('data-level', 'error')
    expect(pills[1]).toHaveAttribute('data-level', 'info')
  })

  it('filters lines to the exact selected level', async () => {
    const user = userEvent.setup()
    fetchWithMock.mockResolvedValue({
      logs: '2026-01-01T00:00:00Z {"level":"error","msg":"boom"}\n'
        + '2026-01-01T00:00:01Z {"level":"warn","msg":"slow"}\n'
        + '2026-01-01T00:00:02Z {"level":"info","msg":"hi"}\n'
    })
    render(<WorkloadLogsViewer {...defaultProps} />)
    await waitFor(() => expect(screen.getAllByTestId('logs-line')).toHaveLength(3))

    // Selecting "warn" shows ONLY warn (not warn-and-above), proving exact match.
    await user.click(screen.getByTestId('logs-level-filter'))
    await user.click(screen.getByTestId('logs-level-option-warn'))

    await waitFor(() => expect(screen.getAllByTestId('logs-line')).toHaveLength(1))
    expect(screen.getByTestId('logs-line')).toHaveTextContent('slow')

    // The level summary still counts all (it ignores the level filter).
    expect(screen.getByTestId('logs-level-summary')).toHaveTextContent('Warn 1')
    expect(screen.getByTestId('logs-level-summary')).toHaveTextContent('Error 1')
    expect(screen.getByTestId('logs-level-summary')).toHaveTextContent('Info 1')
  })

  it('strips ANSI escape codes from rendered lines', async () => {
     
    fetchWithMock.mockResolvedValue({ logs: '2026-01-01T00:00:00Z \x1b[31mred error\x1b[0m text\n' })
    render(<WorkloadLogsViewer {...defaultProps} />)
    await waitFor(() => expect(screen.getByTestId('logs-line')).toBeInTheDocument())
    expect(screen.getByTestId('logs-line').textContent).toBe('red error text')
  })

  it('pretty-prints JSON lines as highlighted code blocks by default, leaving plain lines intact', async () => {
    fetchWithMock.mockResolvedValue({
      logs: '2026-01-01T00:00:00Z {"level":"info","msg":"hello"}\n2026-01-01T00:00:01Z plain text line\n'
    })
    render(<WorkloadLogsViewer {...defaultProps} />)
    await waitFor(() => expect(screen.getAllByTestId('logs-line')).toHaveLength(2))

    // Formatted mode is the default and the toggle reflects it.
    const toggle = screen.getByTestId('logs-format-toggle')
    expect(toggle).toHaveAttribute('aria-pressed', 'true')

    // The JSON line is indented and syntax-highlighted. The plain line also
    // renders as a code block (so all lines share the same font and size) but
    // with its text unchanged and no syntax highlighting.
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

  it('strips all styling in raw mode: plain rows, no timestamp pills or highlight', async () => {
    const user = userEvent.setup()
    fetchWithMock.mockResolvedValue({
      logs: '2026-01-01T00:00:00Z {"level":"info","msg":"hello"}\n2026-01-01T00:00:01Z plain text line\n'
    })
    render(<WorkloadLogsViewer {...defaultProps} />)
    await waitFor(() => expect(screen.getAllByTestId('logs-line')).toHaveLength(2))

    // Switching off formatted mode enters raw mode.
    const toggle = screen.getByTestId('logs-format-toggle')
    await user.click(toggle)
    expect(toggle).toHaveAttribute('aria-pressed', 'false')

    // Lines render as plain rows, JSON shown compact exactly as received.
    const rows = screen.getAllByTestId('logs-line')
    expect(rows[0].tagName).toBe('DIV')
    expect(rows[0].textContent).toBe('{"level":"info","msg":"hello"}')
    expect(rows[1].tagName).toBe('DIV')
    expect(rows[1].textContent).toBe('plain text line')

    // The timestamp pills (row separators) are gone in raw mode.
    expect(screen.queryByTestId('logs-timestamp')).not.toBeInTheDocument()
  })

  it('does not highlight the latest line in raw mode when new logs arrive', async () => {
    const user = userEvent.setup()
    fetchWithMock.mockResolvedValue({ logs: '2026-01-01T00:00:00Z line one\n2026-01-01T00:00:01Z line two\n' })
    render(<WorkloadLogsViewer {...defaultProps} />)
    await waitFor(() => expect(screen.getAllByTestId('logs-line')).toHaveLength(2))

    // Enter raw mode.
    await user.click(screen.getByTestId('logs-format-toggle'))

    // A new entry arrives on the next fetch (triggered by changing the line count).
    fetchWithMock.mockResolvedValue({ logs: '2026-01-01T00:00:00Z line one\n2026-01-01T00:00:01Z line two\n2026-01-01T00:00:02Z line three\n' })
    await user.selectOptions(screen.getByTestId('logs-lines-select'), '500')
    await waitFor(() => expect(screen.getAllByTestId('logs-line')).toHaveLength(3))

    // Raw mode has no timestamp pills, so nothing is ever highlighted.
    expect(screen.queryByTestId('logs-timestamp')).not.toBeInTheDocument()
  })

  it('toggles fullscreen mode', async () => {
    const user = userEvent.setup()
    render(<WorkloadLogsViewer {...defaultProps} />)
    const toggle = screen.getByTestId('logs-fullscreen-toggle')
    expect(toggle).toHaveAttribute('aria-pressed', 'false')

    await user.click(toggle)
    expect(toggle).toHaveAttribute('aria-pressed', 'true')
  })

  it('shows the follow mode in the footer corner, toggling between Following and Snapshot', async () => {
    const user = userEvent.setup()
    render(<WorkloadLogsViewer {...defaultProps} />)
    await waitFor(() => expect(screen.getByTestId('logs-content')).toBeInTheDocument())

    // Follow is on by default, so the corner reads "Following".
    const mode = screen.getByTestId('logs-mode')
    expect(mode).toHaveTextContent('Following')

    // Turning follow off switches the corner to "Snapshot".
    await user.click(screen.getByTestId('logs-follow-toggle'))
    expect(screen.getByTestId('logs-mode')).toHaveTextContent('Snapshot')
  })

  it('scrolls to the latest logs when the footer mode indicator is clicked', async () => {
    const user = userEvent.setup()
    render(<WorkloadLogsViewer {...defaultProps} />)
    await waitFor(() => expect(screen.getByTestId('logs-content')).toBeInTheDocument())

    // jsdom has no layout, so stub the scroll metrics to verify the handler pins
    // the body to the bottom (scrollTop := scrollHeight).
    const body = screen.getByTestId('logs-body')
    let scrollTop = 0
    Object.defineProperty(body, 'scrollHeight', { configurable: true, value: 4321 })
    Object.defineProperty(body, 'scrollTop', { configurable: true, get: () => scrollTop, set: v => { scrollTop = v } })

    await user.click(screen.getByTestId('logs-mode'))
    expect(scrollTop).toBe(4321)
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

  it('appends new lines on follow polls via sinceTime, deduping the overlap', async () => {
    vi.useFakeTimers()
    try {
      const flush = async () => { await act(async () => { await Promise.resolve(); await Promise.resolve() }) }

      // Initial load returns two timestamped lines.
      fetchWithMock.mockResolvedValueOnce({ logs: '2026-01-01T00:00:00Z line one\n2026-01-01T00:00:01Z line two\n' })
      // The follow poll re-sends the last line (sinceTime is second-granular) plus a new one.
      fetchWithMock.mockResolvedValue({ logs: '2026-01-01T00:00:01Z line two\n2026-01-01T00:00:02Z line three\n' })

      render(<WorkloadLogsViewer {...defaultProps} />)
      await flush()
      expect(screen.getAllByTestId('logs-line')).toHaveLength(2)

      // The initial load asks for the tail, not a sinceTime window.
      expect(fetchWithMock.mock.calls[0][0].endpoint).toContain('tailLines=100')
      expect(fetchWithMock.mock.calls[0][0].endpoint).not.toContain('sinceTime')

      // Advance to the first follow poll and let it resolve.
      await act(async () => { vi.advanceTimersByTime(5000) })
      await flush()

      // The overlapping "line two" is deduped; only "line three" is appended.
      const rows = screen.getAllByTestId('logs-line')
      expect(rows.map(r => r.textContent)).toEqual(['line one', 'line two', 'line three'])

      // The poll narrows to entries newer than the last seen line, and still
      // sends tailLines so a catch-up stays bounded to the user's selection.
      const pollCall = fetchWithMock.mock.calls[fetchWithMock.mock.calls.length - 1][0]
      expect(pollCall.endpoint).toContain('sinceTime=2026-01-01T00%3A00%3A01Z')
      expect(pollCall.endpoint).toContain('tailLines=100')
    } finally {
      vi.useRealTimers()
    }
  })

  it('skips follow polls while a reset fetch is still in flight', async () => {
    vi.useFakeTimers()
    try {
      const flush = async () => { await act(async () => { await Promise.resolve(); await Promise.resolve() }) }

      fetchWithMock.mockResolvedValueOnce({ logs: '2026-01-01T00:00:00Z line one\n2026-01-01T00:00:01Z line two\n' })
      render(<WorkloadLogsViewer {...defaultProps} />)
      await flush()
      expect(fetchWithMock).toHaveBeenCalledTimes(1)

      // Change the line count: this starts a reset fetch that never resolves,
      // so the viewer stays in the "resetting" state.
      fetchWithMock.mockReturnValueOnce(new Promise(() => {}))
      await act(async () => { fireEvent.change(screen.getByTestId('logs-lines-select'), { target: { value: '500' } }) })
      await flush()
      expect(fetchWithMock).toHaveBeenCalledTimes(2)

      // A follow tick while the reset is in flight must NOT fire an append poll.
      await act(async () => { vi.advanceTimersByTime(5000) })
      await flush()
      expect(fetchWithMock).toHaveBeenCalledTimes(2)
    } finally {
      vi.useRealTimers()
    }
  })

  it('drops a stale in-flight append poll after a parameter reset', async () => {
    vi.useFakeTimers()
    try {
      const flush = async () => { await act(async () => { await Promise.resolve(); await Promise.resolve() }) }

      // Initial load for the first stream.
      fetchWithMock.mockResolvedValueOnce({ logs: '2026-01-01T00:00:00Z app one\n2026-01-01T00:00:01Z app two\n' })
      render(<WorkloadLogsViewer {...defaultProps} />)
      await flush()
      expect(screen.getAllByTestId('logs-line').map(r => r.textContent)).toEqual(['app one', 'app two'])

      // The next follow poll is left in flight (its promise never resolves yet).
      let resolveAppend
      fetchWithMock.mockReturnValueOnce(new Promise((resolve) => { resolveAppend = resolve }))
      await act(async () => { vi.advanceTimersByTime(5000) })

      // Meanwhile the user changes the line count, which resets the buffer.
      fetchWithMock.mockResolvedValueOnce({ logs: '2026-01-01T00:00:05Z reset one\n' })
      await act(async () => { fireEvent.change(screen.getByTestId('logs-lines-select'), { target: { value: '500' } }) })
      await flush()
      expect(screen.getAllByTestId('logs-line').map(r => r.textContent)).toEqual(['reset one'])

      // The stale append (from before the reset) now resolves; it must be ignored.
      await act(async () => { resolveAppend({ logs: '2026-01-01T00:00:02Z app three\n' }) })
      await flush()
      expect(screen.getAllByTestId('logs-line').map(r => r.textContent)).toEqual(['reset one'])
    } finally {
      vi.useRealTimers()
    }
  })

  it('shows a failed follow poll inline at the tail without clearing the buffer', async () => {
    vi.useFakeTimers()
    try {
      const flush = async () => { await act(async () => { await Promise.resolve(); await Promise.resolve() }) }

      fetchWithMock.mockResolvedValueOnce({ logs: '2026-01-01T00:00:00Z line one\n2026-01-01T00:00:01Z line two\n' })
      render(<WorkloadLogsViewer {...defaultProps} />)
      await flush()
      expect(screen.getAllByTestId('logs-line')).toHaveLength(2)

      // The follow poll fails.
      fetchWithMock.mockRejectedValueOnce(new Error('Pod default/my-pod not found'))
      await act(async () => { vi.advanceTimersByTime(5000) })
      await flush()

      // The buffer stays on screen and the error is shown inline at the tail,
      // not as the full-pane error banner.
      expect(screen.getAllByTestId('logs-line')).toHaveLength(2)
      expect(screen.getByTestId('logs-follow-error')).toHaveTextContent('Pod default/my-pod not found')
      expect(screen.queryByTestId('logs-error')).not.toBeInTheDocument()

      // The next poll succeeds: the inline error clears and the new line appends.
      fetchWithMock.mockResolvedValueOnce({ logs: '2026-01-01T00:00:02Z line three\n' })
      await act(async () => { vi.advanceTimersByTime(5000) })
      await flush()
      expect(screen.queryByTestId('logs-follow-error')).not.toBeInTheDocument()
      expect(screen.getAllByTestId('logs-line').map(r => r.textContent)).toEqual(['line one', 'line two', 'line three'])
    } finally {
      vi.useRealTimers()
    }
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

  it('does not highlight the last visible line when new logs are filtered out', async () => {
    const user = userEvent.setup()
    fetchWithMock.mockResolvedValue({ logs: '2026-01-01T00:00:00Z keep me\n2026-01-01T00:00:01Z keep me too\n' })
    render(<WorkloadLogsViewer {...defaultProps} />)
    await waitFor(() => expect(screen.getAllByTestId('logs-line')).toHaveLength(2))

    // Keep only the lines containing "keep"; both current lines match.
    await user.type(screen.getByTestId('logs-filter-input'), 'keep')
    await waitFor(() => expect(screen.getAllByTestId('logs-line')).toHaveLength(2))

    // A new entry arrives that does not match the filter, so the visible set is
    // unchanged. The unchanged last line must not be highlighted as if it were new.
    fetchWithMock.mockResolvedValue({ logs: '2026-01-01T00:00:00Z keep me\n2026-01-01T00:00:01Z keep me too\n2026-01-01T00:00:02Z noise\n' })
    await user.selectOptions(screen.getByTestId('logs-lines-select'), '500')
    await waitFor(() => {
      const call = fetchWithMock.mock.calls.at(-1)[0]
      expect(call.endpoint).toContain('tailLines=500')
    })
    expect(screen.getAllByTestId('logs-line')).toHaveLength(2)
    expect(screen.getAllByTestId('logs-timestamp').some(el => el.getAttribute('data-latest') === 'true')).toBe(false)
  })
})
