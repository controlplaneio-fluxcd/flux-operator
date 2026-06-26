// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useEffect, useMemo, useState } from 'preact/hooks'
import { appliedTheme } from '../../../utils/theme'
import { fetchWithMock } from '../../../utils/fetch'
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
 * Editable YAML block for live Kubernetes object edits.
 */
export function EditableYamlBlock({ data, onSaved }) {
  const yamlText = useMemo(() => data ? yaml.dump(data, { indent: 2, lineWidth: -1 }) : '', [data])
  const [editing, setEditing] = useState(false)
  const [value, setValue] = useState(yamlText)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState(null)
  const [message, setMessage] = useState(null)

  useEffect(() => {
    if (!editing) {
      setValue(yamlText)
    }
  }, [yamlText, editing])

  const startEditing = () => {
    setValue(yamlText)
    setError(null)
    setMessage(null)
    setEditing(true)
  }

  const cancelEditing = () => {
    setValue(yamlText)
    setError(null)
    setEditing(false)
  }

  const saveYaml = async () => {
    setSaving(true)
    setError(null)
    setMessage(null)
    try {
      const response = await fetchWithMock({
        endpoint: '/api/v1/object',
        mockPath: '../mock/action',
        mockExport: 'mockObjectEdit',
        method: 'PUT',
        body: {
          apiVersion: data?.apiVersion,
          kind: data?.kind,
          namespace: data?.metadata?.namespace || '',
          name: data?.metadata?.name,
          yaml: value
        }
      })
      setMessage(response?.message || 'Object updated')
      setEditing(false)
      if (onSaved) {
        await onSaved()
      }
    } catch (err) {
      setError(err.message)
    } finally {
      setSaving(false)
    }
  }

  if (!editing) {
    return (
      <div class="space-y-3">
        <div class="flex items-center justify-between gap-3">
          <p class="text-xs text-gray-500 dark:text-gray-400">
            Edit applies changes to the live Kubernetes object using your RBAC identity.
          </p>
          <button
            type="button"
            onClick={startEditing}
            class="px-3 py-1.5 text-xs font-medium rounded border border-flux-blue text-flux-blue hover:bg-blue-50 dark:border-blue-400 dark:text-blue-400 dark:hover:bg-blue-900/30"
          >
            Edit YAML
          </button>
        </div>
        {message && (
          <div class="p-2 rounded border border-green-200 bg-green-50 text-xs text-green-800 dark:border-green-800 dark:bg-green-900/20 dark:text-green-200">
            {message}
          </div>
        )}
        <YamlBlock data={data} />
      </div>
    )
  }

  return (
    <div class="space-y-3">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <p class="text-xs text-gray-500 dark:text-gray-400">
          Review carefully. Saving updates the live object immediately.
        </p>
        <div class="flex items-center gap-2">
          <button
            type="button"
            onClick={cancelEditing}
            disabled={saving}
            class="px-3 py-1.5 text-xs font-medium rounded border border-gray-300 text-gray-700 hover:bg-gray-50 disabled:text-gray-400 dark:border-gray-600 dark:text-gray-300 dark:hover:bg-gray-800"
          >
            Cancel
          </button>
          <button
            type="button"
            onClick={saveYaml}
            disabled={saving}
            class="px-3 py-1.5 text-xs font-medium rounded border border-flux-blue bg-flux-blue text-white hover:bg-blue-600 disabled:opacity-50"
          >
            {saving ? 'Saving…' : 'Save YAML'}
          </button>
        </div>
      </div>
      {error && (
        <div class="p-2 rounded border border-red-200 bg-red-50 text-xs text-red-800 dark:border-red-800 dark:bg-red-900/20 dark:text-red-200">
          {error}
        </div>
      )}
      <textarea
        aria-label="YAML object editor"
        value={value}
        onInput={(event) => setValue(event.currentTarget.value)}
        spellcheck={false}
        class="w-full min-h-[24rem] p-3 rounded-md border border-gray-300 bg-gray-50 font-mono text-xs leading-5 text-gray-900 focus:outline-none focus:ring-2 focus:ring-flux-blue dark:border-gray-700 dark:bg-gray-900 dark:text-gray-100"
      />
    </div>
  )
}
