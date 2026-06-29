// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

/**
 * ToggleGroup - a segmented control: one button per option, the selected one
 * highlighted, clicking calls onChange with its value. With a `label` it renders a
 * settings-row layout (label column + fixed-width control); without one it renders
 * just the button group sized to its content (toolbar use). Provide `ariaLabel`
 * when there is no visible `label`.
 *
 * @param {Object} props
 * @param {string} [props.label] - Row label and group aria-label (settings layout)
 * @param {string} [props.ariaLabel] - aria-label when there is no visible label
 * @param {Array<{value: any, label: string, testid?: string}>} props.options - The choices
 * @param {any} props.value - The currently selected option value
 * @param {Function} props.onChange - Called with the chosen value
 * @param {string} [props.testid] - data-testid for the group container
 * @param {string} [props.groupClass] - Extra classes for the button-group container
 */
export function ToggleGroup({ label, ariaLabel, options, value, onChange, testid, groupClass = '' }) {
  const group = (
    <div
      class={`${label ? 'flex w-60 max-w-full' : 'inline-flex'} rounded-md border border-gray-300 dark:border-gray-600 overflow-hidden ${groupClass}`}
      role="group"
      aria-label={label || ariaLabel}
      data-testid={testid}
    >
      {options.map((o, i) => (
        <button
          key={o.value}
          type="button"
          onClick={() => onChange(o.value)}
          class={`flex-1 px-3 py-1 text-xs text-center focus:outline-none focus-visible:ring-2 focus-visible:ring-flux-blue focus-visible:ring-inset ${
            i > 0 ? 'border-l border-gray-300 dark:border-gray-600' : ''
          } ${
            value === o.value
              ? 'bg-flux-blue text-white'
              : 'bg-white dark:bg-gray-700 text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-600'
          }`}
          data-testid={o.testid}
          aria-pressed={value === o.value}
        >
          {o.label}
        </button>
      ))}
    </div>
  )

  // No label: just the button group (toolbar variant).
  if (!label) return group

  // Labelled: settings-row layout with a fixed-width label column.
  return (
    <div class="flex items-center gap-3">
      <span class="text-xs font-medium text-gray-600 dark:text-gray-300 w-20 flex-shrink-0">{label}</span>
      {group}
    </div>
  )
}
