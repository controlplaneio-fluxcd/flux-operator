// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useEffect } from 'preact/hooks'

const BASE_TITLE = 'Flux Status'
const HOME_TITLE = 'Flux Status Page'

/**
 * Sets the document title
 * @param {string|null} pageTitle - The page-specific title, or null for home page
 */
export function setPageTitle(pageTitle) {
  if (pageTitle) {
    document.title = `${pageTitle} - ${BASE_TITLE}`
  } else {
    document.title = HOME_TITLE
  }
}

/**
 * Hook to set the document title on component mount
 * @param {string|null} pageTitle - The page-specific title, or null for home page
 */
export function usePageTitle(pageTitle) {
  useEffect(() => {
    setPageTitle(pageTitle)
  }, [pageTitle])
}
