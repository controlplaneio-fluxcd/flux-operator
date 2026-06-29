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

// Number of mounted components currently relying on the Prism theme link. The
// link is shared, so it is created on the first mount and removed only when the
// last consumer unmounts — otherwise closing one consumer (e.g. a modal) would
// strip highlighting from others still on screen.
let prismThemeRefCount = 0

// (Re)create the shared theme <link>, replacing any stale one, using the href
// for the current theme.
function setPrismThemeLink() {
  const existingLink = document.getElementById(LINK_ID)
  if (existingLink) {
    existingLink.remove()
  }
  const link = document.createElement('link')
  link.id = LINK_ID
  link.rel = 'stylesheet'
  link.href = appliedTheme.value === 'dark' ? prismDark : prismLight
  document.head.appendChild(link)
}

/**
 * Hook that dynamically loads the appropriate Prism theme based on the current app theme.
 *
 * This hook manages a shared <link> element in the document head that loads either:
 * - prism.css (light theme)
 * - prism-tomorrow.css (dark theme)
 *
 * The link is reference-counted so multiple components can use it concurrently;
 * it is removed only when the last consumer unmounts. The theme automatically
 * updates when appliedTheme changes.
 *
 * @example
 * function MyComponent() {
 *   usePrismTheme()
 *   // Now Prism highlighting will use the correct theme
 *   return <pre><code class="language-yaml">...</code></pre>
 * }
 */
export function usePrismTheme() {
  // Manage the shared link on first mount / last unmount.
  useEffect(() => {
    if (prismThemeRefCount === 0) {
      setPrismThemeLink()
    }
    prismThemeRefCount++

    return () => {
      prismThemeRefCount--
      if (prismThemeRefCount === 0) {
        const link = document.getElementById(LINK_ID)
        if (link) {
          link.remove()
        }
      }
    }
  }, [])

  // Update the href in place when the theme changes, without disturbing the
  // reference count or removing the link other consumers still rely on.
  useEffect(() => {
    const link = document.getElementById(LINK_ID)
    if (link) {
      link.href = appliedTheme.value === 'dark' ? prismDark : prismLight
    }
  }, [appliedTheme.value])
}

/**
 * YAML code block with syntax highlighting.
 *
 * @param {Object} props
 * @param {*} props.data - The object to render as YAML
 * @param {boolean} [props.nested] - When true, drops the block's own padding,
 *   background and rounding so it flows inside an already-padded container (e.g.
 *   the compact list detail panel) instead of nesting a second padded box.
 */
export function YamlBlock({ data, nested = false }) {
  const highlighted = useMemo(() => {
    if (!data) return ''
    const yamlStr = yaml.dump(data, { indent: 2, lineWidth: -1 })
    return Prism.highlight(yamlStr, Prism.languages.yaml, 'yaml')
  }, [data])

  // Inline styles win over the Prism theme stylesheet, whose
  // `pre[class*="language-"]` rule sets its own background, padding, margin and
  // radius. In nested mode we strip all of that so the YAML flows inside the
  // already-padded detail panel instead of nesting a second padded box. The
  // monospace YAML also reads visually larger than the surrounding 12px sans
  // text, so nested mode shrinks it to 11px to match the compact detail panel.
  const fontSize = nested ? '11px' : '12px'
  const preStyle = nested
    ? `font-size: ${fontSize}; line-height: 1.5; background: transparent; padding: 0; margin: 0;`
    : `font-size: ${fontSize}; line-height: 1.5;`

  return (
    <pre
      class={`overflow-x-auto language-yaml${nested ? '' : ' p-3 bg-gray-50 dark:bg-gray-900 rounded-md'}`}
      style={preStyle}
    >
      <code class="language-yaml" style={`font-size: ${fontSize}; background: transparent;`} dangerouslySetInnerHTML={{ __html: highlighted }} />
    </pre>
  )
}
