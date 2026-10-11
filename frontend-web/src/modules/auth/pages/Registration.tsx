import {useRef, useState} from 'react';
import type {FormEvent} from 'react';
import {Link} from 'react-router-dom';
import {BrandShell} from '../../../shared/brand/BrandShell';
import {plans, recommendedModule, planTotals, formatMoney} from './registrationModel.mjs';
import './registration.css';

const steps = ['Perfil da empresa','Módulos','Seus dados','Revisão'];
const storageHelp:Record<string,string> = {
  Local:'Dados no seu computador. As funções locais não dependem de internet.',
  'Servidor próprio':'Dados no servidor da sua empresa, com acesso pela rede configurada.',
  Nuvem:'Acesso pela internet, conforme a disponibilidade do serviço contratado.',
  Híbrido:'Combina armazenamento local e nuvem, conforme os recursos disponíveis no plano.',
};
export default function Registration() {
  const [step,setStep] = useState(0);
  const [size,setSize] = useState('MEI'), [activity,setActivity] = useState('Varejo');
  const [storage,setStorage] = useState('Local'), [users,setUsers] = useState('2');
  const [selected,setSelected] = useState<string[]>(['pos']);
  const [others,setOthers] = useState(false), [message,setMessage] = useState('');
  const [name,setName] = useState(''), [email,setEmail] = useState(''), [company,setCompany] = useState('');
  const [reviewed,setReviewed] = useState(false);
  const heading = useRef<HTMLHeadingElement>(null);
  const totals = planTotals(selected), recommended = recommendedModule(activity);
  const chosen = plans.filter(plan => selected.includes(plan.id));
  function go(next:number) {setStep(next); setMessage(''); requestAnimationFrame(() => heading.current?.focus());}
  function changeActivity(value:string) {
    setActivity(value); const id = recommendedModule(value); setSelected(id ? [id] : []); setOthers(false); setReviewed(false); setMessage('');
  }
  function toggle(id:string) {setSelected(value => value.includes(id) ? value.filter(item => item !== id) : [...value,id]);setReviewed(false);setMessage('');}
  function submit(event:FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (step === 1 && selected.length === 0) {setMessage('Selecione pelo menos um módulo para continuar.');return;}
    if (step < 3) {go(step + 1);return;}
    setMessage('Sua configuração foi revisada. A contratação online ainda não está disponível. Nenhuma conta foi criada, nenhum dado foi enviado e nenhuma cobrança foi realizada.');
  }
  function cards(ids:string[]) {
    return <div className="plan-grid">{plans.filter(plan => ids.includes(plan.id)).map(plan => <button type="button" key={plan.id} className={`plan-card ${selected.includes(plan.id) ? 'selected' : ''}`} aria-pressed={selected.includes(plan.id)} onClick={() => toggle(plan.id)}>
      <span className="plan-card-top"><span className="plan-badge">{plan.id === recommended ? 'Recomendado' : 'Opcional'}</span><span className="plan-check" aria-hidden="true">{selected.includes(plan.id) ? '✓' : '+'}</span></span>
      <h3>{plan.name}</h3><p className="plan-price">{formatMoney(plan.price)}<small>/mês</small></p>
      {plan.previous > plan.price && <><p className="plan-previous">Antes: {formatMoney(plan.previous)}/mês</p><p className="plan-savings">Economize {formatMoney(plan.previous - plan.price)}/mês</p></>}
      <p className="plan-description">{plan.description}</p><span className="plan-selection">{selected.includes(plan.id) ? 'Selecionado' : 'Selecionar módulo'}</span>
    </button>)}</div>;
  }
  return <BrandShell><section className="registration">
    <div className="registration-intro"><Link className="brand-link registration-back-login" viewTransition to="/local/login">← Voltar ao login</Link><p className="brand-eyebrow">SUA EMPRESA, SEU JEITO</p><h1>Comece com o que você precisa</h1><p className="brand-state">Escolha os módulos para sua atividade. Cada funcionário terá suas próprias permissões.</p></div>
    <ol className="registration-progress" aria-label="Etapas do cadastro">{steps.map((label,index) => <li key={label} aria-current={step === index ? 'step' : undefined} className={index < step ? 'completed' : index === step ? 'current' : ''}><span aria-hidden="true">{index < step ? '✓' : index+1}</span><span>{label}</span></li>)}</ol>
    <form onSubmit={submit}>
      <h2 ref={heading} tabIndex={-1} className="registration-heading">{step+1}. {steps[step]}</h2>
      {step === 0 && <section className="brand-panel"><div className="catalog-form">
        <label>Porte / enquadramento<select value={size} onChange={event => {setSize(event.target.value);setReviewed(false);}}>{['MEI','Microempresa','Empresa de pequeno porte','Média empresa','Grande empresa'].map(value => <option key={value}>{value}</option>)}</select></label>
        <label>Atividade principal<select value={activity} onChange={event => changeActivity(event.target.value)}>{['Varejo','Produção / indústria','Escritório de RH','Contabilidade / contador'].map(value => <option key={value}>{value}</option>)}</select></label>
        <label>Onde guardar seus dados<select value={storage} aria-describedby="storage-help" onChange={event => {setStorage(event.target.value);setReviewed(false);}}>{Object.keys(storageHelp).map(value => <option key={value}>{value}</option>)}</select><span id="storage-help" className="registration-field-help">{storageHelp[storage]}</span></label>
        <label>Usuários<input required type="number" min="1" max="10000" step="1" value={users} onChange={event => {setUsers(event.target.value);setReviewed(false);}} /><span className="registration-field-help">Pessoas que vão acessar o sistema. As permissões são definidas pela empresa.</span></label>
      </div>{size === 'MEI' && <p className="brand-state">A sugestão inicial é um acesso para o dono e um para funcionário. Usuários do sistema e vínculos de emprego são informações diferentes.</p>}</section>}
      {step === 1 && <><p className="brand-state">Para {activity.toLocaleLowerCase('pt-BR')}, recomendamos começar com este módulo. Você pode ajustar sua escolha.</p>{cards(recommended ? [recommended] : [])}<button type="button" className="brand-secondary registration-more" aria-expanded={others} aria-controls="other-modules" onClick={() => setOthers(value => !value)}>{others ? 'Ocultar outros módulos' : 'Ver outros módulos'} <span aria-hidden="true">{others ? '−' : '+'}</span></button><div id="other-modules" hidden={!others}>{others && cards(plans.filter(plan => plan.id !== recommended).map(plan => plan.id))}</div><p className="brand-state">Os módulos opcionais só entram no total quando você os seleciona.</p></>}
      {step === 2 && <section className="brand-panel"><p className="brand-state">Informe seus dados para revisar a configuração. Eles permanecem nesta tela.</p><div className="catalog-form"><label>Seu nome<input required autoComplete="name" maxLength={150} value={name} onChange={event => {setName(event.target.value);setReviewed(false);}} /></label><label>E-mail<input required type="email" autoComplete="email" maxLength={254} value={email} onChange={event => {setEmail(event.target.value);setReviewed(false);}} /></label><label className="registration-full-field">Nome da empresa<input required autoComplete="organization" maxLength={150} value={company} onChange={event => {setCompany(event.target.value);setReviewed(false);}} /></label></div></section>}
      {step === 3 && <section className="brand-panel registration-review"><h3>Confira sua escolha</h3><dl><div><dt>Empresa</dt><dd>{company} · {size}</dd></div><div><dt>Responsável</dt><dd>{name} · {email}</dd></div><div><dt>Configuração</dt><dd>{activity} · {storage} · {users} usuário(s)</dd></div><div><dt>Módulos</dt><dd>{chosen.map(plan => plan.name).join(', ')}</dd></div><div><dt>Total mensal</dt><dd>{formatMoney(totals.monthly)}</dd></div></dl><p className="brand-message">A contratação online está em preparação. Esta revisão não cria uma conta nem realiza cobrança.</p><label className="registration-confirm"><input type="checkbox" required checked={reviewed} onChange={event => setReviewed(event.target.checked)} />Conferi minha configuração e entendo que não haverá cobrança nesta etapa.</label></section>}
      {message && <p className="brand-message" role="status">{message}</p>}
      <div className="registration-sticky"><div className="registration-total" aria-live="polite"><span>{chosen.length === 1 ? chosen[0].name : `${chosen.length} módulos selecionados`}</span><strong>{formatMoney(totals.monthly)}<small>/mês</small></strong>{totals.savings > 0 && <span className="plan-savings">Economize {formatMoney(totals.savings)}/mês</span>}</div><div className="registration-sticky-actions">{step > 0 && <button type="button" className="brand-secondary" onClick={() => go(step-1)}>Voltar</button>}<button type="submit" className="brand-primary" disabled={step === 1 && selected.length === 0}>{step === 3 ? 'Concluir revisão' : 'Continuar'}<span aria-hidden="true"> →</span></button></div></div>
    </form>
  </section></BrandShell>;
}
