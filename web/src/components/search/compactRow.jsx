// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0
//
// Shared row UI kit for the compact list view: the loading spinner, the
// expand chevron, the favorite star, the colored kind chip, the namespace/name
// link, the spinner-on-press disclosure hook, and the animated reveal.

import { useState } from 'preact/hooks'
import { getKindChipAlias } from '../../utils/constants'

/**
 * Chevron - expand caret that rotates 90° when its row is open.
 *
 * @param {Object} props
 * @param {boolean} props.open - Whether the row is expanded
 */
export function Chevron({ open }) {
  return (
    <svg class={`w-3.5 h-3.5 transition-transform ${open ? 'rotate-90' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
    </svg>
  )
}

/**
 * Spinner - circular loading spinner. Defaults to the chevron size (w-3.5 h-3.5)
 * so swapping it for the row's expand caret causes no layout shift; pass `cls` to
 * size it elsewhere (e.g. the toolbar/tab-nav count loaders use w-4 h-4).
 *
 * @param {Object} props
 * @param {string} [props.cls] - Size classes for the SVG
 */
export function Spinner({ cls = 'w-3.5 h-3.5' }) {
  return (
    <svg class={`${cls} animate-spin`} viewBox="0 0 24 24" fill="none">
      <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
      <path class="opacity-90" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
    </svg>
  )
}

// Inner two-tone "namespace/name" label shared by both NameLink variants: muted
// namespace, emphasized name, both turning flux-blue on row hover (the row sets
// the `group` class).
function NameSpans({ namespace, name }) {
  return (
    <>
      <span class="text-gray-500 dark:text-gray-400 group-hover:text-flux-blue dark:group-hover:text-blue-400">{namespace}/</span><span class="font-semibold text-gray-900 dark:text-gray-100 group-hover:text-flux-blue dark:group-hover:text-blue-400">{name}</span>
    </>
  )
}

/**
 * NameLink - the row's "namespace/name" dashboard link. Rendered twice so the
 * label can be full-width on mobile (grows to fill the row) and fixed-width on
 * desktop (capped so the message/timestamp columns get room); one of the pair is
 * always hidden by the breakpoint.
 *
 * @param {Object} props
 * @param {string} props.href - Dashboard URL
 * @param {string} props.namespace - Resource namespace
 * @param {string} props.name - Resource name
 * @param {string} [props.cls] - Extra classes applied to both variants (e.g. a
 *   focus ring)
 */
export function NameLink({ href, namespace, name, cls = '' }) {
  return (
    <>
      {/* Mobile: full-width, grows to fill the row */}
      <a href={href} class={`sm:hidden min-w-0 flex-1 truncate text-sm group ${cls}`}>
        <NameSpans namespace={namespace} name={name} />
      </a>
      {/* Desktop: fixed-width, capped at 40% */}
      <a href={href} class={`hidden sm:block sm:shrink-0 truncate text-sm sm:max-w-[40%] group ${cls}`}>
        <NameSpans namespace={namespace} name={name} />
      </a>
    </>
  )
}

/**
 * Star - favorite toggle, yellow when active. The caller is responsible for
 * stopping event propagation in `onClick` when the row itself is clickable.
 *
 * @param {Object} props
 * @param {boolean} props.active - Whether the item is favorited
 * @param {Function} props.onClick - Toggle handler
 */
export function Star({ active, onClick }) {
  return (
    <button
      onClick={onClick}
      class={`shrink-0 rounded p-0.5 transition-colors ${active ? 'text-yellow-500' : 'text-gray-300 hover:text-gray-400 dark:text-gray-600 dark:hover:text-gray-400'}`}
      title={active ? 'Remove from favorites' : 'Add to favorites'}
    >
      <svg class="w-4 h-4" fill={active ? 'currentColor' : 'none'} stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11.049 2.927c.3-.921 1.603-.921 1.902 0l1.519 4.674a1 1 0 00.95.69h4.915c.969 0 1.371 1.24.588 1.81l-3.976 2.888a1 1 0 00-.363 1.118l1.518 4.674c.3.922-.755 1.688-1.538 1.118l-3.976-2.888a1 1 0 00-1.176 0l-3.976 2.888c-.783.57-1.838-.197-1.538-1.118l1.518-4.674a1 1 0 00-.363-1.118l-3.976-2.888c-.784-.57-.38-1.81.588-1.81h4.914a1 1 0 00.951-.69l1.519-4.674z" />
      </svg>
    </button>
  )
}

// Shared chip geometry: fixed width so chips align into a column regardless of
// the alias length.
export const CHIP_BASE = 'shrink-0 w-[74px] text-center rounded px-1 py-0.5 text-[10px] font-semibold uppercase tracking-wide'

// Neutral chip palette, used for workloads whose own status is unknown in the
// list (the index only carries the reconciler's status, not the workload's).
export const NEUTRAL_CHIP = 'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-300'

/**
 * KindChip - kind pill colored by status. `cls` adds caller-controlled
 * breakpoint visibility (e.g. the desktop top-row chip hides on mobile, where
 * the chip is rendered again on the second row beside the status word and
 * timestamp).
 *
 * @param {Object} props
 * @param {string} props.kind - Kubernetes kind, shortened via {@link getKindChipAlias}
 * @param {string} props.colorClass - Tailwind classes for the chip background/text
 * @param {string} [props.title] - Tooltip; defaults to the full kind
 * @param {string} [props.cls] - Extra classes (e.g. responsive visibility)
 */
export function KindChip({ kind, colorClass, title, cls = '' }) {
  return <span title={title || kind} class={`${CHIP_BASE} ${colorClass} ${cls}`}>{getKindChipAlias(kind)}</span>
}

/**
 * useDisclosure - disclosure state for a row whose panel fetches before it
 * reveals: the expand button spins while the lazily mounted content loads, then
 * the panel animates open. Collapsing unmounts the panel, so each expand
 * re-mounts and re-fetches fresh data rather than replaying a cached snapshot.
 * The fetching child must call the returned `onReady` when its data has loaded
 * (success or error).
 *
 * @returns {{open: boolean, mounted: boolean, loading: boolean, toggle: Function, onReady: Function}}
 */
export function useDisclosure() {
  const [open, setOpen] = useState(false)
  const [mounted, setMounted] = useState(false)
  const [loading, setLoading] = useState(false)

  // Reveal the panel once its fetch settles (success or error).
  const onReady = () => { setLoading(false); setOpen(true) }

  // First click mounts the panel and starts its fetch (reveal happens via
  // onReady). Any later click — while still loading, or while revealed —
  // discards the panel, so the next expand re-mounts and re-fetches instead of
  // showing stale data.
  const toggle = () => {
    if (mounted) { setOpen(false); setLoading(false); setMounted(false); return }
    setMounted(true); setLoading(true)
  }
  return { open, mounted, loading, toggle, onReady }
}

/**
 * patchRowInSignal - refresh a single row inside a list signal in place. Finds the
 * row matching `kind/namespace/name`, and if any field in `patch` differs, replaces
 * just that row with a `{ ...row, ...patch }` copy (new array, same identity for the
 * untouched rows). No-op when the row is gone or every patched field already matches,
 * so an unchanged detail fetch triggers no re-render.
 *
 * Used by the list rows to write a row's summary back from its expanded detail
 * panel (the detail's reconcilerRef is computed by the same server status the list
 * uses), so a row that listed a stale status updates when expanded.
 *
 * @param {import('@preact/signals').Signal} sig - Signal holding the row array
 * @param {{kind: string, namespace: string, name: string}} id - Row identity
 * @param {Object} patch - Fields to overwrite on the matched row
 */
export function patchRowInSignal(sig, { kind, namespace, name }, patch) {
  const cur = sig.value.find(r => r.kind === kind && r.namespace === namespace && r.name === name)
  if (!cur || Object.keys(patch).every(k => cur[k] === patch[k])) return
  sig.value = sig.value.map(r => r === cur ? { ...r, ...patch } : r)
}

/**
 * Reveal - animated disclosure: collapses to zero height via an animatable grid
 * row while fading and sliding the content in. Children stay in the DOM when
 * collapsed so a lazily mounted detail panel can fetch before the panel opens.
 * Honors prefers-reduced-motion.
 *
 * @param {Object} props
 * @param {boolean} props.open - Whether the content is revealed
 * @param {*} props.children - Content to reveal
 */
export function Reveal({ open, children }) {
  return (
    <div class={`grid transition-all duration-300 ease-out motion-reduce:transition-none ${open ? 'grid-rows-[1fr] opacity-100' : 'grid-rows-[0fr] opacity-0'}`}>
      {/* `inert` when collapsed: the content stays mounted (so a lazily mounted
          detail panel can fetch before opening) but is removed from the tab order
          and the a11y tree, so keyboard users can't focus the hidden tabs/links. */}
      <div {...(open ? {} : { inert: true })} class={`overflow-hidden transition-transform duration-300 ease-out ${open ? 'translate-y-0' : '-translate-y-1'}`}>
        {children}
      </div>
    </div>
  )
}
