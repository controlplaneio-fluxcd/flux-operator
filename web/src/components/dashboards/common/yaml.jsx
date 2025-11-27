// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useEffect, useMemo } from 'preact/hooks'
import { appliedTheme } from '../../../utils/theme'
import yaml from 'js-yaml'
import Prism from 'prismjs'
import 'prismjs/components/prism-yaml'

// Import Prism themes as URLs for dynamic loading
import prismLight from 'prismjs/themes/prism.css?url'
import prismDark from 'prismjs/themes/prism-tomorrow.css?url'

const LINK_ID = 'prism-theme-link'

/**
 * Hook that dynamically loads the appropriate Prism theme based on the current app theme.
 *
 * This hook manages a <link> element in the document head that loads either:
 * - prism.css (light theme)
 * - prism-tomorrow.css (dark theme)
 *
 * The theme automatically updates when appliedTheme changes.
 *
 * @example
 * function MyComponent() {
 *   usePrismTheme()
 *   // Now Prism highlighting will use the correct theme
 *   return <pre><code class="language-yaml">...</code></pre>
 * }
 */
export function usePrismTheme() {
  useEffect(() => {
    // Remove existing Prism theme link if present
    const existingLink = document.getElementById(LINK_ID)
    if (existingLink) {
      existingLink.remove()
    }

    // Add new theme link based on current theme
    const link = document.createElement('link')
    link.id = LINK_ID
    link.rel = 'stylesheet'
    link.href = appliedTheme.value === 'dark' ? prismDark : prismLight
    document.head.appendChild(link)

    // Cleanup on unmount
    return () => {
      const link = document.getElementById(LINK_ID)
      if (link) {
        link.remove()
      }
    }
  }, [appliedTheme.value])
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
