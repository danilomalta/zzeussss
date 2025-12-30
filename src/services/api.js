import axios from 'axios'

const API_BASE_URL = import.meta.env.VITE_API_URL || 'http://localhost:3000/api'

// Create axios instance with default config
const api = axios.create({
  baseURL: API_BASE_URL,
  timeout: 10000,
  headers: {
    'Content-Type': 'application/json',
  },
})

// Request interceptor - Add auth token to requests
api.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem('auth_token')
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    }
    return config
  },
  (error) => {
    return Promise.reject(error)
  }
)

// Response interceptor - Handle errors globally
api.interceptors.response.use(
  (response) => response,
  (error) => {
    // Handle network errors
    if (!error.response) {
      error.message = 'Network error. Please check your connection.'
      return Promise.reject(error)
    }

    const { status, data } = error.response

    // Handle 401 - Unauthorized
    if (status === 401) {
      localStorage.removeItem('auth_token')
      localStorage.removeItem('user')
      // Don't redirect immediately - let the component handle it
      error.isUnauthorized = true
    }

    // Handle 429 - Too Many Requests
    if (status === 429) {
      error.message = data?.error || 'Too many requests. Please try again later.'
    }

    // Handle 500+ - Server errors
    if (status >= 500) {
      error.message = data?.error || 'Server error. Please try again later.'
    }

    // Use server error message or default
    error.message = data?.error || error.message || 'An error occurred'

    return Promise.reject(error)
  }
)

export default api

