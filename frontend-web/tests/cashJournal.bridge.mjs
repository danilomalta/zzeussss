// Only the disposable Go HTTP fixture supplies this process's synthetic session.
import assert from 'node:assert/strict';
import {createCashJournal} from '../src/core/cashJournal/source.mjs';
import {journalCSV} from '../src/core/cashJournal/model.mjs';
const base=process.env.TITAN_BRIDGE_URL,token=process.env.TITAN_BRIDGE_TOKEN,session=JSON.parse(process.env.TITAN_BRIDGE_SESSION||'null');
if(!/^http:\/\/127\.0\.0\.1:\d+$/.test(base||'')||!token||!session)throw Error('Disposable Go HTTP fixture required');
const fetcher=(path,options)=>fetch(new URL(path,base),options),response=await fetcher('/local/v1/capabilities',{headers:{Authorization:'Bearer '+token}});assert.equal(response.status,200);const capabilities=await response.json(),api=createCashJournal({token,session,capabilities,isCurrent:()=>true,fetcher});
const page=await api.read('turn-one');assert.equal(page.total_count,5);assert.equal(page.summary.expected_cents,1200);assert.equal(page.summary.debit_cents,8196);assert.equal(page.items[4].category,'sale_reversal');
const beyond=await api.read('turn-one',100);assert.equal(beyond.items.length,0);assert.deepEqual(beyond.summary,page.summary);
const file=await api.exportCSV('turn-one');assert.equal(file.name,'titan-caixa-lancamentos.csv');assert.equal(file.total,5);assert.equal(file.data[0],0xef);const all=journalCSV(file.data,'turn-one');assert.deepEqual(all.items,page.items);assert.deepEqual(all.summary,page.summary);
const unicode=await api.read('turn-Á QA');assert.equal(unicode.total_count,1);assert.equal(unicode.summary.expected_cents,0);const unicodeCSV=await api.exportCSV('turn-Á QA');assert.equal(journalCSV(unicodeCSV.data,'turn-Á QA').total_count,1);
const literal=await api.read('turn-%literal');assert.equal(literal.summary.expected_cents,0);assert.equal(journalCSV((await api.exportCSV('turn-%literal')).data,'turn-%literal').total_count,1);
api.dispose();console.log('PASS real Go HTTP + frontend cash journal: original operator, closed drawer, exact whole totals, separate sale/reversal, empty page preserving summary, complete validated server CSV and original BOM. Disposable SQLite and synthetic session only.');
