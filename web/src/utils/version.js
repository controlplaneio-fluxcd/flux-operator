// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { signal } from '@preact/signals'

// localStorage keys
const VERSION_STORAGE_KEY = 'flux-operator-version'
const UUID_STORAGE_KEY = 'flux-operator-uuid'
const UPDATE_INFO_STORAGE_KEY = 'flux-operator-update-info'

// Version check API endpoint
const VERSION_API_URL = 'https://fluxoperator.dev/api/v1/version'

/**
 * Gets the stored update info from localStorage
 * @returns {Object|null} The stored update info or null if not set
 */
export function getStoredUpdateInfo() {
  try {
    const stored = localStorage.getItem(UPDATE_INFO_STORAGE_KEY)
    return stored ? JSON.parse(stored) : null
  } catch {
    return null
  }
}

// Signal for update information (restored from localStorage on load)
export const updateInfo = signal(getStoredUpdateInfo())

/**
 * Gets or creates a unique user identifier
 * @returns {string} The user's UUID
 */
export function getOrCreateUUID() {
  let uuid = localStorage.getItem(UUID_STORAGE_KEY)
  if (!uuid) {
    uuid = crypto.randomUUID()
    localStorage.setItem(UUID_STORAGE_KEY, uuid)
  }
  return uuid
}

/**
 * Checks for updates by calling the version API
 * Only runs in production mode. Silently ignores errors.
 *
 * @param {string} version - The current Flux Operator version
 * @param {string} fluxVersion - The current Flux distribution version
 * @param {Object} env - Optional environment object for testing (defaults to import.meta.env)
 * @returns {Promise<void>}
 */
export async function checkForUpdates(version, fluxVersion, env = import.meta.env) {
  // Skip in development/test mode
  if (env.MODE !== 'production') {
    return
  }

  try {
    const uuid = getOrCreateUUID()
    const response = await fetch(VERSION_API_URL, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ version, flux_version: fluxVersion, uuid })
    })

    if (!response.ok) {
      // Silently ignore errors (e.g., 502 Bad Gateway)
      return
    }

    const data = await response.json()

    // Persist successful response to localStorage
    localStorage.setItem(UPDATE_INFO_STORAGE_KEY, JSON.stringify(data))

    // Update the signal
    updateInfo.value = data
  } catch {
    // Silently ignore network errors
  }
}

/**
 * Gets the stored Flux Operator version from localStorage
 * @returns {string|null} The stored version or null if not set
 */
export function getStoredVersion() {
  return localStorage.getItem(VERSION_STORAGE_KEY)
}

/**
 * Checks if the Flux Operator version has changed and triggers a reload if needed
 *
 * This function compares the new version against the stored version in localStorage.
 * If a version change is detected (and a previous version was stored), it triggers
 * a full page reload to ensure the browser loads the latest assets.
 *
 * Also calls checkForUpdates API when:
 * - First time storing a version (new user)
 * - Version changes (operator upgrade)
 *
 * @param {string} newVersion - The current Flux Operator version from the API
 * @param {string} fluxVersion - The current Flux distribution version
 * @returns {boolean} True if a reload was triggered, false otherwise
 */
export function checkVersionChange(newVersion, fluxVersion) {
  // Skip if no version provided
  if (!newVersion) {
    return false
  }

  const storedVersion = getStoredVersion()
  const cachedUpdateInfo = getStoredUpdateInfo()
  // Only skip if we have cached info for THIS version
  const hasCachedInfoForVersion = cachedUpdateInfo?.current === newVersion

  // Store the new version
  localStorage.setItem(VERSION_STORAGE_KEY, newVersion)

  // Check for updates if we don't have cached info for this specific version
  if (!hasCachedInfoForVersion) {
    checkForUpdates(newVersion, fluxVersion)
  }

  // If there was a previous version and it's different, trigger reload
  if (storedVersion && storedVersion !== newVersion) {
    window.location.reload()
    return true
  }

  return false
}
