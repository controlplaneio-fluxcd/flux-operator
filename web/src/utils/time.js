// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

/**
 * Format a timestamp into a human-readable relative time string.
 *
 * @param {string|Date} timestamp - The timestamp to format
 * @returns {string} Formatted time string (e.g., "just now", "5m ago", "3h ago", or absolute date)
 *
 * Examples:
 * - Less than 1 minute: "just now"
 * - Less than 60 minutes: "5m ago"
 * - Less than 24 hours: "3h ago"
 * - Older (same year): "Jan 15, 02:30 PM"
 * - Older (different year): "Jan 15, 2024, 02:30 PM"
 */
export const formatTimestamp = (timestamp) => {
  const date = new Date(timestamp)
  const now = new Date()
  const diffMs = now - date
  const diffMins = Math.floor(diffMs / 60000)

  if (diffMins < 1) return 'just now'
  if (diffMins < 60) return `${diffMins}m ago`
  if (diffMins < 1440) return `${Math.floor(diffMins / 60)}h ago`

  // Include year if date is from a different year
  const includeYear = date.getFullYear() !== now.getFullYear()
  const options = {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit'
  }
  if (includeYear) {
    options.year = 'numeric'
  }
  return date.toLocaleString('en-US', options)
}

/**
 * Format a date into an absolute time string (HH:MM:SS format).
 *
 * @param {Date|null} date - The date to format
 * @returns {string} Formatted time string (e.g., "02:30:45 PM") or "Never" if date is null
 *
 * Examples:
 * - Valid date: "02:30:45 PM"
 * - Null/undefined: "Never"
 */
export const formatTime = (date) => {
  if (!date) return 'Never'
  return new Intl.DateTimeFormat('en-US', {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  }).format(date)
}