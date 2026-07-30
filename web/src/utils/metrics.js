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
 * Compute the percentage of a usage value relative to a denominator.
 * Returns null when the denominator is not set (zero).
 * @param {number} value - Usage value
 * @param {number} denominator - Requests or limits value, 0 when unset
 * @returns {number|null} - Rounded percentage or null
 */
export function percentOf(value, denominator) {
  if (typeof value !== 'number' || typeof denominator !== 'number' || denominator <= 0) return null
  return Math.round((value / denominator) * 100)
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
 * Build the per-pod usage list for one value field, sorted by usage
 * descending. Pods without a current usage sample are kept with a
 * null value and sorted last.
 * @param {Array<{name: string, metrics?: Object}>} pods - workloadInfo.pods
 * @param {string} field - "cpu" or "memory"
 * @returns {Array<{name: string, value: number|null}>} - Sorted usage list
 */
export function podUsageSeries(pods, field) {
  const items = []
  for (const pod of pods || []) {
    const value = pod?.metrics?.[field]
    if (typeof value === 'number' && isFinite(value) && value >= 0) {
      items.push({ name: pod.name, value })
    } else {
      items.push({ name: pod.name, value: null })
    }
  }
  return items.sort((a, b) => {
    if (a.value === null && b.value === null) return a.name.localeCompare(b.name)
    if (a.value === null) return 1
    if (b.value === null) return -1
    return b.value - a.value
  })
}

// Row budget of the per-pod usage bars.
const POD_BARS_BUDGET = 9
const POD_BARS_TOP = 4
const POD_BARS_BOTTOM = 2
const POD_BARS_NA_MAX = 2

/**
 * Trim a per-pod usage list (podUsageSeries output) to a fixed row
 * budget: the top and bottom pods by usage stay visible, the middle
 * collapses into a single aggregate row, and sampleless pods beyond a
 * small count collapse into a single "collecting" row.
 * @param {Array<{name: string, value: number|null}>} items - Sorted
 *   per-pod usage from podUsageSeries
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
