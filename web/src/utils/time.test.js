// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { formatTimestamp, formatTime } from './time'

describe('formatTimestamp', () => {
  const now = new Date()

  it('should return "just now" for timestamps less than 1 minute ago', () => {
    const date = new Date(now.getTime() - 30 * 1000) // 30 seconds ago
    expect(formatTimestamp(date)).toBe('just now')
  })

  it('should return "Xm ago" for timestamps less than 60 minutes ago', () => {
    const date = new Date(now.getTime() - 5 * 60 * 1000) // 5 minutes ago
    expect(formatTimestamp(date)).toBe('5m ago')
  })

  it('should return "Xh ago" for timestamps less than 24 hours ago', () => {
    const date = new Date(now.getTime() - 3 * 60 * 60 * 1000) // 3 hours ago
    expect(formatTimestamp(date)).toBe('3h ago')
  })

  it('should return absolute date for timestamps older than 24 hours', () => {
    const date = new Date(now.getTime() - 25 * 60 * 60 * 1000) // 25 hours ago
    // Mock Date.toLocaleString to ensure consistent output across environments
    const mockToLocaleString = vi.fn(() => 'Jan 15, 02:30 PM');
    date.toLocaleString = mockToLocaleString;
    expect(formatTimestamp(date)).toMatch(/\w{3} \d{1,2}, \d{2}:\d{2} (AM|PM)/)
  })
})

describe('formatTime', () => {
  it('should return "Never" for null or undefined input', () => {
    expect(formatTime(null)).toBe('Never')
    expect(formatTime(undefined)).toBe('Never')
  })

  it('should format a Date object into HH:MM:SS AM/PM format', () => {
    const date = new Date('2025-11-16T14:30:45Z') // 2:30:45 PM UTC
    // Adjust for local timezone if necessary, or use a fixed locale for testing
    // For simplicity, we'll test against a known output for a specific locale/timezone
    // This might need adjustment based on the test runner's environment
    const expectedTimeRegex = /\d{2}:\d{2}:\d{2} (AM|PM)/
    expect(formatTime(date)).toMatch(expectedTimeRegex)
  })
})
