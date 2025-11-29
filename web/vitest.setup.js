// web/vitest.setup.js

// Import jest-dom matchers for DOM assertions
import '@testing-library/jest-dom/vitest'

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

// Expose the mocks for individual test files to reset/inspect
global.localStorageMock = localStorageMock
global.matchMediaMock = matchMediaMock

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
