// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, vi, afterEach } from 'vitest'
import { downloadBlob } from './download'

describe('downloadBlob', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('creates an object URL, clicks an anchor with the filename and revokes the URL', () => {
    window.URL.createObjectURL = vi.fn(() => 'blob:mock')
    window.URL.revokeObjectURL = vi.fn()

    let downloadName
    let href
    const clickSpy = vi.spyOn(window.HTMLAnchorElement.prototype, 'click').mockImplementation(function () {
      downloadName = this.download
      href = this.href
    })

    const blob = new window.Blob(['hello'], { type: 'text/plain' })
    downloadBlob(blob, 'out.log')

    expect(window.URL.createObjectURL).toHaveBeenCalledWith(blob)
    expect(clickSpy).toHaveBeenCalled()
    expect(downloadName).toBe('out.log')
    expect(href).toContain('blob:mock')
    expect(window.URL.revokeObjectURL).toHaveBeenCalledWith('blob:mock')
  })
})
