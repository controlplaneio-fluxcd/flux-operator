// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/preact'
import userEvent from '@testing-library/user-event'
import { ExportedInputsPanel } from './ExportedInputsPanel'

describe('ExportedInputsPanel component', () => {
  const mockResourceSetInputProvider = {
    apiVersion: 'fluxcd.controlplane.io/v1',
    kind: 'ResourceSetInputProvider',
    metadata: {
      name: 'github-prs',
      namespace: 'flux-system'
    },
    spec: {
      type: 'GitHubPullRequest',
      url: 'https://github.com/example/repo',
      filter: {
        limit: 50,
        includeBranch: 'feat/*',
        labels: ['deploy', 'preview']
      }
    },
    status: {
      reconcilerRef: {
        lastReconciled: '2025-01-15T10:00:00Z'
      },
      exportedInputs: [
        {
          id: '4',
          author: 'stefanprodan',
          branch: 'kubernetes/helm-set-limits',
          sha: 'bf5d6e01cf802734853f6f3417b237e3ad0ba35d',
          title: 'kubernetes(helm): Add default resources limits'
        },
        {
          id: '3',
          author: 'stefanprodan',
          branch: 'feat/ui-footer',
          sha: '8492c0b5b2094fe720776c8ace1b9690ff258f53',
          title: 'feat(ui): Add footer'
        }
      ]
    }
  }

  const mockStaticProvider = {
    apiVersion: 'fluxcd.controlplane.io/v1',
    kind: 'ResourceSetInputProvider',
    metadata: {
      name: 'static-inputs',
      namespace: 'default'
    },
    spec: {
      type: 'Static'
    },
    status: {
      reconcilerRef: {
        lastReconciled: '2025-01-15T12:00:00Z'
      },
      exportedInputs: []
    }
  }

  const mockOCIProvider = {
    apiVersion: 'fluxcd.controlplane.io/v1',
    kind: 'ResourceSetInputProvider',
    metadata: {
      name: 'oci-tags',
      namespace: 'flux-system'
    },
    spec: {
      type: 'OCIArtifactTag',
      url: 'oci://ghcr.io/example/app',
      filter: {
        semver: '>=1.0.0',
        limit: 10
      },
      skip: {
        labels: ['!approved', 'draft']
      }
    },
    status: {
      reconcilerRef: {
        lastReconciled: '2025-01-15T14:00:00Z'
      },
      exportedInputs: [
        {
          id: '48955639',
          tag: '6.0.4',
          sha: 'sha256:d4ec9861522d4961b2acac5a070ef4f92d732480dff2062c2f3a1dcf9a5d1e91',
          team: {
            name: 'devops-team',
            handle: '@devops'
          }
        }
      ]
    }
  }

  beforeEach(() => {
    // Reset any state between tests
  })

  describe('Component rendering', () => {
    it('should render the exported inputs panel with header', () => {
      render(<ExportedInputsPanel resourceData={mockResourceSetInputProvider} />)

      expect(screen.getByTestId('exported-inputs-panel')).toBeInTheDocument()
      expect(screen.getByText('Exported Inputs')).toBeInTheDocument()
    })

    it('should be expanded by default', () => {
      render(<ExportedInputsPanel resourceData={mockResourceSetInputProvider} />)

      expect(screen.getByText('Overview')).toBeInTheDocument()
      expect(screen.getByRole('button', { name: /exported inputs/i })).toHaveAttribute('aria-expanded', 'true')
    })

    it('should show Overview tab by default', () => {
      render(<ExportedInputsPanel resourceData={mockResourceSetInputProvider} />)

      expect(screen.getByText('Overview')).toBeInTheDocument()
      expect(screen.getByText('Type')).toBeInTheDocument()
    })
  })

  describe('Expand/collapse functionality', () => {
    it('should collapse when header is clicked', async () => {
      const user = userEvent.setup()
      render(<ExportedInputsPanel resourceData={mockResourceSetInputProvider} />)

      // Initially expanded
      expect(screen.getByText('Overview')).toBeInTheDocument()

      // Click to collapse
      const toggleButton = screen.getByRole('button', { name: /exported inputs/i })
      await user.click(toggleButton)

      // Content should be hidden
      await waitFor(() => {
        expect(screen.queryByText('Overview')).not.toBeInTheDocument()
      })
    })

    it('should expand when collapsed header is clicked', async () => {
      const user = userEvent.setup()
      render(<ExportedInputsPanel resourceData={mockResourceSetInputProvider} />)

      const toggleButton = screen.getByRole('button', { name: /exported inputs/i })

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

  describe('Overview tab - Type badge', () => {
    it('should display type with green badge for non-Static types', () => {
      render(<ExportedInputsPanel resourceData={mockResourceSetInputProvider} />)

      expect(screen.getByText('Type')).toBeInTheDocument()
      const badge = screen.getByText('GitHubPullRequest')
      expect(badge).toBeInTheDocument()
      expect(badge).toHaveClass('bg-green-100')
    })

    it('should display type with blue badge for Static type', () => {
      render(<ExportedInputsPanel resourceData={mockStaticProvider} />)

      const badge = screen.getByText('Static')
      expect(badge).toBeInTheDocument()
      expect(badge).toHaveClass('bg-blue-100')
    })
  })

  describe('Overview tab - Source information', () => {
    it('should display source URL from spec.url', () => {
      render(<ExportedInputsPanel resourceData={mockResourceSetInputProvider} />)

      expect(screen.getByText('Source')).toBeInTheDocument()
      expect(screen.getByText('https://github.com/example/repo')).toBeInTheDocument()
    })

    it('should not display Source when url is not present', () => {
      render(<ExportedInputsPanel resourceData={mockStaticProvider} />)

      expect(screen.queryByText('Source')).not.toBeInTheDocument()
    })

    it('should display labels as comma-separated list', () => {
      render(<ExportedInputsPanel resourceData={mockResourceSetInputProvider} />)

      expect(screen.getByText('Labels')).toBeInTheDocument()
      expect(screen.getByText('deploy, preview')).toBeInTheDocument()
    })

    it('should not display Labels when not present', () => {
      render(<ExportedInputsPanel resourceData={mockStaticProvider} />)

      expect(screen.queryByText('Labels')).not.toBeInTheDocument()
    })

    it('should display skip labels as comma-separated list', () => {
      render(<ExportedInputsPanel resourceData={mockOCIProvider} />)

      expect(screen.getByText('Skip')).toBeInTheDocument()
      expect(screen.getByText('!approved, draft')).toBeInTheDocument()
    })

    it('should not display Skip when not present', () => {
      render(<ExportedInputsPanel resourceData={mockResourceSetInputProvider} />)

      expect(screen.queryByText('Skip')).not.toBeInTheDocument()
    })
  })

  describe('Overview tab - Filter fields', () => {
    it('should display includeBranch filter', () => {
      render(<ExportedInputsPanel resourceData={mockResourceSetInputProvider} />)

      expect(screen.getByText('Include branch')).toBeInTheDocument()
      expect(screen.getByText('feat/*')).toBeInTheDocument()
    })

    it('should display semver filter', () => {
      render(<ExportedInputsPanel resourceData={mockOCIProvider} />)

      expect(screen.getByText('Semver')).toBeInTheDocument()
      expect(screen.getByText('>=1.0.0')).toBeInTheDocument()
    })

    it('should display multiple filter fields', () => {
      const mockMultipleFilters = {
        ...mockResourceSetInputProvider,
        spec: {
          ...mockResourceSetInputProvider.spec,
          filter: {
            includeBranch: 'feat/*',
            excludeBranch: 'wip/*',
            includeTag: 'v*',
            excludeTag: '*-rc*'
          }
        }
      }

      render(<ExportedInputsPanel resourceData={mockMultipleFilters} />)

      expect(screen.getByText('Include branch')).toBeInTheDocument()
      expect(screen.getByText('Exclude branch')).toBeInTheDocument()
      expect(screen.getByText('Include tag')).toBeInTheDocument()
      expect(screen.getByText('Exclude tag')).toBeInTheDocument()
    })

    it('should not display filter fields when not present', () => {
      render(<ExportedInputsPanel resourceData={mockStaticProvider} />)

      expect(screen.queryByText('Include branch')).not.toBeInTheDocument()
      expect(screen.queryByText('Semver')).not.toBeInTheDocument()
    })
  })

  describe('Overview tab - Stats', () => {
    it('should display fetched at timestamp', () => {
      render(<ExportedInputsPanel resourceData={mockResourceSetInputProvider} />)

      expect(screen.getByText('Fetched at')).toBeInTheDocument()
      // The exact format depends on locale, but should contain date parts
      const textContent = document.body.textContent
      expect(textContent).toContain('2025')
    })

    it('should display "-" when lastReconciled is not available', () => {
      const mockNoLastReconciled = {
        ...mockResourceSetInputProvider,
        status: {
          ...mockResourceSetInputProvider.status,
          reconcilerRef: {}
        }
      }

      render(<ExportedInputsPanel resourceData={mockNoLastReconciled} />)

      expect(screen.getByText('Fetched at')).toBeInTheDocument()
      expect(screen.getByText('-')).toBeInTheDocument()
    })

    it('should display total exported with custom limit', () => {
      render(<ExportedInputsPanel resourceData={mockResourceSetInputProvider} />)

      expect(screen.getByText('Total exported')).toBeInTheDocument()
      expect(screen.getByText('2 (max 50)')).toBeInTheDocument()
    })

    it('should display total exported with default limit of 100', () => {
      const mockNoLimit = {
        ...mockResourceSetInputProvider,
        spec: {
          ...mockResourceSetInputProvider.spec,
          filter: {
            includeBranch: 'feat/*'
          }
        }
      }

      render(<ExportedInputsPanel resourceData={mockNoLimit} />)

      expect(screen.getByText('2 (max 100)')).toBeInTheDocument()
    })

    it('should display 0 when no exported inputs', () => {
      render(<ExportedInputsPanel resourceData={mockStaticProvider} />)

      expect(screen.getByText('0 (max 100)')).toBeInTheDocument()
    })
  })

  describe('Values tab', () => {
    it('should switch to Values tab when clicked', async () => {
      const user = userEvent.setup()
      render(<ExportedInputsPanel resourceData={mockResourceSetInputProvider} />)

      const valuesTab = screen.getByText('Values')
      await user.click(valuesTab)

      await waitFor(() => {
        expect(screen.getByText('#4')).toBeInTheDocument()
        expect(screen.getByText('#3')).toBeInTheDocument()
      })
    })

    it('should display exported inputs in YAML format', async () => {
      const user = userEvent.setup()
      render(<ExportedInputsPanel resourceData={mockResourceSetInputProvider} />)

      await user.click(screen.getByText('Values'))

      await waitFor(() => {
        // Check that YAML blocks are rendered (content is in pre/code elements)
        const panel = screen.getByTestId('exported-inputs-panel')
        expect(panel.textContent).toContain('stefanprodan')
        expect(panel.textContent).toContain('kubernetes/helm-set-limits')
      })
    })

    it('should display nested objects in YAML format', async () => {
      const user = userEvent.setup()
      render(<ExportedInputsPanel resourceData={mockOCIProvider} />)

      await user.click(screen.getByText('Values'))

      await waitFor(() => {
        expect(screen.getByText('#48955639')).toBeInTheDocument()
        // Nested team object should be rendered in YAML
        const panel = screen.getByTestId('exported-inputs-panel')
        expect(panel.textContent).toContain('devops-team')
      })
    })

    it('should display "No exported inputs available" when empty', async () => {
      const user = userEvent.setup()
      render(<ExportedInputsPanel resourceData={mockStaticProvider} />)

      await user.click(screen.getByText('Values'))

      await waitFor(() => {
        expect(screen.getByText('No exported inputs available')).toBeInTheDocument()
      })
    })

    it('should switch back to Overview tab', async () => {
      const user = userEvent.setup()
      render(<ExportedInputsPanel resourceData={mockResourceSetInputProvider} />)

      // Switch to Values
      await user.click(screen.getByText('Values'))
      await waitFor(() => {
        expect(screen.getByText('#4')).toBeInTheDocument()
      })

      // Switch back to Overview
      await user.click(screen.getByText('Overview'))
      await waitFor(() => {
        expect(screen.getByText('Type')).toBeInTheDocument()
        expect(screen.queryByText('#4')).not.toBeInTheDocument()
      })
    })

    it('should use index as fallback when id is missing', async () => {
      const user = userEvent.setup()
      const mockNoId = {
        ...mockResourceSetInputProvider,
        status: {
          ...mockResourceSetInputProvider.status,
          exportedInputs: [
            { branch: 'main', sha: 'abc123' },
            { branch: 'develop', sha: 'def456' }
          ]
        }
      }

      render(<ExportedInputsPanel resourceData={mockNoId} />)

      await user.click(screen.getByText('Values'))

      await waitFor(() => {
        expect(screen.getByText('#1')).toBeInTheDocument()
        expect(screen.getByText('#2')).toBeInTheDocument()
      })
    })
  })

  describe('Edge cases', () => {
    it('should handle null resourceData gracefully', () => {
      render(<ExportedInputsPanel resourceData={null} />)

      expect(screen.getByTestId('exported-inputs-panel')).toBeInTheDocument()
      expect(screen.getByText('Exported Inputs')).toBeInTheDocument()
    })

    it('should handle undefined resourceData gracefully', () => {
      render(<ExportedInputsPanel resourceData={undefined} />)

      expect(screen.getByTestId('exported-inputs-panel')).toBeInTheDocument()
    })

    it('should handle missing spec gracefully', () => {
      const mockNoSpec = {
        apiVersion: 'fluxcd.controlplane.io/v1',
        kind: 'ResourceSetInputProvider',
        metadata: {
          name: 'test',
          namespace: 'default'
        },
        status: {
          exportedInputs: []
        }
      }

      render(<ExportedInputsPanel resourceData={mockNoSpec} />)

      expect(screen.getByText('Exported Inputs')).toBeInTheDocument()
      expect(screen.queryByText('Type')).not.toBeInTheDocument()
      expect(screen.queryByText('Source')).not.toBeInTheDocument()
    })

    it('should handle missing status gracefully', () => {
      const mockNoStatus = {
        apiVersion: 'fluxcd.controlplane.io/v1',
        kind: 'ResourceSetInputProvider',
        metadata: {
          name: 'test',
          namespace: 'default'
        },
        spec: {
          type: 'GitHubPullRequest',
          url: 'https://github.com/example/repo'
        }
      }

      render(<ExportedInputsPanel resourceData={mockNoStatus} />)

      expect(screen.getByText('GitHubPullRequest')).toBeInTheDocument()
      expect(screen.getByText('0 (max 100)')).toBeInTheDocument()
      expect(screen.getByText('-')).toBeInTheDocument() // Fetched at
    })

    it('should handle empty filter object', () => {
      const mockEmptyFilter = {
        ...mockResourceSetInputProvider,
        spec: {
          ...mockResourceSetInputProvider.spec,
          filter: {}
        }
      }

      render(<ExportedInputsPanel resourceData={mockEmptyFilter} />)

      expect(screen.queryByText('Include branch')).not.toBeInTheDocument()
      expect(screen.queryByText('Semver')).not.toBeInTheDocument()
      expect(screen.getByText('2 (max 100)')).toBeInTheDocument() // Default limit
    })

    it('should handle empty labels array', () => {
      const mockEmptyLabels = {
        ...mockResourceSetInputProvider,
        spec: {
          ...mockResourceSetInputProvider.spec,
          filter: {
            ...mockResourceSetInputProvider.spec.filter,
            labels: []
          }
        }
      }

      render(<ExportedInputsPanel resourceData={mockEmptyLabels} />)

      expect(screen.queryByText('Labels')).not.toBeInTheDocument()
    })

    it('should handle empty skip labels array', () => {
      const mockEmptySkipLabels = {
        ...mockOCIProvider,
        spec: {
          ...mockOCIProvider.spec,
          skip: {
            labels: []
          }
        }
      }

      render(<ExportedInputsPanel resourceData={mockEmptySkipLabels} />)

      expect(screen.queryByText('Skip')).not.toBeInTheDocument()
    })
  })

  describe('Mobile display', () => {
    it('should show Info label on mobile (sm:hidden)', () => {
      render(<ExportedInputsPanel resourceData={mockResourceSetInputProvider} />)

      // The Info span exists but is hidden on larger screens
      const infoSpan = screen.getByText('Info')
      expect(infoSpan).toHaveClass('sm:hidden')
    })

    it('should show Overview label on desktop (hidden sm:inline)', () => {
      render(<ExportedInputsPanel resourceData={mockResourceSetInputProvider} />)

      const overviewSpan = screen.getByText('Overview')
      expect(overviewSpan).toHaveClass('hidden')
      expect(overviewSpan).toHaveClass('sm:inline')
    })
  })
})
