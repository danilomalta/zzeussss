import {internalMaintenanceWords} from '../../../core/internalMaintenance/words.mjs';
import {salesWords} from '../../../core/salesAnalysis/words';
import {menu90} from '../../../core/operation90/menu';
import {menu89} from '../../../core/enterprise89/menu';
import {can86,enabled86} from '../../../core/sectors86/rules.mjs';
import {menuWords86} from '../../../core/sectors86/menu';
import {useProfile86} from '../../../core/sectors86/useProfile';
import {allowed85,areas85} from '../../../core/business85/rules.mjs';
import {menuWords85} from '../../../core/business85/menu';
import {menuLabel84} from '../../../core/operations84/menu';
import {allowed84,areas84} from '../../../core/operations84/rules.mjs';
import {permitted as lossPermitted76} from '../../../core/loss76/engine.mjs';
import MenuIcon75 from '../../../shared/brand/MenuIcon75';
import MenuStatus75,{MenuLegend75} from '../../../shared/brand/MenuStatus75';
import {words75} from '../../../core/context75/words';
import '../../../shared/brand/context75.css';
import {words74} from '../../../core/context74/words';
import {permitted} from "../../../core/workbench73/engine.mjs";
import {workbenchWords} from "../../../core/workbench73/words";
import zh72 from '../../../i18n/menu72/zh-CN.json';
import fr72 from '../../../i18n/menu72/fr.json';
import es72 from '../../../i18n/menu72/es.json';
import en72 from '../../../i18n/menu72/en.json';
import pt72 from '../../../i18n/menu72/pt-BR.json';
import {useState} from 'react';
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
import './menu72.css';
const copy72:Record<string,typeof pt72>={'pt-BR':pt72,pt:pt72,en:en72,es:es72,fr:fr72,'zh-CN':zh72,zh:zh72};

const copy:Record<string,typeof pt>={'pt-BR':pt,pt,en,es,fr,'zh-CN':zh,zh};
const entries=[
 {id:'integrations',path:'/local/integrations',icon:'⇄'},
 {id:'hr',path:'/local/hr',icon:'♧'},
 {id:'portfolio',path:'/local/overview',icon:'▥'},
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
 const v75=words75(i18n.resolvedLanguage||i18n.language);
 const w74=words74(i18n.resolvedLanguage||i18n.language);
 const w73=workbenchWords(i18n.resolvedLanguage||i18n.language);
 const [query,setQuery]=useState('');
 const w=copy72[i18n.resolvedLanguage||i18n.language]||pt72;
 const words=copy[i18n.resolvedLanguage||i18n.language]||pt;
 const {capabilities}=useLocalAccess();
 const profile86=useProfile86(capabilities);
 const areas=allowedAreas(capabilities);
 const logout=useLocalSession(state=>state.endSession);
 return <main className="brand-app-content titan-main-menu">
  <h1>{settings?words.settings:words.title}</h1>
  <p className="brand-state">{settings?words.preferences:words.choose}</p>
  {settings?<><Link className="brand-secondary titan-menu-back" to="/local/home">← {words.back}</Link><section className="brand-panel titan-menu-settings"><h2>{t('theme.appearance')}</h2><ThemeToggle/><h2>{words.languageContact}</h2><ContactDrawer/><button type="button" className="brand-secondary" onClick={()=>void logout()}>{t('workspace.logout')}</button></section></>:<>
   <MenuLegend75/>
   <label className="menu72-search">{w.search}<input type="search" value={query} onChange={event=>setQuery(event.target.value)} placeholder={w.search}/></label>
   {(['operation','management','administration'] as const).map(group=>{
    const existing=entries.filter(item=>areas.includes(item.id)||(item.id==='integrations'&&capabilities?.role==='owner')).map(item=>({...item,label:item.id==='integrations'?w74.integrations:t('workspace.'+item.id),group:['pos','catalog','stock','cash','orders','prices','map'].includes(item.id)?'operation':['hr','portfolio','staff'].includes(item.id)?'management':'administration',planned:false}));
    if(capabilities?.role==='owner' && Object.keys(import.meta.glob('./LocalSpreadsheet.tsx')).length)existing.push({id:'sheets',path:'/local/sheets',icon:'▦',label:w.sheets,group:'management',planned:false});
    if(areas84.some(a=>allowed84(capabilities,a)))existing.push({id:'operations83',path:'/local/operations',icon:'▦',label:menuLabel84(i18n.resolvedLanguage||i18n.language),group:'management',planned:false});
    if(capabilities?.permissions.includes('view_accounting'))existing.push({id:'reports',path:'/local/reports',icon:'▥',label:salesWords(i18n.resolvedLanguage||i18n.language).title,group:'management',planned:false});
    const planned=[['finance','management','portfolio'],['fiscal','administration','accounting'],['accounting','administration','accounting'],['commercial','operation','orders'],['purchasing','operation','orders'],['production','operation','production'],['discounts','operation','catalog'],['fleet','operation','fleet'],['documents','administration','staff'],['reports','management','portfolio'],['food','operation','pos'],['services','operation','orders']].filter(([, ,area])=>capabilities?.role==='owner'||areas.includes(area)).map(([id,section])=>({id,path:'',icon:'◇',label:w[id as keyof typeof w],group:section,planned:true})).filter(item=>item.id!=='reports'||!capabilities?.permissions.includes('view_accounting'));
    const routes:Record<string,string>={finance:"finance",fiscal:"fiscal",accounting:"accounting"};
    for(const item of planned){const target=routes[item.id];if(target&&permitted(capabilities,target)){item.path="/local/workbench?area="+target;item.planned=false;}}
    if(permitted(capabilities,"pdv"))existing.push({id:"payment-tools",path:"/local/workbench?area=pdv",icon:"≍",label:w73.allocation,group:"operation",planned:false});
    if(permitted(capabilities,"accountant"))existing.push({id:"accountant-tools",path:"/local/workbench?area=accountant",icon:"▤",label:w73.accountant,group:"administration",planned:false});
    if(lossPermitted76(capabilities)&&!planned.some(item=>item.id==='discounts')){existing.push({id:'discounts',path:'/local/losses',icon:'%',label:w.discounts,group:'operation',planned:false});}for(const item of planned){if(item.id==='discounts'&&lossPermitted76(capabilities)){item.path='/local/losses';item.planned=false;}}
    for(const item of planned){if(item.id==='production'&&allowed84(capabilities,'quality')){item.path='/local/operations?area=quality';item.planned=false;}}
    for(const area85 of areas85){const item=planned.find(item=>item.id===area85);if(allowed85(capabilities,area85)){if(item){item.path='/local/'+area85;item.label=menuWords85(i18n.resolvedLanguage||i18n.language)[area85];item.planned=false;}else existing.push({id:area85,path:'/local/'+area85,icon:'▦',label:menuWords85(i18n.resolvedLanguage||i18n.language)[area85],group:area85==='documents'?'administration':'operation',planned:false});}else if(item){item.group='restricted85';}}
    for(const item of planned){if(item.id==='food'){if(can86(capabilities,'food')&&profile86&&enabled86(profile86,'food')){item.path='/local/food';item.label=menuWords86(i18n.resolvedLanguage||i18n.language).food;item.planned=false;}else item.group='restricted86';}if(item.id==='production'&&can86(capabilities,'recipes')){item.path='/local/recipes';item.label=menuWords86(i18n.resolvedLanguage||i18n.language).recipes;item.planned=false;}}
    for(const a86 of ['delivery'] as const){if(can86(capabilities,a86))existing.push({id:a86,path:'/local/'+a86,icon:'⇄',label:menuWords86(i18n.resolvedLanguage||i18n.language)[a86],group:'operation',planned:false});}
    menu89(existing,planned,capabilities,profile86,i18n.resolvedLanguage||i18n.language);
    menu90(existing,planned,capabilities,i18n.resolvedLanguage||i18n.language);
    if(capabilities?.permissions.includes('view_orders'))existing.push({id:'maintenance',path:'/local/maintenance',icon:'⚙',label:internalMaintenanceWords(i18n.resolvedLanguage||i18n.language).title,group:'operation',planned:false});
    const cards=[...existing,...planned,...(group==='administration'?[{id:'settings',path:'/local/settings',icon:'⚙',label:words.settings,group,planned:false}]:[])].filter(item=>item.group===group&&item.label.toLocaleLowerCase().includes(query.toLocaleLowerCase())).sort((a,b)=>Number(b.id==='pos')-Number(a.id==='pos'));
    if(!cards.length)return null;
    return <section key={group} className="menu72-section"><h2>{w[group]}</h2><nav className="titan-menu-grid" aria-label={w[group]}>{cards.map(item=>item.planned?<article className="brand-panel titan-menu-card menu72-planned" key={item.id} aria-disabled="true" title={w.unavailable}><span className="titan-menu-icon" aria-hidden="true"><MenuIcon75 id={item.id}/></span><strong>{item.label}</strong><MenuStatus75 item={item}/></article>:<Link className={'brand-panel titan-menu-card'+(item.id==='pos'?' menu75-pos':'')} key={item.id} to={item.path}><span className="titan-menu-icon" aria-hidden="true"><MenuIcon75 id={item.id}/></span><strong>{item.label}</strong>{item.id==='pos'&&<small>{v75.posDescription}</small>}<MenuStatus75 item={item}/></Link>)}</nav></section>;
   })}

  </>}
 </main>;
}
