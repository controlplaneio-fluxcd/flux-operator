// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

// Mock action response for POST /api/v1/resource/action
export const mockAction = (body) => {
  return {
    success: true,
    message: `${body.action.charAt(0).toUpperCase() + body.action.slice(1)} triggered for ${body.namespace}/${body.name}`
  }
}

// Mock workload action response for POST /api/v1/workload/action
export const mockWorkloadAction = (body) => {
  const actionMessages = {
    restart: body.kind === 'CronJob' ? 'Job created' : 'Rollout restart triggered'
  }
  const message = actionMessages[body.action] || `${body.action} triggered`
  return {
    success: true,
    message: `${message} for ${body.namespace}/${body.name}`
  }
}
