import {useState} from 'react';
import {createProductionMutations,parseProductionInteger,parseProductionQuantity} from '../../../core/local/productionMutations.mjs';
import type {RecipeInput} from '../../../core/local/productionMutations.mjs';
import type {RecipeVersion} from '../../../core/local/productionRead.mjs';
import {exactQuantity as q} from '../../../core/local/operationsRead.mjs';
import {useReadTask,ReadState,ReadSection} from './readPanel';
import {useProductionMutations} from './ProductionPending';
const client=createProductionMutations(),units=['unit','kg','g','liter','ml','meter'];
const decimal=(n:number)=>{const s=String(n).padStart(4,'0');return `${s.slice(0,-3)},${s.slice(-3)}`;};
interface IngredientDraft {product_id:string;unit:string;quantity:string}
export default function RecipeEditor(){
 const mutations=useProductionMutations(),loaded=useReadTask<RecipeVersion>();
 const [source,setSource]=useState(''),[recipeID,setRecipeID]=useState(''),[revision,setRevision]=useState('0'),[name,setName]=useState(''),[product,setProduct]=useState(''),[unit,setUnit]=useState('unit'),[yieldValue,setYield]=useState(''),[ingredients,setIngredients]=useState<IngredientDraft[]>([{product_id:'',unit:'g',quantity:''}]);
 function edit(v:RecipeVersion){setRecipeID(v.recipe_id);setRevision(String(v.revision));setName(v.name);setProduct(v.output_product_id);setUnit(v.output_unit);setYield(decimal(v.yield_milli));setIngredients(v.ingredients.map(i=>({product_id:i.product_id,unit:i.unit,quantity:decimal(i.quantity_milli)})));}
 function ingredient(index:number,field:keyof IngredientDraft,value:string){setIngredients(items=>items.map((item,i)=>i===index?{...item,[field]:value}:item));}
 async function publish(){
  let input:RecipeInput|null=null;
  const ok=await mutations.submit(()=>{const next:RecipeInput={operation_id:crypto.randomUUID(),recipe_id:recipeID.trim()||crypto.randomUUID(),version_id:crypto.randomUUID(),expected_revision:parseProductionInteger(revision,true),name:name.trim(),output_product_id:product.trim(),output_unit:unit,yield_milli:parseProductionQuantity(yieldValue),ingredients:ingredients.map(i=>({product_id:i.product_id.trim(),unit:i.unit,quantity_milli:parseProductionQuantity(i.quantity)}))};input=next;return {kind:'recipe',input:next};});
  if(ok&&input){const saved=input as RecipeInput;setRecipeID(saved.recipe_id);setRevision(String(saved.expected_revision+1));}
 }
 return <ReadSection title="Cadastrar receita ou publicar nova versão"><p>Uma publicação cria uma versão imutável. Para uma receita nova, deixe o ID vazio e use revisão 0. Para alterar, carregue a versão e confira a revisão esperada. Publicar não gera estoque.</p>
 <form className="operation-form" onSubmit={e=>{e.preventDefault();void loaded.run(t=>client.version(t,source.trim()));}}><label className="brand-field">ID da versão para carregar<input maxLength={128} value={source} onChange={e=>{setSource(e.target.value);loaded.clear();}}/></label><button className="brand-secondary" disabled={loaded.busy||mutations.blocked||!source.trim()}>Consultar versão para editar</button></form><ReadState task={loaded}/>{loaded.data&&<div><p>{loaded.data.name}, revisão {loaded.data.revision}, rendimento {q(loaded.data.yield_milli)} {loaded.data.output_unit}.</p><button className="brand-secondary" disabled={mutations.blocked} onClick={()=>edit(loaded.data!)}>Preencher nova versão com estes dados</button></div>}
 <form className="operation-panel" onSubmit={e=>{e.preventDefault();void publish();}}>
 <fieldset disabled={mutations.blocked}><div className="operation-form"><label className="brand-field">ID da receita (vazio para nova)<input maxLength={128} value={recipeID} onChange={e=>setRecipeID(e.target.value)}/></label><label className="brand-field">Revisão esperada<input required inputMode="numeric" value={revision} onChange={e=>setRevision(e.target.value)}/></label><label className="brand-field">Nome<input required maxLength={255} value={name} onChange={e=>setName(e.target.value)}/></label></div>
 <div className="operation-form"><label className="brand-field">ID do produto resultante<input required maxLength={128} value={product} onChange={e=>setProduct(e.target.value)}/></label><label className="brand-field">Unidade do produto<select value={unit} onChange={e=>setUnit(e.target.value)}>{units.map(u=><option key={u}>{u}</option>)}</select></label><label className="brand-field">Rendimento por batida<input required inputMode="decimal" placeholder="10,000" value={yieldValue} onChange={e=>setYield(e.target.value)}/></label></div>
 <h3>Ingredientes</h3><p>Use produtos existentes e sua unidade atual no catálogo. Quantidades com até três casas decimais, sem separador de milhar. Não há conversão automática ao publicar.</p>
 {ingredients.map((i,index)=><div className="operation-form" key={index}><label className="brand-field">Ingrediente {index+1} · ID do produto<input required maxLength={128} value={i.product_id} onChange={e=>ingredient(index,'product_id',e.target.value)}/></label><label className="brand-field">Unidade<select value={i.unit} onChange={e=>ingredient(index,'unit',e.target.value)}>{units.map(u=><option key={u}>{u}</option>)}</select></label><label className="brand-field">Quantidade por batida<input required inputMode="decimal" value={i.quantity} onChange={e=>ingredient(index,'quantity',e.target.value)}/></label><button type="button" className="brand-secondary" disabled={ingredients.length===1} onClick={()=>setIngredients(items=>items.filter((_,n)=>n!==index))}>Remover ingrediente {index+1}</button></div>)}
 <div className="operation-actions"><button type="button" className="brand-secondary" disabled={ingredients.length>=100} onClick={()=>setIngredients(items=>[...items,{product_id:'',unit:'g',quantity:''}])}>Adicionar ingrediente</button><button className="brand-primary">Publicar versão da receita</button></div></fieldset></form>
 </ReadSection>;
}
