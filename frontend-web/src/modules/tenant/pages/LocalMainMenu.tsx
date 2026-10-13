import {Link} from 'react-router-dom';
import {useTranslation} from 'react-i18next';
import {useLocalAccess} from '../../../core/local/LocalAccess';
import {allowedAreas} from '../../../core/local/accessModel.mjs';
import {useLocalSession} from '../../../core/local/useLocalSession';
import {ThemeToggle} from '../../../shared/brand/BrandShell';
import ContactDrawer from '../../../shared/brand/ContactDrawer';
import pt from '../../../i18n/menu/pt-BR.json';
import en from '../../../i18n/menu/en.json';
import es from '../../../i18n/menu/es.json';
import fr from '../../../i18n/menu/fr.json';
import zh from '../../../i18n/menu/zh-CN.json';
import './localMainMenu.css';

const copy:Record<string,typeof pt>={'pt-BR':pt,pt,en,es,fr,'zh-CN':zh,zh};
const entries=[
 {id:'map',path:'/local/map',icon:'⌖'},
 {id:'pos',path:'/local/pos',icon:'▦'},
 {id:'catalog',path:'/local/catalog',icon:'▤'},
 {id:'stock',path:'/local/stock',icon:'▥'},
 {id:'cash',path:'/local/cash',icon:'◈'},
 {id:'orders',path:'/local/orders',icon:'⇄'},
 {id:'prices',path:'/local/prices',icon:'≍'},
 {id:'staff',path:'/local/staff',icon:'♧'},
 {id:'plans',path:'/local/subscription',icon:'▧'},
];
export default function LocalMainMenu({settings=false}:{settings?:boolean}){
 const {t,i18n}=useTranslation();
 const words=copy[i18n.resolvedLanguage||i18n.language]||pt;
 const {capabilities}=useLocalAccess();
 const areas=allowedAreas(capabilities);
 const logout=useLocalSession(state=>state.endSession);
 return <main className="brand-app-content titan-main-menu">
  <h1>{settings?words.settings:words.title}</h1>
  <p className="brand-state">{settings?words.preferences:words.choose}</p>
  {settings?<><Link className="brand-secondary titan-menu-back" to="/local/home">← {words.back}</Link><section className="brand-panel titan-menu-settings"><h2>{t('theme.appearance')}</h2><ThemeToggle/><h2>{words.languageContact}</h2><ContactDrawer/><button type="button" className="brand-secondary" onClick={()=>void logout()}>{t('workspace.logout')}</button></section></>:<>
   <nav className="titan-menu-grid" aria-label={words.title}>
    {entries.filter(item=>areas.includes(item.id)).map(item=><Link className="brand-panel titan-menu-card" key={item.id} to={item.path}><span className="titan-menu-icon" aria-hidden="true">{item.icon}</span><strong>{t('workspace.'+item.id)}</strong></Link>)}
    <Link className="brand-panel titan-menu-card" to="/local/settings"><span className="titan-menu-icon" aria-hidden="true">⚙</span><strong>{words.settings}</strong></Link>
   </nav>
  </>}
 </main>;
}
