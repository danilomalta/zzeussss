"""Real public challenge/proof in the isolated lab. Not a second-PC SQLite installer."""
import argparse
import json
import os
from pathlib import Path
import re
import sqlite3
import titan_local_lab as local

STATE = Path.home()/'.local/share/titansystem/pair54'

def prints(tool,path):
    out=local.run([tool,'inspect','--in',str(path)])
    values=[]
    for label in ('Ed25519 SHA-256','X25519 SHA-256'):
        m=re.search(re.escape(label)+r': ([0-9a-f]{64})',out)
        if not m: raise local.guard.LabError('Descritor público não confirmado.')
        values.append(m.group(1))
    return values

def facts(device):
    value,_=local.metadata()
    with sqlite3.connect((local.STATE/'store.sqlite').as_uri()+'?mode=ro',uri=True) as db:
        scope=(value['tenant_id'],value['store_id'],device)
        status=db.execute('SELECT status FROM device_pairings WHERE tenant_id=? AND store_id=? AND device_id=?',scope).fetchone()
        events=db.execute('SELECT action FROM device_pairing_events WHERE tenant_id=? AND device_id=? ORDER BY rowid',(value['tenant_id'],device)).fetchall()
        grants=db.execute('SELECT count(*) FROM sync_incoming_peers WHERE tenant_id=? AND store_id=? AND sender_device_id=?',scope).fetchone()[0]
        encryption=db.execute('SELECT count(*) FROM device_encryption_keys WHERE tenant_id=? AND store_id=? AND device_id=?',scope).fetchone()[0]
    return {'status':status[0] if status else None,'events':[v[0] for v in events],'event_grants':grants,'transport_trust':encryption}

def verify():
    local.guard.private_path(STATE,True)
    local.guard.private_path(STATE/'result.json')
    result=json.loads((STATE/'result.json').read_text())
    parent,_=local.metadata()
    if result.get('root')!=str(local.ROOT) or result.get('parent_device')!=parent['device_id']:
        raise local.guard.LabError('Resultado pertence a outra instalação.')
    device=result.get('candidate_device','')
    if not re.fullmatch('[0-9a-f-]{36}',device) or device==parent['device_id']:
        raise local.guard.LabError('Identidade candidata inválida.')
    value=facts(device)
    if value!={'status':'approved','events':['requested','proved','approved'],'event_grants':0,'transport_trust':0}:
        raise local.guard.LabError('Pareamento/auditoria/permissões não correspondem ao resultado esperado.')
    print('PASSOU: chave independente, desafio, prova, aprovação do dono e replay sem auditoria duplicada.')
    print('Permissões de eventos e confiança de transporte continuam sem concessão ao candidato.')
    print('Demonstração no mesmo PC: um SQLite principal e um candidato com chaves próprias, sem SQLite secundário.')

def demo():
    value,password=local.metadata()
    if STATE.exists() or STATE.is_symlink():
        verify()
        return
    for parent in STATE.parents:
        if parent.is_symlink(): raise local.guard.LabError('Diretório privado não pode atravessar symlink.')
    STATE.mkdir(parents=True,mode=0o700)
    tool=str(STATE/'titan-peer')
    print('Compilando ferramenta de pareamento completa...',flush=True)
    local.run(['go','build','-trimpath','-o',tool,'./cmd/titan-peer'],cwd=local.ROOT/'backend',timeout=900)
    Path(tool).chmod(0o700)
    parent_file=local.STATE/'peer-public.json'
    parent_prints=prints(tool,parent_file)
    candidate=STATE/'candidate'
    # Both endpoints are freshly owned by this lab; this does not approve a remote file.
    local.run([tool,'candidate-init','--parent',str(parent_file),'--dir',str(candidate),
               '--signing-sha256',parent_prints[0],'--encryption-sha256',parent_prints[1]])
    public=candidate/'peer-public.json'
    candidate_prints=prints(tool,public)
    descriptor=json.loads(public.read_text())
    device=descriptor['binding']['Device']['DeviceID']
    if device==value['device_id'] or facts(device)['status'] is not None:
        raise local.guard.LabError('Candidato não é independente ou já está registrado.')
    challenge=STATE/'challenge-public.json'
    answer=STATE/'answer-public.json'
    owner_args=[*local.arguments(),'--owner',value['owner_id'],'--signing-sha256',candidate_prints[0],
                '--encryption-sha256',candidate_prints[1]]
    local.run([tool,'pair-start',*owner_args,'--in',str(public),'--out',str(challenge),
               '--name','Candidato do laboratório54'],password=password+'\n')
    if facts(device)['status']!='pending': raise local.guard.LabError('Desafio aprovou o candidato prematuramente.')
    inspection=json.loads(local.run([tool,'pair-inspect','--in',str(challenge)]))
    if inspection['requester_sha256']!=parent_prints[0] or inspection['expired'] or inspection['approved']:
        raise local.guard.LabError('Solicitante/prazo do desafio não confirmado.')
    local.run([tool,'pair-answer','--station',str(candidate/'device.station'),'--in',str(challenge),
               '--out',str(answer),'--requester-sha256',parent_prints[0]])
    if facts(device)['status']!='pending': raise local.guard.LabError('Prova aprovou o candidato sem o dono.')
    proof=json.loads(local.run([tool,'pair-inspect','--in',str(answer)]))
    if proof['kind']!='answer' or proof['expired']: raise local.guard.LabError('Prova não confirmada.')
    finish=[tool,'pair-finish',*owner_args,'--in',str(answer)]
    local.run(finish,password=password+'\n')
    first=facts(device)
    local.run(finish,password=password+'\n')
    if first!=facts(device): raise local.guard.LabError('Replay duplicou aprovação ou auditoria.')
    # Exchange files may contain only public fields; never candidate secrets.
    station=json.loads((candidate/'device.station').read_text())
    for path in (public,challenge,answer):
        text=path.read_text()
        if 'private_key' in text or station['private_key'] in text or password in text:
            raise local.guard.LabError('Arquivo de troca contém conteúdo privado. Não compartilhar.')
    local.write_private(STATE/'result.json',json.dumps({'root':str(local.ROOT),'parent_device':value['device_id'],
                                                    'candidate_device':device}))
    verify()

def main():
    parser=argparse.ArgumentParser(description='Demonstração local de pareamento; não concede eventos ou instala segundo SQLite.')
    parser.add_argument('action',choices=['demo','verify'])
    action=parser.parse_args().action
    try:
        {'demo':demo,'verify':verify}[action]()
        return 0
    except local.guard.LabError as e:
        print('Pare: '+str(e)); return 1
    except (OSError,ValueError,KeyError,TypeError,sqlite3.Error):
        print('Pare: estado do pareamento não confirmado. Preserve banco e arquivos; não reaplique.'); return 1
if __name__=='__main__': raise SystemExit(main())
