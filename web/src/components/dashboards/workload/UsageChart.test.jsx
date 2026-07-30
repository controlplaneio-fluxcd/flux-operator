// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render } from '@testing-library/preact'
import { UsageChart } from './UsageChart'
import { formatCores } from '../../../utils/metrics'
import uPlot from 'uplot'

// jsdom has no canvas, so uPlot is mocked and the component is asserted
// through its lifecycle: create, setData on refresh, destroy on unmount.
vi.mock('uplot', () => {
  const instances = []
  const uPlotMock = vi.fn(function (opts, data, container) {
    this.opts = opts
    this.data = data
    this.container = container
    this.width = opts.width
    this.setData = vi.fn(d => { this.data = d })
    this.setSize = vi.fn()
    this.destroy = vi.fn()
    instances.push(this)
  })
  uPlotMock.instances = instances
  return { default: uPlotMock }
})

vi.mock('uplot/dist/uPlot.min.css', () => ({}))

describe('UsageChart component', () => {
  const data = [[1753693200, 1753693260], [0.1, 0.2]]

  beforeEach(() => {
    uPlot.instances.length = 0
    uPlot.mockClear()
  })

  it('creates a chart with the usage series', () => {
    render(
      <UsageChart data={data} hasLimit={false} colorKey="cpu" formatValue={formatCores} testId="cpu-chart" />
    )

    expect(uPlot).toHaveBeenCalledTimes(1)
    const instance = uPlot.instances[0]
    expect(instance.data).toEqual(data)
    // One placeholder x series and one usage series, no threshold.
    expect(instance.opts.series).toHaveLength(2)
    expect(instance.opts.legend.show).toBe(false)
  })

  it('adds a dashed threshold series when hasLimit is set', () => {
    const withLimit = [...data, [0.5, 0.5]]
    render(
      <UsageChart data={withLimit} hasLimit={true} colorKey="cpu" formatValue={formatCores} testId="cpu-chart" />
    )

    const instance = uPlot.instances[0]
    expect(instance.opts.series).toHaveLength(3)
    expect(instance.opts.series[2].dash).toEqual([6, 4])
  })

  it('pushes fresh data into the existing chart without recreating it', () => {
    const { rerender } = render(
      <UsageChart data={data} hasLimit={false} colorKey="cpu" formatValue={formatCores} testId="cpu-chart" />
    )

    const next = [[1753693200, 1753693260, 1753693320], [0.1, 0.2, 0.3]]
    rerender(
      <UsageChart data={next} hasLimit={false} colorKey="cpu" formatValue={formatCores} testId="cpu-chart" />
    )

    expect(uPlot).toHaveBeenCalledTimes(1)
    expect(uPlot.instances[0].setData).toHaveBeenCalledWith(next)
  })

  it('destroys the chart on unmount', () => {
    const { unmount } = render(
      <UsageChart data={data} hasLimit={false} colorKey="cpu" formatValue={formatCores} testId="cpu-chart" />
    )

    unmount()
    expect(uPlot.instances[0].destroy).toHaveBeenCalledTimes(1)
  })

  it('renders single-line minute-aligned x-axis labels', () => {
    render(
      <UsageChart data={data} hasLimit={false} colorKey="cpu" formatValue={formatCores} testId="cpu-chart" />
    )

    const xAxis = uPlot.instances[0].opts.axes[0]
    // Custom formatter overrides uPlot's two-line date/time default.
    const labels = xAxis.values(null, [1753693200])
    expect(labels).toHaveLength(1)
    expect(labels[0]).not.toContain('\n')
    // Tick steps never go below one minute (the sample interval),
    // which would produce duplicate HH:MM labels.
    expect(Math.min(...xAxis.incrs)).toBe(60)
    // Ticks must leave room for a ~50px "05:12 PM" label plus a gap;
    // uPlot's default 50px spacing packs the labels edge-to-edge.
    expect(xAxis.space).toBeGreaterThanOrEqual(80)
  })

  it('formats y-axis ticks with the provided formatter', () => {
    render(
      <UsageChart data={data} hasLimit={false} colorKey="cpu" formatValue={formatCores} testId="cpu-chart" />
    )

    const yAxis = uPlot.instances[0].opts.axes[1]
    expect(yAxis.values(null, [0, 0.1, 0.2])).toEqual(['0m', '100m', '200m'])
  })

  it('sizes the y-axis to its widest tick label', () => {
    render(
      <UsageChart data={data} hasLimit={false} colorKey="cpu" formatValue={formatCores} testId="cpu-chart" />
    )

    const yAxis = uPlot.instances[0].opts.axes[1]
    const u = {
      axes: [null, { font: ['11px Inter'], gap: 5, _size: 40 }],
      ctx: { measureText: vi.fn(s => ({ width: s.length * 10 })), font: '' },
    }

    // Widest label "200m": 40px wide + 5 gap + 8 pad.
    expect(yAxis.size(u, ['0m', '100m', '200m'], 1, 1)).toBe(53)
    // The pre-layout pass without values falls back to the minimum.
    expect(yAxis.size(u, null, 1, 1)).toBe(13)
    // Later layout cycles reuse the computed size to avoid loops.
    expect(yAxis.size(u, ['0m'], 1, 2)).toBe(40)
  })

  it('divides the measured label width by the device pixel ratio', () => {
    const original = window.devicePixelRatio
    window.devicePixelRatio = 2
    try {
      render(
        <UsageChart data={data} hasLimit={false} colorKey="cpu" formatValue={formatCores} testId="cpu-chart" />
      )

      const yAxis = uPlot.instances[0].opts.axes[1]
      const u = {
        axes: [null, { font: ['11px Inter'], gap: 5, _size: 40 }],
        ctx: { measureText: vi.fn(() => ({ width: 40 })), font: '' },
      }
      // measureText reports canvas pixels: 40 / dpr 2 = 20 CSS px + 5 gap + 8 pad.
      expect(yAxis.size(u, ['200m'], 1, 1)).toBe(33)
    } finally {
      window.devicePixelRatio = original
    }
  })

  it('hides a final x-axis label that would clip at the right edge', () => {
    render(
      <UsageChart data={data} hasLimit={false} colorKey="cpu" formatValue={formatCores} testId="cpu-chart" />
    )

    const xAxis = uPlot.instances[0].opts.axes[0]
    const u = {
      scales: { x: { min: 0, max: 1800 } },
      bbox: { width: 540 },
      axes: [{ font: ['11px Inter'] }],
      ctx: { measureText: vi.fn(() => ({ width: 52 })), font: '' },
    }

    // A tick 10s from the edge: 3px + 8 pad < 26 half-label -> hidden.
    let labels = xAxis.values(u, [600, 1790])
    expect(labels[1]).toBeNull()
    expect(labels[0]).not.toBeNull()

    // A tick 120s from the edge has room for the label.
    labels = xAxis.values(u, [600, 1680])
    expect(labels[1]).not.toBeNull()
  })

  it('registers a draw hook only when an annotation is set', () => {
    render(
      <UsageChart data={data} hasLimit={false} colorKey="cpu" formatValue={formatCores} testId="cpu-chart" />
    )
    expect(uPlot.instances[0].opts.hooks.draw).toBeUndefined()

    render(
      <UsageChart
        data={data} hasLimit={false} colorKey="cpu" formatValue={formatCores}
        annotation={{ time: 1753693230, label: 'rolled out' }} testId="cpu-chart"
      />
    )
    expect(uPlot.instances[1].opts.hooks.draw).toHaveLength(1)
  })

  it('draws the annotation line inside the plot area', () => {
    render(
      <UsageChart
        data={data} hasLimit={false} colorKey="cpu" formatValue={formatCores}
        annotation={{ time: 1753693230, label: 'rolled out' }} testId="cpu-chart"
      />
    )

    const ctx = {
      save: vi.fn(), restore: vi.fn(), setLineDash: vi.fn(),
      beginPath: vi.fn(), moveTo: vi.fn(), lineTo: vi.fn(), stroke: vi.fn(),
    }
    const u = {
      ctx,
      scales: { x: { min: 1753693200, max: 1753693260 } },
      bbox: { top: 10, height: 100 },
      valToPos: vi.fn(() => 42),
    }

    uPlot.instances[0].opts.hooks.draw[0](u)
    expect(u.valToPos).toHaveBeenCalledWith(1753693230, 'x', true)
    expect(ctx.moveTo).toHaveBeenCalledWith(42, 10)
    expect(ctx.lineTo).toHaveBeenCalledWith(42, 110)
    expect(ctx.stroke).toHaveBeenCalledTimes(1)

    // An annotation outside the visible range draws nothing.
    u.scales.x = { min: 1753693240, max: 1753693260 }
    ctx.stroke.mockClear()
    uPlot.instances[0].opts.hooks.draw[0](u)
    expect(ctx.stroke).not.toHaveBeenCalled()
  })

  it('names the annotation in the tooltip only on the nearest sample', () => {
    // Irregular gaps (a failed scrape): the annotation at t=350 is
    // nearest to the third sample despite being 50s away from it.
    const irregular = [[100, 160, 400], [0.1, 0.2, 0.3]]
    const { container } = render(
      <UsageChart
        data={irregular} hasLimit={false} colorKey="cpu" formatValue={formatCores}
        annotation={{ time: 350, label: 'rolled out' }} testId="cpu-chart"
      />
    )

    const setCursor = uPlot.instances[0].opts.hooks.setCursor[0]
    const tooltip = container.querySelector('.usage-chart-tooltip')
    const u = {
      cursor: { idx: 2 },
      data: irregular,
      valToPos: () => 10,
      over: { clientWidth: 300 },
    }

    setCursor(u)
    expect(tooltip.textContent).toContain('rolled out')

    u.cursor.idx = 1
    setCursor(u)
    expect(tooltip.textContent).not.toContain('rolled out')
  })
})
