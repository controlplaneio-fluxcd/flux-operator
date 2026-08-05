// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

// Utilities for shaping and formatting the CPU/Memory usage metrics
// served by the workload API (workloadInfo.metrics and pod.metrics).

/**
 * Format a CPU value expressed in cores for display.
 * Values below one core are shown as millicores (kubectl top style).
 * @param {number} cores - CPU usage in cores
 * @returns {string} - Formatted value, e.g. "120m" or "1.25"
 */
export function formatCores(cores) {
  if (typeof cores !== 'number' || !isFinite(cores) || cores <= 0) return '0m'
  if (cores < 1) {
    const millicores = Math.round(cores * 1000)
    // Values rounding up to a full core fall through to the cores format.
    if (millicores < 1000) return `${millicores}m`
  }
  return cores >= 10 ? cores.toFixed(1) : cores.toFixed(2)
}

/**
 * Format a memory value expressed in bytes for display.
 * @param {number} bytes - Memory usage in bytes
 * @returns {string} - Formatted value, e.g. "128 MiB" or "1.2 GiB"
 */
export function formatBytes(bytes) {
  if (typeof bytes !== 'number' || !isFinite(bytes) || bytes <= 0) return '0'
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB']
  let value = bytes
  let unit = 0
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024
    unit++
  }
  const digits = value >= 100 || Number.isInteger(value) ? 0 : value >= 10 ? 1 : 2
  return `${value.toFixed(digits)} ${units[unit]}`
}

/**
 * Percentage of a value relative to a denominator with double artifacts
 * rounded away (0.18/0.2 evaluates to 89.999...96) and negative usage
 * clamped to zero. Shared by the display and severity paths so the
 * shown percentage and its color always agree.
 */
function ratioPercent(value, denominator) {
  return Math.max(0, Math.round((value / denominator) * 100 * 1e6) / 1e6)
}

/**
 * Compute the percentage of a usage value relative to a denominator.
 * Returns null when the denominator is not set (zero) or either
 * side is not a finite number. The percentage is truncated, not
 * rounded, so the displayed value stays consistent with the severity
 * thresholds: 89.5% must not display as "90%" next to a non-critical
 * color.
 * @param {number} value - Usage value
 * @param {number} denominator - Requests or limits value, 0 when unset
 * @returns {number|null} - Truncated percentage or null
 */
export function percentOf(value, denominator) {
  if (typeof value !== 'number' || !isFinite(value)) return null
  if (typeof denominator !== 'number' || !isFinite(denominator) || denominator <= 0) return null
  return Math.floor(ratioPercent(value, denominator))
}

/**
 * Build the secondary percentage text for a usage value, e.g.
 * "12% of request · 6% of limit". Returns null when neither
 * requests nor limits are set.
 * @param {number} value - Current usage value
 * @param {number} requests - Requests sum, 0 when unset
 * @param {number} limits - Limits sum, 0 when unset
 * @returns {string|null} - Display text or null
 */
export function percentText(value, requests, limits) {
  const fragments = []
  const reqPct = percentOf(value, requests)
  if (reqPct !== null) fragments.push(`${reqPct}% of request`)
  const limPct = percentOf(value, limits)
  if (limPct !== null) fragments.push(`${limPct}% of limit`)
  return fragments.length > 0 ? fragments.join(' · ') : null
}

/**
 * Classify a usage percentage: 'critical' at >=90%, 'warn' at >=80%,
 * null below that or when the percentage is unknown. The thresholds are
 * deliberately conservative: memory at 70-85% of limit is healthy
 * bin-packing, not a warning.
 * @param {number|null} pct - Usage percentage
 * @returns {string|null} - 'critical', 'warn' or null
 */
export function percentSeverity(pct) {
  if (typeof pct !== 'number') return null
  if (pct >= 90) return 'critical'
  if (pct >= 80) return 'warn'
  return null
}

/**
 * Classify usage proximity to its limit (see percentSeverity), using
 * the exact ratio so 89.5% does not classify as critical; percentOf
 * truncates the same ratio for display, keeping the shown percentage
 * and the color in agreement. Critical CPU means throttling, critical
 * memory means OOM-kill risk.
 * @param {number} value - Current usage value
 * @param {number} limit - Limit, 0 when unset
 * @returns {string|null} - 'critical', 'warn' or null
 */
export function limitSeverity(value, limit) {
  if (typeof value !== 'number' || !isFinite(value)) return null
  if (typeof limit !== 'number' || !isFinite(limit) || limit <= 0) return null
  return percentSeverity(ratioPercent(value, limit))
}

/**
 * Convert metrics samples into uPlot aligned data for one value field.
 * @param {Array<{t: string, cpu: number, memory: number}>} samples - API samples
 * @param {string} field - "cpu" or "memory"
 * @param {number} limit - Limits sum, drawn as a threshold series when set
 *   and within 2x of the peak usage
 * @returns {{data: Array<Array<number>>, hasLimit: boolean}} - uPlot aligned
 *   data: [timestamps (seconds), usage values, threshold values?]
 */
export function buildChartData(samples, field, limit) {
  const xs = []
  const ys = []
  let max = 0
  for (const s of samples || []) {
    const ts = Date.parse(s.t)
    const value = s[field]
    if (Number.isNaN(ts) || typeof value !== 'number') continue
    xs.push(Math.floor(ts / 1000))
    ys.push(value)
    if (value > max) max = value
  }

  const hasLimit = typeof limit === 'number' && limit > 0 && max > 0 && limit <= max * 2
  const data = [xs, ys]
  if (hasLimit) {
    data.push(xs.map(() => limit))
  }
  return { data, hasLimit }
}

/**
 * Tick increments for byte-valued chart axes: powers of two so the
 * y-axis lands on round binary values.
 */
export const BINARY_TICK_INCRS = Array.from({ length: 44 }, (_, i) => 2 ** i)

/**
 * Extract the latest usage sample from the workload metrics.
 * @param {{samples?: Array}} metrics - workloadInfo.metrics object
 * @returns {{cpu: number, memory: number}|null} - Latest usage or null
 */
export function latestSample(metrics) {
  const samples = metrics?.samples
  if (!Array.isArray(samples) || samples.length === 0) return null
  return samples[samples.length - 1]
}

/**
 * Report whether the workload metrics carry enough samples to chart
 * (at least two).
 * @param {{samples?: Array}} metrics - workloadInfo.metrics object
 * @returns {boolean} - True when the metrics are chartable
 */
export function hasChartableMetrics(metrics) {
  return Array.isArray(metrics?.samples) && metrics.samples.length >= 2
}

/**
 * Build the per-item (pod or workload) usage list for one value field,
 * sorted by usage descending. Items without a current usage sample are
 * kept with a null value and sorted last; ties compare the item's
 * namespace/name. Each entry carries the item's spec limit for the
 * field (0 when unset) and, for workload items, the kind and namespace
 * so rows can be keyed and linked to their dashboard.
 * @param {Array<{name: string, kind?: string, namespace?: string,
 *   metrics?: Object, resources?: Object}>} list - workloadInfo.pods or
 *   workload rows built from the workloads batch response
 * @param {string} field - "cpu" or "memory"
 * @returns {Array<{name: string, value: number|null, limit: number,
 *   kind?: string, namespace?: string}>} - Sorted usage list
 */
export function usageSeries(list, field) {
  const limitField = field === 'cpu' ? 'cpuLimits' : 'memoryLimits'
  const items = []
  for (const item of list || []) {
    const value = item?.metrics?.[field]
    const rawLimit = item?.resources?.[limitField]
    const limit = typeof rawLimit === 'number' && isFinite(rawLimit) && rawLimit > 0 ? rawLimit : 0
    const entry = { name: item.name, value: null, limit }
    if (item.kind) entry.kind = item.kind
    if (item.namespace) entry.namespace = item.namespace
    if (typeof value === 'number' && isFinite(value) && value >= 0) {
      entry.value = value
    }
    items.push(entry)
  }
  const label = item => (item.namespace ? `${item.namespace}/${item.name}` : item.name)
  return items.sort((a, b) => {
    if (a.value === null && b.value === null) return label(a).localeCompare(label(b))
    if (a.value === null) return 1
    if (b.value === null) return -1
    return (b.value - a.value) || label(a).localeCompare(label(b))
  })
}

/**
 * Merge multiple sample series into one by summing the values that
 * share a timestamp. The result is sorted chronologically. Mirror of
 * the Go sumSeries used by the workload endpoints.
 * @param {Array<Array<{t: string, cpu: number, memory: number}>>} seriesList - API sample arrays
 * @returns {Array<{t: string, cpu: number, memory: number}>} - Merged series
 */
export function sumSeries(seriesList) {
  const byTime = new Map()
  for (const samples of seriesList || []) {
    for (const s of samples || []) {
      const agg = byTime.get(s.t) || { t: s.t, cpu: 0, memory: 0 }
      agg.cpu += s.cpu
      agg.memory += s.memory
      byTime.set(s.t, agg)
    }
  }
  return [...byTime.values()].sort((a, b) => Date.parse(a.t) - Date.parse(b.t))
}

// Row budget of the per-pod usage bars.
const POD_BARS_BUDGET = 9
const POD_BARS_TOP = 3
const POD_BARS_BOTTOM = 2
const POD_BARS_NA_MAX = 2

/**
 * Trim a per-pod usage list (usageSeries output) to a fixed row
 * budget: the top and bottom pods by usage stay visible, the middle
 * collapses into a single aggregate row, and sampleless pods beyond a
 * small count collapse into a single "collecting" row.
 * @param {Array<{name: string, value: number|null}>} items - Sorted
 *   per-pod usage from usageSeries
 * @returns {Array<Object>} - Display rows: {type: 'pod', name, value},
 *   {type: 'elision', count, min, max, avg} or {type: 'collecting', count}
 */
export function trimPodUsage(items) {
  const list = items || []
  if (list.length <= POD_BARS_BUDGET) {
    return list.map(item => ({ type: 'pod', ...item }))
  }

  const measured = list.filter(item => item.value !== null)
  const sampleless = list.filter(item => item.value === null)
  const rows = []

  const top = measured.slice(0, POD_BARS_TOP)
  const bottomCount = Math.min(POD_BARS_BOTTOM, Math.max(measured.length - top.length, 0))
  const bottom = bottomCount > 0 ? measured.slice(measured.length - bottomCount) : []
  const middle = measured.slice(top.length, measured.length - bottom.length)

  rows.push(...top.map(item => ({ type: 'pod', ...item })))
  if (middle.length === 1) {
    // A single hidden pod takes no less space than showing it.
    rows.push({ type: 'pod', ...middle[0] })
  } else if (middle.length > 1) {
    // The list is sorted descending: first is the max, last the min.
    rows.push({
      type: 'elision',
      count: middle.length,
      min: middle[middle.length - 1].value,
      max: middle[0].value,
      avg: middle.reduce((acc, item) => acc + item.value, 0) / middle.length,
    })
  }
  rows.push(...bottom.map(item => ({ type: 'pod', ...item })))

  if (sampleless.length <= POD_BARS_NA_MAX) {
    rows.push(...sampleless.map(item => ({ type: 'pod', ...item })))
  } else {
    rows.push({ type: 'collecting', count: sampleless.length })
  }

  return rows
}

/**
 * Resolve an event timestamp to a chart annotation time.
 * Returns the time in epoch seconds when it falls inside the sampled
 * window, or null otherwise.
 * @param {{samples?: Array}} metrics - workloadInfo.metrics object
 * @param {string} at - Event timestamp (RFC 3339), e.g. rolledOutAt
 * @returns {number|null} - Annotation time in seconds or null
 */
export function usageAnnotation(metrics, at) {
  if (!at || !hasChartableMetrics(metrics)) return null
  const ts = Date.parse(at)
  if (Number.isNaN(ts)) return null
  const seconds = Math.floor(ts / 1000)
  const samples = metrics.samples
  const first = Math.floor(Date.parse(samples[0].t) / 1000)
  const last = Math.floor(Date.parse(samples[samples.length - 1].t) / 1000)
  return seconds >= first && seconds <= last ? seconds : null
}
