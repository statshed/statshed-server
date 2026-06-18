/**
 * AIDEV-NOTE: Input component tests
 */

import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import Input from './Input'

describe('Input', () => {
  it('renders input element correctly', () => {
    render(<Input name="test" />)
    expect(screen.getByRole('textbox')).toBeInTheDocument()
  })

  it('renders label when provided', () => {
    render(<Input name="email" label="Email Address" />)
    expect(screen.getByLabelText('Email Address')).toBeInTheDocument()
  })

  it('associates label with input via id', () => {
    render(<Input name="email" label="Email Address" id="email-input" />)
    const input = screen.getByLabelText('Email Address')
    expect(input).toHaveAttribute('id', 'email-input')
  })

  it('uses name as id fallback when id not provided', () => {
    render(<Input name="username" label="Username" />)
    const input = screen.getByLabelText('Username')
    expect(input).toHaveAttribute('id', 'username')
  })

  it('handles value changes', () => {
    const handleChange = vi.fn()
    render(<Input name="test" onChange={handleChange} />)
    const input = screen.getByRole('textbox')
    fireEvent.change(input, { target: { value: 'new value' } })
    expect(handleChange).toHaveBeenCalled()
  })

  it('displays error message when error prop is provided', () => {
    render(<Input name="email" error="Invalid email address" />)
    expect(screen.getByText('Invalid email address')).toBeInTheDocument()
  })

  it('applies error styling when error prop is provided', () => {
    render(<Input name="email" error="Invalid email" />)
    const input = screen.getByRole('textbox')
    expect(input).toHaveClass('border-red-500')
    expect(input).toHaveAttribute('aria-invalid', 'true')
  })

  it('displays helper text when provided and no error', () => {
    render(<Input name="password" helperText="Must be 8+ characters" />)
    expect(screen.getByText('Must be 8+ characters')).toBeInTheDocument()
  })

  it('hides helper text when error is present', () => {
    render(
      <Input
        name="password"
        helperText="Must be 8+ characters"
        error="Password too short"
      />
    )
    expect(screen.queryByText('Must be 8+ characters')).not.toBeInTheDocument()
    expect(screen.getByText('Password too short')).toBeInTheDocument()
  })

  it('forwards ref to input element', () => {
    const ref = { current: null as HTMLInputElement | null }
    render(<Input name="test" ref={ref} />)
    expect(ref.current).toBeInstanceOf(HTMLInputElement)
  })

  it('applies disabled state correctly', () => {
    render(<Input name="test" disabled />)
    expect(screen.getByRole('textbox')).toBeDisabled()
  })

  it('applies placeholder correctly', () => {
    render(<Input name="test" placeholder="Enter text..." />)
    expect(screen.getByPlaceholderText('Enter text...')).toBeInTheDocument()
  })

  it('passes through additional props', () => {
    render(<Input name="test" type="email" maxLength={100} data-testid="email-input" />)
    const input = screen.getByRole('textbox')
    expect(input).toHaveAttribute('type', 'email')
    expect(input).toHaveAttribute('maxLength', '100')
    expect(input).toHaveAttribute('data-testid', 'email-input')
  })

  it('sets aria-describedby when error is present', () => {
    render(<Input name="email" id="email-field" error="Invalid email" />)
    const input = screen.getByRole('textbox')
    expect(input).toHaveAttribute('aria-describedby', 'email-field-error')
  })

  it('links helper text via aria-describedby so it is announced', () => {
    render(<Input name="password" id="pw" helperText="Must be 8+ characters" />)
    const input = screen.getByRole('textbox')
    expect(input).toHaveAttribute('aria-describedby', 'pw-helper')
    expect(screen.getByText('Must be 8+ characters')).toHaveAttribute('id', 'pw-helper')
  })

  it('associates the label with the input even when neither id nor name is given', () => {
    render(<Input label="Search" />)
    // Throws if the label is not associated with a control.
    const input = screen.getByLabelText('Search')
    expect(input.getAttribute('id')).toBeTruthy()
  })
})
