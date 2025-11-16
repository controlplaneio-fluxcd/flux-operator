import { fluxKinds } from '../utils/constants'

/**
 * FilterForm component - Reusable filter form for Events and Resources
 *
 * @param {Object} props
 * @param {Signal} props.kindSignal - Signal for selected kind
 * @param {Signal} props.nameSignal - Signal for selected name
 * @param {Signal} props.namespaceSignal - Signal for selected namespace
 * @param {Array<string>} props.namespaces - Array of namespace names from report
 * @param {Function} props.onClear - Callback function to clear filters
 */
export function FilterForm({ kindSignal, nameSignal, namespaceSignal, namespaces, onClear }) {
  return (
    <div class="flex flex-wrap gap-4 items-center">
      {/* Name Filter */}
      <div class="flex-1 min-w-[200px]">
        <input
          id="filter-name"
          name="name"
          type="text"
          value={nameSignal.value}
          onChange={(e) => nameSignal.value = e.target.value}
          placeholder="Resource name (* for wildcard)"
          class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 placeholder-gray-400 dark:placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-flux-blue"
        />
      </div>

      {/* Namespace Filter */}
      <div class="flex-1 min-w-[200px]">
        <select
          id="filter-namespace"
          name="namespace"
          value={namespaceSignal.value}
          onChange={(e) => namespaceSignal.value = e.target.value}
          class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-flux-blue"
        >
          <option value="">All Namespaces</option>
          {(namespaces || []).map(ns => (
            <option key={ns} value={ns}>{ns}</option>
          ))}
        </select>
      </div>

      {/* Kind Filter */}
      <div class="flex-1 min-w-[200px]">
        <select
          id="filter-kind"
          name="kind"
          value={kindSignal.value}
          onChange={(e) => kindSignal.value = e.target.value}
          class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-flux-blue"
        >
          <option value="">All Kinds</option>
          {fluxKinds.map(kind => (
            <option key={kind} value={kind}>{kind}</option>
          ))}
        </select>
      </div>

      {/* Clear Filters Button */}
      <div>
        <button
          onClick={onClear}
          class="px-4 py-2 text-sm text-gray-700 dark:text-gray-300 hover:text-gray-900 dark:hover:text-white focus:outline-none whitespace-nowrap"
        >
          Clear
        </button>
      </div>
    </div>
  )
}
