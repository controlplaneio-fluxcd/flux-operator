// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/preact'
import { ProfilePage } from './ProfilePage'
import { reportData } from '../../app'
import { favorites } from '../../utils/favorites'
import { navHistory } from '../../utils/navHistory'

// Mock the favorites module
vi.mock('../../utils/favorites', async () => {
  const { signal } = await import('@preact/signals')
  return {
    favorites: signal([])
  }
})

// Mock the navHistory module
vi.mock('../../utils/navHistory', async () => {
  const { signal } = await import('@preact/signals')
  return {
    navHistory: signal([])
  }
})

describe('ProfilePage', () => {
  beforeEach(() => {
    // Reset mocks
    vi.clearAllMocks()

    // Reset favorites and navHistory signals
    favorites.value = []
    navHistory.value = []

    // Set mock user info in reportData
    reportData.value = {
      spec: {
        userInfo: {
          username: 'test-user'
        }
      }
    }
  })

  describe('Header Section', () => {
    it('should render username', () => {
      render(<ProfilePage />)

      expect(screen.getByText('test-user')).toBeInTheDocument()
    })

    it('should display Profile label', () => {
      render(<ProfilePage />)

      expect(screen.getByText('Profile')).toBeInTheDocument()
    })

    it('should show fallback when no username', () => {
      reportData.value = {
        spec: {
          userInfo: {}
        }
      }

      render(<ProfilePage />)

      expect(screen.getByText('unknown')).toBeInTheDocument()
    })

  })

  describe('Identity Panel', () => {
    it('should render Identity panel title', () => {
      render(<ProfilePage />)

      expect(screen.getByText('Identity')).toBeInTheDocument()
    })

    it('should show Overview tabs by default', () => {
      render(<ProfilePage />)

      // There are two Overview tabs (Identity and Local Storage panels)
      const overviewTabs = screen.getAllByText('Overview')
      expect(overviewTabs.length).toBe(2)
    })

    it('should show Kubernetes RBAC as Disabled when no impersonation', () => {
      render(<ProfilePage />)

      expect(screen.getByText('Kubernetes RBAC')).toBeInTheDocument()
      // Both impersonation and SSO are disabled, so there are two Disabled badges
      const disabledBadges = screen.getAllByText('Disabled')
      expect(disabledBadges.length).toBeGreaterThanOrEqual(1)
    })

    it('should show Single Sign-On as Disabled when no provider', () => {
      render(<ProfilePage />)

      expect(screen.getByText('Single Sign-On')).toBeInTheDocument()
      // There are two Disabled badges (impersonation and SSO)
      const disabledBadges = screen.getAllByText('Disabled')
      expect(disabledBadges.length).toBe(2)
    })

    it('should show Kubernetes RBAC as Enabled when impersonation exists', () => {
      reportData.value = {
        spec: {
          userInfo: {
            username: 'test-user',
            impersonation: {
              username: 'user@example.com',
              groups: ['fluxcd:maintainers']
            }
          }
        }
      }

      render(<ProfilePage />)

      expect(screen.getByText('Kubernetes RBAC')).toBeInTheDocument()
      expect(screen.getByText('Enabled')).toBeInTheDocument()
    })

    it('should show Single Sign-On as Enabled when provider exists', () => {
      reportData.value = {
        spec: {
          userInfo: {
            username: 'test-user',
            provider: {
              iss: 'https://accounts.example.com',
              sub: '1234567890'
            }
          }
        }
      }

      render(<ProfilePage />)

      expect(screen.getByText('Single Sign-On')).toBeInTheDocument()
      expect(screen.getByText('Enabled')).toBeInTheDocument()
    })
  })

  describe('Identity Panel - Kubernetes Tab', () => {
    it('should show Kubernetes tab when impersonation exists', () => {
      reportData.value = {
        spec: {
          userInfo: {
            username: 'test-user',
            impersonation: {
              username: 'user@example.com',
              groups: ['fluxcd:maintainers', 'fluxcd:developers']
            }
          }
        }
      }

      render(<ProfilePage />)

      expect(screen.getByText('Kubernetes')).toBeInTheDocument()
    })

    it('should not show Kubernetes tab when impersonation is missing', () => {
      reportData.value = {
        spec: {
          userInfo: {
            username: 'test-user'
          }
        }
      }

      render(<ProfilePage />)

      expect(screen.queryByText('Kubernetes')).not.toBeInTheDocument()
    })

    it('should show impersonation JSON when Kubernetes tab is clicked', () => {
      reportData.value = {
        spec: {
          userInfo: {
            username: 'test-user',
            impersonation: {
              username: 'user@example.com',
              groups: ['fluxcd:maintainers', 'fluxcd:developers']
            }
          }
        }
      }

      render(<ProfilePage />)

      // Click Kubernetes tab
      fireEvent.click(screen.getByText('Kubernetes'))

      // Check that the JSON is rendered (content will be syntax highlighted)
      expect(screen.getByText(/user@example\.com/)).toBeInTheDocument()
      expect(screen.getByText(/fluxcd:maintainers/)).toBeInTheDocument()
    })

    it('should show impersonation groups in JSON when username is missing', () => {
      reportData.value = {
        spec: {
          userInfo: {
            username: 'test-user',
            impersonation: {
              groups: ['fluxcd:maintainers']
            }
          }
        }
      }

      render(<ProfilePage />)

      // Click Kubernetes tab
      fireEvent.click(screen.getByText('Kubernetes'))

      // The JSON should still show the groups
      expect(screen.getByText(/fluxcd:maintainers/)).toBeInTheDocument()
    })
  })

  describe('Identity Panel - SSO Tab', () => {
    it('should show SSO tab when provider exists', () => {
      reportData.value = {
        spec: {
          userInfo: {
            username: 'test-user',
            provider: {
              iss: 'https://accounts.example.com',
              sub: '1234567890',
              email: 'user@example.com'
            }
          }
        }
      }

      render(<ProfilePage />)

      expect(screen.getByText('SSO')).toBeInTheDocument()
    })

    it('should not show SSO tab when provider is missing', () => {
      reportData.value = {
        spec: {
          userInfo: {
            username: 'test-user'
          }
        }
      }

      render(<ProfilePage />)

      expect(screen.queryByText('SSO')).not.toBeInTheDocument()
    })

    it('should show provider JSON when SSO tab is clicked', () => {
      reportData.value = {
        spec: {
          userInfo: {
            username: 'test-user',
            provider: {
              iss: 'https://accounts.example.com',
              sub: '1234567890',
              email: 'user@example.com'
            }
          }
        }
      }

      render(<ProfilePage />)

      // Click SSO tab
      fireEvent.click(screen.getByText('SSO'))

      // Check that the JSON is rendered (content will be syntax highlighted)
      expect(screen.getByText(/accounts\.example\.com/)).toBeInTheDocument()
    })
  })

  describe('Local Storage Section', () => {
    it('should display favorites count', () => {
      favorites.value = [
        { kind: 'FluxInstance', namespace: 'flux-system', name: 'flux' },
        { kind: 'Kustomization', namespace: 'default', name: 'app' }
      ]

      render(<ProfilePage />)

      expect(screen.getByText('2 items')).toBeInTheDocument()
    })

    it('should display singular "item" for one favorite', () => {
      favorites.value = [
        { kind: 'FluxInstance', namespace: 'flux-system', name: 'flux' }
      ]

      render(<ProfilePage />)

      expect(screen.getByText('1 item')).toBeInTheDocument()
    })

    it('should display navigation history count', () => {
      navHistory.value = [
        { kind: 'FluxInstance', namespace: 'flux-system', name: 'flux' },
        { kind: 'Kustomization', namespace: 'default', name: 'app' },
        { kind: 'HelmRelease', namespace: 'default', name: 'nginx' }
      ]

      render(<ProfilePage />)

      expect(screen.getByText('3 items')).toBeInTheDocument()
    })

    it('should display 0 items when no favorites or history', () => {
      favorites.value = []
      navHistory.value = []

      render(<ProfilePage />)

      const zeroItems = screen.getAllByText('0 items')
      expect(zeroItems.length).toBe(2) // One for favorites, one for history
    })

    it('should show Local Storage section header', () => {
      render(<ProfilePage />)

      expect(screen.getByText('Local Storage')).toBeInTheDocument()
    })

    it('should show Favorites label', () => {
      render(<ProfilePage />)

      expect(screen.getByText('Favorites')).toBeInTheDocument()
    })

    it('should show Navigation History label', () => {
      render(<ProfilePage />)

      expect(screen.getByText('Navigation History')).toBeInTheDocument()
    })
  })

  describe('Complete User Info', () => {
    it('should render all sections when all data is present', () => {
      reportData.value = {
        spec: {
          userInfo: {
            username: 'Flux User',
            impersonation: {
              username: 'user@example.com',
              groups: ['fluxcd:maintainers']
            },
            provider: {
              iss: 'https://accounts.example.com',
              sub: '1234567890',
              email: 'user@example.com'
            }
          }
        }
      }

      favorites.value = [{ kind: 'FluxInstance', namespace: 'flux-system', name: 'flux' }]
      navHistory.value = [{ kind: 'Kustomization', namespace: 'default', name: 'app' }]

      render(<ProfilePage />)

      // Header section
      expect(screen.getByText('Profile')).toBeInTheDocument()
      expect(screen.getByText('Flux User')).toBeInTheDocument()

      // Identity panel with tabs
      expect(screen.getByText('Identity')).toBeInTheDocument()
      expect(screen.getAllByText('Overview').length).toBe(2) // Identity and Local Storage panels
      expect(screen.getByText('Kubernetes')).toBeInTheDocument()
      expect(screen.getByText('SSO')).toBeInTheDocument()

      // Overview tab shows both as Enabled
      const enabledBadges = screen.getAllByText('Enabled')
      expect(enabledBadges.length).toBe(2)

      // Local Storage section
      expect(screen.getByText('Local Storage')).toBeInTheDocument()
      expect(screen.getAllByText('1 item').length).toBe(2) // favorites and history
    })
  })
})
