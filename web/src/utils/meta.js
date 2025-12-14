// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useEffect } from 'preact/hooks'

const BASE_TITLE = 'Flux Status'
const HOME_TITLE = 'Flux Status'
const HOME_DESCRIPTION = 'Real-time visibility into your GitOps pipelines'

/**
 * Sets the document title and og:title
 * @param {string|null} pageTitle - The page-specific title, or null for home page
 */
export function setPageTitle(pageTitle) {
  const title = pageTitle ? `${pageTitle} - ${BASE_TITLE}` : HOME_TITLE
  document.title = title
  const ogTitle = document.querySelector('meta[property="og:title"]')
  if (ogTitle) {
    ogTitle.setAttribute('content', title)
  }
}

/**
 * Sets the meta description and og:description
 * @param {string|null} description - The page description, or null for default
 */
export function setPageDescription(description) {
  const content = description || HOME_DESCRIPTION
  const metaDescription = document.querySelector('meta[name="description"]')
  if (metaDescription) {
    metaDescription.setAttribute('content', content)
  }
  const ogDescription = document.querySelector('meta[property="og:description"]')
  if (ogDescription) {
    ogDescription.setAttribute('content', content)
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

/**
 * Hook to set both document title and meta description on component mount
 * @param {string|null} pageTitle - The page-specific title, or null for home page
 * @param {string|null} description - The page description, or null for default
 */
export function usePageMeta(pageTitle, description) {
  useEffect(() => {
    setPageTitle(pageTitle)
    setPageDescription(description)
  }, [pageTitle, description])
}
