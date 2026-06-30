// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { formatTime } from '../../../utils/time'

// Map each pale card tint to a one-step-darker shade for the status icon disc, so
// the circle reads as a filled disc against the card in both themes. Written as
// literal class strings (not derived from bgColor) so Tailwind bundles every
// shade — including bg-orange-100, which is not used anywhere else. Unmapped
// colors fall back to the card tint.
const ICON_DISC_BG = {
  'bg-green-50': 'bg-green-100',
  'bg-blue-50': 'bg-blue-100',
  'bg-red-50': 'bg-red-100',
  'bg-yellow-50': 'bg-yellow-100',
  'bg-orange-50': 'bg-orange-100',
  'bg-gray-50': 'bg-gray-100'
}

/**
 * StatusHeroCard - Shared header card for the resource, workload, and cluster
 * dashboards. Renders the status-colored card shell, the icon disc, and the
 * standard "Last Updated" block, so the three dashboards share one implementation
 * (and one place to style the disc).
 *
 * Two content modes:
 *   - Structured: pass `kind`/`name`/`namespace` (+ optional `titleAction`) for the
 *     standard KIND / name / Namespace layout (resource and workload pages).
 *   - Custom: pass `children` for a bespoke middle section (cluster overall status).
 *
 * @param {string} props.bgColor - Tailwind bg class for the card tint (e.g. bg-green-50)
 * @param {string} props.borderColor - Tailwind border class (e.g. border-success)
 * @param {preact.VNode} props.icon - Icon rendered inside the disc
 * @param {string} [props.href] - When set, the whole card is a link to this URL
 * @param {number} [props.lastUpdatedAt] - Timestamp for the Last Updated block; omit to hide it
 * @param {string} [props.kind] - Resource kind (structured mode)
 * @param {string} [props.name] - Resource name; its presence selects structured mode
 * @param {string} [props.namespace] - Resource namespace (structured mode)
 * @param {preact.VNode} [props.titleAction] - Node rendered next to the name (e.g. a favorite toggle)
 * @param {preact.VNode} [props.children] - Custom middle content (used when `name` is absent)
 */
export function StatusHeroCard({
  bgColor,
  borderColor,
  icon,
  href,
  lastUpdatedAt,
  kind,
  name,
  namespace,
  titleAction,
  children
}) {
  const iconBg = ICON_DISC_BG[bgColor] || bgColor

  const cardClass = `card ${bgColor} dark:bg-opacity-20 border-2 ${borderColor}`
  const Wrapper = href ? 'a' : 'div'
  const wrapperProps = href
    ? { href, class: `${cardClass} w-full text-left cursor-pointer hover:shadow-lg transition-shadow focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-flux-blue block` }
    : { class: cardClass }

  return (
    <Wrapper {...wrapperProps}>
      <div class="flex items-center space-x-4">
        <div class="flex-shrink-0">
          <div class={`w-16 h-16 rounded-full ${iconBg} dark:bg-opacity-30 flex items-center justify-center`}>
            {icon}
          </div>
        </div>
        <div class="flex-grow min-w-0">
          {name != null ? (
            <>
              <span class="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase">{kind}</span>
              <h1 class="text-lg sm:text-2xl font-semibold text-gray-900 dark:text-white break-all flex items-center gap-2">
                {name}
                {titleAction}
              </h1>
              <span class="text-xs sm:text-sm text-gray-500 dark:text-gray-400">Namespace: {namespace}</span>
            </>
          ) : children}
        </div>
        {lastUpdatedAt != null && (
          <div class="hidden md:block text-right flex-shrink-0">
            <div class="text-sm text-gray-600 dark:text-gray-400">Last Updated</div>
            <div class="text-lg font-semibold text-gray-900 dark:text-white">{formatTime(lastUpdatedAt)}</div>
          </div>
        )}
      </div>
    </Wrapper>
  )
}
