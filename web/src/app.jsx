import { useEffect } from 'preact/hooks'
import { signal } from '@preact/signals'
import { fetchWithMock } from './utils/fetch'
import './utils/theme'
import { ConnectionStatus } from './components/ConnectionStatus'
import { Header } from './components/Header'
import { ClusterStatus } from './components/ClusterStatus'
import { ClusterInfo } from './components/ClusterInfo'
import { ClusterSync } from './components/ClusterSync'
import { ComponentList } from './components/ComponentList'
import { ReconcilerList } from './components/ReconcilerList'
import { SearchView } from './components/SearchView'
import { Footer } from './components/Footer'

// Global signals for data and state
export const fluxReport = signal(null)
export const lastUpdated = signal(null)
export const isLoading = signal(true)

// Connection status can be 'loading', 'connected', or 'disconnected'
export const connectionStatus = signal('loading')

// View state: true = search view, false = dashboard view
export const showSearchView = signal(false)

// Fetch report data from remote API or mock data based on environment variable
export async function fetchFluxReport() {
  // Only set loading state if not already loading
  if (connectionStatus.value !== 'loading') {
    connectionStatus.value = 'loading'
  }

  try {
    fluxReport.value = await fetchWithMock({
      endpoint: '/api/v1/report',
      mockPath: '../mock/report',
      mockExport: 'mockReport'
    })
    connectionStatus.value = 'connected'
    lastUpdated.value = new Date()
    isLoading.value = false
  } catch (error) {
    console.error('Failed to fetch report:', error)
    connectionStatus.value = 'disconnected'
    lastUpdated.value = new Date()
    isLoading.value = false
  }
}

export function App() {
  // Initial data fetch
  useEffect(() => {
    fetchFluxReport()

    // Auto-refresh every 30 seconds
    const interval = setInterval(fetchFluxReport, 30000)

    return () => clearInterval(interval)
  }, [])

  // Loading state: waiting for data and not disconnected
  if (isLoading.value && !fluxReport.value) {
    return (
      <div class="min-h-screen bg-gray-50 dark:bg-gray-900 transition-colors flex flex-col">
        <ConnectionStatus />
        <div class="flex items-center justify-center flex-1">
          <div class="text-center">
            <div class="animate-spin rounded-full h-12 w-12 border-b-2 border-flux-blue mx-auto"></div>
            <p class="mt-4 text-gray-600 dark:text-gray-400">Loading Flux status...</p>
          </div>
        </div>
      </div>
    )
  }

  // Error state: disconnected and no data
  if (!fluxReport.value && connectionStatus.value === 'disconnected') {
    return (
      <div class="min-h-screen bg-gray-50 dark:bg-gray-900 transition-colors flex flex-col">
        <ConnectionStatus />
        <div class="flex items-center justify-center flex-1">
          <div class="text-center">
            <p class="text-red-600 dark:text-red-400 text-lg font-semibold">Failed to load Flux report</p>
            <p class="mt-2 text-gray-600 dark:text-gray-400 text-sm">
              Unable to connect to the server. Retrying automatically...
            </p>
          </div>
        </div>
      </div>
    )
  }

  const { spec } = fluxReport.value

  return (
    <div class="min-h-screen bg-gray-50 dark:bg-gray-900 transition-colors flex flex-col">
      <ConnectionStatus />
      <Header />

      {/* Conditional view: Search or Dashboard */}
      {showSearchView.value ? (
        <SearchView />
      ) : (
        <main class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8 flex-grow w-full">
          <div class="space-y-6">
            <ClusterStatus report={spec} />

            {spec.operator && (
              <ClusterInfo
                cluster={spec.cluster}
                distribution={spec.distribution}
                operator={spec.operator}
                components={spec.components}
                metrics={spec.metrics}
              />
            )}

            {spec.sync && (
              <ClusterSync sync={spec.sync} />
            )}

            {spec.components && spec.components.length > 0 && (
              <ComponentList components={spec.components} metrics={spec.metrics} />
            )}

            {spec.reconcilers && spec.reconcilers.length > 0 && (
              <ReconcilerList reconcilers={spec.reconcilers} />
            )}
          </div>
        </main>
      )}

      <Footer />
    </div>
  )
}
