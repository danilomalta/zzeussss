import { Link } from 'react-router-dom'
import useLocalization from '../../hooks/useLocalization.js'

export default function SideDrawer({ open, onClose }) {
  const t = useLocalization()
  return (
    <aside className="side-drawer" role="dialog" aria-modal="true">
      <div style={{ display: 'flex', justifyContent: 'flex-end', padding: 12 }}>
        <button className="icon-button" aria-label="Close" onClick={onClose}>✕</button>
      </div>
      <nav>
        <Link to="#" onClick={onClose}>{t('nav.contact')}</Link>
        <Link to="#" onClick={onClose}>{t('nav.help')}</Link>
        <Link to="#" onClick={onClose}>{t('nav.reseller')}</Link>
        <Link to="#" onClick={onClose}>{t('nav.about')}</Link>
      </nav>
    </aside>
  )
}
