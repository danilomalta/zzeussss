import ContextBar74 from './ContextBar74';
import OpenInSheets70 from './OpenInSheets70';
import {Link,Navigate,Outlet,useLocation} from 'react-router-dom';
import {useTranslation} from 'react-i18next';
import {useLocalAccess} from '../../core/local/LocalAccess';
import {allowedAreas,areaForRoute} from '../../core/local/accessModel.mjs';
import {BrandLogo,ThemeToggle} from './BrandShell';
import ContactDrawer from './ContactDrawer';
import './workspace.css';
export default function LocalWorkspace(){
 const {t}=useTranslation();const location=useLocation();const {capabilities}=useLocalAccess();
 if(!allowedAreas(capabilities).includes(areaForRoute(location.pathname)))return <Navigate to="/local/home" replace/>;
 const home=['/local','/local/','/local/home'].includes(location.pathname);
 return <div className="brand-page workspace workspace-menu-only"><section className="workspace-body">
  <header className="workspace-top"><BrandLogo/><div className="workspace-top-actions">
   {!home&&<Link className="brand-secondary workspace-back-menu" to="/local/home">← {t('navigation.backToMenu')}</Link>}
   <OpenInSheets70/><ThemeToggle/><ContactDrawer/>
  </div></header>
  <ContextBar74/>
  <div className="workspace-scroll"><Outlet/></div>
 </section></div>;
}
