// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/preact'
import userEvent from '@testing-library/user-event'
import { ArtifactPanel } from './ArtifactPanel'

describe('ArtifactPanel component', () => {
  const mockGitRepository = {
    apiVersion: 'source.toolkit.fluxcd.io/v1',
    kind: 'GitRepository',
    metadata: {
      name: 'flux-system',
      namespace: 'flux-system'
    },
    spec: {
      url: 'https://github.com/example/repo.git',
      ref: {
        branch: 'main'
      },
      interval: '1m'
    },
    status: {
      artifact: {
        lastUpdateTime: '2025-01-15T10:00:00Z',
        size: 13059,
        revision: 'refs/heads/main@sha1:abc123'
      }
    }
  }

  const mockOCIRepository = {
    apiVersion: 'source.toolkit.fluxcd.io/v1beta2',
    kind: 'OCIRepository',
    metadata: {
      name: 'cert-manager',
      namespace: 'cert-manager'
    },
    spec: {
      url: 'oci://ghcr.io/controlplaneio-fluxcd/flux-operator-manifests',
      ref: {
        tag: 'latest'
      },
      verify: {
        provider: 'cosign'
      },
      interval: '1h'
    },
    status: {
      artifact: {
        lastUpdateTime: '2025-01-15T12:00:00Z',
        size: 25600,
        revision: 'latest@sha256:abc123def456',
        metadata: {
          'org.opencontainers.image.created': '2025-01-15T10:00:00Z',
          'org.opencontainers.image.revision': 'v1.0.0'
        }
      }
    }
  }

  beforeEach(() => {
    // Reset any state between tests
  })

  describe('Component rendering', () => {
    it('should render the artifact panel with header', () => {
      render(<ArtifactPanel resourceData={mockGitRepository} />)

      expect(screen.getByTestId('artifact-panel')).toBeInTheDocument()
      expect(screen.getByText('Artifact')).toBeInTheDocument()
    })

    it('should be expanded by default', () => {
      render(<ArtifactPanel resourceData={mockGitRepository} />)

      expect(screen.getByText('Overview')).toBeInTheDocument()
      expect(screen.getByRole('button', { name: /artifact/i })).toHaveAttribute('aria-expanded', 'true')
    })

    it('should show Overview tab by default', () => {
      render(<ArtifactPanel resourceData={mockGitRepository} />)

      expect(screen.getByText('Overview')).toBeInTheDocument()
      expect(screen.getByText('Source Type')).toBeInTheDocument()
    })
  })

  describe('Expand/collapse functionality', () => {
    it('should collapse when header is clicked', async () => {
      const user = userEvent.setup()
      render(<ArtifactPanel resourceData={mockGitRepository} />)

      // Initially expanded
      expect(screen.getByText('Overview')).toBeInTheDocument()

      // Click to collapse
      const toggleButton = screen.getByRole('button', { name: /artifact/i })
      await user.click(toggleButton)

      // Content should be hidden
      await waitFor(() => {
        expect(screen.queryByText('Overview')).not.toBeInTheDocument()
      })
    })

    it('should expand when collapsed header is clicked', async () => {
      const user = userEvent.setup()
      render(<ArtifactPanel resourceData={mockGitRepository} />)

      const toggleButton = screen.getByRole('button', { name: /artifact/i })

      // Collapse
      await user.click(toggleButton)
      await waitFor(() => {
        expect(screen.queryByText('Overview')).not.toBeInTheDocument()
      })

      // Expand again
      await user.click(toggleButton)
      await waitFor(() => {
        expect(screen.getByText('Overview')).toBeInTheDocument()
      })
    })
  })

  describe('Overview tab - Source information', () => {
    it('should display source type from kind', () => {
      render(<ArtifactPanel resourceData={mockGitRepository} />)

      expect(screen.getByText('Source Type')).toBeInTheDocument()
      expect(screen.getByText('GitRepository')).toBeInTheDocument()
    })

    it('should display source URL from spec.url', () => {
      render(<ArtifactPanel resourceData={mockGitRepository} />)

      expect(screen.getByText('Source URL')).toBeInTheDocument()
      expect(screen.getByText('https://github.com/example/repo.git')).toBeInTheDocument()
    })

    it('should display source URL from spec.endpoint for Bucket', () => {
      const mockBucket = {
        ...mockGitRepository,
        kind: 'Bucket',
        spec: {
          endpoint: 's3.amazonaws.com',
          bucketName: 'my-bucket',
          interval: '1m'
        }
      }

      render(<ArtifactPanel resourceData={mockBucket} />)

      expect(screen.getByText('Source URL')).toBeInTheDocument()
      expect(screen.getByText('s3.amazonaws.com')).toBeInTheDocument()
    })

    it('should display branch ref', () => {
      render(<ArtifactPanel resourceData={mockGitRepository} />)

      expect(screen.getByText('Source Ref')).toBeInTheDocument()
      expect(screen.getByText('branch: main')).toBeInTheDocument()
    })

    it('should display tag ref', () => {
      render(<ArtifactPanel resourceData={mockOCIRepository} />)

      expect(screen.getByText('Source Ref')).toBeInTheDocument()
      expect(screen.getByText('tag: latest')).toBeInTheDocument()
    })

    it('should display semver ref', () => {
      const mockWithSemver = {
        ...mockGitRepository,
        spec: {
          ...mockGitRepository.spec,
          ref: { semver: '>=1.0.0' }
        }
      }

      render(<ArtifactPanel resourceData={mockWithSemver} />)

      expect(screen.getByText('semver: >=1.0.0')).toBeInTheDocument()
    })

    it('should display commit ref', () => {
      const mockWithCommit = {
        ...mockGitRepository,
        spec: {
          ...mockGitRepository.spec,
          ref: { commit: 'abc123def456' }
        }
      }

      render(<ArtifactPanel resourceData={mockWithCommit} />)

      expect(screen.getByText('commit: abc123def456')).toBeInTheDocument()
    })

    it('should display bucket name for Bucket source', () => {
      const mockBucket = {
        ...mockGitRepository,
        kind: 'Bucket',
        spec: {
          endpoint: 's3.amazonaws.com',
          bucketName: 'my-bucket',
          interval: '1m'
        }
      }

      render(<ArtifactPanel resourceData={mockBucket} />)

      expect(screen.getByText('bucket: my-bucket')).toBeInTheDocument()
    })

    it('should display sourceRef as kind/namespace/name', () => {
      const mockExternalArtifact = {
        apiVersion: 'source.toolkit.fluxcd.io/v1beta2',
        kind: 'ExternalArtifact',
        metadata: {
          name: 'my-artifact',
          namespace: 'default'
        },
        spec: {
          sourceRef: {
            kind: 'HelmRepository',
            name: 'bitnami',
            namespace: 'flux-system'
          },
          interval: '1h'
        },
        status: {
          artifact: {
            lastUpdateTime: '2025-01-15T10:00:00Z',
            size: 1024,
            revision: '1.0.0'
          }
        }
      }

      render(<ArtifactPanel resourceData={mockExternalArtifact} />)

      expect(screen.getByText('HelmRepository/flux-system/bitnami')).toBeInTheDocument()
    })

    it('should use resource namespace as fallback for sourceRef without namespace', () => {
      const mockExternalArtifact = {
        apiVersion: 'source.toolkit.fluxcd.io/v1beta2',
        kind: 'ExternalArtifact',
        metadata: {
          name: 'my-artifact',
          namespace: 'my-namespace'
        },
        spec: {
          sourceRef: {
            kind: 'HelmRepository',
            name: 'bitnami'
            // No namespace specified
          },
          interval: '1h'
        },
        status: {
          artifact: {
            lastUpdateTime: '2025-01-15T10:00:00Z',
            size: 1024,
            revision: '1.0.0'
          }
        }
      }

      render(<ArtifactPanel resourceData={mockExternalArtifact} />)

      expect(screen.getByText('HelmRepository/my-namespace/bitnami')).toBeInTheDocument()
    })

    it('should not display Source Ref when not available', () => {
      const mockNoRef = {
        ...mockGitRepository,
        spec: {
          url: 'https://github.com/example/repo.git',
          interval: '1m'
        }
      }

      render(<ArtifactPanel resourceData={mockNoRef} />)

      expect(screen.queryByText('Source Ref')).not.toBeInTheDocument()
    })
  })

  describe('Overview tab - Signature display', () => {
    it('should display "None" with yellow badge when no verify spec', () => {
      render(<ArtifactPanel resourceData={mockGitRepository} />)

      expect(screen.getByText('Signature')).toBeInTheDocument()
      const badge = screen.getByText('None')
      expect(badge).toBeInTheDocument()
      expect(badge).toHaveClass('bg-yellow-100')
    })

    it('should display provider with green badge when verify.provider exists', () => {
      render(<ArtifactPanel resourceData={mockOCIRepository} />)

      const badge = screen.getByText('cosign')
      expect(badge).toBeInTheDocument()
      expect(badge).toHaveClass('bg-green-100')
    })

    it('should display "pgp" with green badge when verify exists but no provider', () => {
      const mockWithPGP = {
        ...mockGitRepository,
        spec: {
          ...mockGitRepository.spec,
          verify: {
            // No provider specified
          }
        }
      }

      render(<ArtifactPanel resourceData={mockWithPGP} />)

      const badge = screen.getByText('pgp')
      expect(badge).toBeInTheDocument()
      expect(badge).toHaveClass('bg-green-100')
    })
  })

  describe('Overview tab - Artifact information', () => {
    it('should display fetched at timestamp', () => {
      render(<ArtifactPanel resourceData={mockGitRepository} />)

      expect(screen.getByText('Fetched at')).toBeInTheDocument()
      // The exact format depends on locale, but should contain date parts
      const textContent = document.body.textContent
      expect(textContent).toContain('2025')
    })

    it('should display size in KiB', () => {
      render(<ArtifactPanel resourceData={mockGitRepository} />)

      expect(screen.getByText('Size')).toBeInTheDocument()
      expect(screen.getByText('12.75 KiB')).toBeInTheDocument() // 13059 / 1024 = 12.75
    })

    it('should display revision', () => {
      render(<ArtifactPanel resourceData={mockGitRepository} />)

      expect(screen.getByText('Revision')).toBeInTheDocument()
      expect(screen.getByText('refs/heads/main@sha1:abc123')).toBeInTheDocument()
    })

    it('should display "Unavailable" when artifact is missing', () => {
      const mockNoArtifact = {
        ...mockGitRepository,
        status: {}
      }

      render(<ArtifactPanel resourceData={mockNoArtifact} />)

      const unavailableElements = screen.getAllByText('Unavailable')
      expect(unavailableElements.length).toBe(3) // Fetched at, Size, Revision
    })

    it('should display "Unavailable" for missing individual fields', () => {
      const mockPartialArtifact = {
        ...mockGitRepository,
        status: {
          artifact: {
            lastUpdateTime: '2025-01-15T10:00:00Z'
            // Missing size and revision
          }
        }
      }

      render(<ArtifactPanel resourceData={mockPartialArtifact} />)

      // Fetched at should be shown
      const textContent = document.body.textContent
      expect(textContent).toContain('2025')

      // Size and Revision should be Unavailable
      const unavailableElements = screen.getAllByText('Unavailable')
      expect(unavailableElements.length).toBe(2) // Size, Revision
    })
  })

  describe('Metadata tab', () => {
    it('should show Metadata tab when metadata exists', () => {
      render(<ArtifactPanel resourceData={mockOCIRepository} />)

      expect(screen.getByText('Metadata')).toBeInTheDocument()
    })

    it('should not show Metadata tab when no metadata', () => {
      render(<ArtifactPanel resourceData={mockGitRepository} />)

      expect(screen.queryByText('Metadata')).not.toBeInTheDocument()
    })

    it('should not show Metadata tab when metadata is empty object', () => {
      const mockEmptyMetadata = {
        ...mockGitRepository,
        status: {
          artifact: {
            ...mockGitRepository.status.artifact,
            metadata: {}
          }
        }
      }

      render(<ArtifactPanel resourceData={mockEmptyMetadata} />)

      expect(screen.queryByText('Metadata')).not.toBeInTheDocument()
    })

    it('should switch to Metadata tab when clicked', async () => {
      const user = userEvent.setup()
      render(<ArtifactPanel resourceData={mockOCIRepository} />)

      const metadataTab = screen.getByText('Metadata')
      await user.click(metadataTab)

      await waitFor(() => {
        expect(screen.getByText('org.opencontainers.image.created')).toBeInTheDocument()
        expect(screen.getByText('2025-01-15T10:00:00Z')).toBeInTheDocument()
      })
    })

    it('should display all metadata key-value pairs', async () => {
      const user = userEvent.setup()
      render(<ArtifactPanel resourceData={mockOCIRepository} />)

      const metadataTab = screen.getByText('Metadata')
      await user.click(metadataTab)

      await waitFor(() => {
        expect(screen.getByText('org.opencontainers.image.created')).toBeInTheDocument()
        expect(screen.getByText('2025-01-15T10:00:00Z')).toBeInTheDocument()
        expect(screen.getByText('org.opencontainers.image.revision')).toBeInTheDocument()
        expect(screen.getByText('v1.0.0')).toBeInTheDocument()
      })
    })

    it('should switch back to Overview tab', async () => {
      const user = userEvent.setup()
      render(<ArtifactPanel resourceData={mockOCIRepository} />)

      // Switch to Metadata
      await user.click(screen.getByText('Metadata'))
      await waitFor(() => {
        expect(screen.getByText('org.opencontainers.image.created')).toBeInTheDocument()
      })

      // Switch back to Overview
      await user.click(screen.getByText('Overview'))
      await waitFor(() => {
        expect(screen.getByText('Source Type')).toBeInTheDocument()
        expect(screen.queryByText('org.opencontainers.image.created')).not.toBeInTheDocument()
      })
    })
  })

  describe('Edge cases', () => {
    it('should handle null resourceData gracefully', () => {
      render(<ArtifactPanel resourceData={null} />)

      expect(screen.getByTestId('artifact-panel')).toBeInTheDocument()
      expect(screen.getByText('Artifact')).toBeInTheDocument()
    })

    it('should handle undefined resourceData gracefully', () => {
      render(<ArtifactPanel resourceData={undefined} />)

      expect(screen.getByTestId('artifact-panel')).toBeInTheDocument()
    })

    it('should handle missing spec gracefully', () => {
      const mockNoSpec = {
        apiVersion: 'source.toolkit.fluxcd.io/v1',
        kind: 'GitRepository',
        metadata: {
          name: 'test',
          namespace: 'default'
        },
        status: {
          artifact: {
            lastUpdateTime: '2025-01-15T10:00:00Z',
            size: 1024,
            revision: 'abc123'
          }
        }
      }

      render(<ArtifactPanel resourceData={mockNoSpec} />)

      expect(screen.getByText('GitRepository')).toBeInTheDocument()
      expect(screen.queryByText('Source URL')).not.toBeInTheDocument()
      expect(screen.queryByText('Source Ref')).not.toBeInTheDocument()
    })

    it('should handle zero size', () => {
      const mockZeroSize = {
        ...mockGitRepository,
        status: {
          artifact: {
            ...mockGitRepository.status.artifact,
            size: 0
          }
        }
      }

      render(<ArtifactPanel resourceData={mockZeroSize} />)

      expect(screen.getByText('0.00 KiB')).toBeInTheDocument()
    })

    it('should handle large size values', () => {
      const mockLargeSize = {
        ...mockGitRepository,
        status: {
          artifact: {
            ...mockGitRepository.status.artifact,
            size: 10485760 // 10 MiB in bytes
          }
        }
      }

      render(<ArtifactPanel resourceData={mockLargeSize} />)

      expect(screen.getByText('10240.00 KiB')).toBeInTheDocument()
    })
  })

  describe('Mobile display', () => {
    it('should show Info label on mobile (sm:hidden)', () => {
      render(<ArtifactPanel resourceData={mockGitRepository} />)

      // The Info span exists but is hidden on larger screens
      const infoSpan = screen.getByText('Info')
      expect(infoSpan).toHaveClass('sm:hidden')
    })

    it('should show Overview label on desktop (hidden sm:inline)', () => {
      render(<ArtifactPanel resourceData={mockGitRepository} />)

      const overviewSpan = screen.getByText('Overview')
      expect(overviewSpan).toHaveClass('hidden')
      expect(overviewSpan).toHaveClass('sm:inline')
    })
  })
})
