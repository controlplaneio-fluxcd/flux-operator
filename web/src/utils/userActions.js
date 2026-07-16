// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { reportData } from '../app'

export const AUTH_NOT_CONFIGURED_TOOLTIP = 'Authentication is not configured'

/**
 * Resolve whether user actions are enabled from a resource/workload payload,
 * falling back to the global FluxReport userInfo.
 *
 * @param {Object} [context] - Object that may carry userActionsEnabled
 * @returns {boolean}
 */
export function isUserActionsEnabled(context) {
  if (context?.userActionsEnabled !== undefined) {
    return Boolean(context.userActionsEnabled)
  }
  if (resourceDataHasUserActionsEnabled(context)) {
    return Boolean(context.status.userActionsEnabled)
  }
  return Boolean(reportData.value?.spec?.userInfo?.userActionsEnabled)
}

function resourceDataHasUserActionsEnabled(context) {
  return context?.status?.userActionsEnabled !== undefined
}

/**
 * Build tooltip text for an action button.
 * Priority: auth not configured > missing permission > state reason > enabled title.
 *
 * @param {Object} options
 * @param {boolean} options.userActionsEnabled - Whether authentication is configured for user actions
 * @param {boolean} options.hasPermission - Whether the user has RBAC for this action
 * @param {string} options.actionLabel - Human-readable action name for permission messages
 * @param {string} [options.stateReason] - Tooltip when disabled due to resource state
 * @param {string} [options.enabledTitle] - Tooltip when the button is enabled
 * @returns {string}
 */
export function getActionTooltip({
  userActionsEnabled,
  hasPermission,
  actionLabel,
  stateReason,
  enabledTitle
}) {
  // Auth-not-configured takes priority over permission and state tooltips so
  // demo/staging deployments can advertise capabilities consistently (#959).
  if (!userActionsEnabled) {
    return AUTH_NOT_CONFIGURED_TOOLTIP
  }
  if (!hasPermission) {
    return `You don't have permission to ${actionLabel}`
  }
  if (stateReason) {
    return stateReason
  }
  return enabledTitle || ''
}

/**
 * Whether an action button should be disabled due to auth or RBAC.
 *
 * @param {boolean} userActionsEnabled
 * @param {boolean} hasPermission
 * @returns {boolean}
 */
export function isActionBlockedByAccess(userActionsEnabled, hasPermission) {
  return !userActionsEnabled || !hasPermission
}
