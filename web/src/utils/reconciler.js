// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0
//
// Shared derivation of a Flux resource's effective reconcile interval and
// timeout, applying the same spec → annotation → per-kind default policy used by
// both the resource dashboard Reconciler panel and the compact resource detail
// panel, so the two never drift.

/**
 * getReconcilerSummary - the reconciler status/message/lastReconciled triple for a
 * resource, preferring the server-computed status.reconcilerRef and falling back to
 * the first status condition. Shared by the resource dashboard Reconciler panel and
 * the compact resource detail panel so the two never drift.
 *
 * @param {Object} resourceData - The full resource object (status.reconcilerRef, status.conditions)
 * @returns {{ref: Object|undefined, status: string, message: string, lastReconciled: string|undefined}}
 */
export function getReconcilerSummary(resourceData) {
  const ref = resourceData?.status?.reconcilerRef
  const firstCondition = resourceData?.status?.conditions?.[0]
  return {
    ref,
    status: ref?.status || 'Unknown',
    message: ref?.message || firstCondition?.message || '',
    lastReconciled: ref?.lastReconciled || firstCondition?.lastTransitionTime
  }
}

/**
 * getReconcileInterval - the effective reconcile interval for a resource:
 * spec.interval, else the reconcileEvery annotation, else the per-kind default
 * (60m for FluxInstance/ResourceSet, 10m for ResourceSetInputProvider), else null.
 *
 * @param {Object} resourceData - The full resource object (apiVersion, kind, spec, metadata)
 * @returns {string|null}
 */
export function getReconcileInterval(resourceData) {
  if (!resourceData) return null

  if (resourceData.spec?.interval) return resourceData.spec.interval

  const annotation = resourceData.metadata?.annotations?.['fluxcd.controlplane.io/reconcileEvery']
  if (annotation) return annotation

  const k = resourceData.kind
  if (k === 'FluxInstance' || k === 'ResourceSet') return '60m'
  if (k === 'ResourceSetInputProvider') return '10m'

  return null
}

/**
 * getReconcileTimeout - the effective reconcile timeout for a resource:
 * spec.timeout, else a per-kind default (1m for source.toolkit kinds, the
 * interval for Kustomization, 5m for HelmRelease, the reconcileTimeout annotation
 * or 5m for FluxInstance/ResourceSet/ResourceSetInputProvider), else null.
 *
 * @param {Object} resourceData - The full resource object
 * @returns {string|null}
 */
export function getReconcileTimeout(resourceData) {
  if (!resourceData) return null

  const k = resourceData.kind
  const spec = resourceData.spec || {}
  const annotations = resourceData.metadata?.annotations || {}

  if (spec.timeout) return spec.timeout
  if (resourceData.apiVersion && resourceData.apiVersion.startsWith('source.toolkit.fluxcd.io')) return '1m'
  if (k === 'Kustomization') return spec.interval || null
  if (k === 'HelmRelease') return '5m'
  if (k === 'FluxInstance' || k === 'ResourceSet' || k === 'ResourceSetInputProvider') {
    return annotations['fluxcd.controlplane.io/reconcileTimeout'] || '5m'
  }

  return null
}
