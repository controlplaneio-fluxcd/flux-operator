// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect } from 'vitest'
import {
  getStatusBadgeClass,
  getWorkloadStatusBadgeClass,
  formatWorkloadStatus,
  getHistoryStatusBadgeClass,
  getHistoryDotClass,
  getEventBadgeClass
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
})
