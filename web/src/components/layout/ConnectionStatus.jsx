// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useSignal } from '@preact/signals'
import { useEffect } from 'preact/hooks'
import { connectionStatus } from '../../app'

/**
 * ConnectionStatus component - Displays server connection status banner
 *
 * Shows a colored banner at the top of the page indicating:
 * - Loading: Gray banner with pulse animation (only if loading > 300ms)
 * - Disconnected: Red banner with error indication
 * - Connected: No banner (hidden)
 */
export function ConnectionStatus() {
  const status = connectionStatus.value
  const isDisconnected = status === 'disconnected'
  const isLoading = status === 'loading'
  const showLoading = useSignal(false)

  // Delay showing loading bar by 300ms to avoid flicker on fast fetches
  useEffect(() => {
    let timer
    if (isLoading) {
      timer = setTimeout(() => {
        showLoading.value = true
      }, 300)
    } else {
      showLoading.value = false
    }
    return () => window.clearTimeout(timer)
  }, [isLoading])

  // Only show the component if disconnected or loading (after delay)
  if (!isDisconnected && !showLoading.value) {
    return null
  }

  // Determine colors based on status
  let barColor = 'bg-gray-400'
  let barHeight = 'h-1'

  if (isDisconnected) {
    barColor = 'bg-red-500'
    barHeight = 'h-1.5'
  }

  return (
    <div class="fixed top-0 left-0 right-0 z-50">
      <div class={`${barColor} ${barHeight} w-full transition-colors duration-300`}>
        {showLoading.value && (
          <div class="absolute inset-0 bg-gradient-to-r from-transparent via-white to-transparent opacity-30 animate-pulse"></div>
        )}
      </div>
      {isDisconnected && (
        <div class="hidden sm:flex justify-center">
          <span class="bg-red-500 text-white text-xs font-medium px-3 py-1 rounded-b shadow-md">
            disconnected
          </span>
        </div>
      )}
    </div>
  )
}
