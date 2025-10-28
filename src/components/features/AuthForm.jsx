import { useState } from 'react'
import { motion } from 'framer-motion'
import Input from '../common/Input.jsx'
import Button from '../common/Button.jsx'
import Logo from '../common/Logo.jsx'
import useLocalization from '../../hooks/useLocalization.js'

export default function AuthForm({ onRegistered }) {
  const t = useLocalization()
  const [isFlipped, setIsFlipped] = useState(false)

  // login
  const [identifier, setIdentifier] = useState('')
  const [password, setPassword] = useState('')
  const [remember, setRemember] = useState(false)

  // register
  const [company, setCompany] = useState('')
  const [doc, setDoc] = useState('')
  const [contact, setContact] = useState('')
  const [email, setEmail] = useState('')
  const [regPass, setRegPass] = useState('')
  const [regPass2, setRegPass2] = useState('')
  const [serverType, setServerType] = useState('cloud')

  const handleLogin = (e) => {
    e.preventDefault()
    if (password.length < 8) return alert('Min 8 chars')
    alert('Login submit (stub)')
  }

  const handleRegister = (e) => {
    e.preventDefault()
    if (regPass.length < 8 || regPass !== regPass2) return alert('Passwords invalid')
    onRegistered?.()
  }

  return (
    <div style={{ perspective: 1200, maxWidth: 440, margin: '0 auto' }}>
      <div style={{ position: 'relative', height: 560 }}>
        <motion.div
          style={{ position: 'absolute', inset: 0, borderRadius: 16, background: 'var(--color-surface)', padding: 24, backfaceVisibility: 'hidden', boxShadow: '0 10px 30px rgba(0,0,0,.25)' }}
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
              <Input label={t('auth.identifier')} value={identifier} onChange={(e) => setIdentifier(e.target.value)} placeholder={t('auth.identifier.placeholder')} required />
              <Input label={t('auth.password')} type="password" value={password} onChange={(e) => setPassword(e.target.value)} placeholder={t('auth.password.placeholder')} required />
              <div style={{ display: 'flex', gap: 12, alignItems: 'center', margin: '8px 0 16px' }}>
                <div style={{ flex: 1 }}>
                  <div style={{ fontSize: 12, color: 'var(--color-muted)' }}>{t('auth.deviceType')}</div>
                  <div style={{ fontSize: 13 }}> {t('auth.webBrowser')} </div>
                </div>
                <label style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  <input type="checkbox" checked={remember} onChange={(e) => setRemember(e.target.checked)} />
                  <span>{t('auth.rememberMe')}</span>
                </label>
                <a href="#" style={{ marginLeft: 'auto', fontSize: 13 }}>{t('auth.forgotPassword')}</a>
              </div>
              <Button type="submit" style={{ width: '100%' }}>{t('actions.login')}</Button>
            </form>
            <div style={{ marginTop: 12, fontSize: 14, textAlign: 'center' }}>
              <a href="#" onClick={(e) => { e.preventDefault(); setIsFlipped(true) }}>{t('actions.dontHaveAccount')}</a>
            </div>
          </div>
        </motion.div>

        <motion.div
          style={{ position: 'absolute', inset: 0, borderRadius: 16, background: 'var(--color-surface)', padding: 24, backfaceVisibility: 'hidden', transform: 'rotateY(180deg)', boxShadow: '0 10px 30px rgba(0,0,0,.25)' }}
          animate={{ rotateY: isFlipped ? 360 : 180 }}
          transition={{ duration: 0.7, ease: 'easeInOut' }}
        >
          <div style={{ display: 'grid', gap: 12 }}>
            <div style={{ display: 'flex', justifyContent: 'center', marginBottom: 12 }}>
              <Logo />
            </div>
            <h2 style={{ margin: 0 }}>{t('actions.signup')}</h2>
            <form onSubmit={handleRegister}>
              <Input label={t('register.companyName')} value={company} onChange={(e) => setCompany(e.target.value)} required />
              <Input label={t('register.cnpjCpf')} value={doc} onChange={(e) => setDoc(e.target.value)} required />
              <Input label={t('register.contact')} value={contact} onChange={(e) => setContact(e.target.value)} required />
              <Input label={t('register.email')} type="email" value={email} onChange={(e) => setEmail(e.target.value)} required />
              <Input label={t('auth.password')} type="password" value={regPass} onChange={(e) => setRegPass(e.target.value)} required />
              <Input label={t('auth.password')} type="password" value={regPass2} onChange={(e) => setRegPass2(e.target.value)} required />

              <div style={{ marginTop: 8 }}>
                <div style={{ marginBottom: 8 }}>{t('register.serverType')}</div>
                <label style={{ display: 'block', marginBottom: 6 }}>
                  <input type="radio" name="stype" value="cloud" checked={serverType==='cloud'} onChange={() => setServerType('cloud')} /> {t('register.cloud')}
                </label>
                <label style={{ display: 'block', marginBottom: 6 }}>
                  <input type="radio" name="stype" value="local" checked={serverType==='local'} onChange={() => setServerType('local')} /> {t('register.local')}
                </label>
                <label style={{ display: 'block' }}>
                  <input type="radio" name="stype" value="hd" checked={serverType==='hd'} onChange={() => setServerType('hd')} /> {t('register.hd')}
                </label>
              </div>

              <Button type="submit" style={{ width: '100%', marginTop: 12 }}>{t('actions.signup')}</Button>
            </form>
            <div style={{ marginTop: 12, fontSize: 14, textAlign: 'center' }}>
              <a href="#" onClick={(e) => { e.preventDefault(); setIsFlipped(false) }}>{t('actions.alreadyHaveAccount')}</a>
            </div>
          </div>
        </motion.div>
      </div>
    </div>
  )
}
