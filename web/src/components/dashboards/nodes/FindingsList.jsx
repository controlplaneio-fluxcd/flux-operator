// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { CHIP_BASE, NEUTRAL_CHIP } from '../../common/rowKit'
import { summaryTitle, lineDetail, severityRank, plural } from '../../../utils/nodes'
import { DashboardPanel } from '../common/panel'

// Chip labels and colors of the Health Checks rows.
export const CHECK_CHIPS = {
  critical: { label: 'Critical', class: 'status-not-ready' },
  warn: { label: 'Warning', class: 'status-warning' },
  info: { label: 'Advisory', class: 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400' },
  skipped: { label: 'Skipped', class: NEUTRAL_CHIP },
  passed: { label: 'Passed', class: 'status-ready' }
}

/**
 * CheckRow - one Health Checks row in the Resources list style: a chip, the
 * title and the detail on one line; chip and detail move to a second line on
 * phones.
 */
function CheckRow({ chip, title, detail }) {
  return (
    <div class="border-b border-gray-100 dark:border-gray-700/60 last:border-0 px-3 py-1.5" data-testid="health-check-row">
      <div class="flex items-center gap-2.5">
        <span class={`${CHIP_BASE} ${chip.class} hidden sm:inline-block`}>{chip.label}</span>
        <span class="min-w-0 flex-1 sm:flex-none sm:shrink-0 sm:max-w-[40%] truncate text-sm font-semibold text-gray-900 dark:text-gray-100" title={title}>{title}</span>
        <span class="hidden sm:block flex-1 min-w-0 truncate text-xs text-gray-500 dark:text-gray-400" title={detail}>{detail}</span>
      </div>
      {/* Mobile-only second line: chip + detail. */}
      <div class="sm:hidden mt-1 flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
        <span class={`${CHIP_BASE} ${chip.class}`}>{chip.label}</span>
        <span class="min-w-0 truncate">{detail}</span>
      </div>
    </div>
  )
}

/**
 * Count the Health Checks rows by kind for the panel subtitle, e.g.
 * "8 critical · 10 warnings · 2 advisories · 2 skipped · 1 passed".
 */
export function checkCounts(lines, checks) {
  const count = (sev) => lines.filter(l => l.severity === sev).length
  const critical = count('critical')
  const warn = count('warn')
  const info = count('info')
  const skipped = checks.filter(c => c.status === 'skipped').length
  const passed = checks.filter(c => c.status === 'passed').length
  return [
    critical > 0 && `${critical} critical`,
    warn > 0 && plural(warn, 'warning'),
    info > 0 && `${info} ${info === 1 ? 'advisory' : 'advisories'}`,
    skipped > 0 && `${skipped} skipped`,
    passed > 0 && `${passed} passed`
  ].filter(Boolean).join(' · ')
}

/**
 * FindingsList - the Health Checks panel, listing every check: one row per
 * raised finding line, worst first (severity, then the backend's code
 * priority, which is the order it ships), then the skipped and the passed
 * checks in catalogue order.
 *
 * @param {Object} props
 * @param {Object} props.data - Snapshot from GET /api/v1/nodes
 * @param {Function} props.shortName - Strips the shared DNS suffix from node names
 */
export function FindingsList({ data, shortName }) {
  // A stable sort keeps the backend's code priority within a severity.
  const lines = [...(data.findings || [])].sort((a, b) => severityRank(a.severity) - severityRank(b.severity))
  const checks = data.checks || []
  const skipped = checks.filter(c => c.status === 'skipped')
  const passed = checks.filter(c => c.status === 'passed')

  return (
    <DashboardPanel
      title="Health Checks"
      subtitle={<p class="text-sm text-gray-600 dark:text-gray-400 mt-1">{checkCounts(lines, checks)}</p>}
    >
      <div class="card overflow-hidden p-0 sm:p-2" data-testid="health-checks">
        {lines.map(l => (
          <CheckRow
            key={l.code}
            chip={CHECK_CHIPS[l.severity] || CHECK_CHIPS.info}
            title={summaryTitle(l)}
            detail={lineDetail(l, data, shortName)}
          />
        ))}
        {skipped.map(c => <CheckRow key={c.name} chip={CHECK_CHIPS.skipped} title={c.name} detail={c.fact} />)}
        {passed.map(c => <CheckRow key={c.name} chip={CHECK_CHIPS.passed} title={c.name} detail={c.fact} />)}
      </div>
    </DashboardPanel>
  )
}
