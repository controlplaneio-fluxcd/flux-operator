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

// containersAll returns the repeated `container` query params of an endpoint.
const containersOf = (endpoint) => [...new URLSearchParams(endpoint.split('?')[1]).getAll('container')]
const podsOf = (endpoint) => [...new URLSearchParams(endpoint.split('?')[1]).getAll('pod')]
const lastCall = () => fetchWithMock.mock.calls.at(-1)[0]

describe('WorkloadLogsViewer component', () => {
  beforeEach(() => {
    vi.clearAllMocks()
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
    // A pod can lag behind another's per-pod cursor, so the response (and thus the
    // append order) is not globally chronological. The viewer must reorder by
    // timestamp, keeping each entry's continuation lines attached.
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
    expect(rows[0].querySelector('code')).toHaveClass('language-json')
    expect(rows[0].querySelector('.token')).not.toBeNull()
    expect(rows[0].textContent).toBe('{\n  "level": "info",\n  "msg": "hello"\n}')
    expect(rows[1].tagName).toBe('PRE')
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

  it('shows the empty state and fires no request when there are no pods to stream', async () => {
    render(<WorkloadLogsViewer {...defaultProps} pods={[]} />)
    await new Promise(r => setTimeout(r, 0))
    expect(fetchWithMock).not.toHaveBeenCalled()
    expect(screen.getByTestId('logs-empty')).toBeInTheDocument()
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
})
