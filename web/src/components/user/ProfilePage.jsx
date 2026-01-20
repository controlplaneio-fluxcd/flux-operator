// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useMemo } from 'preact/hooks'
import { useState } from 'preact/hooks'
import { reportData } from '../../app'
import { usePageMeta } from '../../utils/meta'
import { formatTime } from '../../utils/time'
import { usePrismTheme } from '../dashboards/common/yaml'
import { DashboardPanel, TabButton } from '../dashboards/common/panel'
import { KubernetesIcon, OpenIDIcon } from '../layout/Icons'
import { favorites } from '../../utils/favorites'
import { navHistory } from '../../utils/navHistory'
import Prism from 'prismjs'
import 'prismjs/components/prism-json'

/**
 * ProfilePage component - User profile page displaying identity information
 *
 * Features:
 * - Header with user icon, username, and role
 * - Identity panel with tabs (Overview, Kubernetes, SSO)
 * - Local Storage section with favorites/history counts and clear data button
 */
export function ProfilePage() {
  usePageMeta('Profile', 'User profile and identity information')
  usePrismTheme()

  const userInfo = reportData.value?.spec?.userInfo

  // Tab state for Identity panel
  const [activeTab, setActiveTab] = useState('overview')

  // Check if features are enabled
  const hasImpersonation = !!userInfo?.impersonation
  const hasProvider = !!userInfo?.provider

  // Highlight impersonation JSON with Prism
  const highlightedImpersonation = useMemo(() => {
    if (!userInfo?.impersonation) return null
    const jsonStr = JSON.stringify(userInfo.impersonation, null, 2)
    return Prism.highlight(jsonStr, Prism.languages.json, 'json')
  }, [userInfo?.impersonation])

  // Highlight provider JSON with Prism
  const highlightedProvider = useMemo(() => {
    if (!userInfo?.provider) return null
    const jsonStr = JSON.stringify(userInfo.provider, null, 2)
    return Prism.highlight(jsonStr, Prism.languages.json, 'json')
  }, [userInfo?.provider])

  // Calculate local storage size
  const storageSize = useMemo(() => {
    try {
      const favoritesSize = (localStorage.getItem('favorites') || '').length * 2 // UTF-16
      const navHistorySize = (localStorage.getItem('nav-history') || '').length * 2
      const totalBytes = favoritesSize + navHistorySize
      if (totalBytes < 1024) return `${totalBytes} B`
      if (totalBytes < 1024 * 1024) return `${(totalBytes / 1024).toFixed(1)} KB`
      return `${(totalBytes / (1024 * 1024)).toFixed(1)} MB`
    } catch {
      return '0 B'
    }
  }, [favorites.value, navHistory.value])

  return (
    <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8 flex-grow w-full">
      <div class="space-y-6">
        {/* Header Card */}
        <div class="card bg-blue-50 dark:bg-opacity-20 border-2 border-flux-blue">
          <div class="flex items-center gap-4">
            {/* User icon in circle */}
            <div class="w-16 h-16 rounded-full flex items-center justify-center flex-shrink-0 bg-flux-blue/10 dark:bg-flux-blue/20">
              <svg class="w-8 h-8 text-flux-blue dark:text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
              </svg>
            </div>
            <div class="flex flex-col min-w-0 flex-1">
              <span class="text-sm font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wide">
                Profile
              </span>
              <h1 class="text-xl font-semibold text-gray-900 dark:text-white truncate">
                {userInfo?.username || 'unknown'}
              </h1>
            </div>
            {/* Session Started - only show when provider has iat claim */}
            {userInfo?.provider?.iat && (
              <div class="hidden md:block text-right flex-shrink-0">
                <div class="text-sm text-gray-600 dark:text-gray-400">Session Started</div>
                <div class="text-lg font-semibold text-gray-900 dark:text-white">{formatTime(new Date(userInfo.provider.iat * 1000))}</div>
              </div>
            )}
          </div>
        </div>

        {/* Identity Panel with Tabs */}
        <DashboardPanel title="Identity" id="identity-panel">
          {/* Tab Navigation */}
          <div class="border-b border-gray-200 dark:border-gray-700 mb-4">
            <nav class="flex space-x-4">
              <TabButton active={activeTab === 'overview'} onClick={() => setActiveTab('overview')}>
                Overview
              </TabButton>
              {hasImpersonation && (
                <TabButton active={activeTab === 'kubernetes'} onClick={() => setActiveTab('kubernetes')}>
                  Kubernetes
                </TabButton>
              )}
              {hasProvider && (
                <TabButton active={activeTab === 'sso'} onClick={() => setActiveTab('sso')}>
                  SSO
                </TabButton>
              )}
            </nav>
          </div>

          {/* Tab Content */}
          {activeTab === 'overview' && (
            <div class="space-y-4">
              {/* Kubernetes RBAC status */}
              <div class="text-sm">
                <span class="text-gray-500 dark:text-gray-400">Kubernetes RBAC</span>
                <span class={`ml-1 inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                  hasImpersonation
                    ? 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400'
                    : 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400'
                }`}>
                  {hasImpersonation ? 'Enabled' : 'Disabled'}
                </span>
              </div>

              {/* Single Sign-On status */}
              <div class="text-sm">
                <span class="text-gray-500 dark:text-gray-400">Single Sign-On</span>
                <span class={`ml-1 inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                  hasProvider
                    ? 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400'
                    : 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400'
                }`}>
                  {hasProvider ? 'Enabled' : 'Disabled'}
                </span>
              </div>
            </div>
          )}

          {activeTab === 'kubernetes' && hasImpersonation && (
            <div>
              <div class="flex items-center gap-2 mb-3">
                <KubernetesIcon className="w-5 h-5 text-gray-500 dark:text-gray-400" />
                <span class="text-sm font-medium text-gray-700 dark:text-gray-300">Impersonation config</span>
              </div>
              <pre class="p-3 bg-gray-50 dark:bg-gray-900 rounded-md overflow-x-auto language-json" style="font-size: 12px; line-height: 1.5;">
                <code class="language-json" style="font-size: 12px;" dangerouslySetInnerHTML={{ __html: highlightedImpersonation }} />
              </pre>
            </div>
          )}

          {activeTab === 'sso' && hasProvider && (
            <div>
              <div class="flex items-center gap-2 mb-3">
                <OpenIDIcon className="w-5 h-5 text-gray-500 dark:text-gray-400" />
                <span class="text-sm font-medium text-gray-700 dark:text-gray-300">Provider claims</span>
              </div>
              <pre class="p-3 bg-gray-50 dark:bg-gray-900 rounded-md overflow-x-auto language-json" style="font-size: 12px; line-height: 1.5;">
                <code class="language-json" style="font-size: 12px;" dangerouslySetInnerHTML={{ __html: highlightedProvider }} />
              </pre>
            </div>
          )}
        </DashboardPanel>

        {/* Local Storage Panel */}
        <DashboardPanel title="Local Storage" id="storage-panel">
          {/* Tab Navigation */}
          <div class="border-b border-gray-200 dark:border-gray-700 mb-4">
            <nav class="flex space-x-4">
              <TabButton active={true} onClick={() => {}}>
                Overview
              </TabButton>
            </nav>
          </div>

          {/* Overview Content */}
          <div class="space-y-4">
            <div class="text-sm">
              <span class="text-gray-500 dark:text-gray-400">Favorites</span>
              <span class="ml-1 text-gray-900 dark:text-white">
                {favorites.value.length} {favorites.value.length === 1 ? 'item' : 'items'}
              </span>
            </div>
            <div class="text-sm">
              <span class="text-gray-500 dark:text-gray-400">Navigation History</span>
              <span class="ml-1 text-gray-900 dark:text-white">
                {navHistory.value.length} {navHistory.value.length === 1 ? 'item' : 'items'}
              </span>
            </div>
            <div class="text-sm">
              <span class="text-gray-500 dark:text-gray-400">Storage Size</span>
              <span class="ml-1 text-gray-900 dark:text-white">{storageSize}</span>
            </div>
          </div>
        </DashboardPanel>
      </div>
    </div>
  )
}
