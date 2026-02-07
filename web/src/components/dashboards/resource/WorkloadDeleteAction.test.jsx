// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/preact'
import userEvent from '@testing-library/user-event'
import { WorkloadDeleteAction } from './WorkloadDeleteAction'

// Mock the fetchWithMock function
vi.mock('../../../utils/fetch', () => ({
  fetchWithMock: vi.fn()
}))

import { fetchWithMock } from '../../../utils/fetch'

describe('WorkloadDeleteAction component', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    fetchWithMock.mockResolvedValue({ success: true, message: 'Pod deleted' })
  })

  const defaultProps = {
    namespace: 'default',
    name: 'my-pod-abc123',
    isPendingDeletion: false,
    onActionStart: vi.fn(),
    onActionComplete: vi.fn(),
    onPodDeleteStart: vi.fn(),
    onPodDeleteFailed: vi.fn()
  }

  it('should render delete button with trash icon', () => {
    render(<WorkloadDeleteAction {...defaultProps} />)

    const button = screen.getByTestId('delete-pod-button')
    expect(button).toBeInTheDocument()
    expect(button.querySelector('svg')).toBeInTheDocument()
    expect(button.title).toBe('Delete pod my-pod-abc123')
  })

  it('should show confirmation dialog on click', async () => {
    const user = userEvent.setup()
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false)

    render(<WorkloadDeleteAction {...defaultProps} />)

    await user.click(screen.getByTestId('delete-pod-button'))

    expect(confirmSpy).toHaveBeenCalledWith(
      'Are you sure you want to delete the pod default/my-pod-abc123?'
    )
    confirmSpy.mockRestore()
  })

  it('should not call API if confirm is cancelled', async () => {
    const user = userEvent.setup()
    vi.spyOn(window, 'confirm').mockReturnValue(false)

    render(<WorkloadDeleteAction {...defaultProps} />)

    await user.click(screen.getByTestId('delete-pod-button'))

    expect(fetchWithMock).not.toHaveBeenCalled()
    expect(defaultProps.onActionStart).not.toHaveBeenCalled()
    expect(defaultProps.onPodDeleteStart).not.toHaveBeenCalled()

    window.confirm.mockRestore()
  })

  it('should call API with correct payload and notify parent after confirm', async () => {
    const user = userEvent.setup()
    vi.spyOn(window, 'confirm').mockReturnValue(true)

    render(<WorkloadDeleteAction {...defaultProps} />)

    await user.click(screen.getByTestId('delete-pod-button'))

    expect(defaultProps.onPodDeleteStart).toHaveBeenCalledWith('my-pod-abc123')

    await waitFor(() => {
      expect(fetchWithMock).toHaveBeenCalledWith({
        endpoint: '/api/v1/workload/action',
        mockPath: '../mock/action',
        mockExport: 'mockWorkloadAction',
        method: 'POST',
        body: {
          kind: 'Pod',
          namespace: 'default',
          name: 'my-pod-abc123',
          action: 'delete'
        }
      })
    })

    window.confirm.mockRestore()
  })

  it('should show spinner and disable button when isPendingDeletion is true', () => {
    render(<WorkloadDeleteAction {...defaultProps} isPendingDeletion={true} />)

    expect(screen.getByTestId('delete-pod-spinner')).toBeInTheDocument()

    const button = screen.getByTestId('delete-pod-button')
    expect(button.disabled).toBe(true)
  })

  it('should not show spinner when isPendingDeletion is false', () => {
    render(<WorkloadDeleteAction {...defaultProps} isPendingDeletion={false} />)

    expect(screen.queryByTestId('delete-pod-spinner')).not.toBeInTheDocument()

    const button = screen.getByTestId('delete-pod-button')
    expect(button.disabled).toBe(false)
  })

  it('should show error message on failure and notify parent', async () => {
    const user = userEvent.setup()
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    fetchWithMock.mockRejectedValue(new Error('Permission denied'))

    render(<WorkloadDeleteAction {...defaultProps} isPendingDeletion={false} />)

    await user.click(screen.getByTestId('delete-pod-button'))

    await waitFor(() => {
      expect(screen.getByTestId('delete-pod-error')).toBeInTheDocument()
      expect(screen.getByText('Permission denied')).toBeInTheDocument()
    })

    // Should notify parent to remove from pending deletions
    await waitFor(() => {
      expect(defaultProps.onPodDeleteFailed).toHaveBeenCalledWith('my-pod-abc123')
    })

    window.confirm.mockRestore()
  })

  it('should call onActionStart and onActionComplete callbacks', async () => {
    const user = userEvent.setup()
    vi.spyOn(window, 'confirm').mockReturnValue(true)

    render(<WorkloadDeleteAction {...defaultProps} />)

    await user.click(screen.getByTestId('delete-pod-button'))

    await waitFor(() => {
      expect(defaultProps.onActionStart).toHaveBeenCalled()
      expect(defaultProps.onActionComplete).toHaveBeenCalled()
    })

    window.confirm.mockRestore()
  })
})
