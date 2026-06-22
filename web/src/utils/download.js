// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

/**
 * downloadBlob triggers a browser download of the given Blob under the given
 * filename, using a temporary object URL that is revoked once the download
 * has been initiated.
 *
 * @param {Blob} blob - The file contents to download.
 * @param {string} filename - The name to save the file as.
 */
export function downloadBlob(blob, filename) {
  const url = window.URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  window.URL.revokeObjectURL(url)
}
