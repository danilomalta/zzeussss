// Opt-in example data; never used by API adapters in live mode.
export const demoCohort83={band:'10k-50k',region:'SP',activity:'retail',period:'2026-10',consent:true};
export const exampleFields83:Record<string,Record<string,string>>={
 closing:{recipient:'DEMO-RECEIVER',cash5000:'1',cash2000:'2',cash500:'1',cash100:'1',reference:'DEMO-NSU-001',amount:'219,00',method:'debit',receiptDate:'2026-10-15T18:00',salePortion:'DEMO-01-DEBIT'},
 journal:{movement:'outflow',amount:'100,00',reference:'DEMO-MOVE-001',notes:'DEMO'},
 returns:{reference:'DEMO-01',name:'SKU-DEMO-01',purchaseDate:'2026-10-13',requestDate:'2026-10-15',notes:'DEMO',channel:'physical',outcome:'loss',method:'pix',refund:'150,00',cost:'90,00',recovery:'50,00'},
 inventory:{schedule:'adhoc',location:'DEMO-DEPOT',name:'SKU-DEMO-01',expectedAtCount:'10',countedQty:'8',movements:'-2',notes:'DEMO'},
 quality:{name:'SKU-DEMO-01',formula:'FORMULA-DEMO-v1',batch:'BATCH-DEMO-01',expiry:'2027-01-15',stockInput:'10000',perBatch:'2000',output:'1000',lossPercent:'2',reference:'REPORT-DEMO-001',notes:'DEMO'},
 supply:{name:'SKU-DEMO-01',minimum:'10',maximum:'30',demand:'medium',rejections:'2',window:'2026-10-16T10:00',ordered:'20',receivedQty:'18',method:'boleto',dueDate:'2026-11-15',settlementPolicy:'negotiated',reference:'ORDER-DEMO-001',notes:'DEMO'},
 opportunities:{band:'10k-50k',activityType:'retail',region:'SP',period:'2026-10',consent:'yes',trial:'yes',buyerAccept:'yes',slotDate:'2026-10-20'},
 performance:{name:'SKU-DEMO-01',period:'2026-10',revenue:'4500,00',productCost:'3000,00',labor:'400,00',waste:'150,00'},
 devices:{topology:'local',allocation:'10',transferQty:'3',originLock:'no'}
};
