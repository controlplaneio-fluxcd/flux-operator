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
          endpoint: '/api/v1/resource/action',
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
          endpoint: '/api/v1/resource/action',
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
          endpoint: '/api/v1/resource/action',
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
          endpoint: '/api/v1/resource/action',
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

    it('should call onActionStart when action begins', async () => {
      const user = userEvent.setup()
      const onActionStart = vi.fn()
      render(<ActionBar {...defaultProps} onActionStart={onActionStart} />)

      await user.click(screen.getByTestId('reconcile-button'))

      await waitFor(() => {
        expect(onActionStart).toHaveBeenCalled()
      })
    })

    it('should call onActionStart for reconcile source action', async () => {
      const user = userEvent.setup()
      const onActionStart = vi.fn()
      render(<ActionBar {...defaultProps} onActionStart={onActionStart} />)

      await user.click(screen.getByTestId('reconcile-source-button'))

      await waitFor(() => {
        expect(onActionStart).toHaveBeenCalled()
      })
    })

    it('should call onActionStart for suspend/resume action', async () => {
      const user = userEvent.setup()
      const onActionStart = vi.fn()
      render(<ActionBar {...defaultProps} onActionStart={onActionStart} />)

      await user.click(screen.getByTestId('suspend-resume-button'))

      await waitFor(() => {
        expect(onActionStart).toHaveBeenCalled()
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

  describe('Download button', () => {
    const downloadableProps = {
      kind: 'GitRepository',
      namespace: 'flux-system',
      name: 'my-repo',
      resourceData: {
        status: {
          reconcilerRef: { status: 'Ready' },
          userActions: ['reconcile', 'suspend', 'resume', 'download'],
          artifact: {
            url: 'http://source-controller.flux-system.svc/artifact.tar.gz'
          }
        }
      },
      onActionComplete: vi.fn()
    }

    it('should render Download button for source kind with artifact', () => {
      render(<ActionBar {...downloadableProps} />)

      expect(screen.getByTestId('download-button')).toBeInTheDocument()
      expect(screen.getByText('Download')).toBeInTheDocument()
    })

    it('should not render Download button for non-source kinds', () => {
      const props = {
        ...downloadableProps,
        kind: 'Kustomization',
        resourceData: {
          status: {
            reconcilerRef: { status: 'Ready' },
            userActions: ['reconcile', 'suspend', 'resume', 'download']
          }
        }
      }
      render(<ActionBar {...props} />)

      expect(screen.queryByTestId('download-button')).not.toBeInTheDocument()
    })

    it('should not render Download button when no artifact present', () => {
      const props = {
        ...downloadableProps,
        resourceData: {
          status: {
            reconcilerRef: { status: 'Ready' },
            userActions: ['reconcile', 'suspend', 'resume', 'download']
            // No artifact
          }
        }
      }
      render(<ActionBar {...props} />)

      expect(screen.queryByTestId('download-button')).not.toBeInTheDocument()
    })

    it('should not render Download button without download permission', () => {
      const props = {
        ...downloadableProps,
        resourceData: {
          status: {
            reconcilerRef: { status: 'Ready' },
            userActions: ['reconcile', 'suspend', 'resume'], // No download permission
            artifact: {
              url: 'http://source-controller.flux-system.svc/artifact.tar.gz'
            }
          }
        }
      }
      render(<ActionBar {...props} />)

      expect(screen.queryByTestId('download-button')).not.toBeInTheDocument()
    })

    it('should trigger download via fetch/blob when clicked', async () => {
      const user = userEvent.setup()
      const mockBlob = new Blob(['test content'], { type: 'application/octet-stream' })
      const mockFetch = vi.spyOn(global, 'fetch').mockResolvedValue({
        ok: true,
        blob: () => Promise.resolve(mockBlob)
      })
      const mockCreateObjectURL = vi.spyOn(window.URL, 'createObjectURL').mockReturnValue('blob:test')
      const mockRevokeObjectURL = vi.spyOn(window.URL, 'revokeObjectURL').mockImplementation(() => {})

      render(<ActionBar {...downloadableProps} />)

      await user.click(screen.getByTestId('download-button'))

      await waitFor(() => {
        expect(mockFetch).toHaveBeenCalledWith(
          '/api/v1/artifact/download?kind=GitRepository&namespace=flux-system&name=my-repo'
        )
      })

      mockFetch.mockRestore()
      mockCreateObjectURL.mockRestore()
      mockRevokeObjectURL.mockRestore()
    })

    it('should render Download button for all downloadable kinds', () => {
      const downloadableKinds = ['Bucket', 'GitRepository', 'OCIRepository', 'HelmChart', 'ExternalArtifact']

      downloadableKinds.forEach(kind => {
        const props = {
          ...downloadableProps,
          kind
        }
        const { unmount } = render(<ActionBar {...props} />)
        expect(screen.getByTestId('download-button')).toBeInTheDocument()
        unmount()
      })
    })
  })

  describe('Download dropdown for ArtifactGenerator', () => {
    const artifactGeneratorProps = {
      kind: 'ArtifactGenerator',
      namespace: 'flux-system',
      name: 'my-generator',
      resourceData: {
        status: {
          reconcilerRef: { status: 'Ready' },
          userActions: ['reconcile', 'suspend', 'resume', 'download'],
          inventory: [
            { name: 'artifact-1', namespace: 'flux-system', filename: 'config.tar.gz', digest: 'sha256:abc123' },
            { name: 'artifact-2', namespace: 'default', filename: 'data.tar.gz', digest: 'sha256:def456' }
          ]
        }
      },
      onActionComplete: vi.fn()
    }

    it('should render download dropdown button for ArtifactGenerator with inventory', () => {
      render(<ActionBar {...artifactGeneratorProps} />)

      expect(screen.getByTestId('download-dropdown-button')).toBeInTheDocument()
      expect(screen.getByTestId('download-dropdown-button')).toHaveTextContent('Download')
    })

    it('should not render download dropdown when no inventory', () => {
      const props = {
        ...artifactGeneratorProps,
        resourceData: {
          status: {
            reconcilerRef: { status: 'Ready' },
            userActions: ['reconcile', 'suspend', 'resume', 'download'],
            inventory: []
          }
        }
      }
      render(<ActionBar {...props} />)

      expect(screen.queryByTestId('download-dropdown-button')).not.toBeInTheDocument()
    })

    it('should not render download dropdown without download permission', () => {
      const props = {
        ...artifactGeneratorProps,
        resourceData: {
          status: {
            reconcilerRef: { status: 'Ready' },
            userActions: ['reconcile', 'suspend', 'resume'], // No download permission
            inventory: [
              { name: 'artifact-1', namespace: 'flux-system', filename: 'config.tar.gz', digest: 'sha256:abc123' }
            ]
          }
        }
      }
      render(<ActionBar {...props} />)

      expect(screen.queryByTestId('download-dropdown-button')).not.toBeInTheDocument()
    })

    it('should open dropdown menu on click', async () => {
      const user = userEvent.setup()
      render(<ActionBar {...artifactGeneratorProps} />)

      expect(screen.queryByTestId('download-dropdown-menu')).not.toBeInTheDocument()

      await user.click(screen.getByTestId('download-dropdown-button'))

      expect(screen.getByTestId('download-dropdown-menu')).toBeInTheDocument()
      expect(screen.getByTestId('download-artifact-artifact-1')).toBeInTheDocument()
      expect(screen.getByTestId('download-artifact-artifact-2')).toBeInTheDocument()
    })

    it('should close dropdown on second click', async () => {
      const user = userEvent.setup()
      render(<ActionBar {...artifactGeneratorProps} />)

      await user.click(screen.getByTestId('download-dropdown-button'))
      expect(screen.getByTestId('download-dropdown-menu')).toBeInTheDocument()

      await user.click(screen.getByTestId('download-dropdown-button'))
      expect(screen.queryByTestId('download-dropdown-menu')).not.toBeInTheDocument()
    })

    it('should trigger download with correct artifact parameters', async () => {
      const user = userEvent.setup()
      const mockBlob = new Blob(['test content'], { type: 'application/octet-stream' })
      const mockFetch = vi.spyOn(global, 'fetch').mockResolvedValue({
        ok: true,
        blob: () => Promise.resolve(mockBlob)
      })
      const mockCreateObjectURL = vi.spyOn(window.URL, 'createObjectURL').mockReturnValue('blob:test')
      const mockRevokeObjectURL = vi.spyOn(window.URL, 'revokeObjectURL').mockImplementation(() => {})

      render(<ActionBar {...artifactGeneratorProps} />)

      await user.click(screen.getByTestId('download-dropdown-button'))
      await user.click(screen.getByTestId('download-artifact-artifact-1'))

      await waitFor(() => {
        expect(mockFetch).toHaveBeenCalledWith(
          '/api/v1/artifact/download?kind=ExternalArtifact&namespace=flux-system&name=artifact-1'
        )
      })

      mockFetch.mockRestore()
      mockCreateObjectURL.mockRestore()
      mockRevokeObjectURL.mockRestore()
    })

    it('should close dropdown after artifact selection', async () => {
      const user = userEvent.setup()
      const mockBlob = new Blob(['test content'], { type: 'application/octet-stream' })
      vi.spyOn(global, 'fetch').mockResolvedValue({
        ok: true,
        blob: () => Promise.resolve(mockBlob)
      })
      vi.spyOn(window.URL, 'createObjectURL').mockReturnValue('blob:test')
      vi.spyOn(window.URL, 'revokeObjectURL').mockImplementation(() => {})

      render(<ActionBar {...artifactGeneratorProps} />)

      await user.click(screen.getByTestId('download-dropdown-button'))
      expect(screen.getByTestId('download-dropdown-menu')).toBeInTheDocument()

      await user.click(screen.getByTestId('download-artifact-artifact-1'))

      await waitFor(() => {
        expect(screen.queryByTestId('download-dropdown-menu')).not.toBeInTheDocument()
      })

      vi.restoreAllMocks()
    })

    it('should display artifact names and namespaces in dropdown', async () => {
      const user = userEvent.setup()
      render(<ActionBar {...artifactGeneratorProps} />)

      await user.click(screen.getByTestId('download-dropdown-button'))

      expect(screen.getByText('artifact-1')).toBeInTheDocument()
      expect(screen.getByText('flux-system')).toBeInTheDocument()
      expect(screen.getByText('artifact-2')).toBeInTheDocument()
      expect(screen.getByText('default')).toBeInTheDocument()
    })

    it('should show error when download fails', async () => {
      const user = userEvent.setup()
      vi.spyOn(global, 'fetch').mockResolvedValue({
        ok: false,
        status: 404,
        text: () => Promise.resolve('Artifact not found')
      })

      render(<ActionBar {...artifactGeneratorProps} />)

      await user.click(screen.getByTestId('download-dropdown-button'))
      await user.click(screen.getByTestId('download-artifact-artifact-1'))

      await waitFor(() => {
        expect(screen.getByTestId('action-error')).toBeInTheDocument()
        expect(screen.getByText('Artifact not found')).toBeInTheDocument()
      })

      vi.restoreAllMocks()
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
