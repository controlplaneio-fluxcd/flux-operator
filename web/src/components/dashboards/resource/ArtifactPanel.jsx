// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { DashboardPanel, TabButton } from '../common/panel'
import { useHashTab } from '../../../utils/hash'

// Valid tabs for the ArtifactPanel
const ARTIFACT_TABS = ['overview', 'metadata']

/**
 * Get the source reference display string
 * Returns "type: value" format for spec.ref fields, "bucket: value" for bucketName,
 * or "kind/namespace/name" for spec.sourceRef
 */
function getSourceRef(spec, namespace) {
  if (!spec) return null

  // Check for bucketName (Bucket source type)
  if (spec.bucketName) {
    return `bucket: ${spec.bucketName}`
  }

  // Check spec.ref fields in priority order
  if (spec.ref) {
    const refTypes = ['branch', 'tag', 'semver', 'name', 'commit']
    for (const refType of refTypes) {
      if (spec.ref[refType]) {
        return `${refType}: ${spec.ref[refType]}`
      }
    }
  }

  // Check for sourceRef (used by ExternalArtifact to reference parent source)
  if (spec.sourceRef) {
    const refKind = spec.sourceRef.kind
    const refName = spec.sourceRef.name
    const refNamespace = spec.sourceRef.namespace || namespace
    return `${refKind}/${refNamespace}/${refName}`
  }

  return null
}

/**
 * Get the signature verification provider
 */
function getSignature(spec) {
  if (!spec?.verify) {
    return 'None'
  }

  if (spec.verify.provider) {
    return spec.verify.provider
  }

  return 'pgp'
}

/**
 * Format size in bytes to KiB
 */
function formatSize(bytes) {
  if (bytes === undefined || bytes === null) {
    return 'Unavailable'
  }
  const kib = bytes / 1024
  return `${kib.toFixed(2)} KiB`
}

/**
 * ArtifactPanel - Displays artifact information for Flux source resources
 */
export function ArtifactPanel({ resourceData }) {
  // Tab state synced with URL hash (e.g., #artifact-metadata)
  const [activeTab, setActiveTab] = useHashTab('artifact', 'overview', ARTIFACT_TABS, 'artifact-panel')

  // Extract data
  const kind = resourceData?.kind
  const namespace = resourceData?.metadata?.namespace
  const spec = resourceData?.spec
  const artifact = resourceData?.status?.artifact
  const metadata = artifact?.metadata

  // Check if metadata tab should be shown
  const hasMetadata = metadata && Object.keys(metadata).length > 0

  // Source info
  const sourceUrl = spec?.url || spec?.endpoint
  const sourceRef = getSourceRef(spec, namespace)
  const signature = getSignature(spec)

  // Artifact info
  const fetchedAt = artifact?.lastUpdateTime
    ? new Date(artifact.lastUpdateTime).toLocaleString().replace(',', '')
    : 'Unavailable'
  const size = formatSize(artifact?.size)
  const revision = artifact?.revision || 'Unavailable'

  return (
    <DashboardPanel title="Artifact" id="artifact-panel">
      {/* Tab Navigation */}
      <div class="border-b border-gray-200 dark:border-gray-700 mb-4">
        <nav class="flex space-x-4">
          <TabButton active={activeTab === 'overview'} onClick={() => setActiveTab('overview')}>
            <span class="sm:hidden">Info</span>
            <span class="hidden sm:inline">Overview</span>
          </TabButton>
          {hasMetadata && (
            <TabButton active={activeTab === 'metadata'} onClick={() => setActiveTab('metadata')}>
              Metadata
            </TabButton>
          )}
        </nav>
      </div>

      {/* Tab Content */}
      {activeTab === 'overview' && (
        <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
          {/* Left column: Source information */}
          <div class="space-y-4">
            {/* Source Type */}
            <div class="text-sm">
              <span class="text-gray-500 dark:text-gray-400">Source Type</span>
              <span class="ml-1 text-gray-900 dark:text-white">{kind}</span>
            </div>

            {/* Source URL */}
            {sourceUrl && (
              <div class="text-sm">
                <span class="text-gray-500 dark:text-gray-400">Source URL</span>
                <span class="ml-1 text-gray-900 dark:text-white break-all">{sourceUrl}</span>
              </div>
            )}

            {/* Source Ref */}
            {sourceRef && (
              <div class="text-sm">
                <span class="text-gray-500 dark:text-gray-400">Source Ref</span>
                <span class="ml-1 text-gray-900 dark:text-white">{sourceRef}</span>
              </div>
            )}

            {/* Signature */}
            <div class="text-sm">
              <span class="text-gray-500 dark:text-gray-400">Signature</span>
              <span class={`ml-1 inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                signature === 'None'
                  ? 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-400'
                  : 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400'
              }`}>
                {signature}
              </span>
            </div>
          </div>

          {/* Right column: Artifact information */}
          <div class="space-y-4 border-gray-200 dark:border-gray-700 border-t pt-4 md:border-t-0 md:border-l md:pt-0 md:pl-6">
            {/* Fetched at */}
            <div class="text-sm">
              <span class="text-gray-500 dark:text-gray-400">Fetched at</span>
              <span class="ml-1 text-gray-900 dark:text-white">{fetchedAt}</span>
            </div>

            {/* Size */}
            <div class="text-sm">
              <span class="text-gray-500 dark:text-gray-400">Size</span>
              <span class="ml-1 text-gray-900 dark:text-white">{size}</span>
            </div>

            {/* Revision */}
            <div class="text-sm">
              <span class="text-gray-500 dark:text-gray-400">Revision</span>
              <span class="ml-1 text-gray-900 dark:text-white break-all">{revision}</span>
            </div>
          </div>
        </div>
      )}

      {/* Metadata Tab */}
      {activeTab === 'metadata' && hasMetadata && (
        <div class="space-y-4">
          {Object.entries(metadata).map(([key, value]) => (
            <div key={key} class="card p-4">
              <div class="text-sm font-medium text-gray-500 dark:text-gray-400 mb-1">
                {key}
              </div>
              <div class="text-sm text-gray-900 dark:text-white break-all">
                <pre class="whitespace-pre-wrap break-all font-sans">{value}</pre>
              </div>
            </div>
          ))}
        </div>
      )}
    </DashboardPanel>
  )
}
