// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { signal } from '@preact/signals'

/**
 * Signal to indicate authentication is required
 * Set to true when any API call returns 401 Unauthorized
 * The App component reacts to this and shows the LoginPage
 */
export const authRequired = signal(false)

/**
 * Check if mock data should be used based on environment
 * @param {Object} env - Environment object (defaults to import.meta.env)
 * @returns {boolean}
 */
export function shouldUseMockData(env = import.meta.env) {
  return env.MODE !== 'production' && env.VITE_USE_MOCK_DATA === 'true'
}

/**
 * Unified API fetch utility that handles mock data vs real API calls
 *
 * @param {Object} options - Fetch options
 * @param {string} options.endpoint - API endpoint to call (e.g., '/api/v1/report')
 * @param {string} options.mockPath - Path to mock data module (e.g., './mock/report')
 * @param {string} options.mockExport - Named export from mock module (e.g., 'mockReport')
 * @param {Object} options.env - Optional environment object for testing (defaults to import.meta.env)
 * @param {string} options.method - HTTP method (defaults to 'GET')
 * @param {any} options.body - Request body for POST/PUT requests (will be JSON stringified)
 * @returns {Promise<any>} - Parsed JSON response or mock data
 */
export async function fetchWithMock({ endpoint, mockPath, mockExport, env, method = 'GET', body }) {
  // Check if we're in dev/test mode AND mock data is enabled
  // In production builds, this entire block gets tree-shaken out
  if (shouldUseMockData(env)) {
    // Simulate network delay for realistic behavior
    await new Promise(resolve => setTimeout(resolve, 300))
    // Dynamic import only happens in non-production mode with mocks enabled
    const module = await import(/* @vite-ignore */ mockPath)
    const mockData = module[mockExport]

    // If the mock export is a function, call it with the endpoint URL or body to support filtering
    // Otherwise, return the static mock data object
    if (typeof mockData === 'function') {
      return mockData(body !== undefined ? body : endpoint)
    } else {
      return mockData
    }
  } else {
    // Fetch from real API
    const fetchOptions = { method }
    if (body !== undefined) {
      fetchOptions.headers = { 'Content-Type': 'application/json' }
      fetchOptions.body = JSON.stringify(body)
    }
    const response = await fetch(endpoint, fetchOptions)
    if (!response.ok) {
      let err
      try {
        err = await response.text()
      } catch (e) {
        err = `Unable to read response body: ${e.message}`
      }

      if (response.status === 401) {
        // Authentication required - set signal to trigger LoginPage
        authRequired.value = true
      }

      throw new Error(`HTTP error! status: ${response.status}, error: ${err}`)
    }
    return await response.json()
  }
}
