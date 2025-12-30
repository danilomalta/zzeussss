import { Link } from 'react-router-dom'
import { motion, AnimatePresence } from 'framer-motion'
import useLocalization from '../../hooks/useLocalization.js'

export default function SideDrawer({ open, onClose }) {
  const t = useLocalization()

  return (
    <AnimatePresence>
      {open && (
        <motion.aside
          className="side-drawer"
          role="dialog"
          aria-modal="true"
          aria-label="Navigation menu"
          initial={{ x: -300 }}
          animate={{ x: 0 }}
          exit={{ x: -300 }}
          transition={{ type: 'spring', damping: 25, stiffness: 200 }}
          style={{
            position: 'fixed',
            inset: '0 auto 0 0',
            width: 300,
            background: 'var(--color-surface)',
            zIndex: 10,
            boxShadow: '2px 0 10px rgba(0,0,0,0.1)',
          }}
        >
          <div style={{ display: 'flex', justifyContent: 'flex-end', padding: 12 }}>
            <button
              className="icon-button"
              aria-label="Close menu"
              onClick={onClose}
              style={{
                background: 'none',
                border: 'none',
                padding: 'var(--space-2)',
                cursor: 'pointer',
                borderRadius: 999,
                color: 'var(--color-text)',
                fontSize: 20,
                width: 32,
                height: 32,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
              }}
            >
              ✕
            </button>
          </div>
          <nav>
            <Link
              to="#"
              onClick={(e) => {
                e.preventDefault()
                onClose()
              }}
              style={{
                display: 'block',
                padding: '14px 18px',
                color: 'var(--color-text)',
                textDecoration: 'none',
                borderBottom: '1px solid color-mix(in srgb, var(--color-text) 10%, transparent)',
                transition: 'background 0.2s ease',
              }}
              onMouseEnter={(e) => {
                e.target.style.background = 'color-mix(in srgb, var(--color-text) 8%, transparent)'
              }}
              onMouseLeave={(e) => {
                e.target.style.background = 'transparent'
              }}
            >
              {t('nav.contact')}
            </Link>
            <Link
              to="#"
              onClick={(e) => {
                e.preventDefault()
                onClose()
              }}
              style={{
                display: 'block',
                padding: '14px 18px',
                color: 'var(--color-text)',
                textDecoration: 'none',
                borderBottom: '1px solid color-mix(in srgb, var(--color-text) 10%, transparent)',
                transition: 'background 0.2s ease',
              }}
              onMouseEnter={(e) => {
                e.target.style.background = 'color-mix(in srgb, var(--color-text) 8%, transparent)'
              }}
              onMouseLeave={(e) => {
                e.target.style.background = 'transparent'
              }}
            >
              {t('nav.help')}
            </Link>
            <Link
              to="#"
              onClick={(e) => {
                e.preventDefault()
                onClose()
              }}
              style={{
                display: 'block',
                padding: '14px 18px',
                color: 'var(--color-text)',
                textDecoration: 'none',
                borderBottom: '1px solid color-mix(in srgb, var(--color-text) 10%, transparent)',
                transition: 'background 0.2s ease',
              }}
              onMouseEnter={(e) => {
                e.target.style.background = 'color-mix(in srgb, var(--color-text) 8%, transparent)'
              }}
              onMouseLeave={(e) => {
                e.target.style.background = 'transparent'
              }}
            >
              {t('nav.reseller')}
            </Link>
            <Link
              to="#"
              onClick={(e) => {
                e.preventDefault()
                onClose()
              }}
              style={{
                display: 'block',
                padding: '14px 18px',
                color: 'var(--color-text)',
                textDecoration: 'none',
                borderBottom: '1px solid color-mix(in srgb, var(--color-text) 10%, transparent)',
                transition: 'background 0.2s ease',
              }}
              onMouseEnter={(e) => {
                e.target.style.background = 'color-mix(in srgb, var(--color-text) 8%, transparent)'
              }}
              onMouseLeave={(e) => {
                e.target.style.background = 'transparent'
              }}
            >
              {t('nav.about')}
            </Link>
          </nav>
        </motion.aside>
      )}
    </AnimatePresence>
  )
}
