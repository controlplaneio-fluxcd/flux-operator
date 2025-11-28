// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useSignal } from '@preact/signals'

/**
 * Collapsible dashboard panel with header and expandable content
 */
export function DashboardPanel({ title, subtitle, id, defaultExpanded = true, children }) {
  const isExpanded = useSignal(defaultExpanded)

  return (
    <div class="card p-0" data-id={id}>
      <button
        onClick={() => isExpanded.value = !isExpanded.value}
        class="w-full px-6 py-4 text-left hover:bg-gray-50 dark:hover:bg-gray-700/30 transition-colors"
        aria-expanded={isExpanded.value}
      >
        <div class="flex items-center justify-between">
          <div>
            <h3 class="text-base sm:text-lg font-semibold text-gray-900 dark:text-white">{title}</h3>
            {subtitle}
          </div>
          <svg
            class={`w-5 h-5 text-gray-400 dark:text-gray-500 transition-transform ${isExpanded.value ? 'rotate-180' : ''}`}
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7"/>
          </svg>
        </div>
      </button>
      {isExpanded.value && (
        <div class="px-6 pt-2 pb-4">
          {children}
        </div>
      )}
    </div>
  )
}

/**
 * Tab button for section tabs
 */
export function TabButton({ active, onClick, children }) {
  return (
    <button
      onClick={onClick}
      class={`py-2 px-1 text-sm font-medium border-b-2 transition-colors ${
        active
          ? 'border-flux-blue text-flux-blue dark:text-blue-400'
          : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300 dark:text-gray-400 dark:hover:text-gray-300'
      }`}
    >
      {children}
    </button>
  )
}
