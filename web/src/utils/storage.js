// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

// Guarded wrappers around Web Storage writes. A storage-disabled environment —
// notably Safari private mode, where setItem throws QuotaExceededError because the
// quota is zero — must never break rendering or a user action, so write failures
// are swallowed. Reads are unaffected: getItem returns null when storage is
// unavailable, which the callers already handle.

/**
 * Write a value to localStorage, ignoring failures (private mode / quota exceeded).
 *
 * @param {string} key - The storage key
 * @param {string} value - The already-serialized string value
 */
export function writeLocalStorage(key, value) {
  try {
    localStorage.setItem(key, value)
  } catch {
    // Storage unavailable or full — skip persisting rather than break the app.
  }
}

/**
 * Write a value to sessionStorage, ignoring failures (private mode / quota exceeded).
 *
 * @param {string} key - The storage key
 * @param {string} value - The already-serialized string value
 */
export function writeSessionStorage(key, value) {
  try {
    window.sessionStorage.setItem(key, value)
  } catch {
    // Storage unavailable or full — skip persisting rather than break the app.
  }
}
