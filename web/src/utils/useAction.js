// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useState, useEffect, useCallback, useRef } from 'preact/hooks'
import { fetchWithMock } from './fetch'

/**
 * useAction - Shared hook for performing API actions with loading/error/success state
 *
 * Provides common action state management:
 * - loading: tracks which action is currently loading (null or action ID string)
 * - error: error message if action failed
 * - showSuccess: tracks which action shows success checkmark
 *
 * @param {Object} options
 * @param {Function} options.onActionStart - Callback when action starts
 * @param {Function} options.onActionComplete - Callback when action completes successfully
 * @returns {Object} Action state and perform function
 */
export function useAction({ onActionStart, onActionComplete } = {}) {
  const [loading, setLoading] = useState(null)
  const [error, setError] = useState(null)
  const [showSuccess, setShowSuccess] = useState(null)
  const successTimeoutRef = useRef(null)

  // Auto-dismiss error after 5 seconds
  useEffect(() => {
    if (error) {
      const timer = window.setTimeout(() => setError(null), 5000)
      return () => window.clearTimeout(timer)
    }
  }, [error])

  // Cleanup success timeout on unmount
  useEffect(() => {
    return () => {
      if (successTimeoutRef.current) {
        window.clearTimeout(successTimeoutRef.current)
      }
    }
  }, [])

  // Clear error manually
  const clearError = useCallback(() => setError(null), [])

  /**
   * Perform an action by calling the API
   *
   * @param {Object} options
   * @param {string} options.endpoint - API endpoint to call
   * @param {Object} options.body - Request body
   * @param {string} options.loadingId - ID to track loading state (defaults to action)
   * @param {string} options.mockPath - Path to mock module
   * @param {string} options.mockExport - Mock function export name
   * @param {boolean} options.showSuccessCheck - Whether to show success checkmark
   */
  const performAction = useCallback(async ({
    endpoint,
    body,
    loadingId,
    mockPath,
    mockExport,
    showSuccessCheck = false
  }) => {
    // Notify parent that action is starting (for faster polling)
    if (onActionStart) {
      onActionStart()
    }

    const actionId = loadingId || body.action
    setLoading(actionId)
    setError(null)

    try {
      await fetchWithMock({
        endpoint,
        mockPath,
        mockExport,
        method: 'POST',
        body
      })

      // Trigger refetch to get updated status and wait for it
      if (onActionComplete) {
        await onActionComplete()
      }

      // Show success checkmark if requested
      if (showSuccessCheck) {
        setShowSuccess(actionId)
        // Clear any existing success timeout
        if (successTimeoutRef.current) {
          window.clearTimeout(successTimeoutRef.current)
        }
        successTimeoutRef.current = window.setTimeout(() => setShowSuccess(null), 2000)
      }
    } catch (err) {
      setError(err.message)
    } finally {
      setLoading(null)
    }
  }, [onActionStart, onActionComplete])

  return {
    loading,
    error,
    showSuccess,
    performAction,
    clearError,
    setShowSuccess
  }
}
