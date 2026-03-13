// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useEffect } from 'preact/hooks'
import { usePageMeta } from '../../../utils/meta'
import { namespaceFilteringDocUrl } from '../../../utils/constants'
import { addToNavHistory } from '../../../utils/navHistory'
import { OverallStatusPanel } from './OverallStatusPanel'
import { InfoPanel } from './InfoPanel'
import { SyncPanel } from './SyncPanel'
import { ControllersPanel } from './ControllersPanel'
import { ReconcilersPanel } from './ReconcilersPanel'
import { Footer } from '../../layout/Footer'

/**
 * Warning panel displayed when the user has no access to any namespaces
 */
function NoNamespaceAccessWarning({ userInfo }) {
  return (
    <div class="card border-warning">
      <div class="flex items-start gap-3">
        <svg class="w-5 h-5 text-yellow-500 dark:text-yellow-400 flex-shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
        <div>
          <h3 class="text-sm font-medium text-yellow-800 dark:text-yellow-200">
            Limited Access
          </h3>
          <p class="mt-1 text-sm text-yellow-700 dark:text-yellow-300">
            You don't have access to any namespaces. Contact your administrator to grant your group the necessary permissions.
            <a
              href={namespaceFilteringDocUrl}
              target="_blank"
              rel="noopener noreferrer"
              onClick={(e) => e.stopPropagation()}
              class="inline-flex items-center align-middle text-yellow-700 underline decoration-yellow-500/70 underline-offset-2 transition-colors hover:text-yellow-800 dark:text-yellow-300 dark:hover:text-yellow-200"
              title="Namespace filtering documentation"
              aria-label="Namespace filtering documentation"
            >
              <svg class="ml-px h-4 w-4 flex-shrink-0 relative -top-px" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
              </svg>
            </a>
          </p>
          {(userInfo?.impersonation) && (userInfo?.impersonation.username || userInfo?.impersonation.groups) && (
            <p class="mt-2 text-xs text-yellow-600 dark:text-yellow-400 font-mono">
              {userInfo.impersonation.username && `User: ${userInfo.impersonation.username}`}
              {userInfo.impersonation.username && userInfo.impersonation.groups && ' · '}
              {userInfo.impersonation.groups && `Groups: ${userInfo.impersonation.groups.join(', ')}`}
            </p>
          )}
        </div>
      </div>
    </div>
  )
}

/**
 * ClusterPage component - Main dashboard displaying cluster status and resources
 *
 * @param {Object} props
 * @param {Object} props.spec - FluxReport spec containing cluster, components, reconcilers, etc.
 * @param {string} props.namespace - FluxReport namespace
 */
export function ClusterPage({ spec, namespace }) {
  usePageMeta(null, null) // Home page uses default title and description

  // Track home page visit in navigation history
  useEffect(() => {
    addToNavHistory('FluxReport', namespace, 'flux')
  }, [namespace])

  return (
    <>
      <main class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8 flex-grow w-full">
        <div class="space-y-6">
          <OverallStatusPanel report={spec} />

          {(!spec.namespaces || spec.namespaces.length === 0) && (
            <NoNamespaceAccessWarning userInfo={spec.userInfo} />
          )}

          {spec.operator && (
            <InfoPanel
              cluster={spec.cluster}
              distribution={spec.distribution}
              operator={spec.operator}
              components={spec.components}
              metrics={spec.metrics}
            />
          )}

          {spec.sync && (
            <SyncPanel sync={spec.sync} namespace={namespace} namespaces={spec.namespaces} />
          )}

          {spec.components && spec.components.length > 0 && (
            <ControllersPanel components={spec.components} metrics={spec.metrics} />
          )}

          {spec.reconcilers && spec.reconcilers.length > 0 && (
            <ReconcilersPanel reconcilers={spec.reconcilers} />
          )}
        </div>
      </main>

      {/* Footer with links and license info */}
      <Footer />
    </>
  )
}
