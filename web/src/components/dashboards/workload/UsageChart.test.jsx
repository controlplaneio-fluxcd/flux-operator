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
  })

  it('formats y-axis ticks with the provided formatter', () => {
    render(
      <UsageChart data={data} hasLimit={false} colorKey="cpu" formatValue={formatCores} testId="cpu-chart" />
    )

    const yAxis = uPlot.instances[0].opts.axes[1]
    expect(yAxis.values(null, [0, 0.1, 0.2])).toEqual(['0m', '100m', '200m'])
  })
})
