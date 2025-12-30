import { useState, useEffect } from 'react'
import { motion } from 'framer-motion'
import { useNavigate } from 'react-router-dom'
import Input from '../common/Input.jsx'
import Button from '../common/Button.jsx'
import Logo from '../common/Logo.jsx'
import Spinner from '../common/Spinner.jsx'
import useLocalization from '../../hooks/useLocalization.js'
import { useAuth } from '../../contexts/AuthContext.jsx'
import { useToast } from '../../contexts/ToastContext.jsx'
import { validateIdentifier, validatePassword, validateRequired, isValidEmail, isValidPhone } from '../../utils/validation.js'

export default function AuthForm({ onRegistered }) {
  const t = useLocalization()
  const { login, register: registerUser } = useAuth()
  const { showSuccess, showError } = useToast()
  const navigate = useNavigate()
  const [isFlipped, setIsFlipped] = useState(false)
  const [isLoading, setIsLoading] = useState(false)

  // Login form state
  const [loginForm, setLoginForm] = useState({
    identifier: '',
    password: '',
    remember: false,
  })
  const [loginErrors, setLoginErrors] = useState({})

  // Register form state
  const [registerForm, setRegisterForm] = useState({
    companyName: '',
    cnpjCpf: '',
    contact: '',
    email: '',
    password: '',
    passwordConfirm: '',
    serverType: 'cloud',
  })
  const [registerErrors, setRegisterErrors] = useState({})
  const [passwordStrength, setPasswordStrength] = useState({ valid: false, errors: [] })

  // Validate login form
  const validateLogin = () => {
    const errors = {}

    const identifierValidation = validateIdentifier(loginForm.identifier)
    if (!identifierValidation.valid) {
      errors.identifier = identifierValidation.error
    }

    if (!loginForm.password) {
      errors.password = 'Password is required'
    } else if (loginForm.password.length < 8) {
      errors.password = 'Password must be at least 8 characters'
    }

    setLoginErrors(errors)
    return Object.keys(errors).length === 0
  }

  // Validate register form
  const validateRegister = () => {
    const errors = {}

    // Company name
    const companyError = validateRequired(registerForm.companyName, 'Company name')
    if (companyError) errors.companyName = companyError

    // CNPJ/CPF
    const docError = validateRequired(registerForm.cnpjCpf, 'CNPJ/CPF')
    if (docError) {
      errors.cnpjCpf = docError
    } else {
      const cleaned = registerForm.cnpjCpf.replace(/[^\d]/g, '')
      if (cleaned.length !== 11 && cleaned.length !== 14) {
        errors.cnpjCpf = 'Please enter a valid CPF (11 digits) or CNPJ (14 digits)'
      }
    }

    // Contact (phone)
    const contactError = validateRequired(registerForm.contact, 'Contact')
    if (contactError) {
      errors.contact = contactError
    } else if (!isValidPhone(registerForm.contact)) {
      errors.contact = 'Please enter a valid phone number'
    }

    // Email
    const emailError = validateRequired(registerForm.email, 'Email')
    if (emailError) {
      errors.email = emailError
    } else if (!isValidEmail(registerForm.email)) {
      errors.email = 'Please enter a valid email address'
    }

    // Password
    const passwordValidation = validatePassword(registerForm.password)
    if (!passwordValidation.valid) {
      errors.password = passwordValidation.errors[0] // Show first error
    }

    // Password confirmation
    if (!registerForm.passwordConfirm) {
      errors.passwordConfirm = 'Please confirm your password'
    } else if (registerForm.password !== registerForm.passwordConfirm) {
      errors.passwordConfirm = 'Passwords do not match'
    }

    setRegisterErrors(errors)
    return Object.keys(errors).length === 0
  }

  // Handle login
  const handleLogin = async (e) => {
    e.preventDefault()
    
    if (!validateLogin()) {
      showError('Please fix the errors before submitting')
      return
    }

    setIsLoading(true)
    try {
      const result = await login({
        identifier: loginForm.identifier.trim(),
        password: loginForm.password,
      })

      if (result.success) {
        showSuccess('Login successful! Redirecting...')
        setTimeout(() => {
          navigate('/onboarding')
        }, 1000)
      } else {
        showError(result.error || 'Login failed. Please try again.')
      }
    } catch (error) {
      showError(error.message || 'An unexpected error occurred')
    } finally {
      setIsLoading(false)
    }
  }

  // Handle register
  const handleRegister = async (e) => {
    e.preventDefault()

    if (!validateRegister()) {
      showError('Please fix the errors before submitting')
      return
    }

    setIsLoading(true)
    try {
      const result = await registerUser({
        companyName: registerForm.companyName.trim(),
        cnpjCpf: registerForm.cnpjCpf.replace(/[^\d]/g, ''),
        email: registerForm.email.trim().toLowerCase(),
        password: registerForm.password,
      })

      if (result.success) {
        showSuccess('Registration successful! Redirecting to login...')
        setTimeout(() => {
          setIsFlipped(false)
          setRegisterForm({
            companyName: '',
            cnpjCpf: '',
            contact: '',
            email: '',
            password: '',
            passwordConfirm: '',
            serverType: 'cloud',
          })
          setRegisterErrors({})
          onRegistered?.()
        }, 1500)
      } else {
        showError(result.error || 'Registration failed. Please try again.')
      }
    } catch (error) {
      showError(error.message || 'An unexpected error occurred')
    } finally {
      setIsLoading(false)
    }
  }

  // Update password strength on password change
  useEffect(() => {
    if (registerForm.password) {
      const validation = validatePassword(registerForm.password)
      setPasswordStrength(validation)
    } else {
      setPasswordStrength({ valid: false, errors: [] })
    }
  }, [registerForm.password])

  // Clear errors when switching forms
  useEffect(() => {
    if (isFlipped) {
      setLoginErrors({})
    } else {
      setRegisterErrors({})
    }
  }, [isFlipped])

  return (
    <div style={{ perspective: 1200, maxWidth: 440, margin: '0 auto' }}>
      <div style={{ position: 'relative', height: 560 }}>
        {/* Login Card */}
        <motion.div
          style={{
            position: 'absolute',
            inset: 0,
            borderRadius: 16,
            background: 'var(--color-surface)',
            padding: 24,
            backfaceVisibility: 'hidden',
            boxShadow: '0 10px 30px rgba(0,0,0,.25)',
          }}
          animate={{ rotateY: isFlipped ? 180 : 0 }}
          transition={{ duration: 0.7, ease: 'easeInOut' }}
        >
          <div style={{ display: 'grid', gap: 12 }}>
            <div style={{ display: 'flex', justifyContent: 'center', marginBottom: 12 }}>
              <Logo />
            </div>
            <h2 style={{ margin: 0 }}>{t('auth.welcomeBack')}</h2>
            <p style={{ marginTop: 0, color: 'var(--color-muted)' }}>{t('auth.loginPrompt')}</p>
            <form onSubmit={handleLogin}>
              <Input
                label={t('auth.identifier')}
                value={loginForm.identifier}
                onChange={(e) => {
                  setLoginForm({ ...loginForm, identifier: e.target.value })
                  if (loginErrors.identifier) {
                    setLoginErrors({ ...loginErrors, identifier: null })
                  }
                }}
                placeholder={t('auth.identifier.placeholder')}
                required
                error={loginErrors.identifier}
              />
              <Input
                label={t('auth.password')}
                type="password"
                value={loginForm.password}
                onChange={(e) => {
                  setLoginForm({ ...loginForm, password: e.target.value })
                  if (loginErrors.password) {
                    setLoginErrors({ ...loginErrors, password: null })
                  }
                }}
                placeholder={t('auth.password.placeholder')}
                required
                error={loginErrors.password}
              />
              <div style={{ display: 'flex', gap: 12, alignItems: 'center', margin: '8px 0 16px' }}>
                <div style={{ flex: 1 }}>
                  <div style={{ fontSize: 12, color: 'var(--color-muted)' }}>{t('auth.deviceType')}</div>
                  <div style={{ fontSize: 13 }}>{t('auth.webBrowser')}</div>
                </div>
                <label style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  <input
                    type="checkbox"
                    checked={loginForm.remember}
                    onChange={(e) => setLoginForm({ ...loginForm, remember: e.target.checked })}
                  />
                  <span>{t('auth.rememberMe')}</span>
                </label>
                <a href="#" style={{ marginLeft: 'auto', fontSize: 13 }}>
                  {t('auth.forgotPassword')}
                </a>
              </div>
              <Button type="submit" style={{ width: '100%' }} disabled={isLoading}>
                {isLoading ? <Spinner /> : t('actions.login')}
              </Button>
            </form>
            <div style={{ marginTop: 12, fontSize: 14, textAlign: 'center' }}>
              <a
                href="#"
                onClick={(e) => {
                  e.preventDefault()
                  setIsFlipped(true)
                }}
              >
                {t('actions.dontHaveAccount')}
              </a>
            </div>
          </div>
        </motion.div>

        {/* Register Card */}
        <motion.div
          style={{
            position: 'absolute',
            inset: 0,
            borderRadius: 16,
            background: 'var(--color-surface)',
            padding: 24,
            backfaceVisibility: 'hidden',
            transform: 'rotateY(180deg)',
            boxShadow: '0 10px 30px rgba(0,0,0,.25)',
            overflowY: 'auto',
          }}
          animate={{ rotateY: isFlipped ? 360 : 180 }}
          transition={{ duration: 0.7, ease: 'easeInOut' }}
        >
          <div style={{ display: 'grid', gap: 12 }}>
            <div style={{ display: 'flex', justifyContent: 'center', marginBottom: 12 }}>
              <Logo />
            </div>
            <h2 style={{ margin: 0 }}>{t('actions.signup')}</h2>
            <form onSubmit={handleRegister}>
              <Input
                label={t('register.companyName')}
                value={registerForm.companyName}
                onChange={(e) => {
                  setRegisterForm({ ...registerForm, companyName: e.target.value })
                  if (registerErrors.companyName) {
                    setRegisterErrors({ ...registerErrors, companyName: null })
                  }
                }}
                required
                error={registerErrors.companyName}
              />
              <Input
                label={t('register.cnpjCpf')}
                value={registerForm.cnpjCpf}
                onChange={(e) => {
                  setRegisterForm({ ...registerForm, cnpjCpf: e.target.value })
                  if (registerErrors.cnpjCpf) {
                    setRegisterErrors({ ...registerErrors, cnpjCpf: null })
                  }
                }}
                required
                error={registerErrors.cnpjCpf}
              />
              <Input
                label={t('register.contact')}
                value={registerForm.contact}
                onChange={(e) => {
                  setRegisterForm({ ...registerForm, contact: e.target.value })
                  if (registerErrors.contact) {
                    setRegisterErrors({ ...registerErrors, contact: null })
                  }
                }}
                required
                error={registerErrors.contact}
              />
              <Input
                label={t('register.email')}
                type="email"
                value={registerForm.email}
                onChange={(e) => {
                  setRegisterForm({ ...registerForm, email: e.target.value })
                  if (registerErrors.email) {
                    setRegisterErrors({ ...registerErrors, email: null })
                  }
                }}
                required
                error={registerErrors.email}
              />
              <div>
                <Input
                  label={t('auth.password')}
                  type="password"
                  value={registerForm.password}
                  onChange={(e) => {
                    setRegisterForm({ ...registerForm, password: e.target.value })
                    if (registerErrors.password) {
                      setRegisterErrors({ ...registerErrors, password: null })
                    }
                  }}
                  required
                  error={registerErrors.password}
                />
                {registerForm.password && passwordStrength.errors.length > 0 && (
                  <div style={{ fontSize: 12, color: 'var(--color-error)', marginTop: -8, marginBottom: 8 }}>
                    {passwordStrength.errors[0]}
                  </div>
                )}
              </div>
              <Input
                label={`${t('auth.password')} (Confirm)`}
                type="password"
                value={registerForm.passwordConfirm}
                onChange={(e) => {
                  setRegisterForm({ ...registerForm, passwordConfirm: e.target.value })
                  if (registerErrors.passwordConfirm) {
                    setRegisterErrors({ ...registerErrors, passwordConfirm: null })
                  }
                }}
                required
                error={registerErrors.passwordConfirm}
              />

              <div style={{ marginTop: 8 }}>
                <div style={{ marginBottom: 8 }}>{t('register.serverType')}</div>
                <label style={{ display: 'block', marginBottom: 6 }}>
                  <input
                    type="radio"
                    name="stype"
                    value="cloud"
                    checked={registerForm.serverType === 'cloud'}
                    onChange={() => setRegisterForm({ ...registerForm, serverType: 'cloud' })}
                  />{' '}
                  {t('register.cloud')}
                </label>
                <label style={{ display: 'block', marginBottom: 6 }}>
                  <input
                    type="radio"
                    name="stype"
                    value="local"
                    checked={registerForm.serverType === 'local'}
                    onChange={() => setRegisterForm({ ...registerForm, serverType: 'local' })}
                  />{' '}
                  {t('register.local')}
                </label>
                <label style={{ display: 'block' }}>
                  <input
                    type="radio"
                    name="stype"
                    value="hd"
                    checked={registerForm.serverType === 'hd'}
                    onChange={() => setRegisterForm({ ...registerForm, serverType: 'hd' })}
                  />{' '}
                  {t('register.hd')}
                </label>
              </div>

              <Button type="submit" style={{ width: '100%', marginTop: 12 }} disabled={isLoading}>
                {isLoading ? <Spinner /> : t('actions.signup')}
              </Button>
            </form>
            <div style={{ marginTop: 12, fontSize: 14, textAlign: 'center' }}>
              <a
                href="#"
                onClick={(e) => {
                  e.preventDefault()
                  setIsFlipped(false)
                }}
              >
                {t('actions.alreadyHaveAccount')}
              </a>
            </div>
          </div>
        </motion.div>
      </div>
    </div>
  )
}
