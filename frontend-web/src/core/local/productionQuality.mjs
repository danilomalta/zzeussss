import {id,integer,positive,array,requireValid,readAPI} from './operationsRead.mjs';
import {validLot,reasonText,recordedTime} from './productionLots.mjs';
const verdict=v=>['passed','failed'].includes(v);
const immutable=['id','result_id','product_id','unit','quantity_milli','code','manufactured_on','expires_on','reason','created_by','created_at'];
export function sameLotMetadata(a,b){return validLot(a)&&validLot(b)&&immutable.every(k=>a[k]===b[k]);}
export function validQualityReview(v,lotID){return !!v&&id(lotID)&&v.lot_id===lotID&&positive(v.revision)&&v.revision<=2147483647&&['operation_id','device_id','actor_id'].every(k=>id(v[k]))&&verdict(v.status)&&reasonText(v.criterion)&&reasonText(v.reason)&&recordedTime(v.created_at)&&validLot(v.lot_snapshot)&&v.lot_snapshot.id===lotID&&v.lot_snapshot.status==='recorded'&&v.lot_snapshot.revision===1;}
export function validLotQuality(v,lotID){return !!v&&id(lotID)&&v.lot_id===lotID&&['recorded','voided'].includes(v.lot_status)&&integer(v.revision)&&v.revision<=2147483647&&(v.revision===0?v.status==='not_assessed'&&v.latest===null:verdict(v.status)&&validQualityReview(v.latest,lotID)&&v.latest.revision===v.revision&&v.latest.status===v.status);}
export function validQualityPage(items,lotID,offset){return id(lotID)&&integer(offset)&&array(items)&&items.length<=50&&items.every((r,n)=>validQualityReview(r,lotID)&&r.revision===offset+n+1)&&new Set(items.map(r=>JSON.stringify([r.device_id,r.operation_id]))).size===items.length;}
export function validQualityAudit(lot,quality,items,offset){
 if(!validLot(lot)||!validLotQuality(quality,lot.id)||quality.lot_status!==lot.status||!validQualityPage(items,lot.id,offset)||items.length!==Math.min(50,Math.max(0,quality.revision-offset))||!items.every(r=>sameLotMetadata(lot,r.lot_snapshot)))return false;
 if(quality.latest&&!sameLotMetadata(lot,quality.latest.lot_snapshot))return false;
 const latest=items.find(r=>r.revision===quality.revision);return !latest||['lot_id','revision','operation_id','device_id','actor_id','status','criterion','reason','created_at'].every(k=>latest[k]===quality.latest[k])&&sameLotMetadata(latest.lot_snapshot,quality.latest.lot_snapshot);
}
export function createProductionQuality(fetcher){const get=readAPI(fetcher);return {
 async audit(token,lotID,offset=0){requireValid(id(lotID)&&integer(offset));const path=`/production/lots/${encodeURIComponent(lotID)}`;
 const lot=await get(path,token,v=>validLot(v)&&v.id===lotID);
 const quality=await get(path+'/quality',token,v=>validLotQuality(v,lotID));
 const page=await get(path+`/quality/history?offset=${offset}`,token,v=>v&&validQualityPage(v.items,lotID,offset));
 requireValid(validQualityAudit(lot,quality,page.items,offset));return {lot,quality,items:page.items,offset,has_more:offset+page.items.length<quality.revision};}
};}
