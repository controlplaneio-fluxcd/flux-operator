// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor, act, fireEvent } from '@testing-library/preact'
import userEvent from '@testing-library/user-event'
import { WorkloadLogsViewer, reconcileEntries } from './WorkloadLogsViewer'
import { logSettings, DEFAULT_LOG_SETTINGS, resetLogSettings } from '../../../utils/logSettings'

vi.mock('../../../utils/fetch', () => ({
  fetchWithMock: vi.fn()
}))

import { fetchWithMock } from '../../../utils/fetch'

// containersAll returns the repeated `container` query params of an endpoint.
const containersOf = (endpoint) => [...new URLSearchParams(endpoint.split('?')[1]).getAll('container')]
const podsOf = (endpoint) => [...new URLSearchParams(endpoint.split('?')[1]).getAll('pod')]
const lastCall = () => fetchWithMock.mock.calls.at(-1)[0]

describe('WorkloadLogsViewer component', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    // Reset the persisted log viewer settings (a module-level signal) so a case
    // that toggles follow/format/lines can't leak its choice into the next.
    logSettings.value = { ...DEFAULT_LOG_SETTINGS }
    // Each line carries a leading timestamp (as the API returns); timestamps are
    // shown as a pill so the viewer strips it from the message text.
    fetchWithMock.mockResolvedValue({ pod: 'my-pod', container: 'app', logs: '2026-01-01T00:00:00Z line one\n2026-01-01T00:00:01Z line two\n' })
  })

  afterEach(() => {
    // Restore real timers so a leaked fake-timer interval can't bleed into the next test.
    vi.useRealTimers()
  })

  // A single-pod workload pre-selected on that pod: the common single-stream case.
  const onePod = (containers = [{ name: 'app', isInit: false }]) => [{ name: 'my-pod', status: 'Running', containers }]
  const defaultProps = {
    kind: 'Deployment',
    namespace: 'default',
    workloadName: 'podinfo',
    pods: onePod(),
    initialPodName: 'my-pod',
    onClose: vi.fn()
  }

  // A two-pod workload (pod ids gqh2x / p8x2k), each with one regular container.
  const twoPods = [
    { name: 'web-abc-gqh2x', status: 'Running', containers: [{ name: 'app', isInit: false }] },
    { name: 'web-abc-p8x2k', status: 'Running', containers: [{ name: 'app', isInit: false }] }
  ]
  const allPodsProps = { ...defaultProps, pods: twoPods, initialPodName: undefined }

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
    expect(select).toHaveValue('all')
    const values = [...select.querySelectorAll('option')].map(o => o.value)
    expect(values).toEqual(['all', 'app::false'])
  })

  it('reports the selected pod via onPodChange on mount and on switch', async () => {
    const user = userEvent.setup()
    const onPodChange = vi.fn()
    render(<WorkloadLogsViewer {...allPodsProps} onPodChange={onPodChange} />)

    // The initial selection (All pods → null) is reported on mount so the parent
    // can put it in the URL.
    await waitFor(() => expect(onPodChange).toHaveBeenCalledWith(null))

    // Switching to a specific pod reports the pod name; switching back reports null.
    const select = await screen.findByTestId('logs-pod-select')
    await user.selectOptions(select, 'web-abc-gqh2x')
    await waitFor(() => expect(onPodChange).toHaveBeenLastCalledWith('web-abc-gqh2x'))
    await user.selectOptions(select, '__all_pods__')
    await waitFor(() => expect(onPodChange).toHaveBeenLastCalledWith(null))
  })

  it('shows the container selector for multiple containers and refetches on change', async () => {
    const user = userEvent.setup()
    const props = { ...defaultProps, pods: onePod([{ name: 'app', isInit: false }, { name: 'sidecar', isInit: false }]) }
    render(<WorkloadLogsViewer {...props} />)

    const select = await screen.findByTestId('logs-container-select')
    await user.selectOptions(select, 'sidecar::false')
    await waitFor(() => expect(lastCall().endpoint).toContain('container=sidecar'))
  })

  it('defaults to "All containers" for multiple regular containers and streams them all', async () => {
    const props = { ...defaultProps, pods: onePod([{ name: 'app', isInit: false }, { name: 'sidecar', isInit: false }]) }
    render(<WorkloadLogsViewer {...props} />)
    await waitFor(() => expect(screen.getByTestId('logs-content')).toBeInTheDocument())

    expect(screen.getByTestId('logs-container-select')).toHaveValue('all')
    const endpoint = fetchWithMock.mock.calls[0][0].endpoint
    expect(containersOf(endpoint)).toEqual(['app', 'sidecar'])
    expect(endpoint).toContain('previous=false')
    // One pod: no repeated pod params.
    expect(podsOf(endpoint)).toEqual([])
  })

  it('includes init containers in "All containers" and keeps them selectable', async () => {
    const user = userEvent.setup()
    const props = {
      ...defaultProps,
      pods: onePod([
        { name: 'setup', isInit: true },
        { name: 'app', isInit: false },
        { name: 'sidecar', isInit: false }
      ])
    }
    render(<WorkloadLogsViewer {...props} />)
    await waitFor(() => expect(screen.getByTestId('logs-content')).toBeInTheDocument())

    // "All containers" now streams every container, init included, sorted.
    expect(containersOf(fetchWithMock.mock.calls[0][0].endpoint)).toEqual(['app', 'setup', 'sidecar'])

    const select = screen.getByTestId('logs-container-select')
    const values = [...select.querySelectorAll('option')].map(o => o.value)
    expect(values).toContain('setup::false')

    await user.selectOptions(select, 'setup::false')
    await waitFor(() => expect(containersOf(lastCall().endpoint)).toEqual(['setup']))
  })

  it('switches from "All containers" to a single container', async () => {
    const user = userEvent.setup()
    const props = { ...defaultProps, pods: onePod([{ name: 'app', isInit: false }, { name: 'sidecar', isInit: false }]) }
    render(<WorkloadLogsViewer {...props} />)
    await waitFor(() => expect(screen.getByTestId('logs-content')).toBeInTheDocument())

    await user.selectOptions(screen.getByTestId('logs-container-select'), 'sidecar::false')
    await waitFor(() => expect(containersOf(lastCall().endpoint)).toEqual(['sidecar']))
  })

  it('defaults to "All containers" and streams every container including init', async () => {
    const props = { ...defaultProps, pods: onePod([{ name: 'setup', isInit: true }, { name: 'app', isInit: false }]) }
    render(<WorkloadLogsViewer {...props} />)
    await waitFor(() => expect(screen.getByTestId('logs-content')).toBeInTheDocument())

    expect(screen.getByTestId('logs-container-select')).toHaveValue('all')
    expect(containersOf(fetchWithMock.mock.calls[0][0].endpoint)).toEqual(['app', 'setup'])
  })

  it('lists init containers in the container dropdown for "All pods"', async () => {
    const pods = [
      { name: 'web-abc-gqh2x', status: 'Running', containers: [{ name: 'setup', isInit: true }, { name: 'app', isInit: false }] },
      { name: 'web-abc-p8x2k', status: 'Running', containers: [{ name: 'setup', isInit: true }, { name: 'app', isInit: false }] }
    ]
    render(<WorkloadLogsViewer {...allPodsProps} pods={pods} />)
    await waitFor(() => expect(screen.getByTestId('logs-content')).toBeInTheDocument())

    // "All containers" streams every container across pods (init included), sorted.
    expect(containersOf(fetchWithMock.mock.calls[0][0].endpoint)).toEqual(['app', 'setup'])

    // The init container is now offered in the dropdown, labelled "init:".
    const select = screen.getByTestId('logs-container-select')
    const labels = [...select.querySelectorAll('option')].map(o => o.textContent)
    expect(labels).toContain('init:setup')
  })

  it('titles the modal "Log Viewer" over the workload kind/namespace/name', async () => {
    render(<WorkloadLogsViewer {...defaultProps} />)
    await waitFor(() => expect(screen.getByTestId('logs-content')).toBeInTheDocument())

    expect(screen.getByTestId('logs-viewer')).toHaveTextContent('Log Viewer')
    expect(screen.getByTestId('logs-title')).toHaveTextContent('Deployment/default/podinfo')
  })

  it('defaults to "All pods" and streams every pod (sorted) when no pod is pre-selected', async () => {
    render(<WorkloadLogsViewer {...allPodsProps} />)
    await waitFor(() => expect(screen.getByTestId('logs-content')).toBeInTheDocument())

    const podSelect = screen.getByTestId('logs-pod-select')
    expect(podSelect).toHaveValue('__all_pods__')
    const options = [...podSelect.querySelectorAll('option')].map(o => o.value)
    expect(options).toEqual(['__all_pods__', 'web-abc-gqh2x', 'web-abc-p8x2k'])

    // The request streams the first pod as `name` and the rest as repeated `pod`.
    const endpoint = fetchWithMock.mock.calls[0][0].endpoint
    expect(endpoint).toContain('name=web-abc-gqh2x')
    expect(podsOf(endpoint)).toEqual(['web-abc-p8x2k'])
    expect(containersOf(endpoint)).toEqual(['app'])
  })

  it('pre-selects the pod passed as initialPodName', async () => {
    render(<WorkloadLogsViewer {...allPodsProps} initialPodName="web-abc-p8x2k" />)
    await waitFor(() => expect(screen.getByTestId('logs-content')).toBeInTheDocument())

    expect(screen.getByTestId('logs-pod-select')).toHaveValue('web-abc-p8x2k')
    const endpoint = fetchWithMock.mock.calls[0][0].endpoint
    expect(endpoint).toContain('name=web-abc-p8x2k')
    expect(podsOf(endpoint)).toEqual([])
  })

  it('narrows to a single pod and resets the container selection on pod change', async () => {
    const user = userEvent.setup()
    const props = {
      ...allPodsProps,
      pods: [
        { name: 'web-abc-gqh2x', containers: [{ name: 'app', isInit: false }, { name: 'sidecar', isInit: false }] },
        { name: 'web-abc-p8x2k', containers: [{ name: 'app', isInit: false }, { name: 'sidecar', isInit: false }] }
      ]
    }
    render(<WorkloadLogsViewer {...props} />)
    await waitFor(() => expect(screen.getByTestId('logs-content')).toBeInTheDocument())

    // Pick a specific container while on "All pods".
    await user.selectOptions(screen.getByTestId('logs-container-select'), 'sidecar::false')
    expect(screen.getByTestId('logs-container-select')).toHaveValue('sidecar::false')

    // Switching to a specific pod resets the container dropdown to "All containers"
    // and narrows the request to that pod.
    await user.selectOptions(screen.getByTestId('logs-pod-select'), 'web-abc-p8x2k')
    await waitFor(() => expect(screen.getByTestId('logs-container-select')).toHaveValue('all'))
    const endpoint = lastCall().endpoint
    expect(endpoint).toContain('name=web-abc-p8x2k')
    expect(podsOf(endpoint)).toEqual([])
  })

  it('tags each row with its pod id in the "All pods" view', async () => {
    fetchWithMock.mockResolvedValue({
      pod: 'web-abc-gqh2x,web-abc-p8x2k',
      container: 'app',
      tagged: true,
      total: 2,
      streamed: 2,
      logs: 'web-abc-gqh2x 2026-01-01T00:00:00Z first\nweb-abc-p8x2k 2026-01-01T00:00:01Z second\n'
    })
    render(<WorkloadLogsViewer {...allPodsProps} />)
    await waitFor(() => expect(screen.getAllByTestId('logs-line')).toHaveLength(2))

    const pills = screen.getAllByTestId('logs-pod-id').map(el => el.textContent)
    expect(pills).toEqual(['gqh2x · ', 'p8x2k · '])
    // The pod tag is stripped from the message text.
    const rows = screen.getAllByTestId('logs-line').map(r => r.textContent)
    expect(rows).toEqual(['first', 'second'])
  })

  it('does not mistake a message starting with a non-pod word + timestamp for a tag', async () => {
    fetchWithMock.mockResolvedValue({
      pod: 'web-abc-gqh2x,web-abc-p8x2k',
      container: 'app',
      tagged: true,
      total: 2,
      streamed: 2,
      // Second line's first token "retry" is not one of the requested pods, so it
      // must render verbatim, not be parsed as a pod tag.
      logs: 'web-abc-gqh2x 2026-01-01T00:00:00Z ok\nretry 2026-01-01T00:00:05Z failed\n'
    })
    render(<WorkloadLogsViewer {...allPodsProps} />)
    await waitFor(() => expect(screen.getAllByTestId('logs-line')).toHaveLength(2))

    // Only the genuinely tagged line has a pod id.
    expect(screen.getAllByTestId('logs-pod-id')).toHaveLength(1)
    const rows = screen.getAllByTestId('logs-line').map(r => r.textContent)
    expect(rows[1]).toBe('retry 2026-01-01T00:00:05Z failed')
  })

  it('orders the merged buffer chronologically when a lagging pod sends older lines', async () => {
    // A lagging pod makes the append order non-chronological; the viewer must
    // reorder by timestamp, keeping each entry's continuation lines attached.
    fetchWithMock.mockResolvedValue({
      pod: 'web-abc-gqh2x,web-abc-p8x2k',
      container: 'app',
      tagged: true,
      total: 2,
      streamed: 2,
      logs: 'web-abc-p8x2k 2026-01-01T00:00:03Z newest\n' +
            'web-abc-gqh2x 2026-01-01T00:00:01Z oldest\n' +
            '  continued\n' +
            'web-abc-gqh2x 2026-01-01T00:00:02Z middle\n'
    })
    render(<WorkloadLogsViewer {...allPodsProps} />)
    await waitFor(() => expect(screen.getAllByTestId('logs-line')).toHaveLength(4))

    const rows = screen.getAllByTestId('logs-line').map(r => r.textContent)
    // Sorted by timestamp; the continuation line stays directly under "oldest".
    expect(rows).toEqual(['oldest', '  continued', 'middle', 'newest'])
  })

  it('does not treat a date-shaped but invalid token as a tagged timestamp', async () => {
    fetchWithMock.mockResolvedValue({
      pod: 'web-abc-gqh2x,web-abc-p8x2k',
      container: 'app',
      tagged: true,
      total: 2,
      streamed: 2,
      // Second line's first token is a requested pod, but the next token is
      // date-shaped yet not a real instant, so it must render verbatim.
      logs: 'web-abc-gqh2x 2026-01-01T00:00:00Z ok\nweb-abc-p8x2k 2026-13-45T99:99:99Z nope\n'
    })
    render(<WorkloadLogsViewer {...allPodsProps} />)
    await waitFor(() => expect(screen.getAllByTestId('logs-line')).toHaveLength(2))

    expect(screen.getAllByTestId('logs-pod-id')).toHaveLength(1)
    const rows = screen.getAllByTestId('logs-line').map(r => r.textContent)
    expect(rows[1]).toBe('web-abc-p8x2k 2026-13-45T99:99:99Z nope')
  })

  it('shows a "showing N of M pods" note when the response is partial', async () => {
    fetchWithMock.mockResolvedValue({
      pod: 'web-abc-gqh2x,web-abc-p8x2k',
      container: 'app',
      tagged: true,
      total: 2,
      streamed: 1,
      partial: true,
      forbidden: 1,
      logs: 'web-abc-gqh2x 2026-01-01T00:00:00Z only one\n'
    })
    render(<WorkloadLogsViewer {...allPodsProps} />)
    await waitFor(() => expect(screen.getByTestId('logs-content')).toBeInTheDocument())

    expect(screen.getByTestId('logs-partial')).toHaveTextContent('showing 1 of 2 pods')
  })

  it('falls back to "All pods" when the pre-selected pod disappears', async () => {
    const { rerender } = render(<WorkloadLogsViewer {...allPodsProps} initialPodName="web-abc-p8x2k" />)
    await waitFor(() => expect(screen.getByTestId('logs-pod-select')).toHaveValue('web-abc-p8x2k'))

    // The pod is removed from the live list; the selection falls back to All pods.
    rerender(<WorkloadLogsViewer {...allPodsProps} initialPodName="web-abc-p8x2k" pods={[twoPods[0]]} />)
    await waitFor(() => expect(screen.getByTestId('logs-pod-select')).toHaveValue('__all_pods__'))

    // A pod reappearing with the same name must not silently snap the view back
    // to it; the fallback to All pods is committed, not just rendered.
    rerender(<WorkloadLogsViewer {...allPodsProps} initialPodName="web-abc-p8x2k" pods={twoPods} />)
    await waitFor(() => expect(screen.getByTestId('logs-pod-select')).toHaveValue('__all_pods__'))
  })

  it('appends per-pod via repeated since cursors while following "All pods"', async () => {
    vi.useFakeTimers()
    try {
      const flush = async () => { await act(async () => { await Promise.resolve(); await Promise.resolve() }) }
      fetchWithMock.mockResolvedValueOnce({
        pod: 'web-abc-gqh2x,web-abc-p8x2k', container: 'app', tagged: true, total: 2, streamed: 2,
        logs: 'web-abc-gqh2x 2026-01-01T00:00:00Z a one\nweb-abc-p8x2k 2026-01-01T00:00:01Z b one\n'
      })
      fetchWithMock.mockResolvedValue({
        pod: 'web-abc-gqh2x,web-abc-p8x2k', container: 'app', tagged: true, total: 2, streamed: 2,
        // Overlap (b one re-sent) plus a new line.
        logs: 'web-abc-p8x2k 2026-01-01T00:00:01Z b one\nweb-abc-gqh2x 2026-01-01T00:00:02Z a two\n'
      })

      render(<WorkloadLogsViewer {...allPodsProps} />)
      await flush()
      expect(screen.getAllByTestId('logs-line').map(r => r.textContent)).toEqual(['a one', 'b one'])
      expect(fetchWithMock.mock.calls[0][0].endpoint).not.toContain('since=')

      await act(async () => { vi.advanceTimersByTime(5000) })
      await flush()

      // The overlap is deduped; only the new line appends.
      expect(screen.getAllByTestId('logs-line').map(r => r.textContent)).toEqual(['a one', 'b one', 'a two'])

      // The follow poll carries a per-pod `since` cursor for each pod.
      const sinces = [...new URLSearchParams(lastCall().endpoint.split('?')[1]).getAll('since')]
      expect(sinces).toContain('web-abc-gqh2x=2026-01-01T00:00:00Z')
      expect(sinces).toContain('web-abc-p8x2k=2026-01-01T00:00:01Z')
    } finally {
      vi.useRealTimers()
    }
  })

  it('keeps a new entry\'s stack frames on append when an identical frame is already buffered', async () => {
    vi.useFakeTimers()
    try {
      const flush = async () => { await act(async () => { await Promise.resolve(); await Promise.resolve() }) }
      // First poll: pod gqh2x panics with a two-line stack (continuation lines are
      // untagged, as the backend tags only timestamped lines).
      fetchWithMock.mockResolvedValueOnce({
        pod: 'web-abc-gqh2x,web-abc-p8x2k', container: 'app', tagged: true, total: 2, streamed: 2,
        logs: 'web-abc-gqh2x 2026-01-01T00:00:00Z panic: boom\n  goroutine 1 [running]:\n  main.crash()\n'
      })
      // Second poll: pod p8x2k panics later with byte-identical stack frames.
      fetchWithMock.mockResolvedValue({
        pod: 'web-abc-gqh2x,web-abc-p8x2k', container: 'app', tagged: true, total: 2, streamed: 2,
        logs: 'web-abc-p8x2k 2026-01-01T00:00:05Z panic: boom\n  goroutine 1 [running]:\n  main.crash()\n'
      })

      render(<WorkloadLogsViewer {...allPodsProps} />)
      await flush()
      // Switch to raw mode so the frames render as flat rows; this test asserts
      // mergeLogs keeps both pods' frames, independent of the fold UI.
      await act(async () => { fireEvent.click(screen.getByTestId('logs-format-toggle')) })
      await act(async () => { vi.advanceTimersByTime(5000) })
      await flush()

      // Both panics keep their full stack: the identical frames appear once per
      // pod, not collapsed by text dedup (which truncated the new trace before).
      const rows = screen.getAllByTestId('logs-line').map(r => r.textContent)
      expect(rows.filter(t => t === '  goroutine 1 [running]:')).toHaveLength(2)
      expect(rows.filter(t => t === '  main.crash()')).toHaveLength(2)
    } finally {
      vi.useRealTimers()
    }
  })

  it('resets the buffer when the pod set changes but not when it is merely reordered', async () => {
    const { rerender } = render(<WorkloadLogsViewer {...allPodsProps} />)
    await waitFor(() => expect(screen.getByTestId('logs-content')).toBeInTheDocument())
    const callsAfterMount = fetchWithMock.mock.calls.length

    // Reorder the same pods: sorted names are unchanged, so no refetch.
    rerender(<WorkloadLogsViewer {...allPodsProps} pods={[twoPods[1], twoPods[0]]} />)
    await new Promise(r => setTimeout(r, 0))
    expect(fetchWithMock.mock.calls.length).toBe(callsAfterMount)

    // Add a pod: the set changed, so the buffer resets with a fresh fetch.
    const threePods = [...twoPods, { name: 'web-abc-zzz99', containers: [{ name: 'app', isInit: false }] }]
    rerender(<WorkloadLogsViewer {...allPodsProps} pods={threePods} />)
    await waitFor(() => expect(fetchWithMock.mock.calls.length).toBeGreaterThan(callsAfterMount))
    expect(podsOf(lastCall().endpoint)).toEqual(['web-abc-p8x2k', 'web-abc-zzz99'])
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
    await waitFor(() => expect(screen.getAllByTestId('logs-line')).toHaveLength(2))
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
    await waitFor(() => expect(lastCall().endpoint).toContain('tailLines=500'))
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

    expect(screen.getByTestId('logs-timestamp')).toHaveTextContent('2026-06-16T00:00:00Z')
    expect(screen.getByTestId('logs-line')).toHaveTextContent('hello world')
    expect(screen.getByTestId('logs-line')).not.toHaveTextContent('2026-06-16T00:00:00Z')
    expect(screen.queryByTestId('logs-timestamps-toggle')).not.toBeInTheDocument()
  })

  it('offers a "(previous)" entry only for restarted containers and refetches with previous=true', async () => {
    const user = userEvent.setup()
    const props = {
      ...defaultProps,
      pods: onePod([
        { name: 'app', isInit: false, restartCount: 2 },
        { name: 'sidecar', isInit: false, restartCount: 0 }
      ])
    }
    render(<WorkloadLogsViewer {...props} />)
    await waitFor(() => expect(fetchWithMock).toHaveBeenCalled())
    expect(fetchWithMock.mock.calls[0][0].endpoint).toContain('previous=false')

    const select = screen.getByTestId('logs-container-select')
    const values = [...select.querySelectorAll('option')].map(o => o.value)
    expect(values).toContain('app::false')
    expect(values).toContain('app::true')
    expect(values).toContain('sidecar::false')
    expect(values).not.toContain('sidecar::true')

    await user.selectOptions(select, 'app::true')
    await waitFor(() => {
      const endpoint = lastCall().endpoint
      expect(endpoint).toContain('container=app')
      expect(endpoint).toContain('previous=true')
    })
  })

  it('downloads the logs as a <pod>.log file for a single pod', async () => {
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
    expect(downloadName).toBe('my-pod.log')
    clickSpy.mockRestore()
  })

  it('downloads as <workload>.log in the "All pods" view', async () => {
    const user = userEvent.setup()
    fetchWithMock.mockResolvedValue({ pod: 'a,b', container: 'app', tagged: true, total: 2, streamed: 2, logs: 'web-abc-gqh2x 2026-01-01T00:00:00Z x\n' })

    let downloadName
    window.URL.createObjectURL = vi.fn(() => 'blob:mock')
    window.URL.revokeObjectURL = vi.fn()
    const clickSpy = vi.spyOn(window.HTMLAnchorElement.prototype, 'click').mockImplementation(function () {
      downloadName = this.download
    })

    render(<WorkloadLogsViewer {...allPodsProps} />)
    await waitFor(() => expect(screen.getByTestId('logs-content')).toBeInTheDocument())

    await user.click(screen.getByTestId('logs-download-button'))
    expect(downloadName).toBe('podinfo.log')
    clickSpy.mockRestore()
  })

  it('shows a per-level count summary in the footer, with a loader while fetching', async () => {
    let resolveFetch
    fetchWithMock.mockReturnValue(new Promise((resolve) => { resolveFetch = resolve }))
    render(<WorkloadLogsViewer {...defaultProps} />)

    expect(screen.getByTestId('logs-loader')).toBeInTheDocument()

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

    await user.click(screen.getByTestId('logs-level-filter'))
    await user.click(screen.getByTestId('logs-level-option-warn'))

    await waitFor(() => expect(screen.getAllByTestId('logs-line')).toHaveLength(1))
    expect(screen.getByTestId('logs-line')).toHaveTextContent('slow')

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

    const toggle = screen.getByTestId('logs-format-toggle')
    expect(toggle).toHaveAttribute('aria-pressed', 'true')

    const rows = screen.getAllByTestId('logs-line')
    expect(rows[0].tagName).toBe('PRE')
    // JSON is highlighted by our own span serializer (no Prism): keys carry the gray
    // key class and the indented structure is preserved byte-for-byte.
    const keySpan = [...rows[0].querySelectorAll('span')].find(s => s.textContent === '"level"')
    expect(keySpan).toBeTruthy()
    expect(keySpan).toHaveClass('text-gray-500')
    expect(rows[0].textContent).toBe('"level": "info",\n"msg": "hello"')
    // A line matching no formatter renders as a wrapping div, not the JSON <pre>.
    expect(rows[1].tagName).toBe('DIV')
    expect(rows[1]).toHaveClass('whitespace-pre-wrap')
    expect(rows[1].textContent).toBe('plain text line')
  })

  it('strips all styling in raw mode: plain rows, no timestamp pills or highlight', async () => {
    const user = userEvent.setup()
    fetchWithMock.mockResolvedValue({
      logs: '2026-01-01T00:00:00Z {"level":"info","msg":"hello"}\n2026-01-01T00:00:01Z plain text line\n'
    })
    render(<WorkloadLogsViewer {...defaultProps} />)
    await waitFor(() => expect(screen.getAllByTestId('logs-line')).toHaveLength(2))

    const toggle = screen.getByTestId('logs-format-toggle')
    await user.click(toggle)
    expect(toggle).toHaveAttribute('aria-pressed', 'false')

    const rows = screen.getAllByTestId('logs-line')
    expect(rows[0].tagName).toBe('DIV')
    expect(rows[0].textContent).toBe('{"level":"info","msg":"hello"}')
    expect(rows[1].textContent).toBe('plain text line')
    expect(screen.queryByTestId('logs-timestamp')).not.toBeInTheDocument()
  })

  it('hides the level filter and shows a line count in raw mode, resetting an active filter', async () => {
    const user = userEvent.setup()
    fetchWithMock.mockResolvedValue({
      logs: '2026-01-01T00:00:00Z {"level":"error","msg":"boom"}\n'
        + '2026-01-01T00:00:01Z {"level":"warn","msg":"slow"}\n'
        + '2026-01-01T00:00:02Z {"level":"info","msg":"hi"}\n'
    })
    render(<WorkloadLogsViewer {...defaultProps} />)
    await waitFor(() => expect(screen.getAllByTestId('logs-line')).toHaveLength(3))

    // Formatted mode: the level filter and per-level legend are present. Narrow to warn.
    expect(screen.getByTestId('logs-level-filter')).toBeInTheDocument()
    expect(screen.getByTestId('logs-level-summary')).toBeInTheDocument()
    await user.click(screen.getByTestId('logs-level-filter'))
    await user.click(screen.getByTestId('logs-level-option-warn'))
    await waitFor(() => expect(screen.getAllByTestId('logs-line')).toHaveLength(1))

    // Raw mode: the filter is hidden and reset to "all" (all 3 lines return), and the
    // per-level legend is replaced by a plain line count.
    await user.click(screen.getByTestId('logs-format-toggle'))
    expect(screen.queryByTestId('logs-level-filter')).not.toBeInTheDocument()
    expect(screen.queryByTestId('logs-level-summary')).not.toBeInTheDocument()
    await waitFor(() => expect(screen.getAllByTestId('logs-line')).toHaveLength(3))
    expect(screen.getByTestId('logs-line-count')).toHaveTextContent('3 log lines')

    // Back to formatted: the filter returns and stays "all" (the warn filter did not
    // survive the round-trip), so all 3 lines remain visible.
    await user.click(screen.getByTestId('logs-format-toggle'))
    expect(screen.getByTestId('logs-level-filter')).toBeInTheDocument()
    expect(screen.getAllByTestId('logs-line')).toHaveLength(3)
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

    expect(screen.getByTestId('logs-mode')).toHaveTextContent('Following')
    await user.click(screen.getByTestId('logs-follow-toggle'))
    expect(screen.getByTestId('logs-mode')).toHaveTextContent('Snapshot')
  })

  it('scrolls to the latest logs when the footer mode indicator is clicked', async () => {
    const user = userEvent.setup()
    render(<WorkloadLogsViewer {...defaultProps} />)
    await waitFor(() => expect(screen.getByTestId('logs-content')).toBeInTheDocument())

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

      fetchWithMock.mockResolvedValueOnce({ logs: '2026-01-01T00:00:00Z line one\n2026-01-01T00:00:01Z line two\n' })
      fetchWithMock.mockResolvedValue({ logs: '2026-01-01T00:00:01Z line two\n2026-01-01T00:00:02Z line three\n' })

      render(<WorkloadLogsViewer {...defaultProps} />)
      await flush()
      expect(screen.getAllByTestId('logs-line')).toHaveLength(2)

      expect(fetchWithMock.mock.calls[0][0].endpoint).toContain('tailLines=100')
      expect(fetchWithMock.mock.calls[0][0].endpoint).not.toContain('sinceTime')

      await act(async () => { vi.advanceTimersByTime(5000) })
      await flush()

      const rows = screen.getAllByTestId('logs-line')
      expect(rows.map(r => r.textContent)).toEqual(['line one', 'line two', 'line three'])

      const pollCall = lastCall()
      expect(pollCall.endpoint).toContain('sinceTime=2026-01-01T00%3A00%3A01Z')
      expect(pollCall.endpoint).toContain('tailLines=100')
    } finally {
      vi.useRealTimers()
    }
  })

  // Part B: parsing is incremental — a follow poll re-parses only the appended
  // lines and reuses the cached entries for the rest. These guard that the
  // incrementally-built buffer is identical to a from-scratch parse across several
  // polls, that the all-pods sort still runs over it, and that front-eviction at
  // MAX_BUFFER_LINES keeps the parsed mirror in lock-step with the string buffer.
  it('matches a from-scratch parse across several incremental follow polls', async () => {
    vi.useFakeTimers()
    try {
      const flush = async () => { await act(async () => { await Promise.resolve(); await Promise.resolve() }) }

      fetchWithMock.mockResolvedValueOnce({ logs: '2026-01-01T00:00:00Z one\n2026-01-01T00:00:01Z two\n' })
      // Each poll re-sends the last buffered line (the overlap) plus a new one.
      fetchWithMock.mockResolvedValueOnce({ logs: '2026-01-01T00:00:01Z two\n2026-01-01T00:00:02Z three\n' })
      fetchWithMock.mockResolvedValueOnce({ logs: '2026-01-01T00:00:02Z three\n2026-01-01T00:00:03Z four\n' })

      render(<WorkloadLogsViewer {...defaultProps} />)
      await flush()

      await act(async () => { vi.advanceTimersByTime(5000) })
      await flush()
      await act(async () => { vi.advanceTimersByTime(5000) })
      await flush()

      // The reused entries plus the freshly parsed tails reconstruct the exact
      // ordered buffer a single fetch of the concatenation would have produced.
      expect(screen.getAllByTestId('logs-line').map(r => r.textContent))
        .toEqual(['one', 'two', 'three', 'four'])
    } finally {
      vi.useRealTimers()
    }
  })

  it('sorts a lagging pod\'s older line into place after an incremental append', async () => {
    vi.useFakeTimers()
    try {
      const flush = async () => { await act(async () => { await Promise.resolve(); await Promise.resolve() }) }

      fetchWithMock.mockResolvedValueOnce({
        pod: 'web-abc-gqh2x,web-abc-p8x2k', container: 'app', tagged: true, total: 2, streamed: 2,
        logs: 'web-abc-gqh2x 2026-01-01T00:00:00Z a one\nweb-abc-p8x2k 2026-01-01T00:00:02Z b one\n'
      })
      // The lagging pod delivers a line older than b one only on the follow poll, so
      // the chronological re-sort must run over the incrementally-parsed buffer.
      fetchWithMock.mockResolvedValue({
        pod: 'web-abc-gqh2x,web-abc-p8x2k', container: 'app', tagged: true, total: 2, streamed: 2,
        logs: 'web-abc-gqh2x 2026-01-01T00:00:01Z a two\n'
      })

      render(<WorkloadLogsViewer {...allPodsProps} />)
      await flush()
      expect(screen.getAllByTestId('logs-line').map(r => r.textContent)).toEqual(['a one', 'b one'])

      await act(async () => { vi.advanceTimersByTime(5000) })
      await flush()

      // a two (ts 1) sorts between a one (ts 0) and b one (ts 2), not at the tail.
      expect(screen.getAllByTestId('logs-line').map(r => r.textContent)).toEqual(['a one', 'a two', 'b one'])
    } finally {
      vi.useRealTimers()
    }
  })

  it('keeps the parsed buffer in lock-step with the string buffer through front-eviction', async () => {
    vi.useFakeTimers()
    try {
      const flush = async () => { await act(async () => { await Promise.resolve(); await Promise.resolve() }) }

      // Fill the buffer to its MAX_BUFFER_LINES (5000) cap, then append two newer
      // lines so the merge front-evicts the two oldest. The eviction branch must
      // reuse the surviving entries and parse only the appended tail.
      const initial = Array.from({ length: 5000 }, (_, i) =>
        `2026-01-01T00:00:00.${String(i).padStart(4, '0')}Z line ${i}`).join('\n') + '\n'
      fetchWithMock.mockResolvedValueOnce({ logs: initial })
      fetchWithMock.mockResolvedValue({
        logs: '2026-01-01T00:00:00.4999Z line 4999\n2026-01-01T00:00:01Z line 5000\n2026-01-01T00:00:02Z line 5001\n'
      })

      render(<WorkloadLogsViewer {...defaultProps} />)
      await flush()
      expect(screen.getAllByTestId('logs-line')).toHaveLength(5000)

      await act(async () => { vi.advanceTimersByTime(5000) })
      await flush()

      // Still capped at 5000: the two oldest evicted, the two newest appended, and
      // every surviving line still parsed (timestamp stripped from the message).
      const rows = screen.getAllByTestId('logs-line')
      expect(rows).toHaveLength(5000)
      expect(rows[0].textContent).toBe('line 2')
      expect(rows.at(-1).textContent).toBe('line 5001')
      expect(rows.at(-2).textContent).toBe('line 5000')
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

      fetchWithMock.mockReturnValueOnce(new Promise(() => {}))
      await act(async () => { fireEvent.change(screen.getByTestId('logs-lines-select'), { target: { value: '500' } }) })
      await flush()
      expect(fetchWithMock).toHaveBeenCalledTimes(2)

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

      fetchWithMock.mockResolvedValueOnce({ logs: '2026-01-01T00:00:00Z app one\n2026-01-01T00:00:01Z app two\n' })
      render(<WorkloadLogsViewer {...defaultProps} />)
      await flush()
      expect(screen.getAllByTestId('logs-line').map(r => r.textContent)).toEqual(['app one', 'app two'])

      let resolveAppend
      fetchWithMock.mockReturnValueOnce(new Promise((resolve) => { resolveAppend = resolve }))
      await act(async () => { vi.advanceTimersByTime(5000) })

      fetchWithMock.mockResolvedValueOnce({ logs: '2026-01-01T00:00:05Z reset one\n' })
      await act(async () => { fireEvent.change(screen.getByTestId('logs-lines-select'), { target: { value: '500' } }) })
      await flush()
      expect(screen.getAllByTestId('logs-line').map(r => r.textContent)).toEqual(['reset one'])

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

      fetchWithMock.mockRejectedValueOnce(new Error('Pod default/my-pod not found'))
      await act(async () => { vi.advanceTimersByTime(5000) })
      await flush()

      expect(screen.getAllByTestId('logs-line')).toHaveLength(2)
      expect(screen.getByTestId('logs-follow-error')).toHaveTextContent('Pod default/my-pod not found')
      expect(screen.queryByTestId('logs-error')).not.toBeInTheDocument()

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

    expect(screen.getAllByTestId('logs-timestamp').some(el => el.getAttribute('data-latest') === 'true')).toBe(false)

    fetchWithMock.mockResolvedValue({ logs: '2026-01-01T00:00:00Z line one\n2026-01-01T00:00:01Z line two\n2026-01-01T00:00:02Z line three\n' })
    await user.selectOptions(screen.getByTestId('logs-lines-select'), '500')
    await waitFor(() => {
      const pills = screen.getAllByTestId('logs-timestamp')
      expect(pills).toHaveLength(3)
      expect(pills[pills.length - 1]).toHaveAttribute('data-latest', 'true')
    })
    expect(screen.getAllByTestId('logs-timestamp')[0]).not.toHaveAttribute('data-latest')
  })

  it('appends via the global sinceTime on follow for a single pod with multiple containers', async () => {
    vi.useFakeTimers()
    try {
      const flush = async () => { await act(async () => { await Promise.resolve(); await Promise.resolve() }) }
      const props = { ...defaultProps, pods: onePod([{ name: 'app', isInit: false }, { name: 'sidecar', isInit: false }]) }
      fetchWithMock.mockResolvedValueOnce({ logs: '2026-01-01T00:00:00Z one\n2026-01-01T00:00:01Z two\n' })
      fetchWithMock.mockResolvedValue({ logs: '2026-01-01T00:00:01Z two\n2026-01-01T00:00:02Z three\n' })

      render(<WorkloadLogsViewer {...props} />)
      await flush()
      const first = fetchWithMock.mock.calls[0][0].endpoint
      expect(containersOf(first)).toEqual(['app', 'sidecar'])
      expect(first).not.toContain('sinceTime')

      await act(async () => { vi.advanceTimersByTime(5000) })
      await flush()
      expect(screen.getAllByTestId('logs-line').map(r => r.textContent)).toEqual(['one', 'two', 'three'])

      // One pod streams via the global sinceTime cursor, not a per-pod `since`.
      const poll = lastCall().endpoint
      expect(poll).toContain('sinceTime=2026-01-01T00%3A00%3A01Z')
      expect(poll).not.toContain('since=')
      expect(containersOf(poll)).toEqual(['app', 'sidecar'])
    } finally {
      vi.useRealTimers()
    }
  })

  it('stays open with an inline error and fires no request when there are no pods to stream', async () => {
    render(<WorkloadLogsViewer {...defaultProps} pods={[]} />)
    await new Promise(r => setTimeout(r, 0))
    expect(fetchWithMock).not.toHaveBeenCalled()
    // Empty pod list surfaces an inline error, not the neutral empty state.
    expect(screen.queryByTestId('logs-empty')).not.toBeInTheDocument()
    expect(screen.getByTestId('logs-no-pods')).toHaveTextContent('The workload has no running pods to stream logs from.')
  })

  it('falls back to the regular containers when the selected container disappears', async () => {
    const user = userEvent.setup()
    const props = { ...defaultProps, pods: onePod([{ name: 'app', isInit: false }, { name: 'sidecar', isInit: false }]) }
    const { rerender } = render(<WorkloadLogsViewer {...props} />)
    await waitFor(() => expect(screen.getByTestId('logs-content')).toBeInTheDocument())

    await user.selectOptions(screen.getByTestId('logs-container-select'), 'sidecar::false')
    await waitFor(() => expect(containersOf(lastCall().endpoint)).toEqual(['sidecar']))

    // The pod loses the 'sidecar' container (same pod, so no container reset): the
    // request must fall back to the regular containers, not the vanished one.
    rerender(<WorkloadLogsViewer {...props} pods={onePod([{ name: 'app', isInit: false }])} />)
    await waitFor(() => expect(containersOf(lastCall().endpoint)).toEqual(['app']))
  })

  it('does not highlight the last visible line when new logs are filtered out', async () => {
    const user = userEvent.setup()
    fetchWithMock.mockResolvedValue({ logs: '2026-01-01T00:00:00Z keep me\n2026-01-01T00:00:01Z keep me too\n' })
    render(<WorkloadLogsViewer {...defaultProps} />)
    await waitFor(() => expect(screen.getAllByTestId('logs-line')).toHaveLength(2))

    await user.type(screen.getByTestId('logs-filter-input'), 'keep')
    await waitFor(() => expect(screen.getAllByTestId('logs-line')).toHaveLength(2))

    fetchWithMock.mockResolvedValue({ logs: '2026-01-01T00:00:00Z keep me\n2026-01-01T00:00:01Z keep me too\n2026-01-01T00:00:02Z noise\n' })
    await user.selectOptions(screen.getByTestId('logs-lines-select'), '500')
    await waitFor(() => expect(lastCall().endpoint).toContain('tailLines=500'))
    expect(screen.getAllByTestId('logs-line')).toHaveLength(2)
    expect(screen.getAllByTestId('logs-timestamp').some(el => el.getAttribute('data-latest') === 'true')).toBe(false)
  })

  it('reflows a structured klog entry into one logs-line holding stacked rows', async () => {
    fetchWithMock.mockResolvedValue({
      logs: '2026-01-01T00:00:00Z E0526 23:03:57.521582       1 leaderelection.go:452] "Error retrieving lease lock" err="i/o timeout" logger="cert-manager.controller"\n'
    })
    render(<WorkloadLogsViewer {...defaultProps} />)
    await waitFor(() => expect(screen.getAllByTestId('logs-line')).toHaveLength(1))

    // Still one logs-line per entry, but it now holds header+message + 2 field rows.
    const rows = screen.getAllByTestId('logs-line-row').map(r => r.textContent)
    expect(rows).toEqual([
      'E0526 23:03:57.521582 1 leaderelection.go:452] Error retrieving lease lock',
      'err: i/o timeout',
      'logger: cert-manager.controller'
    ])
  })

  it('strips a trailing CR from a CRLF stream so it does not leak into a field', async () => {
    fetchWithMock.mockResolvedValue({
      logs: '2026-01-01T00:00:00Z level=info msg=hi controller=foo\r\n'
    })
    render(<WorkloadLogsViewer {...defaultProps} />)
    await waitFor(() => expect(screen.getAllByTestId('logs-line-row').length).toBeGreaterThan(0))

    const rows = screen.getAllByTestId('logs-line-row').map(r => r.textContent)
    expect(rows).toContain('controller: foo')
    expect(rows.some(t => t.includes('\r'))).toBe(false)
  })

  it('leaves a structured entry verbatim in raw mode', async () => {
    const user = userEvent.setup()
    fetchWithMock.mockResolvedValue({
      logs: '2026-01-01T00:00:00Z level=info msg="reconcile complete" controller=gitrepository\n'
    })
    render(<WorkloadLogsViewer {...defaultProps} />)
    await waitFor(() => expect(screen.getAllByTestId('logs-line-row')).toHaveLength(3))

    // Format off renders byte-for-byte: one logs-line, no row split. Assert exact
    // textContent (toHaveTextContent normalizes whitespace) to catch a tab/CR change.
    await user.click(screen.getByTestId('logs-format-toggle'))
    await waitFor(() => expect(screen.queryByTestId('logs-line-row')).toBeNull())
    const lines = screen.getAllByTestId('logs-line')
    expect(lines).toHaveLength(1)
    expect(lines[0].textContent).toBe('level=info msg="reconcile complete" controller=gitrepository')
  })

  describe('stack-trace grouping', () => {
    // A Go panic followed by a flush-left line that closes the trace. Frames are
    // timestamped (the CRI case), so each is its own entry the grouper folds.
    const GO_PANIC_LOGS =
      '2026-01-01T00:00:00.000000000Z panic: boom from c\n' +
      '2026-01-01T00:00:00.000000001Z goroutine 1 [running]:\n' +
      '2026-01-01T00:00:00.000000002Z main.c(...)\n' +
      '2026-01-01T00:00:00.000000003Z \t/m.go:2\n' +
      '2026-01-01T00:00:00.000000004Z after the panic\n'

    it('folds a trace under its head and expands it on click', async () => {
      const user = userEvent.setup()
      fetchWithMock.mockResolvedValue({ logs: GO_PANIC_LOGS })
      render(<WorkloadLogsViewer {...defaultProps} />)
      await waitFor(() => expect(screen.getByTestId('logs-content')).toBeInTheDocument())

      // Collapsed: the head shows, the frames are hidden behind the fold control.
      expect(screen.getByTestId('logs-content')).toHaveTextContent('panic: boom from c')
      expect(screen.queryByText('main.c(...)')).toBeNull()
      expect(screen.getByTestId('logs-group-fold')).toHaveTextContent('3 frames')
      // The flush-left line after the trace is a separate, always-visible entry.
      expect(screen.getByText('after the panic')).toBeInTheDocument()

      await user.click(screen.getByTestId('logs-group-fold'))
      expect(screen.getByText('main.c(...)')).toBeInTheDocument()
      expect(screen.getByText('goroutine 1 [running]:')).toBeInTheDocument()
    })

    it('renders every trace line flat in raw mode', async () => {
      const user = userEvent.setup()
      fetchWithMock.mockResolvedValue({ logs: GO_PANIC_LOGS })
      render(<WorkloadLogsViewer {...defaultProps} />)
      await waitFor(() => expect(screen.getByTestId('logs-content')).toBeInTheDocument())

      await user.click(screen.getByTestId('logs-format-toggle'))
      expect(screen.getByText('main.c(...)')).toBeInTheDocument()
      expect(screen.queryByTestId('logs-group-fold')).toBeNull()
    })

    it('keeps a whole trace on a frame match and drops it on a negated frame match', async () => {
      const user = userEvent.setup()
      fetchWithMock.mockResolvedValue({ logs: GO_PANIC_LOGS })
      render(<WorkloadLogsViewer {...defaultProps} />)
      await waitFor(() => expect(screen.getByTestId('logs-content')).toBeInTheDocument())

      const filter = screen.getByTestId('logs-filter-input')
      // A needle inside a folded frame keeps the whole trace (head stays visible).
      await user.type(filter, 'main.c')
      expect(screen.getByTestId('logs-content')).toHaveTextContent('panic: boom from c')

      // The negated complement drops the whole trace, even though the needle is
      // only in a buried frame.
      await user.clear(filter)
      await user.type(filter, '!main.c')
      expect(screen.queryByText('panic: boom from c')).toBeNull()
    })

    it('surfaces a level-bumped trace when filtering on Error', async () => {
      const user = userEvent.setup()
      // The bare `Error:` head detects as info; the grouper bumps a recognized
      // trace head to error so the level filter surfaces it.
      fetchWithMock.mockResolvedValue({
        logs:
          '2026-01-01T00:00:00.000000000Z Error: boom from c\n' +
          '2026-01-01T00:00:00.000000001Z     at c ([eval]:1:56)\n' +
          '2026-01-01T00:00:00.000000002Z plain info line\n'
      })
      render(<WorkloadLogsViewer {...defaultProps} />)
      await waitFor(() => expect(screen.getByTestId('logs-content')).toBeInTheDocument())

      await user.click(screen.getByTestId('logs-level-filter'))
      await user.click(screen.getByTestId('logs-level-option-error'))

      expect(screen.getByTestId('logs-content')).toHaveTextContent('Error: boom from c')
      expect(screen.queryByText('plain info line')).toBeNull()
    })

    it('keeps the pod id on a grouped trace head in the all-pods view', async () => {
      fetchWithMock.mockResolvedValue({
        pod: 'web-abc-gqh2x,web-abc-p8x2k', container: 'app', tagged: true, total: 2, streamed: 2,
        logs:
          'web-abc-gqh2x 2026-01-01T00:00:00.000000000Z panic: boom\n' +
          '  goroutine 1 [running]:\n' +
          '  main.crash()\n'
      })
      render(<WorkloadLogsViewer {...allPodsProps} />)
      await waitFor(() => expect(screen.getByTestId('logs-content')).toBeInTheDocument())

      // The trace folds (untimestamped frames included); its head pill keeps the pod id.
      expect(screen.getByTestId('logs-pod-id')).toHaveTextContent('gqh2x')
      expect(screen.getByTestId('logs-group-fold')).toBeInTheDocument()
      expect(screen.queryByText('main.crash()')).toBeNull()
    })

    it('folds two pods\' interleaved timestamped traces per pod in the all-pods view', async () => {
      const user = userEvent.setup()
      // Both pods crash in the same window; with every frame timestamped (the CRI
      // case) the backend interleaves them frame-by-frame. Per-pod folding must
      // keep each trace intact instead of fragmenting around the other pod's lines.
      fetchWithMock.mockResolvedValue({
        pod: 'web-abc-gqh2x,web-abc-p8x2k', container: 'app', tagged: true, total: 2, streamed: 2,
        logs:
          'web-abc-gqh2x 2026-01-01T00:00:00.000000000Z panic: boom A\n' +
          'web-abc-p8x2k 2026-01-01T00:00:00.000000001Z panic: boom B\n' +
          'web-abc-gqh2x 2026-01-01T00:00:00.000000002Z goroutine 1 [running]:\n' +
          'web-abc-p8x2k 2026-01-01T00:00:00.000000003Z goroutine 2 [running]:\n' +
          'web-abc-gqh2x 2026-01-01T00:00:00.000000004Z main.a(...)\n' +
          'web-abc-p8x2k 2026-01-01T00:00:00.000000005Z main.b(...)\n'
      })
      render(<WorkloadLogsViewer {...allPodsProps} />)
      await waitFor(() => expect(screen.getByTestId('logs-content')).toBeInTheDocument())

      // Two folded groups, each headed by its own pod id; all frames collapsed.
      expect(screen.getAllByTestId('logs-group-fold')).toHaveLength(2)
      expect(screen.getAllByTestId('logs-pod-id')).toHaveLength(2)
      expect(screen.queryByText('main.a(...)')).toBeNull()
      expect(screen.queryByText('main.b(...)')).toBeNull()

      // Expanding pod A's trace reveals only its frame, never pod B's.
      await user.click(screen.getAllByTestId('logs-group-fold')[0])
      expect(screen.getByText('main.a(...)')).toBeInTheDocument()
      expect(screen.queryByText('main.b(...)')).toBeNull()
    })

    it('keeps a folded group expanded across an append that adds a later entry', async () => {
      vi.useFakeTimers()
      try {
        const flush = async () => { await act(async () => { await Promise.resolve(); await Promise.resolve() }) }
        fetchWithMock.mockResolvedValueOnce({
          logs:
            '2026-01-01T00:00:00.000000000Z panic: boom from c\n' +
            '2026-01-01T00:00:00.000000001Z goroutine 1 [running]:\n' +
            '2026-01-01T00:00:00.000000002Z main.c(...)\n'
        })
        fetchWithMock.mockResolvedValue({ logs: '2026-01-01T00:00:05.000000000Z later line\n' })

        render(<WorkloadLogsViewer {...defaultProps} />)
        await flush()
        await act(async () => { fireEvent.click(screen.getByTestId('logs-group-fold')) })
        expect(screen.getByText('main.c(...)')).toBeInTheDocument()

        // A follow poll appends a later line; the trace's identity key is unchanged,
        // so its local expand state survives the re-render.
        await act(async () => { vi.advanceTimersByTime(5000) })
        await flush()
        expect(screen.getByText('later line')).toBeInTheDocument()
        expect(screen.getByText('main.c(...)')).toBeInTheDocument()
      } finally {
        vi.useRealTimers()
      }
    })
  })

  describe('unstructured-run grouping', () => {
    // A curl -v dump: co-timestamped, structure-less lines that the format layer
    // leaves plain, so they group under one timestamp pill — rendered in full, not
    // folded behind an expand control.
    const CURL_LOGS =
      '2026-01-01T00:00:00.000000000Z * Host frontend was resolved.\n' +
      '2026-01-01T00:00:00.000000001Z *   Trying 10.0.0.1:80...\n' +
      '2026-01-01T00:00:00.000000002Z > GET /api/info HTTP/1.1\n' +
      '2026-01-01T00:00:00.000000003Z < HTTP/1.1 200 OK\n' +
      '2026-01-01T00:00:00.000000004Z * Connection #0 to host frontend left intact\n'

    it('groups a co-timestamped curl dump under one pill with every line visible', async () => {
      fetchWithMock.mockResolvedValue({ logs: CURL_LOGS })
      render(<WorkloadLogsViewer {...defaultProps} />)
      await waitFor(() => expect(screen.getByTestId('logs-content')).toBeInTheDocument())

      // Grouped, not folded: all lines render, nothing hidden, no expand control.
      expect(screen.queryByTestId('logs-group-fold')).toBeNull()
      expect(screen.getByText('* Host frontend was resolved.')).toBeInTheDocument()
      expect(screen.getByText('> GET /api/info HTTP/1.1')).toBeInTheDocument()
      expect(screen.getByText('< HTTP/1.1 200 OK')).toBeInTheDocument()
      expect(screen.getByText('* Connection #0 to host frontend left intact')).toBeInTheDocument()

      // One timestamp block heads the whole run (a single separator pill), default
      // info level (not error-tinted).
      const pills = screen.getAllByTestId('logs-timestamp')
      expect(pills).toHaveLength(1)
      expect(pills[0]).toHaveAttribute('data-level', 'info')
    })

    it('does not group structured logger lines emitted milliseconds apart', async () => {
      // Leading app Logback date → parseJava recognizes each as structured, so they
      // stay separate entries despite the tight timestamp window.
      fetchWithMock.mockResolvedValue({
        logs:
          '2026-01-01T00:00:00.000000000Z 2026-01-01 00:00:00.000 [main] DEBUG com.x - one\n' +
          '2026-01-01T00:00:00.000000002Z 2026-01-01 00:00:00.002 [main] DEBUG com.x - two\n'
      })
      render(<WorkloadLogsViewer {...defaultProps} />)
      await waitFor(() => expect(screen.getByTestId('logs-content')).toBeInTheDocument())

      // Each structured line is its own timestamp block.
      expect(screen.getAllByTestId('logs-timestamp')).toHaveLength(2)
      expect(screen.getByTestId('logs-content')).toHaveTextContent('one')
      expect(screen.getByTestId('logs-content')).toHaveTextContent('two')
    })

    it('does not group a date-less leveled console line into a burst', async () => {
      // `[main] DEBUG com.x - msg` renders plain (no leading digit for parseJava),
      // but detectLevel finds DEBUG, so the level catch keeps it out of a burst even
      // though decorateLine fell through to plain.
      fetchWithMock.mockResolvedValue({
        logs:
          '2026-01-01T00:00:00.000000000Z [main] DEBUG com.x - one\n' +
          '2026-01-01T00:00:00.000000002Z [main] DEBUG com.x - two\n' +
          '2026-01-01T00:00:00.000000004Z [main] WARN com.x - three\n'
      })
      render(<WorkloadLogsViewer {...defaultProps} />)
      await waitFor(() => expect(screen.getByTestId('logs-content')).toBeInTheDocument())

      expect(screen.getAllByTestId('logs-timestamp')).toHaveLength(3)
    })
  })

  describe('container-scoped grouping', () => {
    // A pod running two containers; the all-containers view tags each line with its
    // container of origin so folding can be scoped per container.
    const twoContainerProps = {
      ...defaultProps,
      pods: onePod([{ name: 'app', isInit: false }, { name: 'envoy', isInit: false }]),
    }

    it('keeps an interleaved container\'s trace intact and never folds in another container', async () => {
      const user = userEvent.setup()
      // app panics while envoy logs a co-timestamped, frame-shaped line right in the
      // middle of app's trace. Per-(pod, container) partitioning keeps app's trace
      // whole (2 frames) and envoy's line a separate, visible entry — without the
      // partition + guard, envoy's line would split or be swallowed by app's trace.
      fetchWithMock.mockResolvedValue({
        pod: 'my-pod', container: 'app,envoy', containerTagged: true,
        logs:
          'app 2026-01-01T00:00:00.000000000Z panic: boom from app\n' +
          'app 2026-01-01T00:00:00.000000001Z goroutine 1 [running]:\n' +
          'envoy 2026-01-01T00:00:00.000000002Z   at Envoy::drain (conn.cc:42)\n' +
          'app 2026-01-01T00:00:00.000000003Z main.crash()\n'
      })
      render(<WorkloadLogsViewer {...twoContainerProps} />)
      await waitFor(() => expect(screen.getByTestId('logs-content')).toBeInTheDocument())

      // Exactly one fold (app's trace), holding both app frames; envoy's line is
      // never folded into it.
      const folds = screen.getAllByTestId('logs-group-fold')
      expect(folds).toHaveLength(1)
      expect(folds[0]).toHaveTextContent('2 frames')
      expect(screen.getByTestId('logs-content')).toHaveTextContent('at Envoy::drain (conn.cc:42)')
      expect(screen.queryByText('main.crash()')).toBeNull()

      // Expanding app's trace reveals both its frames (the one that arrived after
      // envoy's line included), never envoy's line.
      await user.click(folds[0])
      expect(screen.getByText('goroutine 1 [running]:')).toBeInTheDocument()
      expect(screen.getByText('main.crash()')).toBeInTheDocument()
    })
  })

  describe('persisted settings', () => {
    it('seeds follow, format and lines from the stored settings', async () => {
      logSettings.value = { follow: false, formatted: false, tail: 500 }
      render(<WorkloadLogsViewer {...defaultProps} />)
      await waitFor(() => expect(screen.getByTestId('logs-content')).toBeInTheDocument())

      expect(screen.getByTestId('logs-follow-toggle')).toHaveAttribute('aria-pressed', 'false')
      expect(screen.getByTestId('logs-format-toggle')).toHaveAttribute('aria-pressed', 'false')
      expect(screen.getByTestId('logs-lines-select')).toHaveValue('500')
    })

    it('persists follow, format and lines changes to the settings signal', async () => {
      render(<WorkloadLogsViewer {...defaultProps} />)
      await waitFor(() => expect(screen.getByTestId('logs-content')).toBeInTheDocument())

      await act(async () => { fireEvent.change(screen.getByTestId('logs-lines-select'), { target: { value: '1000' } }) })
      await act(async () => { fireEvent.click(screen.getByTestId('logs-follow-toggle')) })
      await act(async () => { fireEvent.click(screen.getByTestId('logs-format-toggle')) })

      expect(logSettings.value).toEqual({ follow: false, formatted: false, tail: 1000 })
    })

    it('does not change an already-open viewer when the settings are reset', async () => {
      render(<WorkloadLogsViewer {...defaultProps} />)
      await waitFor(() => expect(screen.getByTestId('logs-content')).toBeInTheDocument())

      await act(async () => { fireEvent.change(screen.getByTestId('logs-lines-select'), { target: { value: '1000' } }) })
      await act(async () => { fireEvent.click(screen.getByTestId('logs-format-toggle')) })

      // Reset the persisted settings (as "Clear local storage" does) while the
      // viewer is open. The signal resets, but the open viewer seeded via peek and
      // never subscribed, so its live state is untouched — the reset only applies
      // the next time the viewer is opened.
      await act(async () => { resetLogSettings() })

      expect(logSettings.value).toEqual(DEFAULT_LOG_SETTINGS)
      expect(screen.getByTestId('logs-lines-select')).toHaveValue('1000')
      expect(screen.getByTestId('logs-format-toggle')).toHaveAttribute('aria-pressed', 'false')
    })
  })
})

// Part B mechanism: reconcileEntries is the pure incremental-parse core, so these
// prove the reuse itself (parse-call count + object identity), not just that the
// final output happens to be right — a full re-parse on every poll would pass the
// component-level output tests but fail these.
describe('reconcileEntries', () => {
  // Distinct object per line so identity reuse is observable.
  const mkEntries = (lines) => lines.map(l => ({ src: l }))

  it('parses every line when there is no previous buffer', () => {
    const parse = vi.fn(l => ({ p: l }))
    const out = reconcileEntries([], [], ['a', 'b'], parse)
    expect(parse).toHaveBeenCalledTimes(2)
    expect(out).toEqual([{ p: 'a' }, { p: 'b' }])
  })

  it('reuses every cached entry and parses only the appended tail when the buffer grows', () => {
    const prevLines = ['a', 'b']
    const prevEntries = mkEntries(prevLines)
    const parse = vi.fn(l => ({ p: l }))
    const out = reconcileEntries(prevLines, prevEntries, ['a', 'b', 'c', 'd'], parse)
    expect(parse.mock.calls.map(c => c[0])).toEqual(['c', 'd'])
    expect(out[0]).toBe(prevEntries[0]) // same object, not re-parsed
    expect(out[1]).toBe(prevEntries[1])
    expect(out).toHaveLength(4)
  })

  it('parses nothing when the buffer is unchanged across a poll', () => {
    const prevLines = ['a', 'b']
    const prevEntries = mkEntries(prevLines)
    const parse = vi.fn(l => ({ p: l }))
    const out = reconcileEntries(prevLines, prevEntries, ['a', 'b'], parse)
    expect(parse).not.toHaveBeenCalled()
    expect(out).toEqual(prevEntries)
  })

  it('reuses the survivors and parses the tail through front-eviction', () => {
    const prevLines = ['a', 'b', 'c', 'd']
    const prevEntries = mkEntries(prevLines)
    const parse = vi.fn(l => ({ p: l }))
    const out = reconcileEntries(prevLines, prevEntries, ['c', 'd', 'e'], parse)
    expect(parse.mock.calls.map(c => c[0])).toEqual(['e'])
    expect(out[0]).toBe(prevEntries[2])
    expect(out[1]).toBe(prevEntries[3])
    expect(out).toHaveLength(3)
  })

  it('finds the eviction offset past an earlier duplicate of the new head line', () => {
    // 'c' also appears at index 0, but the real overlap is the [c,d] suffix; the
    // scan starts at 1 and the verify loop confirms the right offset.
    const prevLines = ['c', 'a', 'c', 'd']
    const prevEntries = mkEntries(prevLines)
    const parse = vi.fn(l => ({ p: l }))
    const out = reconcileEntries(prevLines, prevEntries, ['c', 'd', 'e'], parse)
    expect(parse.mock.calls.map(c => c[0])).toEqual(['e'])
    expect(out[0]).toBe(prevEntries[2])
    expect(out[1]).toBe(prevEntries[3])
  })

  it('falls back to a full parse when the overlap diverges', () => {
    // lines[0] matches at drop=1, but the next line differs, so the verify loop
    // rejects the reuse and re-parses everything (the mirror can never drift).
    const prevLines = ['a', 'b', 'c']
    const prevEntries = mkEntries(prevLines)
    const parse = vi.fn(l => ({ p: l }))
    const out = reconcileEntries(prevLines, prevEntries, ['b', 'X', 'Y'], parse)
    expect(parse.mock.calls.map(c => c[0])).toEqual(['b', 'X', 'Y'])
    expect(out).toEqual([{ p: 'b' }, { p: 'X' }, { p: 'Y' }])
  })

  it('falls back to a full parse when the computed overlap exceeds the new buffer', () => {
    // The offset scan lands on drop=3 (reuse=2), but only one line remains, so the
    // reuse>lines.length guard forces a full parse rather than reading past the end.
    const prevLines = ['a', 'b', 'c', 'd', 'e']
    const prevEntries = mkEntries(prevLines)
    const parse = vi.fn(l => ({ p: l }))
    const out = reconcileEntries(prevLines, prevEntries, ['d'], parse)
    expect(parse.mock.calls.map(c => c[0])).toEqual(['d'])
    expect(out).toEqual([{ p: 'd' }])
  })
})
