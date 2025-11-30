// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useEffect } from 'preact/hooks'
import { usePageTitle } from '../../../utils/title'
import { addToNavHistory } from '../../../utils/navHistory'
import { OverallStatusPanel } from './OverallStatusPanel'
import { InfoPanel } from './InfoPanel'
import { SyncPanel } from './SyncPanel'
import { ControllersPanel } from './ControllersPanel'
import { ReconcilersPanel } from './ReconcilersPanel'
import { Footer } from '../../layout/Footer'

/**
 * ClusterPage component - Main dashboard displaying cluster status and resources
 *
 * @param {Object} props
 * @param {Object} props.spec - FluxReport spec containing cluster, components, reconcilers, etc.
 * @param {string} props.namespace - FluxReport namespace
 */
export function ClusterPage({ spec, namespace }) {
  usePageTitle(null) // Home page uses default title

  // Track home page visit in navigation history
  useEffect(() => {
    addToNavHistory('FluxReport', namespace, 'flux')
  }, [namespace])

  return (
    <>
      <main class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8 flex-grow w-full">
        <div class="space-y-6">
          <OverallStatusPanel report={spec} />

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
            <SyncPanel sync={spec.sync} namespace={namespace} />
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
