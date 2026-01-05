/**
 * Validation utilities
 * Professional validation functions for forms
 */

/**
 * Email validation regex
 */
const EMAIL_REGEX = /^[^\s@]+@[^\s@]+\.[^\s@]+$/

/**
 * CPF validation (Brazilian document)
 * @param {string} cpf - CPF to validate
 * @returns {boolean}
 */
export function isValidCPF(cpf) {
  if (!cpf) return false
  const cleaned = cpf.replace(/[^\d]/g, '')
  if (cleaned.length !== 11) return false
  if (/^(\d)\1{10}$/.test(cleaned)) return false // All same digits

  let sum = 0
  for (let i = 0; i < 9; i++) {
    sum += parseInt(cleaned.charAt(i)) * (10 - i)
  }
  let digit = 11 - (sum % 11)
  if (digit >= 10) digit = 0
  if (digit !== parseInt(cleaned.charAt(9))) return false

  sum = 0
  for (let i = 0; i < 10; i++) {
    sum += parseInt(cleaned.charAt(i)) * (11 - i)
  }
  digit = 11 - (sum % 11)
  if (digit >= 10) digit = 0
  if (digit !== parseInt(cleaned.charAt(10))) return false

  return true
}

/**
 * CNPJ validation (Brazilian company document)
 * @param {string} cnpj - CNPJ to validate
 * @returns {boolean}
 */
export function isValidCNPJ(cnpj) {
  if (!cnpj) return false
  const cleaned = cnpj.replace(/[^\d]/g, '')
  if (cleaned.length !== 14) return false
  if (/^(\d)\1{13}$/.test(cleaned)) return false // All same digits

  let length = cleaned.length - 2
  let numbers = cleaned.substring(0, length)
  const digits = cleaned.substring(length)
  let sum = 0
  let pos = length - 7

  for (let i = length; i >= 1; i--) {
    sum += numbers.charAt(length - i) * pos--
    if (pos < 2) pos = 9
  }

  let result = sum % 11 < 2 ? 0 : 11 - (sum % 11)
  if (result !== parseInt(digits.charAt(0))) return false

  length = length + 1
  numbers = cleaned.substring(0, length)
  sum = 0
  pos = length - 7

  for (let i = length; i >= 1; i--) {
    sum += numbers.charAt(length - i) * pos--
    if (pos < 2) pos = 9
  }

  result = sum % 11 < 2 ? 0 : 11 - (sum % 11)
  if (result !== parseInt(digits.charAt(1))) return false

  return true
}

/**
 * Email validation
 * @param {string} email - Email to validate
 * @returns {boolean}
 */
export function isValidEmail(email) {
  if (!email) return false
  return EMAIL_REGEX.test(email.trim())
}

/**
 * Password strength validation
 * @param {string} password - Password to validate
 * @returns {{valid: boolean, errors: string[]}}
 */
export function validatePassword(password) {
  const errors = []

  if (!password) {
    return { valid: false, errors: ['Password is required'] }
  }

  if (password.length < 8) {
    errors.push('Password must be at least 8 characters')
  }

  if (password.length > 128) {
    errors.push('Password must be less than 128 characters')
  }

  if (!/[a-z]/.test(password)) {
    errors.push('Password must contain at least one lowercase letter')
  }

  if (!/[A-Z]/.test(password)) {
    errors.push('Password must contain at least one uppercase letter')
  }

  if (!/\d/.test(password)) {
    errors.push('Password must contain at least one number')
  }

  return {
    valid: errors.length === 0,
    errors,
  }
}

/**
 * Validate identifier (email, CPF, or CNPJ)
 * @param {string} identifier - Identifier to validate
 * @returns {{valid: boolean, type: string|null, error: string|null}}
 */
export function validateIdentifier(identifier) {
  if (!identifier || !identifier.trim()) {
    return { valid: false, type: null, error: 'Identifier is required' }
  }

  const trimmed = identifier.trim()

  if (isValidEmail(trimmed)) {
    return { valid: true, type: 'email', error: null }
  }

  if (isValidCPF(trimmed)) {
    return { valid: true, type: 'cpf', error: null }
  }

  if (isValidCNPJ(trimmed)) {
    return { valid: true, type: 'cnpj', error: null }
  }

  return {
    valid: false,
    type: null,
    error: 'Please enter a valid email, CPF, or CNPJ',
  }
}

/**
 * Phone validation (Brazilian format)
 * @param {string} phone - Phone to validate
 * @returns {boolean}
 */
export function isValidPhone(phone) {
  if (!phone) return false
  const cleaned = phone.replace(/[^\d]/g, '')
  return cleaned.length >= 10 && cleaned.length <= 11
}

/**
 * Generic required field validation
 * @param {string} value - Value to validate
 * @param {string} fieldName - Name of the field for error message
 * @returns {string|null}
 */
export function validateRequired(value, fieldName = 'This field') {
  if (!value || !value.trim()) {
    return `${fieldName} is required`
  }
  return null
}


