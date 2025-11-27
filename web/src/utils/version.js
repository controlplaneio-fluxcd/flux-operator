// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

// localStorage key for storing the Flux Operator version
const VERSION_STORAGE_KEY = 'flux-operator-version'

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
 * @param {string} newVersion - The current Flux Operator version from the API
 * @returns {boolean} True if a reload was triggered, false otherwise
 */
export function checkVersionChange(newVersion) {
  // Skip if no version provided
  if (!newVersion) {
    return false
  }

  const storedVersion = getStoredVersion()

  // Store the new version
  localStorage.setItem(VERSION_STORAGE_KEY, newVersion)

  // If there was a previous version and it's different, trigger reload
  if (storedVersion && storedVersion !== newVersion) {
    window.location.reload()
    return true
  }

  return false
}
