// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useMemo } from 'preact/hooks'
import { fluxKinds, workloadKinds } from '../../../utils/constants'

/**
 * Build graph data from resource data
 * @param {object} resourceData - The resource data
 * @returns {object} Graph data with sources, reconciler, and inventory groups
 */
export function buildGraphData(resourceData) {
  const sources = []
  let upstream = null
  let helmChart = null

  if (resourceData?.status?.sourceRef) {
    // Check for upstream origin URL
    if (resourceData.status.sourceRef.originURL) {
      const originURL = resourceData.status.sourceRef.originURL
      // Extract last part of the URL (repo name or path segment)
      const urlParts = originURL.replace(/\.git$/, '').split('/').filter(Boolean)
      const upstreamName = urlParts[urlParts.length - 1] || originURL

      upstream = {
        kind: 'Upstream',
        name: upstreamName,
        url: originURL,
        isClickable: originURL.startsWith('https://'),
        accentBorder: true
      }
    }

    // Normal source from sourceRef
    sources.push({
      kind: resourceData.status.sourceRef.kind,
      name: resourceData.status.sourceRef.name,
      namespace: resourceData.status.sourceRef.namespace || resourceData.metadata?.namespace,
      status: resourceData.status.sourceRef.status || 'Unknown',
      isClickable: true,
      url: resourceData.status.sourceRef.url || null,
      accentBorder: false
    })

    // Check for HelmChart when source is HelmRepository
    if (resourceData.status.sourceRef.kind === 'HelmRepository' && resourceData.status?.helmChart) {
      // helmChart is in format "namespace/name"
      const [chartNamespace, chartName] = resourceData.status.helmChart.split('/')
      const chartVersion = resourceData.spec?.chart?.spec?.version

      helmChart = {
        kind: 'HelmChart',
        name: chartName,
        namespace: chartNamespace,
        version: `semver ${chartVersion || '*'}`,
        isClickable: true
      }
    }
  } else if (resourceData?.kind === 'FluxInstance' && resourceData?.spec?.distribution?.registry) {
    // FluxInstance uses distribution as source
    const distroVersion = resourceData.spec.distribution.version
    sources.push({
      kind: 'Distro',
      name: distroVersion ? `Flux ${distroVersion}` : 'Flux',
      namespace: null,
      status: 'Ready',
      isClickable: false,
      url: resourceData.spec.distribution.registry,
      accentBorder: true
    })
  } else if (resourceData?.kind === 'ArtifactGenerator' && resourceData?.spec?.sources?.length > 0) {
    // ArtifactGenerator uses spec.sources array
    const defaultNamespace = resourceData.metadata?.namespace
    resourceData.spec.sources.forEach(src => {
      sources.push({
        kind: src.kind,
        name: src.name,
        namespace: src.namespace || defaultNamespace,
        status: 'Unknown',
        isClickable: true,
        url: null,
        accentBorder: true
      })
    })
  }

  const reconciler = {
    kind: resourceData?.kind,
    name: resourceData?.metadata?.name,
    namespace: resourceData?.metadata?.namespace,
    status: resourceData?.status?.reconcilerRef?.status || 'Unknown',
    revision: resourceData?.status?.lastAttemptedRevision || resourceData?.status?.lastAppliedRevision || null
  }

  // Handle inventory as array or object with entries
  const rawInventory = resourceData?.status?.inventory
  const inventoryItems = Array.isArray(rawInventory)
    ? rawInventory
    : (rawInventory?.entries || [])

  // Group inventory items
  const flux = []
  const workloads = []
  const resources = {}

  inventoryItems.forEach(item => {
    if (fluxKinds.includes(item.kind)) {
      flux.push({
        kind: item.kind,
        name: item.name,
        namespace: item.namespace
      })
    } else if (workloadKinds.includes(item.kind)) {
      workloads.push({
        kind: item.kind,
        name: item.name,
        namespace: item.namespace
      })
    } else {
      resources[item.kind] = (resources[item.kind] || 0) + 1
    }
  })

  return {
    upstream,
    sources,
    helmChart,
    reconciler,
    inventory: { flux, workloads, resources }
  }
}

/**
 * Get border color class based on status
 */
function getStatusBorderClass(status) {
  switch (status) {
  case 'Ready':
    return 'border border-green-500 dark:border-green-400'
  case 'Failed':
    return 'border border-red-500 dark:border-red-400'
  case 'Progressing':
    return 'border border-blue-500 dark:border-blue-400'
  case 'Suspended':
    return 'border border-yellow-500 dark:border-yellow-400'
  default:
    return 'border border-gray-400 dark:border-gray-500'
  }
}

/**
 * Node card component for source and reconciler
 */
function NodeCard({ kind, name, namespace, status, revision, version, url, onClick, isClickable: clickableProp, accentBorder }) {
  // Use explicit isClickable prop if provided, otherwise check if onClick and kind is a Flux kind
  const isClickable = clickableProp !== undefined ? (clickableProp && onClick) : (onClick && fluxKinds.includes(kind))
  const borderClass = accentBorder
    ? 'border border-purple-500 dark:border-purple-400'
    : getStatusBorderClass(status)

  const displayName = namespace ? `${namespace}/${name}` : name
  const subtext = revision || version || url

  return (
    <div
      class={`bg-white dark:bg-gray-800 rounded-lg p-3 shadow-sm ${borderClass} ${
        isClickable ? 'cursor-pointer hover:shadow-md transition-shadow' : ''
      }`}
      onClick={isClickable ? onClick : undefined}
      role={isClickable ? 'button' : undefined}
      tabIndex={isClickable ? 0 : undefined}
    >
      <div class="flex items-center gap-2 mb-1">
        <span class="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase">{kind}</span>
      </div>
      <div class="text-sm font-medium text-gray-900 dark:text-white truncate" title={displayName}>
        {displayName}
      </div>
      {subtext && (
        <div class="text-xs text-gray-500 dark:text-gray-400 truncate mt-1" title={subtext}>
          {subtext}
        </div>
      )}
    </div>
  )
}

/**
 * Inventory group card component
 * @param {string} title - Group title
 * @param {number} count - Total item count
 * @param {array|object} items - Array of items (for itemList mode) or object of kind counts
 * @param {boolean} isItemList - If true, renders individual items; if false, renders grouped counts
 * @param {function} onItemClick - Click handler for individual items
 * @param {function} onTitleClick - Click handler for the title
 * @param {boolean} alwaysShow - If true, always render even with no items
 */
function GroupCard({ title, count, items, isItemList, onItemClick, onTitleClick, alwaysShow }) {
  const hasItems = isItemList ? items.length > 0 : Object.keys(items).length > 0

  if (!hasItems && !alwaysShow) return null

  // Check if all items share the same namespace (for item lists)
  const showNamespace = isItemList && items.length > 0 &&
    !items.every(item => item.namespace === items[0].namespace)

  return (
    <div class="bg-gray-50 dark:bg-transparent rounded-lg p-3 border border-gray-200 dark:border-gray-600 w-full max-w-full sm:max-w-[280px] justify-self-center">
      <div
        class={`text-sm font-medium text-gray-700 dark:text-gray-300 mb-2 pb-2 border-b border-gray-200 dark:border-gray-600 ${onTitleClick ? 'cursor-pointer hover:text-flux-blue dark:hover:text-blue-400' : ''}`}
        onClick={onTitleClick}
        role={onTitleClick ? 'button' : undefined}
      >
        {title} ({count}){onTitleClick && ' →'}
      </div>
      <div class="space-y-1 pt-1">
        {isItemList ? (
          // Items shown individually with kind and name
          items.map((item, idx) => {
            const isClickable = onItemClick && fluxKinds.includes(item.kind)
            return (
              <div
                key={idx}
                class="text-xs py-1"
                onClick={isClickable ? (e) => {
                  e.stopPropagation()
                  onItemClick(item)
                } : undefined}
                role={isClickable ? 'button' : undefined}
                tabIndex={isClickable ? 0 : undefined}
              >
                <div class="text-xs text-gray-500 dark:text-gray-400">{item.kind}</div>
                <div
                  class={`text-sm truncate ${isClickable ? 'hover:underline hover:text-flux-blue dark:hover:text-blue-400 cursor-pointer' : ''} font-medium`}
                  title={`${item.namespace}/${item.name}`}
                >
                  {item.name}{isClickable && ' →'}
                </div>
                {showNamespace && (
                  <div class="text-xs text-gray-400 dark:text-gray-500 truncate">{item.namespace}</div>
                )}
              </div>
            )
          })
        ) : (
          // Resources shown as kind counts (sorted alphabetically)
          Object.keys(items).length > 0 ? (
            Object.entries(items)
              .sort(([a], [b]) => a.localeCompare(b))
              .map(([kind, kindCount]) => (
                <div key={kind} class="flex items-center justify-between text-xs text-gray-900 dark:text-gray-100 py-0.5">
                  <span class="truncate" title={kind}>{kind}</span>
                  <span class="ml-2 font-medium">{kindCount}</span>
                </div>
              ))
          ) : (
            <div class="text-xs text-gray-400 dark:text-gray-500 italic">No resources</div>
          )
        )}
      </div>
    </div>
  )
}

/**
 * Connector line using CSS
 */
function ConnectorLine() {
  return (
    <div class="flex flex-col items-center h-8">
      <div class="w-px h-6 bg-gray-300 dark:bg-gray-600" />
      <div
        class="w-0 h-0 border-l-[4px] border-r-[4px] border-t-[5px] border-l-transparent border-r-transparent border-t-gray-300 dark:border-t-gray-600"
      />
    </div>
  )
}

/**
 * Fan-out connector to inventory groups using CSS with rounded corners
 */
function InventoryConnector({ targetCount }) {
  if (targetCount === 0) return null

  // Grid column centers: 1 col = 50%, 2 cols = 25%/75%, 3 cols = 16.67%/50%/83.33%
  const getTargetPositions = (count) => {
    if (count === 1) return ['50%']
    if (count === 2) return ['25%', '75%']
    return ['16.67%', '50%', '83.33%']
  }

  const targets = getTargetPositions(targetCount)
  const lineColor = 'border-gray-300 dark:border-gray-600'

  // Single target - just a straight line
  if (targetCount === 1) {
    return (
      <div class="relative w-full h-8">
        <div class="absolute left-1/2 top-0 flex flex-col items-center -translate-x-1/2">
          <div class="w-px h-6 bg-gray-300 dark:bg-gray-600" />
          <div class="w-0 h-0 border-l-[4px] border-r-[4px] border-t-[5px] border-l-transparent border-r-transparent border-t-gray-300 dark:border-t-gray-600" />
        </div>
      </div>
    )
  }

  return (
    <div class="relative w-full h-8">
      {/* Vertical line from center */}
      <div class="absolute left-1/2 top-0 w-px h-3 bg-gray-300 dark:bg-gray-600 -translate-x-1/2" />

      {/* Left corner with rounded edge */}
      <div
        class={`absolute top-3 h-3 border-t border-l ${lineColor} rounded-tl-md`}
        style={{ left: `calc(${targets[0]} - 0.5px)`, right: '50%' }}
      />

      {/* Right corner with rounded edge */}
      <div
        class={`absolute top-3 h-3 border-t border-r ${lineColor} rounded-tr-md`}
        style={{ left: '50%', right: `calc(100% - ${targets[targets.length - 1]} - 0.5px)` }}
      />

      {/* Center vertical line (if 3 targets) */}
      {targetCount === 3 && (
        <div class="absolute left-1/2 top-3 flex flex-col items-center -translate-x-1/2">
          <div class="w-px h-3 bg-gray-300 dark:bg-gray-600" />
          <div class="w-0 h-0 border-l-[4px] border-r-[4px] border-t-[5px] border-l-transparent border-r-transparent border-t-gray-300 dark:border-t-gray-600" />
        </div>
      )}

      {/* Left arrow */}
      <div class="absolute flex flex-col items-center" style={{ left: targets[0], top: '23px', transform: 'translateX(-50%)' }}>
        <div class="w-0 h-0 border-l-[4px] border-r-[4px] border-t-[5px] border-l-transparent border-r-transparent border-t-gray-300 dark:border-t-gray-600" />
      </div>

      {/* Right arrow */}
      <div class="absolute flex flex-col items-center" style={{ left: targets[targets.length - 1], top: '23px', transform: 'translateX(-50%)' }}>
        <div class="w-0 h-0 border-l-[4px] border-r-[4px] border-t-[5px] border-l-transparent border-r-transparent border-t-gray-300 dark:border-t-gray-600" />
      </div>
    </div>
  )
}

/**
 * Fan-in connector from multiple sources to reconciler using CSS with rounded corners
 */
function SourcesConnector({ sourceCount }) {
  if (sourceCount === 0) return null

  const lineColor = 'border-gray-300 dark:border-gray-600'

  // Single source - just a straight line
  if (sourceCount === 1) {
    return (
      <div class="relative w-full h-8">
        <div class="absolute left-1/2 top-0 flex flex-col items-center -translate-x-1/2">
          <div class="w-px h-6 bg-gray-300 dark:bg-gray-600" />
          <div class="w-0 h-0 border-l-[4px] border-r-[4px] border-t-[5px] border-l-transparent border-r-transparent border-t-gray-300 dark:border-t-gray-600" />
        </div>
      </div>
    )
  }

  // Grid column centers: 2 cols = 25%/75%, 3 cols = 16.67%/50%/83.33%
  const getSourcePositions = (count) => {
    if (count === 2) return ['25%', '75%']
    return ['16.67%', '50%', '83.33%']
  }

  const positions = getSourcePositions(sourceCount)

  return (
    <div class="relative w-full h-8">
      {/* Left vertical line down from source */}
      <div class="absolute flex flex-col items-center" style={{ left: positions[0], top: '0', transform: 'translateX(-50%)' }}>
        <div class="w-px h-3 bg-gray-300 dark:bg-gray-600" />
      </div>

      {/* Right vertical line down from source */}
      <div class="absolute flex flex-col items-center" style={{ left: positions[positions.length - 1], top: '0', transform: 'translateX(-50%)' }}>
        <div class="w-px h-3 bg-gray-300 dark:bg-gray-600" />
      </div>

      {/* Center vertical line (if 3 sources) */}
      {sourceCount === 3 && (
        <div class="absolute left-1/2 top-0 w-px h-3 bg-gray-300 dark:bg-gray-600 -translate-x-1/2" />
      )}

      {/* Left corner with rounded edge */}
      <div
        class={`absolute top-3 h-3 border-b border-l ${lineColor} rounded-bl-md`}
        style={{ left: `calc(${positions[0]} - 0.5px)`, right: '50%' }}
      />

      {/* Right corner with rounded edge */}
      <div
        class={`absolute top-3 h-3 border-b border-r ${lineColor} rounded-br-md`}
        style={{ left: '50%', right: `calc(100% - ${positions[positions.length - 1]} - 0.5px)` }}
      />

      {/* Center line down to reconciler with arrow */}
      <div class="absolute left-1/2 top-6 flex flex-col items-center -translate-x-1/2">
        <div class="w-0 h-0 border-l-[4px] border-r-[4px] border-t-[5px] border-l-transparent border-r-transparent border-t-gray-300 dark:border-t-gray-600" />
      </div>
    </div>
  )
}

/**
 * GraphTabContent - Visual dependency graph for the resource
 */
export function GraphTabContent({ resourceData, namespace, onNavigate, setActiveTab }) {
  const graphData = useMemo(() => buildGraphData(resourceData), [resourceData])

  const { upstream, sources, helmChart, reconciler, inventory } = graphData
  const { flux, workloads, resources } = inventory

  // Calculate counts
  const fluxCount = flux.length
  const workloadsCount = workloads.length
  const resourcesCount = Object.values(resources).reduce((sum, count) => sum + count, 0)

  // Check if inventory is completely empty
  const inventoryEmpty = fluxCount === 0 && workloadsCount === 0 && resourcesCount === 0

  // Count how many inventory groups to show
  // Resources group shows if it has items OR if entire inventory is empty
  const activeGroups = [
    fluxCount > 0,
    workloadsCount > 0,
    resourcesCount > 0 || inventoryEmpty
  ].filter(Boolean).length

  // Handle Flux item click
  const handleFluxItemClick = (item) => {
    onNavigate?.({
      kind: item.kind,
      name: item.name,
      namespace: item.namespace || namespace
    })
  }

  // Handle source click
  const handleSourceClick = (source) => {
    if (source?.isClickable) {
      onNavigate?.({
        kind: source.kind,
        name: source.name,
        namespace: source.namespace || namespace
      })
    }
  }

  // Handle HelmChart click
  const handleHelmChartClick = () => {
    if (helmChart?.isClickable) {
      onNavigate?.({
        kind: helmChart.kind,
        name: helmChart.name,
        namespace: helmChart.namespace || namespace
      })
    }
  }

  return (
    <div class="flex flex-col items-center py-4" data-testid="graph-tab-content">
      {/* Upstream Node */}
      {upstream && (
        <>
          <div class="w-full max-w-full sm:max-w-[280px]">
            <NodeCard
              kind={upstream.kind}
              name={upstream.name}
              url={upstream.url}
              isClickable={upstream.isClickable}
              onClick={upstream.isClickable ? () => window.open(upstream.url, '_blank', 'noopener,noreferrer') : undefined}
              accentBorder={true}
            />
          </div>
          <ConnectorLine />
        </>
      )}

      {/* Source Nodes */}
      {sources.length > 0 && (
        <>
          {sources.length === 1 ? (
            // Single source - centered
            <div class="w-full max-w-full sm:max-w-[280px]">
              <NodeCard
                kind={sources[0].kind}
                name={sources[0].name}
                namespace={sources[0].namespace}
                status={sources[0].status}
                url={sources[0].url}
                onClick={() => handleSourceClick(sources[0])}
                isClickable={sources[0].isClickable}
                accentBorder={sources[0].accentBorder}
              />
            </div>
          ) : (
            // Multiple sources - grid layout
            <>
              {/* Desktop: grid of sources */}
              <div class="hidden sm:block w-full">
                <div class={`grid w-full gap-4 ${
                  sources.length === 2 ? 'grid-cols-2' : 'grid-cols-3'
                }`}>
                  {sources.slice(0, 3).map((source, idx) => (
                    <div key={idx} class="w-full max-w-[280px] justify-self-center">
                      <NodeCard
                        kind={source.kind}
                        name={source.name}
                        namespace={source.namespace}
                        status={source.status}
                        url={source.url}
                        onClick={() => handleSourceClick(source)}
                        isClickable={source.isClickable}
                        accentBorder={source.accentBorder}
                      />
                    </div>
                  ))}
                </div>
              </div>
              {/* Mobile: stack sources vertically */}
              <div class="sm:hidden w-full space-y-2">
                {sources.slice(0, 3).map((source, idx) => (
                  <div key={idx} class="w-full">
                    <NodeCard
                      kind={source.kind}
                      name={source.name}
                      namespace={source.namespace}
                      status={source.status}
                      url={source.url}
                      onClick={() => handleSourceClick(source)}
                      isClickable={source.isClickable}
                      accentBorder={source.accentBorder}
                    />
                  </div>
                ))}
              </div>
            </>
          )}
          {/* Desktop: fan-in connector for multiple sources */}
          <div class="hidden sm:block w-full">
            <SourcesConnector sourceCount={Math.min(sources.length, 3)} />
          </div>
          {/* Mobile: simple vertical connector */}
          <div class="sm:hidden">
            <ConnectorLine />
          </div>
        </>
      )}

      {/* HelmChart Node (between source and reconciler) */}
      {helmChart && (
        <>
          <div class="w-full max-w-full sm:max-w-[280px]">
            <NodeCard
              kind={helmChart.kind}
              name={helmChart.name}
              namespace={helmChart.namespace}
              version={helmChart.version}
              onClick={handleHelmChartClick}
              isClickable={helmChart.isClickable}
              accentBorder={true}
            />
          </div>
          <ConnectorLine />
        </>
      )}

      {/* Reconciler Node (Current) */}
      <div class="w-full max-w-full sm:max-w-[280px]">
        <NodeCard
          kind={reconciler.kind}
          name={reconciler.name}
          namespace={reconciler.namespace}
          status={reconciler.status}
          revision={reconciler.revision}
          onClick={onNavigate ? () => onNavigate({
            kind: reconciler.kind,
            name: reconciler.name,
            namespace: reconciler.namespace
          }) : undefined}
          isClickable={!!onNavigate}
        />
      </div>

      {/* Inventory Groups - stacked on mobile, grid on desktop */}
      {activeGroups > 0 && (
        <>
          {/* Desktop: fan-out connector */}
          <div class="hidden sm:block w-full">
            <InventoryConnector targetCount={activeGroups} />
          </div>
          {/* Mobile: simple vertical connector */}
          <div class="sm:hidden">
            <ConnectorLine />
          </div>
          <div class={`grid w-full gap-4 items-start grid-cols-1 ${
            activeGroups === 1 ? 'sm:grid-cols-1 max-w-xs mx-auto' :
              activeGroups === 2 ? 'sm:grid-cols-2' :
                'sm:grid-cols-3'
          }`}>
            {fluxCount > 0 && (
              <GroupCard
                title="Flux Resources"
                count={fluxCount}
                items={flux}
                isItemList={true}
                onItemClick={handleFluxItemClick}
              />
            )}
            {workloadsCount > 0 && (
              <GroupCard
                title="Workloads"
                count={workloadsCount}
                items={workloads}
                isItemList={true}
                onTitleClick={setActiveTab ? () => setActiveTab('workloads') : undefined}
              />
            )}
            {(resourcesCount > 0 || inventoryEmpty) && (
              <GroupCard
                title="Resources"
                count={resourcesCount}
                items={resources}
                isItemList={false}
                onTitleClick={resourcesCount > 0 && setActiveTab ? () => setActiveTab('inventory') : undefined}
                alwaysShow={inventoryEmpty}
              />
            )}
          </div>
        </>
      )}

    </div>
  )
}
