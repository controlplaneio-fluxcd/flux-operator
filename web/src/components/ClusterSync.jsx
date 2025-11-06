import { signal } from '@preact/signals'

// Store collapsed state for the sync section
const isExpanded = signal(true)

export function ClusterSync({ sync }) {
  const isSuspended = sync.status && sync.status.startsWith('Suspended')

  const getStatusInfo = () => {
    if (isSuspended) {
      return {
        icon: (
          <svg class="w-5 h-5 text-blue-600 dark:text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 9v6m4-6v6m7-3a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
        ),
        badge: 'status-badge bg-blue-100 dark:bg-blue-900/30 text-blue-800 dark:text-blue-300',
        label: 'Suspended'
      }
    } else if (sync.ready) {
      return {
        icon: (
          <svg class="w-5 h-5 text-success" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
        ),
        badge: 'status-badge status-ready',
        label: 'Synced'
      }
    } else {
      return {
        icon: (
          <svg class="w-5 h-5 text-danger" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
        ),
        badge: 'status-badge status-not-ready',
        label: 'Not Synced'
      }
    }
  }

  const statusInfo = getStatusInfo()

  return (
    <div class="card">
      <button
        onClick={() => isExpanded.value = !isExpanded.value}
        class={`w-full text-left hover:opacity-80 transition-opacity ${isExpanded.value ? 'mb-4' : ''}`}
      >
        <div class="flex items-center justify-between">
          <div>
            <h3 class="text-lg font-semibold text-gray-900 dark:text-white">Cluster Sync</h3>
            <div class="flex items-center space-x-4 mt-1">
              <p class="text-sm text-gray-600 dark:text-gray-400">{sync.id}</p>
              {!sync.ready && !isSuspended && (
                <span class="status-badge status-not-ready">
                  failing
                </span>
              )}
            </div>
          </div>
          <svg
            class={`w-5 h-5 text-gray-400 dark:text-gray-500 transition-transform ${isExpanded.value ? 'rotate-180' : ''}`}
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
          </svg>
        </div>
      </button>

      {isExpanded.value && (
        <div class="space-y-2">
          <div class="flex flex-col sm:flex-row sm:items-center gap-2 text-sm text-gray-900 dark:text-white break-all">
            <div class="flex items-center gap-2">
              <svg class="w-5 h-5 flex-shrink-0 text-blue-600 dark:text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12.75L11.25 15 15 9.75M21 12c0 1.268-.63 2.39-1.593 3.068a3.745 3.745 0 01-1.043 3.296 3.745 3.745 0 01-3.296 1.043A3.745 3.745 0 0112 21c-1.268 0-2.39-.63-3.068-1.593a3.746 3.746 0 01-3.296-1.043 3.745 3.745 0 01-1.043-3.296A3.745 3.745 0 013 12c0-1.268.63-2.39 1.593-3.068a3.745 3.745 0 011.043-3.296 3.746 3.746 0 013.296-1.043A3.746 3.746 0 0112 3c1.268 0 2.39.63 3.068 1.593a3.746 3.746 0 013.296 1.043 3.746 3.746 0 011.043 3.296A3.745 3.745 0 0121 12z" />
              </svg>
              {sync.source}
            </div>
            <div class="flex items-center gap-2">
              <svg class="w-4 h-4 flex-shrink-0 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
              </svg>
              {sync.path}
            </div>
          </div>
          <div class="flex items-start gap-2 text-sm text-gray-900 dark:text-white">
            <span class="flex-shrink-0 mt-0.5">{statusInfo.icon}</span>
            <span class="whitespace-pre-wrap break-all max-w-full overflow-hidden">{sync.status}</span>
          </div>
        </div>
      )}
    </div>
  )
}
