// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/preact'
import { signal } from '@preact/signals'
import { FilterForm } from './FilterForm'
import { fluxKinds, eventSeverities, resourceStatuses } from '../utils/constants'

describe('FilterForm', () => {
  let kindSignal
  let nameSignal
  let namespaceSignal
  let severitySignal
  let statusSignal
  let onClear

  const mockNamespaces = ['flux-system', 'default', 'kube-system']

  beforeEach(() => {
    kindSignal = signal('')
    nameSignal = signal('')
    namespaceSignal = signal('')
    severitySignal = signal('')
    statusSignal = signal('')
    onClear = vi.fn()
  })

  describe('Rendering - Required Fields', () => {
    it('should render name input field', () => {
      render(
        <FilterForm
          kindSignal={kindSignal}
          nameSignal={nameSignal}
          namespaceSignal={namespaceSignal}
          namespaces={mockNamespaces}
          onClear={onClear}
        />
      )

      const nameInput = screen.getByPlaceholderText(/Resource name/)
      expect(nameInput).toBeInTheDocument()
      expect(nameInput).toHaveAttribute('type', 'text')
    })

    it('should render namespace dropdown', () => {
      render(
        <FilterForm
          kindSignal={kindSignal}
          nameSignal={nameSignal}
          namespaceSignal={namespaceSignal}
          namespaces={mockNamespaces}
          onClear={onClear}
        />
      )

      const namespaceSelect = document.querySelector('#filter-namespace')
      expect(namespaceSelect).toBeInTheDocument()
    })

    it('should render kind dropdown with all Flux kinds', () => {
      render(
        <FilterForm
          kindSignal={kindSignal}
          nameSignal={nameSignal}
          namespaceSignal={namespaceSignal}
          namespaces={mockNamespaces}
          onClear={onClear}
        />
      )

      const kindSelect = document.querySelector('#filter-kind')
      expect(kindSelect).toBeInTheDocument()

      // Check that all flux kinds are present
      fluxKinds.forEach(kind => {
        expect(screen.getByRole('option', { name: kind })).toBeInTheDocument()
      })
    })

    it('should render namespace dropdown with provided namespaces', () => {
      render(
        <FilterForm
          kindSignal={kindSignal}
          nameSignal={nameSignal}
          namespaceSignal={namespaceSignal}
          namespaces={mockNamespaces}
          onClear={onClear}
        />
      )

      mockNamespaces.forEach(ns => {
        expect(screen.getByRole('option', { name: ns })).toBeInTheDocument()
      })
    })

    it('should render clear button', () => {
      render(
        <FilterForm
          kindSignal={kindSignal}
          nameSignal={nameSignal}
          namespaceSignal={namespaceSignal}
          namespaces={mockNamespaces}
          onClear={onClear}
        />
      )

      const clearButton = screen.getByRole('button', { name: /clear/i })
      expect(clearButton).toBeInTheDocument()
    })

    it('should show default option placeholders', () => {
      render(
        <FilterForm
          kindSignal={kindSignal}
          nameSignal={nameSignal}
          namespaceSignal={namespaceSignal}
          namespaces={mockNamespaces}
          onClear={onClear}
        />
      )

      expect(screen.getByRole('option', { name: 'All namespaces' })).toBeInTheDocument()
      expect(screen.getByRole('option', { name: 'All kinds' })).toBeInTheDocument()
    })
  })

  describe('Rendering - Conditional Fields', () => {
    it('should render severity dropdown when severitySignal provided', () => {
      render(
        <FilterForm
          kindSignal={kindSignal}
          nameSignal={nameSignal}
          namespaceSignal={namespaceSignal}
          severitySignal={severitySignal}
          namespaces={mockNamespaces}
          onClear={onClear}
        />
      )

      const severitySelect = document.querySelector('#filter-severity')
      expect(severitySelect).toBeInTheDocument()

      // Check that all severities are present
      eventSeverities.forEach(severity => {
        expect(screen.getByRole('option', { name: severity })).toBeInTheDocument()
      })
      expect(screen.getByRole('option', { name: 'All severities' })).toBeInTheDocument()
    })

    it('should not render severity dropdown when severitySignal not provided', () => {
      render(
        <FilterForm
          kindSignal={kindSignal}
          nameSignal={nameSignal}
          namespaceSignal={namespaceSignal}
          namespaces={mockNamespaces}
          onClear={onClear}
        />
      )

      const severitySelect = document.querySelector('#filter-severity')
      expect(severitySelect).not.toBeInTheDocument()
    })

    it('should render status dropdown when statusSignal provided', () => {
      render(
        <FilterForm
          kindSignal={kindSignal}
          nameSignal={nameSignal}
          namespaceSignal={namespaceSignal}
          statusSignal={statusSignal}
          namespaces={mockNamespaces}
          onClear={onClear}
        />
      )

      const statusSelect = document.querySelector('#filter-status')
      expect(statusSelect).toBeInTheDocument()

      // Check that all statuses are present
      resourceStatuses.forEach(status => {
        expect(screen.getByRole('option', { name: status })).toBeInTheDocument()
      })
      expect(screen.getByRole('option', { name: 'All statuses' })).toBeInTheDocument()
    })

    it('should not render status dropdown when statusSignal not provided', () => {
      render(
        <FilterForm
          kindSignal={kindSignal}
          nameSignal={nameSignal}
          namespaceSignal={namespaceSignal}
          namespaces={mockNamespaces}
          onClear={onClear}
        />
      )

      const statusSelect = document.querySelector('#filter-status')
      expect(statusSelect).not.toBeInTheDocument()
    })
  })

  describe('User Interactions', () => {
    it('should update kindSignal when kind dropdown changes', () => {
      render(
        <FilterForm
          kindSignal={kindSignal}
          nameSignal={nameSignal}
          namespaceSignal={namespaceSignal}
          namespaces={mockNamespaces}
          onClear={onClear}
        />
      )

      const kindSelect = document.querySelector('#filter-kind')
      fireEvent.change(kindSelect, { target: { value: 'GitRepository' } })

      expect(kindSignal.value).toBe('GitRepository')
    })

    it('should update nameSignal when name input changes', () => {
      render(
        <FilterForm
          kindSignal={kindSignal}
          nameSignal={nameSignal}
          namespaceSignal={namespaceSignal}
          namespaces={mockNamespaces}
          onClear={onClear}
        />
      )

      const nameInput = screen.getByPlaceholderText(/Resource name/)
      fireEvent.change(nameInput, { target: { value: 'flux-system' } })

      expect(nameSignal.value).toBe('flux-system')
    })

    it('should update namespaceSignal when namespace dropdown changes', () => {
      render(
        <FilterForm
          kindSignal={kindSignal}
          nameSignal={nameSignal}
          namespaceSignal={namespaceSignal}
          namespaces={mockNamespaces}
          onClear={onClear}
        />
      )

      const namespaceSelect = document.querySelector('#filter-namespace')
      fireEvent.change(namespaceSelect, { target: { value: 'flux-system' } })

      expect(namespaceSignal.value).toBe('flux-system')
    })

    it('should update severitySignal when severity dropdown changes', () => {
      render(
        <FilterForm
          kindSignal={kindSignal}
          nameSignal={nameSignal}
          namespaceSignal={namespaceSignal}
          severitySignal={severitySignal}
          namespaces={mockNamespaces}
          onClear={onClear}
        />
      )

      const severitySelect = document.querySelector('#filter-severity')
      fireEvent.change(severitySelect, { target: { value: 'Warning' } })

      expect(severitySignal.value).toBe('Warning')
    })

    it('should update statusSignal when status dropdown changes', () => {
      render(
        <FilterForm
          kindSignal={kindSignal}
          nameSignal={nameSignal}
          namespaceSignal={namespaceSignal}
          statusSignal={statusSignal}
          namespaces={mockNamespaces}
          onClear={onClear}
        />
      )

      const statusSelect = document.querySelector('#filter-status')
      fireEvent.change(statusSelect, { target: { value: 'Ready' } })

      expect(statusSignal.value).toBe('Ready')
    })

    it('should call onClear when clear button clicked', () => {
      render(
        <FilterForm
          kindSignal={kindSignal}
          nameSignal={nameSignal}
          namespaceSignal={namespaceSignal}
          namespaces={mockNamespaces}
          onClear={onClear}
        />
      )

      const clearButton = screen.getByRole('button', { name: /clear/i })
      fireEvent.click(clearButton)

      expect(onClear).toHaveBeenCalledTimes(1)
    })
  })

  describe('Signal Values Display', () => {
    it('should display current signal values in form fields', () => {
      kindSignal.value = 'Kustomization'
      nameSignal.value = 'apps'
      namespaceSignal.value = 'default'

      render(
        <FilterForm
          kindSignal={kindSignal}
          nameSignal={nameSignal}
          namespaceSignal={namespaceSignal}
          namespaces={mockNamespaces}
          onClear={onClear}
        />
      )

      expect(screen.getByPlaceholderText(/Resource name/)).toHaveValue('apps')
      expect(document.querySelector('#filter-kind')).toHaveValue('Kustomization')
      expect(document.querySelector('#filter-namespace')).toHaveValue('default')
    })
  })
})
