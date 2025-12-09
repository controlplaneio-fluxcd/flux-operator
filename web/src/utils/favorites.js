// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { signal, effect } from '@preact/signals'

// LocalStorage key for favorites
const STORAGE_KEY = 'favorites'

/**
 * Get favorites from localStorage
 * @returns {Array} Array of favorite objects [{kind, namespace, name}, ...]
 */
const getFavoritesFromStorage = () => {
  try {
    const stored = localStorage.getItem(STORAGE_KEY)
    return stored ? JSON.parse(stored) : []
  } catch {
    return []
  }
}

/**
 * Generate a unique key for a favorite resource
 * @param {string} kind - Resource kind
 * @param {string} namespace - Resource namespace
 * @param {string} name - Resource name
 * @returns {string} Unique key in format "kind/namespace/name"
 */
export function getFavoriteKey(kind, namespace, name) {
  return `${kind}/${namespace}/${name}`
}

// Reactive signal for favorites list
// New favorites are added to the beginning of the array (top of list)
export const favorites = signal(getFavoritesFromStorage())

// Sync favorites to localStorage whenever they change
effect(() => {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(favorites.value))
})

/**
 * Check if a resource is favorite
 * @param {string} kind - Resource kind
 * @param {string} namespace - Resource namespace
 * @param {string} name - Resource name
 * @returns {boolean} True if resource is favorite
 */
export function isFavorite(kind, namespace, name) {
  const key = getFavoriteKey(kind, namespace, name)
  return favorites.value.some(f => getFavoriteKey(f.kind, f.namespace, f.name) === key)
}

/**
 * Add a resource to favorites (at the beginning of the list)
 * @param {string} kind - Resource kind
 * @param {string} namespace - Resource namespace
 * @param {string} name - Resource name
 */
export function addFavorite(kind, namespace, name) {
  if (!isFavorite(kind, namespace, name)) {
    // Add to beginning of array (top of list)
    favorites.value = [{ kind, namespace, name }, ...favorites.value]
  }
}

/**
 * Remove a resource from favorites
 * @param {string} kind - Resource kind
 * @param {string} namespace - Resource namespace
 * @param {string} name - Resource name
 */
export function removeFavorite(kind, namespace, name) {
  const key = getFavoriteKey(kind, namespace, name)
  favorites.value = favorites.value.filter(f => getFavoriteKey(f.kind, f.namespace, f.name) !== key)
}

/**
 * Toggle favorite status of a resource
 * @param {string} kind - Resource kind
 * @param {string} namespace - Resource namespace
 * @param {string} name - Resource name
 */
export function toggleFavorite(kind, namespace, name) {
  if (isFavorite(kind, namespace, name)) {
    removeFavorite(kind, namespace, name)
  } else {
    addFavorite(kind, namespace, name)
  }
}

/**
 * Reorder favorites by moving a favorite from one index to another
 * @param {Array} newOrder - New array of favorites in desired order
 */
export function reorderFavorites(newOrder) {
  favorites.value = newOrder
}

/**
 * Clear all favorites
 */
export function clearFavorites() {
  favorites.value = []
}
