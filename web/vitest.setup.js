// web/vitest.setup.js

// Import jest-dom matchers for DOM assertions
import '@testing-library/jest-dom/vitest'
import { act, cleanup } from '@testing-library/preact'

// Unmount rendered trees inside act() after each test. Preact runs useEffect
// cleanups of unmounted components after paint, so without act() they would
// fire later, during the next test. Registered after the testing-library
// auto-cleanup hook, so it runs first (afterEach hooks run in reverse order).
afterEach(() => act(() => cleanup()))

// Mock localStorage globally before any modules are imported
const localStorageMock = {
  getItem: vi.fn(),
  setItem: vi.fn(),
  clear: vi.fn(),
}

Object.defineProperty(global, 'localStorage', {
  value: localStorageMock,
  writable: true,
})

// Mock matchMedia globally
const matchMediaMock = vi.fn((query) => ({
  matches: false, // Default to light theme for system preference
  addEventListener: vi.fn(),
  removeEventListener: vi.fn(),
}))

Object.defineProperty(global, 'matchMedia', {
  value: matchMediaMock,
  writable: true,
})

// Mock crypto.randomUUID globally
const cryptoMock = {
  randomUUID: vi.fn(() => 'test-uuid-1234'),
}

Object.defineProperty(global, 'crypto', {
  value: cryptoMock,
  writable: true,
})

// Expose the mocks for individual test files to reset/inspect
global.localStorageMock = localStorageMock
global.matchMediaMock = matchMediaMock
global.cryptoMock = cryptoMock

// Suppress expected console messages during tests
// These are logged by the application code when testing error scenarios
const suppressedErrorPatterns = [
  /Failed to fetch/,
  /Failed to parse URL/,
  /Network error/,
  /Network connection failed/,
]

const suppressedWarnPatterns = [
  /getMockWorkload:/,
]

const originalConsoleError = console.error
console.error = (...args) => {
  const message = args.join(' ')
  const shouldSuppress = suppressedErrorPatterns.some(pattern => pattern.test(message))
  if (!shouldSuppress) {
    originalConsoleError(...args)
  }
}

const originalConsoleWarn = console.warn
console.warn = (...args) => {
  const message = args.join(' ')
  const shouldSuppress = suppressedWarnPatterns.some(pattern => pattern.test(message))
  if (!shouldSuppress) {
    originalConsoleWarn(...args)
  }
}
