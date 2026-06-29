// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0
//
// Shared tabbed detail layout for the compact list view, with two dedicated
// responsive layouts:
//
//   - Mobile (default): a horizontal row of tabs above a full-width content box.
//     The active tab is a filled pill that shares the content background. A
//     vertical rail would waste the narrow viewport, so it is not used here.
//   - Desktop (sm+): a vertical "folder tab" rail on the left and a content panel
//     on the right. The active tab becomes a nub that merges into the panel.
//
// Desktop folder-tab technique: the content panel is the only bordered, rounded
// box. The active nub carries a 3-sided border (top/left/bottom) with its left
// corners rounded, and overlaps the panel's left border by exactly one pixel
// (sm:-mr-px). The nub paints above the panel (sm:z-10) and shares its
// background, so the nub's background covers that 1px of the panel's border along
// the nub's height: the seam disappears and tab + panel become one shape. The nub
// and panel borders are the same width and color, so the combined outline is
// continuous with equal rounded-md corners.

import { getDashboardUrl } from '../../utils/routing'
import { getStatusBadgeClass } from '../../utils/status'

// Rail width (desktop only) that lines the content panel up under the row's name
// column: the rail starts under the kind chip (see the wrapper's left padding in
// the list rows) and spans the chip→name gap.
const RAIL = 'sm:w-[84px]'

// Static class fragments for the tab buttons (kept as literals so Tailwind's JIT
// can see every utility at build time). `select-none` stops the label text from
// being highlighted on click. There is no color transition so switching tabs is
// instant with no highlight fade.
//
// Base: a rounded pill on mobile; on desktop the pill rounding is dropped and a
// constant 1px border (transparent until active) keeps the rail layout stable.
const TAB_BASE =
  'text-left whitespace-nowrap select-none px-2.5 py-1.5 text-xs font-medium focus:outline-none rounded-md sm:rounded-none sm:border sm:border-transparent'
// Active marker: shares the panel background (`bg-gray-100`, asserted by tests).
// Mobile = filled pill; desktop adds the folder-nub border + overlap.
const TAB_ACTIVE =
  'bg-gray-100 dark:bg-gray-900 text-gray-900 dark:text-white sm:relative sm:z-10 sm:-mr-px sm:rounded-l-md sm:border-r-0 sm:border-gray-200 sm:dark:border-gray-700'
const TAB_INACTIVE =
  'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300'

/**
 * TabbedPanel - the compact detail layout, with dedicated mobile and desktop
 * views.
 *
 * On mobile the tabs are a horizontal row above a full-width content box (the
 * active tab is a filled pill sharing the content background). On desktop the
 * tabs become a vertical folder-tab rail on the left: inactive tabs are plain
 * muted text and the active tab is a nub that flows into the content panel,
 * sharing one background and one continuous border with no seam.
 *
 * The content panel is the bordered, rounded box. On desktop its top-left corner
 * is squared only when the first tab is active, so the active nub provides that
 * corner instead of the panel's rounding peeking out from behind the
 * 1px-overlapping nub. On mobile every corner stays rounded.
 *
 * @param {Object} props
 * @param {Array<{id: string, label: string}>} props.tabs - Tab definitions
 * @param {string} props.active - The id of the active tab
 * @param {Function} props.onSelect - Called with a tab id when a tab is clicked
 * @param {*} props.children - The active tab's content
 */
export function TabbedPanel({ tabs, active, onSelect, children }) {
  // Square off the panel's top-left corner (desktop only) when the first tab is
  // active, so the nub's rounded top-left becomes the shape's top-left corner.
  const isFirstActive = tabs.length > 0 && tabs[0].id === active
  const panelCorners = `rounded-md ${isFirstActive ? 'sm:rounded-tl-none' : ''}`

  return (
    <div class="flex flex-col sm:flex-row">
      {/* Tab strip: a horizontal scrollable row on mobile, a vertical rail on
          desktop. Inactive tabs are plain text; the active tab shares the panel
          background and (on desktop) overhangs the panel's left border. */}
      <nav class={`flex flex-row sm:flex-col shrink-0 gap-1 sm:gap-0 mb-2 sm:mb-0 overflow-x-auto sm:overflow-visible ${RAIL}`}>
        {tabs.map(t => (
          <button
            key={t.id}
            onClick={() => onSelect(t.id)}
            class={`${TAB_BASE} ${t.id === active ? TAB_ACTIVE : TAB_INACTIVE}`}
          >
            {t.label}
          </button>
        ))}
      </nav>

      {/* Content panel: the bordered, rounded box. On desktop the active nub
          overlaps its left border by 1px (hidden by the nub's matching
          background), so the two merge into one silhouette. The top padding
          matches the tab's (py-1.5) so the first desktop content line lines up
          with the first tab label. Tall content (huge YAML, long inventory) is
          capped at 60vh and scrolls inside the panel so one expanded row never
          takes over the page. */}
      <div
        class={`flex-1 min-w-0 max-h-[60vh] overflow-y-auto border border-gray-200 dark:border-gray-700 bg-gray-100 dark:bg-gray-900 px-3 pt-1.5 pb-3 ${panelCorners}`}
      >
        {children}
      </div>
    </div>
  )
}

/**
 * Field - an inline label + value, stacked with even vertical rhythm (mirrors
 * the Resource dashboard Reconciler panel). Skipped when the value is empty.
 *
 * @param {Object} props
 * @param {string} props.label - Field label
 * @param {*} props.children - Field value
 */
export function Field({ label, children }) {
  if (children === null || children === undefined || children === '') return null
  return (
    <div>
      <span class="text-gray-500 dark:text-gray-400">{label}</span>{' '}
      <span class="text-gray-900 dark:text-gray-100 break-words">{children}</span>
    </div>
  )
}

/**
 * ResourceLink - the shared style for an inline link to a resource: a plain
 * flux-blue `namespace/name` link with no icon. Used by the resource and
 * workload detail tabs so every resource link looks identical.
 *
 * @param {Object} props
 * @param {string} props.kind - Resource kind (used to build the dashboard URL)
 * @param {string} props.namespace - Resource namespace
 * @param {string} props.name - Resource name
 */
export function ResourceLink({ kind, namespace, name }) {
  return (
    <a
      href={getDashboardUrl(kind, namespace, name)}
      class="text-flux-blue dark:text-blue-400 hover:underline break-all"
    >
      {namespace}/{name}
    </a>
  )
}

/**
 * StatusBadge - a small rounded status pill, rendered identically wherever a
 * resource or workload status appears in a detail tab. Renders nothing when the
 * status is empty.
 *
 * @param {Object} props
 * @param {string} props.status - Status text (e.g. "Ready", "Idle")
 * @param {string} [props.colorClass] - Badge background/text classes; defaults to
 *   the Flux resource palette via {@link getStatusBadgeClass} (workload tabs pass
 *   the kstatus palette instead)
 */
export function StatusBadge({ status, colorClass }) {
  if (!status) return null
  return (
    <span class={`inline-flex items-center px-2 py-0.5 rounded-full text-[10px] font-medium align-middle ${colorClass || getStatusBadgeClass(status)}`}>
      {status}
    </span>
  )
}
