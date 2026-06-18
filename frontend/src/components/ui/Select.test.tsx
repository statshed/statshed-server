/**
 * AIDEV-NOTE: Select component tests
 */

import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import Select from './Select'

const mockOptions = [
  { value: 'option1', label: 'Option 1' },
  { value: 'option2', label: 'Option 2' },
  { value: 'option3', label: 'Option 3' },
]

describe('Select', () => {
  it('renders select element correctly', () => {
    render(<Select name="test" options={mockOptions} />)
    expect(screen.getByRole('combobox')).toBeInTheDocument()
  })

  it('renders all options', () => {
    render(<Select name="test" options={mockOptions} />)
    expect(screen.getByRole('option', { name: 'Option 1' })).toBeInTheDocument()
    expect(screen.getByRole('option', { name: 'Option 2' })).toBeInTheDocument()
    expect(screen.getByRole('option', { name: 'Option 3' })).toBeInTheDocument()
  })

  it('renders label when provided', () => {
    render(<Select name="status" label="Status" options={mockOptions} />)
    expect(screen.getByLabelText('Status')).toBeInTheDocument()
  })

  it('associates label with select via id', () => {
    render(<Select name="status" label="Status" id="status-select" options={mockOptions} />)
    const select = screen.getByLabelText('Status')
    expect(select).toHaveAttribute('id', 'status-select')
  })

  it('uses name as id fallback when id not provided', () => {
    render(<Select name="status" label="Status" options={mockOptions} />)
    const select = screen.getByLabelText('Status')
    expect(select).toHaveAttribute('id', 'status')
  })

  it('handles value changes', () => {
    const handleChange = vi.fn()
    render(<Select name="test" options={mockOptions} onChange={handleChange} />)
    const select = screen.getByRole('combobox')
    fireEvent.change(select, { target: { value: 'option2' } })
    expect(handleChange).toHaveBeenCalled()
  })

  it('renders placeholder option when provided', () => {
    render(<Select name="test" options={mockOptions} placeholder="Select an option..." />)
    const placeholder = screen.getByRole('option', { name: 'Select an option...' })
    expect(placeholder).toBeInTheDocument()
    expect(placeholder).toBeDisabled()
  })

  it('displays error message when error prop is provided', () => {
    render(<Select name="status" options={mockOptions} error="Please select a status" />)
    expect(screen.getByText('Please select a status')).toBeInTheDocument()
  })

  it('applies error styling when error prop is provided', () => {
    render(<Select name="status" options={mockOptions} error="Invalid selection" />)
    const select = screen.getByRole('combobox')
    expect(select).toHaveClass('border-red-500')
    expect(select).toHaveAttribute('aria-invalid', 'true')
  })

  it('displays helper text when provided and no error', () => {
    render(<Select name="status" options={mockOptions} helperText="Choose your status" />)
    expect(screen.getByText('Choose your status')).toBeInTheDocument()
  })

  it('hides helper text when error is present', () => {
    render(
      <Select
        name="status"
        options={mockOptions}
        helperText="Choose your status"
        error="Required field"
      />
    )
    expect(screen.queryByText('Choose your status')).not.toBeInTheDocument()
    expect(screen.getByText('Required field')).toBeInTheDocument()
  })

  it('forwards ref to select element', () => {
    const ref = { current: null as HTMLSelectElement | null }
    render(<Select name="test" options={mockOptions} ref={ref} />)
    expect(ref.current).toBeInstanceOf(HTMLSelectElement)
  })

  it('applies disabled state correctly', () => {
    render(<Select name="test" options={mockOptions} disabled />)
    expect(screen.getByRole('combobox')).toBeDisabled()
  })

  it('passes through additional props', () => {
    render(
      <Select
        name="test"
        options={mockOptions}
        required
        data-testid="test-select"
      />
    )
    const select = screen.getByRole('combobox')
    expect(select).toHaveAttribute('required')
    expect(select).toHaveAttribute('data-testid', 'test-select')
  })

  it('sets aria-describedby when error is present', () => {
    render(<Select name="status" id="status-field" options={mockOptions} error="Invalid" />)
    const select = screen.getByRole('combobox')
    expect(select).toHaveAttribute('aria-describedby', 'status-field-error')
  })

  it('links helper text via aria-describedby so it is announced', () => {
    render(<Select name="status" id="st" options={mockOptions} helperText="Choose your status" />)
    const select = screen.getByRole('combobox')
    expect(select).toHaveAttribute('aria-describedby', 'st-helper')
    expect(screen.getByText('Choose your status')).toHaveAttribute('id', 'st-helper')
  })

  it('associates the label with the select even when neither id nor name is given', () => {
    render(<Select label="Status" options={mockOptions} />)
    const select = screen.getByLabelText('Status')
    expect(select.getAttribute('id')).toBeTruthy()
  })

  it('sets the correct value when controlled', () => {
    render(<Select name="test" options={mockOptions} value="option2" onChange={() => {}} />)
    const select = screen.getByRole('combobox') as HTMLSelectElement
    expect(select.value).toBe('option2')
  })
})
