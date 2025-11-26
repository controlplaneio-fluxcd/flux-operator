// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

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
