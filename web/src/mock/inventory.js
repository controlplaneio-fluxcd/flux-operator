// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

// Mock data for the inventory objects endpoint (POST /api/v1/inventory/objects).
// The backend returns, per requested item, a status + a sanitized manifest (Secret
// data masked, runtime metadata stripped). The mock synthesizes a plausible object
// from each requested item so any inventory row can be expanded in dev mode.

import { isFluxInventoryItem, isWorkloadInventoryItem } from '../utils/constants'

// synthStatus returns a plausible {status, statusMessage} for the item, mirroring
// the backend's hybrid computeObjectStatus (Flux Ready / workload rollout / kstatus).
function synthStatus(item) {
  if (isFluxInventoryItem(item)) return { status: 'Ready', statusMessage: 'Applied revision: main@sha1:9b9218f' }
  if (item.kind === 'CronJob') return { status: 'Idle', statusMessage: '0 0 * * *' }
  if (isWorkloadInventoryItem(item)) return { status: 'Current', statusMessage: 'Replicas: 1' }
  return { status: 'Current', statusMessage: '' }
}

// synthObject builds a sanitized manifest for the item: always apiVersion/kind/
// metadata, plus a body appropriate to the kind (spec, data, or rules) and a status
// only for kinds that carry one (Flux + workloads).
function synthObject(item) {
  const metadata = {
    name: item.name,
    labels: { 'app.kubernetes.io/managed-by': 'flux' }
  }
  if (item.namespace) metadata.namespace = item.namespace

  const obj = { apiVersion: item.apiVersion, kind: item.kind, metadata }

  if (isFluxInventoryItem(item)) {
    obj.spec = { interval: '10m', prune: true }
    obj.status = {
      conditions: [
        { type: 'Ready', status: 'True', reason: 'ReconciliationSucceeded', message: 'Applied revision: main@sha1:9b9218f' }
      ]
    }
    return obj
  }

  if (isWorkloadInventoryItem(item)) {
    if (item.kind === 'CronJob') {
      obj.spec = { schedule: '0 0 * * *', jobTemplate: { spec: { template: { spec: { containers: [{ name: 'job', image: 'busybox:1.36' }] } } } } }
      obj.status = { lastScheduleTime: '2026-06-29T00:00:00Z' }
    } else {
      obj.spec = {
        replicas: 1,
        selector: { matchLabels: { app: item.name } },
        template: { metadata: { labels: { app: item.name } }, spec: { containers: [{ name: 'app', image: 'nginx:1.27' }] } }
      }
      obj.status = { replicas: 1, readyReplicas: 1, availableReplicas: 1 }
    }
    return obj
  }

  // Spec-less kinds: the detail view's Spec tab falls back to the object body.
  switch (item.kind) {
  case 'ConfigMap':
    obj.data = { 'app.properties': 'color=blue\nreplicas=1' }
    break
  case 'Secret':
    // Masked by the backend's maskSecretValues; the mock mirrors the result.
    obj.data = { token: '***redacted***', password: '***redacted***' }
    break
  case 'ClusterRole':
  case 'Role':
    obj.rules = [{ apiGroups: [''], resources: ['pods'], verbs: ['get', 'list', 'watch'] }]
    break
  case 'CustomResourceDefinition':
    obj.spec = { group: 'source.toolkit.fluxcd.io', scope: 'Namespaced', names: { kind: 'GitRepository', plural: 'gitrepositories' } }
    break
  default:
    obj.spec = {}
  }
  return obj
}

/**
 * getMockInventoryObjects - mock for POST /api/v1/inventory/objects. Called by
 * fetchWithMock with the request body. Returns one result per requested item.
 * Names containing "missing" simulate a pruned object (NotFound) so the detail
 * view's not-found state is reachable in dev mode.
 *
 * @param {object} body - Request body with an `objects` array
 * @returns {{objects: Array<object>}}
 */
export function getMockInventoryObjects(body) {
  const items = body?.objects || []
  const objects = items.map((item) => {
    const base = {
      apiVersion: item.apiVersion,
      kind: item.kind,
      namespace: item.namespace,
      name: item.name
    }
    if ((item.name || '').includes('missing')) {
      return { ...base, error: 'NotFound' }
    }
    const { status, statusMessage } = synthStatus(item)
    return { ...base, status, statusMessage, object: synthObject(item) }
  })
  return { objects }
}
