import api from './api.js'

/**
 * Authentication service
 * Handles all authentication-related API calls
 */
export const authService = {
  /**
   * Login user
   * @param {Object} credentials - Login credentials
   * @param {string} credentials.identifier - Email, CPF, or CNPJ
   * @param {string} credentials.password - User password
   * @returns {Promise<{token: string, user: Object}>}
   */
  async login({ identifier, password }) {
    const response = await api.post('/auth/login', {
      email: identifier, // Backend expects 'email' field
      password,
    })
    return response.data
  },

  /**
   * Register new company and user
   * @param {Object} data - Registration data
   * @returns {Promise<{message: string}>}
   */
  async register({ companyName, cnpjCpf, email, password }) {
    const response = await api.post('/auth/register', {
      company_name: companyName,
      cnpj: cnpjCpf,
      email,
      password,
    })
    return response.data
  },

  /**
   * Logout user (client-side only)
   */
  logout() {
    localStorage.removeItem('auth_token')
    localStorage.removeItem('user')
  },

  /**
   * Get current user from localStorage
   * @returns {Object|null}
   */
  getCurrentUser() {
    const userStr = localStorage.getItem('user')
    if (!userStr) return null
    try {
      return JSON.parse(userStr)
    } catch {
      return null
    }
  },

  /**
   * Get auth token from localStorage
   * @returns {string|null}
   */
  getToken() {
    return localStorage.getItem('auth_token')
  },

  /**
   * Check if user is authenticated
   * @returns {boolean}
   */
  isAuthenticated() {
    return !!this.getToken() && !!this.getCurrentUser()
  },
}

