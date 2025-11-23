// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useMemo } from 'preact/hooks'
import yaml from 'js-yaml'
import Prism from 'prismjs'
import 'prismjs/components/prism-yaml'

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

/**
 * YAML code block with syntax highlighting
 */
export function YamlBlock({ data }) {
  const highlighted = useMemo(() => {
    if (!data) return ''
    const yamlStr = yaml.dump(data, { indent: 2, lineWidth: -1 })
    return Prism.highlight(yamlStr, Prism.languages.yaml, 'yaml')
  }, [data])

  return (
    <pre class="p-3 bg-gray-50 dark:bg-gray-900 rounded-md overflow-x-auto language-yaml" style="font-size: 12px; line-height: 1.5;">
      <code class="language-yaml" style="font-size: 12px;" dangerouslySetInnerHTML={{ __html: highlighted }} />
    </pre>
  )
}

/**
 * Get event status badge class
 */
export function getEventBadgeClass(type) {
  return type === 'Normal'
    ? 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400'
    : 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400'
}
