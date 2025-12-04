// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { signal, effect } from '@preact/signals'

// Theme modes
export const themes = {
  light: 'light',
  dark: 'dark',
  auto: 'auto'
}

// Get initial theme from localStorage or default to auto
const getInitialTheme = () => {
  const stored = localStorage.getItem('theme')
  return stored || themes.auto
}

// Check system preference
const getSystemTheme = () => {
  if (typeof window === 'undefined') return themes.light
  const systemPrefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches
  return systemPrefersDark ? themes.dark : themes.light
}

// Current theme selection (light/dark/auto)
export const themeMode = signal(getInitialTheme())

// Actual applied theme (light/dark only)
export const appliedTheme = signal(
  themeMode.value === themes.auto ? getSystemTheme() : themeMode.value
)

// Update applied theme when mode changes or system preference changes
effect(() => {
  const mode = themeMode.value

  if (mode === themes.auto) {
    appliedTheme.value = getSystemTheme()
  } else {
    appliedTheme.value = mode
  }

  // Save to localStorage
  localStorage.setItem('theme', mode)

  // Apply to document
  if (appliedTheme.value === themes.dark) {
    document.documentElement.classList.add('dark')
  } else {
    document.documentElement.classList.remove('dark')
  }
})

// Listen for system theme changes when in auto mode
if (typeof window !== 'undefined') {
  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', (e) => {
    if (themeMode.value === themes.auto) {
      appliedTheme.value = e.matches ? themes.dark : themes.light
    }
  })
}

// Toggle between themes
export const cycleTheme = () => {
  const themeOrder = [themes.auto, themes.dark, themes.light]
  const currentIndex = themeOrder.indexOf(themeMode.value)
  const nextIndex = (currentIndex + 1) % themeOrder.length
  themeMode.value = themeOrder[nextIndex]
}
