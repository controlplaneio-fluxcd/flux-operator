// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach, vi } from 'vitest'
import { fetchWithMock, shouldUseMockData } from './fetch'

describe('shouldUseMockData', () => {
  it('should return true for non-production mode with VITE_USE_MOCK_DATA=true', () => {
    const env = { MODE: 'development', VITE_USE_MOCK_DATA: 'true' }
    expect(shouldUseMockData(env)).toBe(true)
  })

  it('should return true for test mode with VITE_USE_MOCK_DATA=true', () => {
    const env = { MODE: 'test', VITE_USE_MOCK_DATA: 'true' }
    expect(shouldUseMockData(env)).toBe(true)
  })

  it('should return false for production mode', () => {
    const env = { MODE: 'production', VITE_USE_MOCK_DATA: 'true' }
    expect(shouldUseMockData(env)).toBe(false)
  })

  it('should return false when VITE_USE_MOCK_DATA is not true', () => {
    const env = { MODE: 'development', VITE_USE_MOCK_DATA: 'false' }
    expect(shouldUseMockData(env)).toBe(false)
  })
})

describe('fetchWithMock', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    vi.resetModules()
  })

  describe('Mock mode (non-production with VITE_USE_MOCK_DATA=true)', () => {
    beforeEach(() => {
      // Mock dynamic imports
      vi.doMock('../mock/report.js', () => ({
        mockReport: { status: 'healthy', version: '1.0.0' }
      }))

      vi.doMock('../mock/events.js', () => ({
        mockEvents: vi.fn((endpoint) => ({ endpoint, filtered: true }))
      }))

    })

    it('should return static mock data from dynamic import', async () => {
      const result = await fetchWithMock({
        endpoint: '/api/v1/report',
        mockPath: '../mock/report.js',
        mockExport: 'mockReport',
        env: { MODE: 'test', VITE_USE_MOCK_DATA: 'true' }
      })

      expect(result).toEqual({ status: 'healthy', version: '1.0.0' })
    })

    it('should call mock function with endpoint URL for filtering', async () => {
      const endpoint = '/api/v1/events?kind=GitRepository'

      const result = await fetchWithMock({
        endpoint,
        mockPath: '../mock/events.js',
        mockExport: 'mockEvents',
        env: { MODE: 'test', VITE_USE_MOCK_DATA: 'true' }
      })

      // Get the mocked function to verify it was called
      const { mockEvents } = await import('../mock/events.js')
      expect(mockEvents).toHaveBeenCalledWith(endpoint)
      expect(result).toEqual({ endpoint, filtered: true })
    })

    it('should simulate 300ms network delay', async () => {
      const startTime = Date.now()

      await fetchWithMock({
        endpoint: '/api/v1/report',
        mockPath: '../mock/report.js',
        mockExport: 'mockReport',
        env: { MODE: 'test', VITE_USE_MOCK_DATA: 'true' }
      })

      const elapsed = Date.now() - startTime

      // Verify delay is at least 300ms (with small tolerance for execution time)
      expect(elapsed).toBeGreaterThanOrEqual(290)
    })
  })

  describe('Production mode (VITE_USE_MOCK_DATA=false)', () => {
    beforeEach(() => {
      // Set up fetch mock
      global.fetch = vi.fn()
    })

    it('should call real fetch API', async () => {
      const mockResponse = {
        ok: true,
        json: vi.fn().mockResolvedValue({ data: 'real api data' })
      }
      global.fetch.mockResolvedValue(mockResponse)

      const endpoint = '/api/v1/report'
      await fetchWithMock({
        endpoint,
        mockPath: '../mock/report.js',
        mockExport: 'mockReport',
        env: { MODE: 'production', VITE_USE_MOCK_DATA: 'false' }
      })

      expect(global.fetch).toHaveBeenCalledWith(endpoint, { method: 'GET' })
    })

    it('should throw error without http status on 403 response', async () => {
      const mockResponse = {
        ok: false,
        status: 403,
        text: () => Promise.resolve('Forbidden access')
      }
      global.fetch.mockResolvedValue(mockResponse)

      await expect(
        fetchWithMock({
          endpoint: '/api/v1/forbidden',
          mockPath: '../mock/report.js',
          mockExport: 'mockReport',
          env: { MODE: 'production', VITE_USE_MOCK_DATA: 'false' }
        })
      ).rejects.toThrow('Forbidden access')
    })

    it('should throw error on non-200 response', async () => {
      const mockResponse = {
        ok: false,
        status: 404
      }
      global.fetch.mockResolvedValue(mockResponse)

      await expect(
        fetchWithMock({
          endpoint: '/api/v1/not-found',
          mockPath: '../mock/report.js',
          mockExport: 'mockReport',
          env: { MODE: 'production', VITE_USE_MOCK_DATA: 'false' }
        })
      ).rejects.toThrow('HTTP error! status: 404')
    })

    it('should return parsed JSON on success', async () => {
      const responseData = { status: 'success', data: [1, 2, 3] }
      const mockResponse = {
        ok: true,
        json: vi.fn().mockResolvedValue(responseData)
      }
      global.fetch.mockResolvedValue(mockResponse)

      const result = await fetchWithMock({
        endpoint: '/api/v1/data',
        mockPath: '../mock/report.js',
        mockExport: 'mockReport',
        env: { MODE: 'production', VITE_USE_MOCK_DATA: 'false' }
      })

      expect(result).toEqual(responseData)
      expect(mockResponse.json).toHaveBeenCalled()
    })
  })
})
