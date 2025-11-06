import { connectionStatus } from '../app'

export function ConnectionStatus() {
  const status = connectionStatus.value
  const isDisconnected = status === 'disconnected'
  const isLoading = status === 'loading'

  // Only show the component if loading or disconnected
  if (!isDisconnected && !isLoading) {
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
        {isLoading && (
          <div class="absolute inset-0 bg-gradient-to-r from-transparent via-white to-transparent opacity-30 animate-pulse"></div>
        )}
      </div>
    </div>
  )
}
