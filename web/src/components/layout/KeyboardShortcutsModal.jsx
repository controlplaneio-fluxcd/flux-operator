// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useRef, useEffect } from 'preact/hooks'
import { keyboardShortcutsOpen } from '../../utils/keyboardShortcuts'
import { useDismiss } from '../../utils/useDismiss'

const KBD_CLS = 'px-1.5 py-0.5 text-xs font-medium bg-gray-100 dark:bg-gray-700 border border-gray-300 dark:border-gray-600 rounded'

function Kbd({ children }) {
  return <kbd class={KBD_CLS}>{children}</kbd>
}

function KeySequence({ keys, join }) {
  if (join === '+') {
    return (
      <div class="flex items-center gap-1 flex-shrink-0">
        {keys.map((key, index) => (
          <span key={`${key}-${index}`} class="inline-flex items-center gap-1">
            {index > 0 && <span class="text-xs text-gray-400">+</span>}
            <Kbd>{key}</Kbd>
          </span>
        ))}
      </div>
    )
  }

  return (
    <div class="inline-flex items-center gap-1 flex-shrink-0">
      {keys.map((key) => (
        <Kbd key={key}>{key}</Kbd>
      ))}
    </div>
  )
}

function ShortcutRow({ keys, description }) {
  return (
    <div class="flex items-center justify-between gap-4 py-1.5">
      <span class="text-sm text-gray-600 dark:text-gray-400">{description}</span>
      <div class="flex items-center gap-1 flex-shrink-0">{keys}</div>
    </div>
  )
}

function ShortcutSection({ title, children }) {
  return (
    <section>
      <h3 class="text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400 mb-2">
        {title}
      </h3>
      <div class="divide-y divide-gray-100 dark:divide-gray-800">
        {children}
      </div>
    </section>
  )
}

const SHORTCUT_SECTIONS = [
  {
    title: 'Help',
    rows: [
      { description: 'Show this dialog', keys: <Kbd>?</Kbd> },
    ],
  },
  {
    title: 'Navigation',
    rows: [
      { description: 'Go to Dashboard', keys: <KeySequence keys={['g', 'd']} /> },
      { description: 'Go to Favorites', keys: <KeySequence keys={['g', 'f']} /> },
      { description: 'Go to Resources', keys: <KeySequence keys={['g', 'r']} /> },
      { description: 'Go to Workloads', keys: <KeySequence keys={['g', 'w']} /> },
      { description: 'Go to Events', keys: <KeySequence keys={['g', 'e']} /> },
    ],
  },
  {
    title: 'Search',
    rows: [
      { description: 'Open quick search', keys: <Kbd>/</Kbd> },
      { description: 'Move up in results', keys: <Kbd>↑</Kbd> },
      { description: 'Move down in results', keys: <Kbd>↓</Kbd> },
      { description: 'Select result', keys: <Kbd>Enter</Kbd> },
      { description: 'Filter by namespace', keys: <Kbd>ns:</Kbd> },
      { description: 'Filter by kind', keys: <Kbd>kind:</Kbd> },
      { description: 'Show most recent', keys: <Kbd>**</Kbd> },
    ],
  },
  {
    title: 'General',
    rows: [
      { description: 'Close dialogs and menus', keys: <Kbd>Esc</Kbd> },
      { description: 'Refresh current view', keys: <KeySequence keys={['Shift', 'R']} join="+" /> },
      { description: 'Previous tab', keys: <Kbd>[</Kbd> },
      { description: 'Next tab', keys: <Kbd>]</Kbd> },
    ],
  },
  {
    title: 'Detail pages',
    rows: [
      { description: 'Copy page link', keys: <Kbd>c</Kbd> },
      { description: 'Toggle favorite', keys: <Kbd>s</Kbd> },
      { description: 'Open logs (workloads)', keys: <Kbd>l</Kbd> },
    ],
  },
]

/**
 * KeyboardShortcutsModal - help dialog listing all keyboard shortcuts.
 */
export function KeyboardShortcutsModal() {
  const dialogRef = useRef(null)

  const handleClose = () => {
    keyboardShortcutsOpen.value = false
  }

  useDismiss(dialogRef, handleClose, keyboardShortcutsOpen.value)

  useEffect(() => {
    if (keyboardShortcutsOpen.value) {
      document.body.style.overflow = 'hidden'
      return () => {
        document.body.style.overflow = ''
      }
    }
  }, [keyboardShortcutsOpen.value])

  if (!keyboardShortcutsOpen.value) {
    return null
  }

  return (
    <div
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4 sm:p-6"
      onClick={handleClose}
      data-testid="keyboard-shortcuts-overlay"
    >
      <div
        ref={dialogRef}
        class="bg-white dark:bg-gray-900 shadow-xl flex flex-col overflow-hidden border border-gray-200 dark:border-gray-700 w-full max-w-2xl max-h-[calc(100vh-2rem)] rounded-lg"
        onClick={(e) => e.stopPropagation()}
        role="dialog"
        aria-modal="true"
        aria-label="Keyboard shortcuts"
        data-testid="keyboard-shortcuts-modal"
      >
        <div class="flex items-center justify-between gap-2 px-4 py-3 border-b border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800">
          <h2 class="text-sm font-semibold text-gray-900 dark:text-white">Keyboard shortcuts</h2>
          <button
            type="button"
            onClick={handleClose}
            class="inline-flex items-center p-1 rounded text-gray-400 hover:text-gray-700 dark:text-gray-500 dark:hover:text-gray-200"
            aria-label="Close keyboard shortcuts"
            data-testid="keyboard-shortcuts-close"
          >
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        <div class="overflow-y-auto px-4 py-4">
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-x-8 gap-y-6">
            {SHORTCUT_SECTIONS.map(section => (
              <ShortcutSection key={section.title} title={section.title}>
                {section.rows.map(row => (
                  <ShortcutRow key={row.description} keys={row.keys} description={row.description} />
                ))}
              </ShortcutSection>
            ))}
          </div>
        </div>
      </div>
    </div>
  )
}
