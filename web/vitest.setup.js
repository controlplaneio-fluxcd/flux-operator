// web/vitest.setup.js

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
