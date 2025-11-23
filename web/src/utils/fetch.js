// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

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
 * @returns {Promise<any>} - Parsed JSON response or mock data
 */
export async function fetchWithMock({ endpoint, mockPath, mockExport, env }) {
  // Check if we're in dev/test mode AND mock data is enabled
  // In production builds, this entire block gets tree-shaken out
  if (shouldUseMockData(env)) {
    // Simulate network delay for realistic behavior
    await new Promise(resolve => setTimeout(resolve, 300))
    // Dynamic import only happens in non-production mode with mocks enabled
    const module = await import(/* @vite-ignore */ mockPath)
    const mockData = module[mockExport]

    // If the mock export is a function, call it with the endpoint URL to support filtering
    // Otherwise, return the static mock data object
    if (typeof mockData === 'function') {
      return mockData(endpoint)
    } else {
      return mockData
    }
  } else {
    // Fetch from real API
    const response = await fetch(endpoint)
    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`)
    }
    return await response.json()
  }
}
