// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

// Mock action response for POST /api/v1/action
export const mockAction = (body) => {
  return {
    success: true,
    message: `${body.action.charAt(0).toUpperCase() + body.action.slice(1)} triggered for ${body.namespace}/${body.name}`
  }
}
