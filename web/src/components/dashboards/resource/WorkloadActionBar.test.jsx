// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/preact'
import userEvent from '@testing-library/user-event'
import { WorkloadActionBar } from './WorkloadActionBar'

// Mock the fetchWithMock function
vi.mock('../../../utils/fetch', () => ({
  fetchWithMock: vi.fn()
}))

import { fetchWithMock } from '../../../utils/fetch'

describe('WorkloadActionBar component', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    fetchWithMock.mockResolvedValue({ success: true, message: 'Action completed' })
  })

  const defaultProps = {
    kind: 'Deployment',
    namespace: 'default',
    name: 'my-app',
    userActions: ['restart'],
    onActionComplete: vi.fn(),
    onActionStart: vi.fn()
  }

  describe('Rendering', () => {
    it('should render restart button for Deployment with restart permission', () => {
      render(<WorkloadActionBar {...defaultProps} />)

      expect(screen.getByTestId('workload-action-bar')).toBeInTheDocument()
      expect(screen.getByTestId('restart-button')).toBeInTheDocument()
      expect(screen.getByText('Rollout Restart')).toBeInTheDocument()
    })

    it('should render restart button for StatefulSet with restart permission', () => {
      render(<WorkloadActionBar {...defaultProps} kind="StatefulSet" />)

      expect(screen.getByTestId('restart-button')).toBeInTheDocument()
    })

    it('should render restart button for DaemonSet with restart permission', () => {
      render(<WorkloadActionBar {...defaultProps} kind="DaemonSet" />)

      expect(screen.getByTestId('restart-button')).toBeInTheDocument()
    })

    it('should not render for CronJob (restart not supported)', () => {
      render(<WorkloadActionBar {...defaultProps} kind="CronJob" />)

      expect(screen.queryByTestId('workload-action-bar')).not.toBeInTheDocument()
    })

    it('should not render when userActions does not include restart', () => {
      render(<WorkloadActionBar {...defaultProps} userActions={[]} />)

      expect(screen.queryByTestId('workload-action-bar')).not.toBeInTheDocument()
    })

    it('should not render when userActions is undefined', () => {
      render(<WorkloadActionBar {...defaultProps} userActions={undefined} />)

      expect(screen.queryByTestId('workload-action-bar')).not.toBeInTheDocument()
    })
  })

  describe('Actions', () => {
    it('should call restart action with correct parameters', async () => {
      const user = userEvent.setup()
      render(<WorkloadActionBar {...defaultProps} />)

      await user.click(screen.getByTestId('restart-button'))

      await waitFor(() => {
        expect(fetchWithMock).toHaveBeenCalledWith({
          endpoint: '/api/v1/workload/action',
          mockPath: '../mock/action',
          mockExport: 'mockWorkloadAction',
          method: 'POST',
          body: {
            kind: 'Deployment',
            namespace: 'default',
            name: 'my-app',
            action: 'restart'
          }
        })
      })
    })

    it('should call onActionStart when action begins', async () => {
      const user = userEvent.setup()
      const onActionStart = vi.fn()
      render(<WorkloadActionBar {...defaultProps} onActionStart={onActionStart} />)

      await user.click(screen.getByTestId('restart-button'))

      await waitFor(() => {
        expect(onActionStart).toHaveBeenCalled()
      })
    })

    it('should call onActionComplete after successful action', async () => {
      const user = userEvent.setup()
      const onActionComplete = vi.fn()
      render(<WorkloadActionBar {...defaultProps} onActionComplete={onActionComplete} />)

      await user.click(screen.getByTestId('restart-button'))

      await waitFor(() => {
        expect(onActionComplete).toHaveBeenCalled()
      })
    })

    it('should show loading spinner while action is in progress', async () => {
      const user = userEvent.setup()
      // Make fetch hang to test loading state
      fetchWithMock.mockImplementation(() => new Promise(() => {}))

      render(<WorkloadActionBar {...defaultProps} />)

      await user.click(screen.getByTestId('restart-button'))

      expect(screen.getByTestId('restart-button').querySelector('.animate-spin')).toBeInTheDocument()
    })

    it('should disable button while action is in progress', async () => {
      const user = userEvent.setup()
      fetchWithMock.mockImplementation(() => new Promise(() => {}))

      render(<WorkloadActionBar {...defaultProps} />)

      await user.click(screen.getByTestId('restart-button'))

      expect(screen.getByTestId('restart-button')).toBeDisabled()
    })
  })

  describe('Error handling', () => {
    it('should display error message when action fails', async () => {
      const user = userEvent.setup()
      fetchWithMock.mockRejectedValue(new Error('Permission denied'))

      render(<WorkloadActionBar {...defaultProps} />)

      await user.click(screen.getByTestId('restart-button'))

      await waitFor(() => {
        expect(screen.getByTestId('workload-action-error')).toBeInTheDocument()
        expect(screen.getByText('Permission denied')).toBeInTheDocument()
      })
    })

    it('should allow dismissing error message', async () => {
      const user = userEvent.setup()
      fetchWithMock.mockRejectedValue(new Error('Test error'))

      render(<WorkloadActionBar {...defaultProps} />)

      await user.click(screen.getByTestId('restart-button'))

      await waitFor(() => {
        expect(screen.getByTestId('workload-action-error')).toBeInTheDocument()
      })

      await user.click(screen.getByTestId('dismiss-error-button'))

      expect(screen.queryByTestId('workload-action-error')).not.toBeInTheDocument()
    })
  })

  describe('Success feedback', () => {
    it('should show success checkmark after successful action', async () => {
      const user = userEvent.setup()
      render(<WorkloadActionBar {...defaultProps} />)

      await user.click(screen.getByTestId('restart-button'))

      await waitFor(() => {
        // The success check icon should be rendered (the checkmark SVG)
        const button = screen.getByTestId('restart-button')
        const checkmark = button.querySelector('path[d="M5 13l4 4L19 7"]')
        expect(checkmark).toBeInTheDocument()
      })
    })
  })

  describe('Different workload kinds', () => {
    const supportedKinds = ['Deployment', 'StatefulSet', 'DaemonSet']
    const unsupportedKinds = ['CronJob', 'Pod', 'Job']

    supportedKinds.forEach(kind => {
      it(`should render for ${kind}`, () => {
        render(<WorkloadActionBar {...defaultProps} kind={kind} />)
        expect(screen.getByTestId('workload-action-bar')).toBeInTheDocument()
      })
    })

    unsupportedKinds.forEach(kind => {
      it(`should not render for ${kind}`, () => {
        render(<WorkloadActionBar {...defaultProps} kind={kind} />)
        expect(screen.queryByTestId('workload-action-bar')).not.toBeInTheDocument()
      })
    })
  })

  describe('Restart in progress detection', () => {
    it('should show loading and disable button when restart is recent and status is InProgress', () => {
      const recentTimestamp = new Date(Date.now() - 10000).toISOString() // 10 seconds ago
      render(
        <WorkloadActionBar
          {...defaultProps}
          status="InProgress"
          restartedAt={recentTimestamp}
        />
      )

      const button = screen.getByTestId('restart-button')
      expect(button).toBeDisabled()
      expect(button.querySelector('.animate-spin')).toBeInTheDocument()
    })

    it('should not disable button when restart is recent but status is Current', () => {
      const recentTimestamp = new Date(Date.now() - 10000).toISOString() // 10 seconds ago
      render(
        <WorkloadActionBar
          {...defaultProps}
          status="Current"
          restartedAt={recentTimestamp}
        />
      )

      const button = screen.getByTestId('restart-button')
      expect(button).not.toBeDisabled()
      expect(button.querySelector('.animate-spin')).not.toBeInTheDocument()
    })

    it('should not disable button when status is InProgress but restart is old', () => {
      const oldTimestamp = new Date(Date.now() - 60000).toISOString() // 60 seconds ago
      render(
        <WorkloadActionBar
          {...defaultProps}
          status="InProgress"
          restartedAt={oldTimestamp}
        />
      )

      const button = screen.getByTestId('restart-button')
      expect(button).not.toBeDisabled()
      expect(button.querySelector('.animate-spin')).not.toBeInTheDocument()
    })

    it('should not disable button when restartedAt is undefined', () => {
      render(
        <WorkloadActionBar
          {...defaultProps}
          status="InProgress"
          restartedAt={undefined}
        />
      )

      const button = screen.getByTestId('restart-button')
      expect(button).not.toBeDisabled()
    })

    it('should not disable button when status is undefined', () => {
      const recentTimestamp = new Date(Date.now() - 10000).toISOString()
      render(
        <WorkloadActionBar
          {...defaultProps}
          status={undefined}
          restartedAt={recentTimestamp}
        />
      )

      const button = screen.getByTestId('restart-button')
      expect(button).not.toBeDisabled()
    })

    it('should not disable button when restartedAt is malformed', () => {
      render(
        <WorkloadActionBar
          {...defaultProps}
          status="InProgress"
          restartedAt="not-a-valid-date"
        />
      )

      const button = screen.getByTestId('restart-button')
      expect(button).not.toBeDisabled()
      expect(button.querySelector('.animate-spin')).not.toBeInTheDocument()
    })

    it('should show success checkmark when restart is recent and status is Current', () => {
      const recentTimestamp = new Date(Date.now() - 10000).toISOString() // 10 seconds ago
      render(
        <WorkloadActionBar
          {...defaultProps}
          status="Current"
          restartedAt={recentTimestamp}
        />
      )

      const button = screen.getByTestId('restart-button')
      expect(button).not.toBeDisabled()
      // Should show checkmark (success) icon
      const checkmark = button.querySelector('path[d="M5 13l4 4L19 7"]')
      expect(checkmark).toBeInTheDocument()
    })
  })
})
