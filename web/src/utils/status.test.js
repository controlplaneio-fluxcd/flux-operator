// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect } from 'vitest'
import {
  getStatusBadgeClass,
  getWorkloadStatusBadgeClass,
  formatWorkloadStatus,
  getHistoryStatusBadgeClass,
  getHistoryDotClass,
  getEventBadgeClass,
  getContainerStateBadgeClass,
  getStatusBarColor,
  getEventBarColor,
  getStatusBorderClass,
  cleanStatus
} from './status'

describe('status utilities', () => {
  describe('getStatusBadgeClass', () => {
    it('should return green for Ready status', () => {
      expect(getStatusBadgeClass('Ready')).toContain('bg-green-100')
      expect(getStatusBadgeClass('Ready')).toContain('text-green-800')
    })

    it('should return red for Failed status', () => {
      expect(getStatusBadgeClass('Failed')).toContain('bg-red-100')
      expect(getStatusBadgeClass('Failed')).toContain('text-red-800')
    })

    it('should return gray for Unknown status', () => {
      expect(getStatusBadgeClass('Unknown')).toContain('bg-gray-100')
      expect(getStatusBadgeClass('Unknown')).toContain('text-gray-800')
    })

    it('should return blue for Progressing status', () => {
      expect(getStatusBadgeClass('Progressing')).toContain('bg-blue-100')
      expect(getStatusBadgeClass('Progressing')).toContain('text-blue-800')
    })

    it('should return yellow for Suspended status', () => {
      expect(getStatusBadgeClass('Suspended')).toContain('bg-yellow-100')
      expect(getStatusBadgeClass('Suspended')).toContain('text-yellow-800')
    })

    it('should return gray for unknown status values', () => {
      expect(getStatusBadgeClass('SomeOtherStatus')).toContain('bg-gray-100')
      expect(getStatusBadgeClass('SomeOtherStatus')).toContain('text-gray-800')
    })

    it('should include dark mode classes', () => {
      expect(getStatusBadgeClass('Ready')).toContain('dark:bg-green-900/30')
      expect(getStatusBadgeClass('Ready')).toContain('dark:text-green-400')
    })
  })

  describe('getWorkloadStatusBadgeClass', () => {
    it('should return green for Current status (backend value)', () => {
      expect(getWorkloadStatusBadgeClass('Current')).toContain('bg-green-100')
    })

    it('should return green for Ready status (display value)', () => {
      expect(getWorkloadStatusBadgeClass('Ready')).toContain('bg-green-100')
    })

    it('should return red for Failed status', () => {
      expect(getWorkloadStatusBadgeClass('Failed')).toContain('bg-red-100')
    })

    it('should return blue for InProgress status (backend value)', () => {
      expect(getWorkloadStatusBadgeClass('InProgress')).toContain('bg-blue-100')
    })

    it('should return blue for Progressing status (display value)', () => {
      expect(getWorkloadStatusBadgeClass('Progressing')).toContain('bg-blue-100')
    })

    it('should return yellow for Terminating status', () => {
      expect(getWorkloadStatusBadgeClass('Terminating')).toContain('bg-yellow-100')
    })

    it('should return gray for unknown status values', () => {
      expect(getWorkloadStatusBadgeClass('Unknown')).toContain('bg-gray-100')
    })
  })

  describe('formatWorkloadStatus', () => {
    it('should transform Current to Ready', () => {
      expect(formatWorkloadStatus('Current')).toBe('Ready')
    })

    it('should transform InProgress to Progressing', () => {
      expect(formatWorkloadStatus('InProgress')).toBe('Progressing')
    })

    it('should pass through Failed unchanged', () => {
      expect(formatWorkloadStatus('Failed')).toBe('Failed')
    })

    it('should pass through Terminating unchanged', () => {
      expect(formatWorkloadStatus('Terminating')).toBe('Terminating')
    })

    it('should pass through unknown values unchanged', () => {
      expect(formatWorkloadStatus('SomeOther')).toBe('SomeOther')
    })
  })

  describe('getHistoryStatusBadgeClass', () => {
    describe('success statuses (green)', () => {
      it('should return green for ReconciliationSucceeded', () => {
        expect(getHistoryStatusBadgeClass('ReconciliationSucceeded')).toContain('bg-green-100')
      })

      it('should return green for CustomSucceeded', () => {
        expect(getHistoryStatusBadgeClass('CustomSucceeded')).toContain('bg-green-100')
      })

      it('should return green for deployed (Helm)', () => {
        expect(getHistoryStatusBadgeClass('deployed')).toContain('bg-green-100')
      })
    })

    describe('failure statuses (red)', () => {
      it('should return red for ReconciliationFailed', () => {
        expect(getHistoryStatusBadgeClass('ReconciliationFailed')).toContain('bg-red-100')
      })

      it('should return red for HealthCheckFailed', () => {
        expect(getHistoryStatusBadgeClass('HealthCheckFailed')).toContain('bg-red-100')
      })

      it('should return red for BuildFailed', () => {
        expect(getHistoryStatusBadgeClass('BuildFailed')).toContain('bg-red-100')
      })

      it('should return red for failed (Helm)', () => {
        expect(getHistoryStatusBadgeClass('failed')).toContain('bg-red-100')
      })
    })

    describe('other statuses (yellow)', () => {
      it('should return yellow for superseded (Helm)', () => {
        expect(getHistoryStatusBadgeClass('superseded')).toContain('bg-yellow-100')
      })

      it('should return yellow for Progressing', () => {
        expect(getHistoryStatusBadgeClass('Progressing')).toContain('bg-yellow-100')
      })

      it('should return yellow for pending-upgrade (Helm)', () => {
        expect(getHistoryStatusBadgeClass('pending-upgrade')).toContain('bg-yellow-100')
      })
    })

    describe('edge cases', () => {
      it('should handle null status', () => {
        expect(getHistoryStatusBadgeClass(null)).toContain('bg-yellow-100')
      })

      it('should handle undefined status', () => {
        expect(getHistoryStatusBadgeClass(undefined)).toContain('bg-yellow-100')
      })

      it('should handle empty string', () => {
        expect(getHistoryStatusBadgeClass('')).toContain('bg-yellow-100')
      })
    })
  })

  describe('getHistoryDotClass', () => {
    it('should return green for success statuses', () => {
      expect(getHistoryDotClass('ReconciliationSucceeded')).toContain('bg-green-500')
      expect(getHistoryDotClass('deployed')).toContain('bg-green-500')
    })

    it('should return red for failure statuses', () => {
      expect(getHistoryDotClass('ReconciliationFailed')).toContain('bg-red-500')
      expect(getHistoryDotClass('HealthCheckFailed')).toContain('bg-red-500')
    })

    it('should return yellow for other statuses', () => {
      expect(getHistoryDotClass('superseded')).toContain('bg-yellow-500')
      expect(getHistoryDotClass('Progressing')).toContain('bg-yellow-500')
    })

    it('should include dark mode classes', () => {
      expect(getHistoryDotClass('deployed')).toContain('dark:bg-green-400')
      expect(getHistoryDotClass('failed')).toContain('dark:bg-red-400')
      expect(getHistoryDotClass('superseded')).toContain('dark:bg-yellow-400')
    })
  })

  describe('getEventBadgeClass', () => {
    it('should return green for Normal events', () => {
      expect(getEventBadgeClass('Normal')).toContain('bg-green-100')
      expect(getEventBadgeClass('Normal')).toContain('text-green-800')
    })

    it('should return red for Warning events', () => {
      expect(getEventBadgeClass('Warning')).toContain('bg-red-100')
      expect(getEventBadgeClass('Warning')).toContain('text-red-800')
    })

    it('should return red for any non-Normal type', () => {
      expect(getEventBadgeClass('Error')).toContain('bg-red-100')
      expect(getEventBadgeClass('Unknown')).toContain('bg-red-100')
    })

    it('should include dark mode classes', () => {
      expect(getEventBadgeClass('Normal')).toContain('dark:bg-green-900/30')
      expect(getEventBadgeClass('Warning')).toContain('dark:bg-red-900/30')
    })
  })

  describe('getContainerStateBadgeClass', () => {
    it('should return blue for Running state', () => {
      expect(getContainerStateBadgeClass('Running')).toContain('bg-blue-100')
      expect(getContainerStateBadgeClass('Running')).toContain('text-blue-800')
    })

    it('should return yellow for Waiting state', () => {
      expect(getContainerStateBadgeClass('Waiting')).toContain('bg-yellow-100')
      expect(getContainerStateBadgeClass('Waiting')).toContain('text-yellow-800')
    })

    it('should return red for Terminated state', () => {
      expect(getContainerStateBadgeClass('Terminated')).toContain('bg-red-100')
      expect(getContainerStateBadgeClass('Terminated')).toContain('text-red-800')
    })

    it('should return cyan for Completed state', () => {
      expect(getContainerStateBadgeClass('Completed')).toContain('bg-cyan-100')
      expect(getContainerStateBadgeClass('Completed')).toContain('text-cyan-800')
    })

    it('should return gray for unknown state', () => {
      expect(getContainerStateBadgeClass('Unknown')).toContain('bg-gray-100')
      expect(getContainerStateBadgeClass('Unknown')).toContain('text-gray-800')
    })

    it('should include dark mode classes', () => {
      expect(getContainerStateBadgeClass('Running')).toContain('dark:bg-blue-900/30')
      expect(getContainerStateBadgeClass('Waiting')).toContain('dark:bg-yellow-900/30')
      expect(getContainerStateBadgeClass('Terminated')).toContain('dark:bg-red-900/30')
      expect(getContainerStateBadgeClass('Completed')).toContain('dark:bg-cyan-900/30')
    })
  })

  describe('getStatusBarColor', () => {
    it('should return green for Ready status', () => {
      expect(getStatusBarColor('Ready')).toBe('bg-green-500 dark:bg-green-600')
    })

    it('should return red for Failed status', () => {
      expect(getStatusBarColor('Failed')).toBe('bg-red-500 dark:bg-red-600')
    })

    it('should return blue for Progressing status', () => {
      expect(getStatusBarColor('Progressing')).toBe('bg-blue-500 dark:bg-blue-600')
    })

    it('should return yellow for Suspended status', () => {
      expect(getStatusBarColor('Suspended')).toBe('bg-yellow-500 dark:bg-yellow-600')
    })

    it('should return gray for Unknown status', () => {
      expect(getStatusBarColor('Unknown')).toBe('bg-gray-600 dark:bg-gray-500')
    })

    it('should return default gray for unrecognized status', () => {
      expect(getStatusBarColor('SomethingElse')).toBe('bg-gray-200 dark:bg-gray-700')
      expect(getStatusBarColor(null)).toBe('bg-gray-200 dark:bg-gray-700')
      expect(getStatusBarColor(undefined)).toBe('bg-gray-200 dark:bg-gray-700')
    })
  })

  describe('getEventBarColor', () => {
    it('should return green for Normal events', () => {
      expect(getEventBarColor('Normal')).toBe('bg-green-500 dark:bg-green-600')
    })

    it('should return red for Warning events', () => {
      expect(getEventBarColor('Warning')).toBe('bg-red-500 dark:bg-red-600')
    })

    it('should return default gray for unrecognized event type', () => {
      expect(getEventBarColor('SomethingElse')).toBe('bg-gray-200 dark:bg-gray-700')
      expect(getEventBarColor(null)).toBe('bg-gray-200 dark:bg-gray-700')
      expect(getEventBarColor(undefined)).toBe('bg-gray-200 dark:bg-gray-700')
    })
  })

  describe('getStatusBorderClass', () => {
    it('should return success border for Ready status', () => {
      expect(getStatusBorderClass('Ready')).toBe('border-success')
    })

    it('should return danger border for Failed status', () => {
      expect(getStatusBorderClass('Failed')).toBe('border-danger')
    })

    it('should return warning border for Suspended status', () => {
      expect(getStatusBorderClass('Suspended')).toBe('border-warning')
    })

    it('should return info border for Progressing status', () => {
      expect(getStatusBorderClass('Progressing')).toBe('border-info')
    })

    it('should return gray border for Unknown status', () => {
      expect(getStatusBorderClass('Unknown')).toBe('border-gray-300 dark:border-gray-600')
    })

    it('should return gray border for unrecognized status', () => {
      expect(getStatusBorderClass('SomethingElse')).toBe('border-gray-300 dark:border-gray-600')
      expect(getStatusBorderClass(null)).toBe('border-gray-300 dark:border-gray-600')
      expect(getStatusBorderClass(undefined)).toBe('border-gray-300 dark:border-gray-600')
    })
  })

  describe('cleanStatus', () => {
    it('should return undefined for falsy input', () => {
      expect(cleanStatus(null)).toBeUndefined()
      expect(cleanStatus(undefined)).toBeUndefined()
      expect(cleanStatus('')).toBeUndefined()
    })

    it('should remove internal fields from status', () => {
      const status = {
        inventory: [{ name: 'item1' }],
        sourceRef: { kind: 'GitRepository', name: 'test' },
        reconcilerRef: { status: 'Ready' },
        exportedInputs: { key: 'value' },
        userActions: true,
        inputProviderRefs: [{ name: 'provider1' }],
        conditions: [{ type: 'Ready', status: 'True' }],
        observedGeneration: 1
      }
      const result = cleanStatus(status)
      expect(result).toEqual({
        conditions: [{ type: 'Ready', status: 'True' }],
        observedGeneration: 1
      })
    })

    it('should preserve other status fields', () => {
      const status = {
        customField: 'value',
        anotherField: 123,
        inventory: [],
        userActions: false
      }
      const result = cleanStatus(status)
      expect(result).toEqual({
        customField: 'value',
        anotherField: 123
      })
    })

    it('should handle status with no internal fields', () => {
      const status = {
        conditions: [],
        observedGeneration: 5
      }
      const result = cleanStatus(status)
      expect(result).toEqual(status)
    })
  })
})
