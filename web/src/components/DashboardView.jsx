// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { ClusterStatus } from './ClusterStatus'
import { ClusterInfo } from './ClusterInfo'
import { ClusterSync } from './ClusterSync'
import { ComponentList } from './ComponentList'
import { ReconcilerList } from './ReconcilerList'
import { Footer } from './Footer'

/**
 * DashboardView component - Main dashboard displaying cluster status and resources
 *
 * @param {Object} props
 * @param {Object} props.spec - FluxReport spec containing cluster, components, reconcilers, etc.
 */
export function DashboardView({ spec }) {
  return (
    <>
      <main class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8 flex-grow w-full">
        <div class="space-y-6">
          <ClusterStatus report={spec} />

          {spec.operator && (
            <ClusterInfo
              cluster={spec.cluster}
              distribution={spec.distribution}
              operator={spec.operator}
              components={spec.components}
              metrics={spec.metrics}
            />
          )}

          {spec.sync && (
            <ClusterSync sync={spec.sync} />
          )}

          {spec.components && spec.components.length > 0 && (
            <ComponentList components={spec.components} metrics={spec.metrics} />
          )}

          {spec.reconcilers && spec.reconcilers.length > 0 && (
            <ReconcilerList reconcilers={spec.reconcilers} />
          )}
        </div>
      </main>

      {/* Footer with links and license info */}
      <Footer />
    </>
  )
}
