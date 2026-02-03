// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

/**
 * Convert a cron expression to human-readable format.
 * Handles common cron patterns; falls back to raw expression for complex ones.
 * @param {string} cron - Cron expression (5 fields: minute hour day month weekday)
 * @returns {string} Human-readable description
 */
export function formatCronExpression(cron) {
  if (!cron || typeof cron !== 'string') return cron

  const parts = cron.trim().split(/\s+/)
  if (parts.length !== 5) return cron

  const [minute, hour, day, month, weekday] = parts

  // Every minute
  if (minute === '*' && hour === '*' && day === '*' && month === '*' && weekday === '*') {
    return 'Every minute'
  }

  // Every N minutes
  if (minute.startsWith('*/') && hour === '*' && day === '*' && month === '*' && weekday === '*') {
    const interval = parseInt(minute.slice(2), 10)
    if (interval === 1) return 'Every minute'
    return `Every ${interval} minutes`
  }

  // Every hour at specific minute
  if (!minute.includes('*') && !minute.includes('/') && hour === '*' && day === '*' && month === '*' && weekday === '*') {
    const min = parseInt(minute, 10)
    if (min === 0) return 'Every hour'
    return `Every hour at minute ${min}`
  }

  // Every N hours
  if (minute === '0' && hour.startsWith('*/') && day === '*' && month === '*' && weekday === '*') {
    const interval = parseInt(hour.slice(2), 10)
    if (interval === 1) return 'Every hour'
    return `Every ${interval} hours`
  }

  // Daily at specific time
  if (!minute.includes('*') && !hour.includes('*') && day === '*' && month === '*' && weekday === '*') {
    const h = parseInt(hour, 10)
    const m = parseInt(minute, 10)
    const time = `${h.toString().padStart(2, '0')}:${m.toString().padStart(2, '0')}`
    return `Daily at ${time}`
  }

  // Weekly on specific day
  if (!minute.includes('*') && !hour.includes('*') && day === '*' && month === '*' && !weekday.includes('*')) {
    const h = parseInt(hour, 10)
    const m = parseInt(minute, 10)
    const time = `${h.toString().padStart(2, '0')}:${m.toString().padStart(2, '0')}`
    const dayNames = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday']
    const dayNum = parseInt(weekday, 10)
    const dayName = dayNames[dayNum] || weekday
    return `Weekly on ${dayName} at ${time}`
  }

  // Monthly on specific day
  if (!minute.includes('*') && !hour.includes('*') && !day.includes('*') && month === '*' && weekday === '*') {
    const h = parseInt(hour, 10)
    const m = parseInt(minute, 10)
    const d = parseInt(day, 10)
    const time = `${h.toString().padStart(2, '0')}:${m.toString().padStart(2, '0')}`
    const suffix = d === 1 ? 'st' : d === 2 ? 'nd' : d === 3 ? 'rd' : 'th'
    return `Monthly on the ${d}${suffix} at ${time}`
  }

  // Fall back to raw expression for complex patterns
  return cron
}

// Regex to validate cron field: digits, *, /, -, or comma
const cronFieldPattern = /^[\d*/,-]+$/

/**
 * Check if a string looks like a cron expression.
 * @param {string} str - String to check
 * @returns {boolean} True if it looks like a cron expression
 */
function isCronExpression(str) {
  const parts = str.trim().split(/\s+/)
  if (parts.length !== 5) return false
  return parts.every(part => cronFieldPattern.test(part))
}

/**
 * Format a cron schedule message for display.
 * Converts cron expression to human-readable format with "Schedule: " prefix.
 * @param {string} message - Cron expression or status message
 * @returns {string} Human-readable schedule or original message
 */
export function formatScheduleMessage(message) {
  if (!message || typeof message !== 'string') return message
  if (!isCronExpression(message)) return message

  return `Schedule: ${formatCronExpression(message)}`
}
