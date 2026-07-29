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
  if (typeof cores !== 'number' || !isFinite(cores) || cores < 0) return '0m'
  if (cores === 0) return '0m'
  if (cores < 1) return `${Math.round(cores * 1000)}m`
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
 * Returns null when the denominator is not set (zero), as inventing a
 * denominator would be misleading — the caller must omit the percentage.
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
 * "12% of request · 6% of limit". Fragments with an unset denominator
 * are omitted; returns null when neither requests nor limits are set.
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
 * @param {number} limit - Limits sum, drawn as a threshold series when it
 *   is set and no further than 2x away from the peak usage (otherwise a
 *   far-off limit would flatten the usage curve)
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
 * y-axis lands on round binary values (128 MiB, 256 MiB, ...).
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
 * Report whether the workload metrics carry enough samples to chart.
 * A single point cannot show a trend, so the charts require at least two.
 * @param {{samples?: Array}} metrics - workloadInfo.metrics object
 * @returns {boolean} - True when the metrics are chartable
 */
export function hasChartableMetrics(metrics) {
  return Array.isArray(metrics?.samples) && metrics.samples.length >= 2
}
