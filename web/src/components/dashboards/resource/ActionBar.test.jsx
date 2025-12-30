// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/preact'
import userEvent from '@testing-library/user-event'
import { ActionBar } from './ActionBar'

// Mock the fetchWithMock function
vi.mock('../../../utils/fetch', () => ({
  fetchWithMock: vi.fn()
}))

import { fetchWithMock } from '../../../utils/fetch'

describe('ActionBar component', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    fetchWithMock.mockResolvedValue({ success: true, message: 'Action completed' })
  })

  const defaultProps = {
    kind: 'Kustomization',
    namespace: 'flux-system',
    name: 'my-app',
    resourceData: {
      status: {
        reconcilerRef: { status: 'Ready' },
        userActions: ['reconcile', 'suspend', 'resume'],
        sourceRef: {
          kind: 'GitRepository',
          namespace: 'flux-system',
          name: 'my-repo',
          status: 'Ready'
        }
      }
    },
    onActionComplete: vi.fn()
  }

  describe('Rendering', () => {
    it('should render action buttons when userActions has actions', () => {
      render(<ActionBar {...defaultProps} />)

      expect(screen.getByTestId('action-bar')).toBeInTheDocument()
      expect(screen.getByTestId('reconcile-button')).toBeInTheDocument()
      expect(screen.getByTestId('suspend-resume-button')).toBeInTheDocument()
    })

    it('should not render when userActions is empty', () => {
      const props = {
        ...defaultProps,
        resourceData: {
          ...defaultProps.resourceData,
          status: { ...defaultProps.resourceData.status, userActions: [] }
        }
      }
      render(<ActionBar {...props} />)

      expect(screen.queryByTestId('action-bar')).not.toBeInTheDocument()
    })

    it('should render Reconcile Source button for Kustomization with sourceRef', () => {
      render(<ActionBar {...defaultProps} />)

      expect(screen.getByTestId('reconcile-source-button')).toBeInTheDocument()
    })

    it('should render Reconcile Source button for HelmRelease with sourceRef', () => {
      const props = { ...defaultProps, kind: 'HelmRelease' }
      render(<ActionBar {...props} />)

      expect(screen.getByTestId('reconcile-source-button')).toBeInTheDocument()
    })

    it('should not render Reconcile Source button for other kinds', () => {
      const props = { ...defaultProps, kind: 'GitRepository' }
      render(<ActionBar {...props} />)

      expect(screen.queryByTestId('reconcile-source-button')).not.toBeInTheDocument()
    })

    it('should not render Reconcile Source button when no sourceRef', () => {
      const props = {
        ...defaultProps,
        resourceData: {
          status: {
            reconcilerRef: { status: 'Ready' },
            userActions: ['reconcile', 'suspend', 'resume']
          }
        }
      }
      render(<ActionBar {...props} />)

      expect(screen.queryByTestId('reconcile-source-button')).not.toBeInTheDocument()
    })

    it('should show Resume button when suspended', () => {
      const props = {
        ...defaultProps,
        resourceData: {
          status: {
            reconcilerRef: { status: 'Suspended' },
            userActions: ['reconcile', 'suspend', 'resume']
          }
        }
      }
      render(<ActionBar {...props} />)

      expect(screen.getByText('Resume')).toBeInTheDocument()
    })

    it('should show Suspend button when not suspended', () => {
      render(<ActionBar {...defaultProps} />)

      expect(screen.getByText('Suspend')).toBeInTheDocument()
    })
  })

  describe('Disabled states', () => {
    it('should not render when userActions is empty', () => {
      const props = {
        ...defaultProps,
        resourceData: {
          status: {
            reconcilerRef: { status: 'Ready' },
            userActions: []
          }
        }
      }
      render(<ActionBar {...props} />)

      // Component should not render at all when no actions allowed
      expect(screen.queryByTestId('action-bar')).not.toBeInTheDocument()
    })

    it('should only disable Reconcile button when status is Progressing', () => {
      const props = {
        ...defaultProps,
        resourceData: {
          status: {
            reconcilerRef: { status: 'Progressing' },
            userActions: ['reconcile', 'suspend', 'resume'],
            sourceRef: defaultProps.resourceData.status.sourceRef
          }
        }
      }
      render(<ActionBar {...props} />)

      expect(screen.getByTestId('reconcile-button')).toBeDisabled()
      expect(screen.getByTestId('reconcile-source-button')).not.toBeDisabled()
      expect(screen.getByTestId('suspend-resume-button')).not.toBeDisabled()
    })

    it('should show spinner in Reconcile button when status is Progressing', () => {
      const props = {
        ...defaultProps,
        resourceData: {
          status: {
            reconcilerRef: { status: 'Progressing' },
            userActions: ['reconcile', 'suspend', 'resume']
          }
        }
      }
      render(<ActionBar {...props} />)

      const reconcileButton = screen.getByTestId('reconcile-button')
      expect(reconcileButton).toHaveTextContent('Reconcile')
      expect(reconcileButton.querySelector('.animate-spin')).toBeInTheDocument()
    })

    it('should disable Reconcile button when resource is Suspended', () => {
      const props = {
        ...defaultProps,
        resourceData: {
          status: {
            reconcilerRef: { status: 'Suspended' },
            userActions: ['reconcile', 'suspend', 'resume']
          }
        }
      }
      render(<ActionBar {...props} />)

      expect(screen.getByTestId('reconcile-button')).toBeDisabled()
    })

    it('should disable Reconcile Source button when source is Suspended', () => {
      const props = {
        ...defaultProps,
        resourceData: {
          status: {
            reconcilerRef: { status: 'Ready' },
            userActions: ['reconcile', 'suspend', 'resume'],
            sourceRef: {
              kind: 'GitRepository',
              namespace: 'flux-system',
              name: 'my-repo',
              status: 'Suspended'
            }
          }
        }
      }
      render(<ActionBar {...props} />)

      expect(screen.getByTestId('reconcile-source-button')).toBeDisabled()
    })
  })

  describe('Actions', () => {
    it('should call reconcile action with correct parameters', async () => {
      const user = userEvent.setup()
      render(<ActionBar {...defaultProps} />)

      await user.click(screen.getByTestId('reconcile-button'))

      await waitFor(() => {
        expect(fetchWithMock).toHaveBeenCalledWith({
          endpoint: '/api/v1/action',
          mockPath: '../mock/action',
          mockExport: 'mockAction',
          method: 'POST',
          body: {
            kind: 'Kustomization',
            namespace: 'flux-system',
            name: 'my-app',
            action: 'reconcile'
          }
        })
      })
    })

    it('should call reconcile source action with source parameters', async () => {
      const user = userEvent.setup()
      render(<ActionBar {...defaultProps} />)

      await user.click(screen.getByTestId('reconcile-source-button'))

      await waitFor(() => {
        expect(fetchWithMock).toHaveBeenCalledWith({
          endpoint: '/api/v1/action',
          mockPath: '../mock/action',
          mockExport: 'mockAction',
          method: 'POST',
          body: {
            kind: 'GitRepository',
            namespace: 'flux-system',
            name: 'my-repo',
            action: 'reconcile'
          }
        })
      })
    })

    it('should call suspend action when Suspend is clicked', async () => {
      const user = userEvent.setup()
      render(<ActionBar {...defaultProps} />)

      await user.click(screen.getByTestId('suspend-resume-button'))

      await waitFor(() => {
        expect(fetchWithMock).toHaveBeenCalledWith({
          endpoint: '/api/v1/action',
          mockPath: '../mock/action',
          mockExport: 'mockAction',
          method: 'POST',
          body: {
            kind: 'Kustomization',
            namespace: 'flux-system',
            name: 'my-app',
            action: 'suspend'
          }
        })
      })
    })

    it('should call resume action when Resume is clicked', async () => {
      const user = userEvent.setup()
      const props = {
        ...defaultProps,
        resourceData: {
          status: {
            reconcilerRef: { status: 'Suspended' },
            userActions: ['reconcile', 'suspend', 'resume']
          }
        }
      }
      render(<ActionBar {...props} />)

      await user.click(screen.getByTestId('suspend-resume-button'))

      await waitFor(() => {
        expect(fetchWithMock).toHaveBeenCalledWith({
          endpoint: '/api/v1/action',
          mockPath: '../mock/action',
          mockExport: 'mockAction',
          method: 'POST',
          body: {
            kind: 'Kustomization',
            namespace: 'flux-system',
            name: 'my-app',
            action: 'resume'
          }
        })
      })
    })

    it('should call onActionComplete after successful action', async () => {
      const user = userEvent.setup()
      const onActionComplete = vi.fn()
      render(<ActionBar {...defaultProps} onActionComplete={onActionComplete} />)

      await user.click(screen.getByTestId('reconcile-button'))

      await waitFor(() => {
        expect(onActionComplete).toHaveBeenCalled()
      })
    })
  })

  describe('Error handling', () => {
    it('should display error message when action fails', async () => {
      const user = userEvent.setup()
      fetchWithMock.mockRejectedValue(new Error('Permission denied'))

      render(<ActionBar {...defaultProps} />)

      await user.click(screen.getByTestId('reconcile-button'))

      await waitFor(() => {
        expect(screen.getByTestId('action-error')).toBeInTheDocument()
        expect(screen.getByText('Permission denied')).toBeInTheDocument()
      })
    })
  })

  describe('Button re-enable states', () => {
    it('should enable buttons when status changes from Progressing to Ready', () => {
      const { rerender } = render(
        <ActionBar
          {...defaultProps}
          resourceData={{
            status: {
              reconcilerRef: { status: 'Progressing' },
              userActions: ['reconcile', 'suspend', 'resume']
            }
          }}
        />
      )

      // Buttons should be disabled during Progressing
      expect(screen.getByTestId('reconcile-button')).toBeDisabled()

      // Rerender with Ready status
      rerender(
        <ActionBar
          {...defaultProps}
          resourceData={{
            status: {
              reconcilerRef: { status: 'Ready' },
              userActions: ['reconcile', 'suspend', 'resume']
            }
          }}
        />
      )

      // Buttons should now be enabled
      expect(screen.getByTestId('reconcile-button')).not.toBeDisabled()
    })

    it('should enable buttons when status changes from Progressing to Failed', () => {
      const { rerender } = render(
        <ActionBar
          {...defaultProps}
          resourceData={{
            status: {
              reconcilerRef: { status: 'Progressing' },
              userActions: ['reconcile', 'suspend', 'resume']
            }
          }}
        />
      )

      expect(screen.getByTestId('reconcile-button')).toBeDisabled()

      rerender(
        <ActionBar
          {...defaultProps}
          resourceData={{
            status: {
              reconcilerRef: { status: 'Failed' },
              userActions: ['reconcile', 'suspend', 'resume']
            }
          }}
        />
      )

      expect(screen.getByTestId('reconcile-button')).not.toBeDisabled()
    })
  })
})
