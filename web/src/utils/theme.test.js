import { themeMode, appliedTheme, themes } from './theme'

describe('theme utilities', () => {
  let initialThemeModule; // Store the module imported in beforeEach

  beforeEach(async () => {
    // Reset module cache to allow re-importing theme.js for initial state tests
    vi.resetModules()

    // Reset mocks for each test
    global.localStorageMock.getItem.mockClear()
    global.localStorageMock.setItem.mockClear()
    global.localStorageMock.clear()

    global.matchMediaMock.mockClear()
    // Set default mock return value for matchMedia to simulate light theme
    global.matchMediaMock.mockReturnValue({
      matches: false, // Default to light theme
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    })

    // Dynamically import the module after mocks are set up for all tests in this suite
    initialThemeModule = await vi.importActual('./theme')

    // Explicitly set signals to a known state for each test to ensure consistency
    // We use the signals from the initialThemeModule here
    themeMode.value = initialThemeModule.themes.auto
    appliedTheme.value = initialThemeModule.themes.light // Default for system light

    // Ensure document class list is clean
    document.documentElement.classList.remove('dark')
  })

  it('should initialize themeMode from localStorage if available', async () => {
    global.localStorageMock.getItem.mockReturnValueOnce(themes.dark)
    vi.resetModules()
    const { themeMode: updatedThemeMode } = await vi.importActual('./theme')
    expect(global.localStorageMock.getItem).toHaveBeenCalledWith('theme')
    expect(updatedThemeMode.value).toBe(themes.dark)
  })

  it('should default themeMode to auto if no localStorage item', async () => {
    global.localStorageMock.getItem.mockReturnValueOnce(null)
    vi.resetModules()
    const { themeMode: updatedThemeMode } = await vi.importActual('./theme')
    expect(global.localStorageMock.getItem).toHaveBeenCalledWith('theme')
    expect(updatedThemeMode.value).toBe(themes.auto)
  })

  it('should update appliedTheme and localStorage when themeMode changes', () => {
    // Use the signals from the shared initialThemeModule
    initialThemeModule.themeMode.value = initialThemeModule.themes.dark
    expect(initialThemeModule.appliedTheme.value).toBe(initialThemeModule.themes.dark)
    expect(global.localStorageMock.setItem).toHaveBeenCalledWith('theme', initialThemeModule.themes.dark)
    expect(document.documentElement.classList.contains('dark')).toBe(true)

    initialThemeModule.themeMode.value = initialThemeModule.themes.light
    expect(initialThemeModule.appliedTheme.value).toBe(initialThemeModule.themes.light)
    expect(global.localStorageMock.setItem).toHaveBeenCalledWith('theme', initialThemeModule.themes.light)
    expect(document.documentElement.classList.contains('dark')).toBe(false)
  })

  it('should cycle through themes correctly', () => {
    // Initial state: auto (system light) is already set in beforeEach
    expect(initialThemeModule.themeMode.value).toBe(initialThemeModule.themes.auto)
    expect(initialThemeModule.appliedTheme.value).toBe(initialThemeModule.themes.light)

    initialThemeModule.cycleTheme()
    // After first cycle: themeMode should be dark, appliedTheme should be dark
    expect(initialThemeModule.themeMode.value).toBe(initialThemeModule.themes.dark)
    expect(initialThemeModule.appliedTheme.value).toBe(initialThemeModule.themes.dark)
    expect(global.localStorageMock.setItem).toHaveBeenCalledWith('theme', initialThemeModule.themes.dark)

    initialThemeModule.cycleTheme()
    // After second cycle: themeMode should be light, appliedTheme should be light
    expect(initialThemeModule.themeMode.value).toBe(initialThemeModule.themes.light)
    expect(initialThemeModule.appliedTheme.value).toBe(initialThemeModule.themes.light)
    expect(global.localStorageMock.setItem).toHaveBeenCalledWith('theme', initialThemeModule.themes.light)

    initialThemeModule.cycleTheme()
    // After third cycle: themeMode should be auto, appliedTheme should be light
    expect(initialThemeModule.themeMode.value).toBe(initialThemeModule.themes.auto)
    expect(initialThemeModule.appliedTheme.value).toBe(initialThemeModule.themes.light) // System is mocked as light
    expect(global.localStorageMock.setItem).toHaveBeenCalledWith('theme', initialThemeModule.themes.auto)
  })

  it('should react to system theme changes when in auto mode', () => {
    // Ensure we are in auto mode using the signals from initialThemeModule
    initialThemeModule.themeMode.value = initialThemeModule.themes.auto
    expect(initialThemeModule.appliedTheme.value).toBe(initialThemeModule.themes.light) // Mocked system is light

    // Get the mock object returned by matchMedia when theme.js was initially imported
    // matchMedia is called once during module load of theme.js
    const mediaQueryList = global.matchMediaMock.mock.results[0].value; // Get the returned object
    const handler = mediaQueryList.addEventListener.mock.calls[0][1]; // Get the handler function

    // Simulate system preference changing to dark
    mediaQueryList.matches = true; // Update the matches property of the returned object
    handler({ matches: true })
    expect(initialThemeModule.appliedTheme.value).toBe(initialThemeModule.themes.dark)
    expect(document.documentElement.classList.contains('dark')).toBe(true)

    // Simulate system changing back to light
    mediaQueryList.matches = false; // Update the matches property
    handler({ matches: false })
    expect(initialThemeModule.appliedTheme.value).toBe(initialThemeModule.themes.light)
    expect(document.documentElement.classList.contains('dark')).toBe(false)
  })

  it('should not react to system theme changes when not in auto mode', () => {
    // Ensure we are in dark mode using the signals from initialThemeModule
    initialThemeModule.themeMode.value = initialThemeModule.themes.dark
    expect(initialThemeModule.appliedTheme.value).toBe(initialThemeModule.themes.dark)

    // Get the mock object returned by matchMedia
    const mediaQueryList = global.matchMediaMock.mock.results[0].value;
    const handler = mediaQueryList.addEventListener.mock.calls[0][1];

    // Simulate system preference changing to light, but themeMode is dark
    mediaQueryList.matches = false; // Update the matches property
    handler({ matches: false })
    expect(initialThemeModule.appliedTheme.value).toBe(initialThemeModule.themes.dark) // Should remain dark
    expect(document.documentElement.classList.contains('dark')).toBe(true)
  })
})
