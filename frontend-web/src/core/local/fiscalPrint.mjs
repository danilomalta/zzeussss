// The document is rendered from archived XML by the authenticated server.
// The popup receives no token, external URL, customer storage or executable script.
export async function printFiscalReceipt({load,signal,open=()=>window.open('','_blank','width=520,height=720')}){
 const popup=open();if(!popup)throw Error('popupBlocked');popup.opener=null;
 const onAbort=()=>popup.close();signal?.addEventListener('abort',onAbort,{once:true});
 try{
  if(signal?.aborted)throw Error('aborted');
  const html=await load();if(signal?.aborted)throw Error('aborted');if(popup.closed)throw Error('closed');
  if(typeof html!=='string'||html.length>2000000||!/<head(?:\s[^>]*)?>/i.test(html))throw Error('invalidReply');
  const policy=`<meta http-equiv="Content-Security-Policy" content="default-src 'none'; script-src 'none'; img-src data:; style-src 'unsafe-inline'; base-uri 'none'; form-action 'none'; object-src 'none'"><meta name="referrer" content="no-referrer">`;
  popup.document.open();popup.document.write(html.replace(/<head(?:\s[^>]*)?>/i,match=>match+policy));popup.document.close();
  await Promise.all(Array.from(popup.document.images||[],image=>image.decode()));
  if(signal?.aborted)throw Error('aborted');if(popup.closed)throw Error('closed');popup.focus();popup.print();return popup;
 }catch(error){popup.close();throw error;}finally{signal?.removeEventListener('abort',onAbort);}
}
