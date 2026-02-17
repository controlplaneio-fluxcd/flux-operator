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
    const result = formatTimestamp(date)
    // Should not be a relative time string
    expect(result).not.toMatch(/ago$/)
    expect(result).not.toBe('just now')
    // Should contain some date/time content
    expect(result.length).toBeGreaterThan(0)
  })

  it('should include year for timestamps from a different year', () => {
    const lastYear = now.getFullYear() - 1
    const date = new Date(lastYear, 0, 15, 14, 30) // Jan 15 of last year
    // Should include the year in the output
    expect(formatTimestamp(date)).toContain(String(lastYear))
  })
})

describe('formatTime', () => {
  it('should return "Never" for null or undefined input', () => {
    expect(formatTime(null)).toBe('Never')
    expect(formatTime(undefined)).toBe('Never')
  })

  it('should format a Date object into a time string with hours, minutes, and seconds', () => {
    const date = new Date('2025-11-16T14:30:45Z') // 2:30:45 PM UTC
    const result = formatTime(date)
    // Should contain time components (works for both 12h and 24h locales)
    expect(result).toMatch(/\d{1,2}:\d{2}:\d{2}/)
  })
})
