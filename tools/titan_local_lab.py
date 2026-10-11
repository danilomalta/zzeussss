"""Native SQLite laboratory. No PostgreSQL, production checkout or remote approvals."""
import argparse
import json
import os
from pathlib import Path
import re
import secrets
import socket
import subprocess
import time
import urllib.request
import urllib.error
import titan_lab as guard

ROOT = Path(__file__).resolve().parents[1]
STATE = Path.home() / '.local/share/titansystem/local53'
PORT = 18181
PUBLIC = Path.home()/'Downloads/TitanSystem_PC53_Descritor_Publico.json'
IDS = ('tenant_id', 'store_id', 'owner_id', 'device_id')

def env():
    value = {k: os.environ[k] for k in ('PATH','HOME','USER') if k in os.environ}
    value['GOTOOLCHAIN'] = 'go1.25.0'
    return value

def run(args, password=None, cwd=None, timeout=180):
    try:
        p = subprocess.run(args, input=password, text=True, stdout=subprocess.PIPE,
                           stderr=subprocess.PIPE, env=env(), cwd=cwd, timeout=timeout)
    except (OSError, subprocess.TimeoutExpired):
        raise guard.LabError('Executável indisponível ou prazo excedido. Estado preservado.') from None
    if p.returncode or len(p.stdout)+len(p.stderr) > 2_000_000:
        raise guard.LabError('Etapa local falhou. Senhas e logs brutos não exibidos.')
    return p.stdout

def write_private(path, text):
    fd = os.open(path, os.O_WRONLY|os.O_CREAT|os.O_EXCL, 0o600)
    with os.fdopen(fd, 'w') as f: f.write(text)

def metadata():
    for parent in STATE.parents:
        if parent.is_symlink(): raise guard.LabError('Diretório privado não pode atravessar symlink.')
    guard.private_path(STATE, True)
    for name in ('owner.json','password.txt','store.sqlite','device.station'):
        guard.private_path(STATE / name)
    value = json.loads((STATE / 'owner.json').read_text())
    if set(value) != set(IDS)|{'root'} or value['root'] != str(ROOT):
        raise guard.LabError('Identidade local pertence a outro projeto ou está incompleta.')
    if any(not re.fullmatch('[0-9a-f-]{36}',value[k]) for k in IDS):
        raise guard.LabError('Identidade inválida.')
    password = (STATE / 'password.txt').read_text()
    if not re.fullmatch('[0-9a-f]{64}',password): raise guard.LabError('Credencial privada inválida.')
    station = json.loads((STATE/'device.station').read_text())
    if any(station.get(k) != value[k] for k in ('tenant_id','store_id','device_id')):
        raise guard.LabError('Aparelho e identidade não correspondem.')
    return value, password

def binary(name):
    path = STATE / name
    info = path.lstat()
    if not path.is_file() or path.is_symlink() or info.st_uid != os.getuid() or info.st_mode & 0o077:
        raise guard.LabError('Executável privado inválido.')
    return str(path)

def arguments():
    return ['--db',str(STATE/'store.sqlite'),'--station',str(STATE/'device.station')]

def initialize():
    if STATE.exists() or STATE.is_symlink():
        metadata()
        print('Instalação local existente preservada.')
        return
    for parent in STATE.parents:
        if parent.is_symlink(): raise guard.LabError('Diretório privado não pode atravessar symlink.')
    STATE.mkdir(parents=True, mode=0o700)
    password = secrets.token_hex(32)
    write_private(STATE/'password.txt', password)
    print('Compilando titan-local e titan-peer completos...', flush=True)
    for name in ('titan-local','titan-peer'):
        run(['go','build','-trimpath','-o',str(STATE/name),'./cmd/'+name], cwd=ROOT/'backend',timeout=900)
        (STATE/name).chmod(0o700)
    output = run([binary('titan-local'),'init',*arguments(),'--empresa','Laboratório local 53',
                  '--loja','Loja isolada 53','--dono','Administrador do laboratório'], password=password+'\n')
    labels = ('Empresa','Loja','Dono (ID para login)','Aparelho')
    value = {'root':str(ROOT)}
    for key,label in zip(IDS,labels):
        match = re.search(r'^'+re.escape(label)+r': ([0-9a-f-]{36})$',output,re.MULTILINE)
        if not match: raise guard.LabError('Inicialização incompleta. Preserve os arquivos; não sobrescreva.')
        value[key] = match.group(1)
    write_private(STATE/'owner.json',json.dumps(value))
    metadata()
    print('SQLite, dono e identidade do aparelho criados no laboratório privado.')

def inspect():
    value,_ = metadata()
    run([binary('titan-local'),'check',*arguments()])
    print('PASSOU: integridade SQLite, referências e aparelho aprovado.')
    print('Dono para login: '+value['owner_id'])

def serve():
    metadata()
    print('Servidor local em http://127.0.0.1:18181/local/v1/health. Ctrl+C encerra preservando dados.',flush=True)
    os.execve(binary('titan-local'), [binary('titan-local'),'serve',*arguments(),'--port',str(PORT)],env())

def http(path, payload=None, token=None, expected=200):
    headers = {'Content-Type':'application/json'}
    if token: headers['Authorization'] = 'Bearer '+token
    req = urllib.request.Request('http://127.0.0.1:'+str(PORT)+'/local/v1/'+path,
                                 data=json.dumps(payload).encode() if payload is not None else None,headers=headers)
    try:
        opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
        with opener.open(req,timeout=5) as response:
            if response.status != expected: raise guard.LabError('Resposta HTTP local inesperada.')
            body = response.read(65537)
            if len(body)>65536: raise guard.LabError('Resposta local excessiva.')
            return json.loads(body) if body else None
    except urllib.error.HTTPError as e:
        if e.code == expected: return None
        raise guard.LabError('HTTP local retornou status inesperado: '+str(e.code)) from None
    except (OSError,ValueError):
        raise guard.LabError('HTTP local não confirmado.') from None

def session_probe(value,password):
    if http('health') != {'status':'local'}: raise guard.LabError('Saúde local inválida.')
    login = http('login',{'identity_id':value['owner_id'],'password':password})
    token = login.get('token') if isinstance(login,dict) else None
    if not isinstance(token,str) or not token: raise guard.LabError('Sessão local ausente.')
    try:
        me = http('me',token=token)
        if not isinstance(me,dict) or any(me.get(k)!=value['owner_id' if k=='identity_id' else k] for k in ('identity_id','tenant_id','store_id','device_id')):
            raise guard.LabError('Contexto da sessão não corresponde ao aparelho.')
    finally:
        http('logout',{},token,204)
    http('me',token=token,expected=401)

def check():
    value,password = metadata()
    # The check owns its child and never kills an unrelated PID or existing service.
    with socket.socket() as s:
        try: s.bind(('127.0.0.1',PORT))
        except OSError: raise guard.LabError('Porta 18181 ocupada. Encerre somente seu laboratório com Ctrl+C.') from None
    before = (STATE/'device.station').read_bytes()
    for attempt in range(2):
        with open(os.devnull,'w') as sink:
            child = subprocess.Popen([binary('titan-local'),'serve',*arguments(),'--port',str(PORT)],
                                     env=env(),stdout=sink,stderr=sink)
            try:
                deadline = time.monotonic()+20
                while True:
                    if child.poll() is not None: raise guard.LabError('Servidor local não iniciou; arquivos preservados.')
                    try:
                        http('health'); break
                    except guard.LabError:
                        if time.monotonic()>deadline: raise
                        time.sleep(.2)
                session_probe(value,password)
            finally:
                if child.poll() is None:
                    child.terminate()
                    try: child.wait(timeout=10)
                    except subprocess.TimeoutExpired:
                        child.kill(); child.wait()
                        raise guard.LabError('Filho do diagnóstico excedeu prazo de parada. Banco preservado.')
            if child.returncode != 0: raise guard.LabError('Encerramento local não confirmado.')
        if before != (STATE/'device.station').read_bytes(): raise guard.LabError('Identidade mudou após reinício.')
    inspect()
    print('PASSOU: HTTP real, login, empresa/loja/aparelho, logout e reinício com identidade preservada.')
    print('SQLite local não depende do PostgreSQL ou da API online. Nenhum parceiro remoto foi aprovado.')

def peer():
    value,password = metadata()
    path = STATE/'peer.encryption'
    if path.exists() or path.is_symlink(): guard.private_path(path)
    else:
        run([binary('titan-peer'),'key-init',*arguments(),'--encryption',str(path),
             '--owner',value['owner_id']],password=password+'\n')
    public = STATE/'peer-public.json'
    if not public.exists():
        run([binary('titan-peer'),'export',*arguments(),'--encryption',str(path),'--out',str(public)])
    guard.private_path(public)
    output = run([binary('titan-peer'),'inspect','--in',str(public)])
    target = PUBLIC
    if target.exists() or target.is_symlink():
        if target.is_symlink() or not target.is_file() or target.read_bytes()!=public.read_bytes():
            raise guard.LabError('Arquivo público em Downloads já existe com conteúdo diferente. Nada sobrescrito.')
    else: write_private(target,public.read_text())
    print(output.strip())
    print('Somente descritor público exportado para: '+str(target))
    print('Não copie device.station, peer.encryption, password.txt ou o banco para outro aparelho.')

def main():
    parser=argparse.ArgumentParser(description='Laboratório SQLite local, separado do online e da produção.')
    parser.add_argument('action',choices=['init','serve','check','inspect','peer'])
    action=parser.parse_args().action
    try:
        {'init':initialize,'serve':serve,'check':check,'inspect':inspect,'peer':peer}[action]()
        return 0
    except guard.LabError as e:
        print('Pare: '+str(e))
        return 1
    except (OSError,ValueError,TypeError,KeyError):
        print('Pare: operação local não concluída. Preserve arquivos, credenciais e banco; não reaplique automaticamente.')
        return 1
if __name__=='__main__': raise SystemExit(main())
