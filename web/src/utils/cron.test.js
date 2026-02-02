// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect } from 'vitest'
import { formatCronExpression, formatScheduleMessage } from './cron'

describe('formatCronExpression', () => {
  describe('invalid inputs', () => {
    it('should return null for null input', () => {
      expect(formatCronExpression(null)).toBeNull()
    })

    it('should return undefined for undefined input', () => {
      expect(formatCronExpression(undefined)).toBeUndefined()
    })

    it('should return non-string inputs as-is', () => {
      expect(formatCronExpression(123)).toBe(123)
    })

    it('should return invalid cron expressions as-is', () => {
      expect(formatCronExpression('invalid')).toBe('invalid')
      expect(formatCronExpression('* * *')).toBe('* * *')
      expect(formatCronExpression('* * * * * *')).toBe('* * * * * *')
    })
  })

  describe('every minute patterns', () => {
    it('should format "* * * * *" as "Every minute"', () => {
      expect(formatCronExpression('* * * * *')).toBe('Every minute')
    })

    it('should format "*/1 * * * *" as "Every minute"', () => {
      expect(formatCronExpression('*/1 * * * *')).toBe('Every minute')
    })
  })

  describe('every N minutes patterns', () => {
    it('should format "*/5 * * * *" as "Every 5 minutes"', () => {
      expect(formatCronExpression('*/5 * * * *')).toBe('Every 5 minutes')
    })

    it('should format "*/15 * * * *" as "Every 15 minutes"', () => {
      expect(formatCronExpression('*/15 * * * *')).toBe('Every 15 minutes')
    })

    it('should format "*/30 * * * *" as "Every 30 minutes"', () => {
      expect(formatCronExpression('*/30 * * * *')).toBe('Every 30 minutes')
    })
  })

  describe('hourly patterns', () => {
    it('should format "0 * * * *" as "Every hour"', () => {
      expect(formatCronExpression('0 * * * *')).toBe('Every hour')
    })

    it('should format "30 * * * *" as "Every hour at minute 30"', () => {
      expect(formatCronExpression('30 * * * *')).toBe('Every hour at minute 30')
    })

    it('should format "0 */1 * * *" as "Every hour"', () => {
      expect(formatCronExpression('0 */1 * * *')).toBe('Every hour')
    })

    it('should format "0 */2 * * *" as "Every 2 hours"', () => {
      expect(formatCronExpression('0 */2 * * *')).toBe('Every 2 hours')
    })

    it('should format "0 */6 * * *" as "Every 6 hours"', () => {
      expect(formatCronExpression('0 */6 * * *')).toBe('Every 6 hours')
    })
  })

  describe('daily patterns', () => {
    it('should format "0 0 * * *" as "Daily at 00:00"', () => {
      expect(formatCronExpression('0 0 * * *')).toBe('Daily at 00:00')
    })

    it('should format "30 9 * * *" as "Daily at 09:30"', () => {
      expect(formatCronExpression('30 9 * * *')).toBe('Daily at 09:30')
    })

    it('should format "0 14 * * *" as "Daily at 14:00"', () => {
      expect(formatCronExpression('0 14 * * *')).toBe('Daily at 14:00')
    })
  })

  describe('weekly patterns', () => {
    it('should format "0 0 * * 0" as "Weekly on Sunday at 00:00"', () => {
      expect(formatCronExpression('0 0 * * 0')).toBe('Weekly on Sunday at 00:00')
    })

    it('should format "0 9 * * 1" as "Weekly on Monday at 09:00"', () => {
      expect(formatCronExpression('0 9 * * 1')).toBe('Weekly on Monday at 09:00')
    })

    it('should format "30 17 * * 5" as "Weekly on Friday at 17:30"', () => {
      expect(formatCronExpression('30 17 * * 5')).toBe('Weekly on Friday at 17:30')
    })
  })

  describe('monthly patterns', () => {
    it('should format "0 0 1 * *" as "Monthly on the 1st at 00:00"', () => {
      expect(formatCronExpression('0 0 1 * *')).toBe('Monthly on the 1st at 00:00')
    })

    it('should format "0 0 2 * *" as "Monthly on the 2nd at 00:00"', () => {
      expect(formatCronExpression('0 0 2 * *')).toBe('Monthly on the 2nd at 00:00')
    })

    it('should format "0 0 3 * *" as "Monthly on the 3rd at 00:00"', () => {
      expect(formatCronExpression('0 0 3 * *')).toBe('Monthly on the 3rd at 00:00')
    })

    it('should format "0 0 15 * *" as "Monthly on the 15th at 00:00"', () => {
      expect(formatCronExpression('0 0 15 * *')).toBe('Monthly on the 15th at 00:00')
    })
  })

  describe('complex patterns (fallback)', () => {
    it('should return complex expressions as-is', () => {
      // Patterns with specific month (not *) fall through all checks
      expect(formatCronExpression('0 0 1 1 *')).toBe('0 0 1 1 *')
      expect(formatCronExpression('0 0 * 6 *')).toBe('0 0 * 6 *')
      // Pattern with both day-of-month and weekday
      expect(formatCronExpression('0 0 15 * 1')).toBe('0 0 15 * 1')
    })
  })

  describe('weekday ranges', () => {
    it('should format weekday ranges as weekly (using first day)', () => {
      // Note: ranges like 1-5 are parsed as the first number
      expect(formatCronExpression('0 0 * * 1-5')).toBe('Weekly on Monday at 00:00')
    })
  })

  describe('whitespace handling', () => {
    it('should handle extra whitespace', () => {
      expect(formatCronExpression('  */5  *  *  *  *  ')).toBe('Every 5 minutes')
    })
  })
})

describe('formatScheduleMessage', () => {
  describe('invalid inputs', () => {
    it('should return null for null input', () => {
      expect(formatScheduleMessage(null)).toBeNull()
    })

    it('should return undefined for undefined input', () => {
      expect(formatScheduleMessage(undefined)).toBeUndefined()
    })

    it('should return non-schedule messages as-is', () => {
      expect(formatScheduleMessage('Some other message')).toBe('Some other message')
      expect(formatScheduleMessage('Ready')).toBe('Ready')
    })
  })

  describe('cron expression formatting', () => {
    it('should format "*/5 * * * *" with Schedule prefix', () => {
      expect(formatScheduleMessage('*/5 * * * *')).toBe('Schedule: Every 5 minutes')
    })

    it('should format "0 * * * *" with Schedule prefix', () => {
      expect(formatScheduleMessage('0 * * * *')).toBe('Schedule: Every hour')
    })

    it('should format "0 0 * * *" with Schedule prefix', () => {
      expect(formatScheduleMessage('0 0 * * *')).toBe('Schedule: Daily at 00:00')
    })

    it('should keep complex schedules with Schedule prefix', () => {
      // Specific month patterns keep the raw cron
      expect(formatScheduleMessage('0 0 1 1 *')).toBe('Schedule: 0 0 1 1 *')
    })
  })
})
