// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useRef, useEffect } from 'preact/hooks'
import uPlot from 'uplot'
import 'uplot/dist/uPlot.min.css'
import { appliedTheme } from '../../../utils/theme'

// Chart heights in CSS pixels.
const CHART_HEIGHT = 180

// Theme parameters validated with the palette checks against the
// card surfaces (white and gray-800).
const CHART_THEMES = {
  light: {
    axis: '#6B7280',
    grid: 'rgba(107, 114, 128, 0.15)',
    threshold: '#9CA3AF',
    tooltipBg: '#ffffff',
    tooltipBorder: '#E5E7EB',
  },
  dark: {
    axis: '#9CA3AF',
    grid: 'rgba(156, 163, 175, 0.15)',
    threshold: '#6B7280',
    tooltipBg: '#111827',
    tooltipBorder: '#374151',
  },
}

// Series colors per theme (validated: light #0066CC/#0D9488, dark #3B82F6/#0D9488).
export const CHART_COLORS = {
  cpu: {
    light: { stroke: '#0066CC', fill: 'rgba(0, 102, 204, 0.12)' },
    dark: { stroke: '#3B82F6', fill: 'rgba(59, 130, 246, 0.16)' },
  },
  memory: {
    light: { stroke: '#0D9488', fill: 'rgba(13, 148, 136, 0.12)' },
    dark: { stroke: '#0D9488', fill: 'rgba(13, 148, 136, 0.18)' },
  },
}

/**
 * Format a sample timestamp (seconds) as a local HH:MM label.
 */
function formatTime(ts) {
  return new Date(ts * 1000).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
}

/**
 * UsageChart - Time-series area chart for CPU or Memory usage rendered with uPlot.
 *
 * @param {Object} props
 * @param {Array} props.data - uPlot aligned data: [timestamps, usage, threshold?]
 * @param {boolean} props.hasLimit - Whether data carries a threshold series
 * @param {string} props.colorKey - "cpu" or "memory", selects the series color
 * @param {Function} props.formatValue - Value formatter for ticks and tooltip
 * @param {Array<number>} [props.tickIncrs] - Optional y-axis tick increments
 *   (e.g. powers of two for byte values)
 * @param {{time: number, label: string}} [props.annotation] - Optional
 *   event marked with a dashed vertical line at the given epoch seconds
 *   and named in the tooltip, e.g. {time, label: 'rolled out'}
 * @param {string} props.testId - data-testid for the chart container
 */
export function UsageChart({ data, hasLimit, colorKey, formatValue, tickIncrs, annotation, testId }) {
  const containerRef = useRef(null)
  const tooltipRef = useRef(null)
  const chartRef = useRef(null)
  const dataRef = useRef(data)
  dataRef.current = data

  // Reading the signal value subscribes the component to theme changes.
  const theme = appliedTheme.value

  // (Re)create the chart when the theme or the data shape changes.
  useEffect(() => {
    const container = containerRef.current
    if (!container) return undefined

    const themeColors = CHART_THEMES[theme] || CHART_THEMES.light
    const seriesColors = (CHART_COLORS[colorKey] || CHART_COLORS.cpu)[theme === 'dark' ? 'dark' : 'light']

    const series = [
      {},
      {
        stroke: seriesColors.stroke,
        fill: seriesColors.fill,
        width: 2,
        points: { show: false },
      },
    ]
    if (hasLimit) {
      series.push({
        stroke: themeColors.threshold,
        dash: [6, 4],
        width: 1,
        points: { show: false },
      })
    }

    const axisFont = '11px Inter, system-ui, sans-serif'
    const opts = {
      width: container.clientWidth || 300,
      height: CHART_HEIGHT,
      legend: { show: false },
      cursor: {
        y: false,
        drag: { x: false, y: false },
        points: { size: 6 },
      },
      scales: {
        y: {
          // Anchor the scale at zero and leave headroom above the peak so
          // the curve never touches the panel edge.
          range: (u, min, max) => [0, (max || 1) * 1.1],
        },
      },
      axes: [
        {
          stroke: themeColors.axis,
          font: axisFont,
          grid: { show: false },
          ticks: { show: false },
          // Single-line HH:MM labels; uPlot's default time formatter adds a
          // second line with the date, which is noise for a ~30 min window.
          values: (u, vals) => vals.map(ts => formatTime(ts)),
          // Samples are one minute apart, so sub-minute ticks would render
          // duplicate HH:MM labels on short series.
          incrs: [60, 120, 300, 600, 900, 1800, 3600],
        },
        {
          stroke: themeColors.axis,
          font: axisFont,
          size: 52,
          grid: { stroke: themeColors.grid, width: 1 },
          ticks: { show: false },
          values: (u, vals) => vals.map(v => formatValue(v)),
          ...(tickIncrs ? { incrs: tickIncrs } : {}),
        },
      ],
      series,
      hooks: {
        setCursor: [
          u => {
            const tooltip = tooltipRef.current
            if (!tooltip) return
            const { idx } = u.cursor
            if (idx == null || u.data[0][idx] == null) {
              tooltip.style.display = 'none'
              return
            }
            const value = u.data[1][idx]
            const limitValue = hasLimit ? u.data[2][idx] : null
            tooltip.textContent = ''
            const timeEl = document.createElement('div')
            timeEl.className = 'usage-chart-tooltip-time'
            timeEl.textContent = formatTime(u.data[0][idx])
            const valueEl = document.createElement('div')
            valueEl.className = 'usage-chart-tooltip-value'
            valueEl.textContent = formatValue(value)
            tooltip.append(timeEl, valueEl)
            if (limitValue != null) {
              const limitEl = document.createElement('div')
              limitEl.className = 'usage-chart-tooltip-limit'
              limitEl.textContent = `limit ${formatValue(limitValue)}`
              tooltip.append(limitEl)
            }
            // Name the annotation line when hovering the sample closest
            // to it. The nearest sample is found by scanning: failed
            // scrapes leave irregular gaps, so a fixed-gap heuristic
            // would attribute the event to the wrong sample.
            if (annotation != null) {
              let nearest = 0
              for (let i = 1; i < u.data[0].length; i++) {
                if (Math.abs(u.data[0][i] - annotation.time) < Math.abs(u.data[0][nearest] - annotation.time)) {
                  nearest = i
                }
              }
              if (idx === nearest) {
                const eventEl = document.createElement('div')
                eventEl.className = 'usage-chart-tooltip-limit'
                eventEl.textContent = `${annotation.label} ${formatTime(annotation.time)}`
                tooltip.append(eventEl)
              }
            }

            // Position the tooltip near the cursor, clamped to the chart box.
            const left = u.valToPos(u.data[0][idx], 'x')
            tooltip.style.display = 'block'
            const chartWidth = u.over.clientWidth
            const tooltipWidth = tooltip.offsetWidth || 80
            const clamped = Math.min(Math.max(left + 8, 0), chartWidth - tooltipWidth)
            tooltip.style.left = `${clamped}px`
            tooltip.style.top = '4px'
          },
        ],
        // Mark the annotated event (e.g. a rollout) with a dashed
        // vertical line across the plot area.
        ...(annotation != null
          ? {
            draw: [
              u => {
                if (annotation.time < u.scales.x.min || annotation.time > u.scales.x.max) return
                const ctx = u.ctx
                const x = u.valToPos(annotation.time, 'x', true)
                ctx.save()
                ctx.strokeStyle = themeColors.threshold
                ctx.lineWidth = window.devicePixelRatio || 1
                ctx.setLineDash([4, 4])
                ctx.beginPath()
                ctx.moveTo(x, u.bbox.top)
                ctx.lineTo(x, u.bbox.top + u.bbox.height)
                ctx.stroke()
                ctx.restore()
              },
            ],
          }
          : {}),
      },
    }

    const chart = new uPlot(opts, dataRef.current, container)
    chartRef.current = chart

    // Track the container size (uPlot needs explicit pixel sizes).
    let resizeObserver
    let frame = 0
    if (typeof window.ResizeObserver !== 'undefined') {
      resizeObserver = new window.ResizeObserver(entries => {
        const width = entries[0]?.contentRect?.width
        if (!width || width === chart.width) return
        window.cancelAnimationFrame(frame)
        frame = window.requestAnimationFrame(() => chart.setSize({ width, height: CHART_HEIGHT }))
      })
      resizeObserver.observe(container)
    }

    return () => {
      resizeObserver?.disconnect()
      window.cancelAnimationFrame(frame)
      chart.destroy()
      chartRef.current = null
    }
    // Depend on the annotation fields rather than the object identity,
    // which changes on every poll re-render.
  }, [theme, hasLimit, colorKey, formatValue, annotation?.time, annotation?.label])

  // Push fresh data into the existing chart without recreating it.
  useEffect(() => {
    if (chartRef.current) {
      chartRef.current.setData(data)
    }
  }, [data])

  return (
    <div class="relative" data-testid={testId}>
      <div ref={containerRef} class="usage-chart" />
      <div
        ref={tooltipRef}
        class="usage-chart-tooltip"
        style={{
          display: 'none',
          position: 'absolute',
          pointerEvents: 'none',
          zIndex: 10,
        }}
      />
    </div>
  )
}
