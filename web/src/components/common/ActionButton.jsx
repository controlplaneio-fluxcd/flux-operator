// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

/**
 * Action button with a reliable hover tooltip when disabled.
 * Native title attributes on disabled buttons are inconsistent across browsers.
 *
 * @param {Object} props
 * @param {import('preact').ComponentChildren} props.children
 * @param {string} [props.title] - Tooltip text
 * @param {boolean} [props.disabled]
 * @param {string} [props.class]
 * @param {string} [props['data-testid']]
 * @param {Function} [props.onClick]
 */
export function ActionButton({ title, disabled, children, ...buttonProps }) {
  const button = (
    <button
      {...buttonProps}
      disabled={disabled}
      onClick={disabled ? undefined : buttonProps.onClick}
    >
      {children}
    </button>
  )

  if (!title) {
    return button
  }

  if (disabled) {
    return (
      <span class="inline-flex" title={title}>
        {button}
      </span>
    )
  }

  return (
    <button {...buttonProps} title={title} onClick={buttonProps.onClick}>
      {children}
    </button>
  )
}
