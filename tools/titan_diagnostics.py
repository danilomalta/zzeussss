#!/usr/bin/env python3
"""Private developer-computer test report; no customer telemetry."""
import argparse
import html
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import webbrowser
ROOT = Path(__file__).resolve().parents[1]
CHECKS = [('Frontend — testes',['npm','--prefix','frontend-web','run','test:local']),('Frontend — build',['npm','--prefix','frontend-web','run','build']),('Backend — testes',['go','test','-count=1','./...']),('Backend — vet',['go','vet','./...']),('Git — diff',['git','diff','--check'])]
def run_checks(root=ROOT,runner=subprocess.run,finder=shutil.which):
    results=[]
    for name,command in CHECKS:
        if not finder(command[0]):
            results.append((name,'NÃO EXECUTADO','Ferramenta indisponível'));continue
        try:
            result=runner(command,cwd=root/'backend' if command[0]=='go' else root,env=dict(os.environ,GOTOOLCHAIN='go1.25.0'),stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,timeout=600,check=False)
            results.append((name,'PASSOU' if result.returncode==0 else 'FALHOU','Código de saída: '+str(result.returncode)))
        except (OSError,subprocess.TimeoutExpired):
            results.append((name,'FALHOU','Comando indisponível ou tempo excedido'))
    return results

def render(results):
    cards=''.join(f'<article><h2>{html.escape(n)}</h2><strong>{html.escape(s)}</strong><p>{html.escape(d)}</p></article>' for n,s,d in results)
    return '''<!doctype html><html lang="pt-BR"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Titan — painel técnico privado</title><style>body{margin:0;background:#121925;color:#f7f9ff;font:16px system-ui}main{max-width:1200px;margin:40px auto;padding:24px}.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(220px,1fr));gap:18px}article,section{padding:24px;background:#202937;border:1px solid #394454;border-radius:20px}h1{font-size:36px}h2{font-size:18px}p{color:#b0b9ca}strong{color:#65adff}section{margin-top:24px}input,select,textarea,button{display:block;margin:12px 0;width:100%;padding:12px;box-sizing:border-box;border-radius:12px;border:1px solid #394454;background:#121925;color:#fff}button{background:#1767f5;cursor:pointer}li{padding:12px 0;white-space:pre-wrap}</style><main><p>TITAN · DESENVOLVEDOR</p><h1>Painel técnico privado</h1><p>Testes deste projeto no seu computador. Nenhum banco comercial é lido para produzir o relatório. Este painel não fica nas contas dos clientes.</p><div class="grid">'''+cards+'''</div><section><h2>Controle de defeitos e testes</h2><p>Anotações técnicas locais. Não inclua credenciais ou dados de clientes.</p><form id="issue"><input id="title" required maxlength="200" placeholder="Defeito"><select id="severity"><option>Baixa</option><option>Média</option><option>Alta</option><option>Crítica</option></select><textarea id="detail" maxlength="2000" placeholder="Passos em instalação de teste"></textarea><button>Adicionar anotação</button></form><ul id="items"></ul><p id="notice" role="status"></p></section><section><h2>Nova rodada</h2><p>Execute no terminal do projeto: <code>python3 -B tools/titan_diagnostics.py</code>. Relatório estático, sem monitoramento remoto. Serviço administrativo com autenticação própria ainda não conectado.</p></section></main><script>
let notes=[];try{const v=JSON.parse(localStorage.getItem('titan-dev-issues')||'[]');if(Array.isArray(v))notes=v.filter(x=>x&&typeof x.title==='string'&&typeof x.detail==='string'&&typeof x.severity==='string');}catch{}
const list=document.getElementById('items');function show(){list.replaceChildren();for(const item of notes){const li=document.createElement('li');li.textContent=item.severity+' · '+item.title+'\\n'+item.detail;list.append(li);}}show();document.getElementById('issue').addEventListener('submit',event=>{event.preventDefault();const title=document.getElementById('title').value.trim();if(!title)return;notes.push({title,severity:document.getElementById('severity').value,detail:document.getElementById('detail').value});try{localStorage.setItem('titan-dev-issues',JSON.stringify(notes));document.getElementById('notice').textContent='Salvo neste navegador.';}catch{document.getElementById('notice').textContent='Somente em memória; armazenamento indisponível.';}event.target.reset();show();});</script></html>'''

def save_report(results,destination=None):
    base=destination or Path.home()/'.local/share/titansystem-developer/reports'
    base.mkdir(parents=True,exist_ok=True,mode=0o700)
    directory=Path(tempfile.mkdtemp(prefix='qa-',dir=base))
    path=directory/'painel-tecnico.html'
    with path.open('x',encoding='utf-8') as stream:
        stream.write(render(results))
    path.chmod(0o600)
    return path

def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--check',action='store_true')
    parser.add_argument('--open',action='store_true',help='Abrir o relatorio privado no navegador deste computador')
    args=parser.parse_args()
    if args.check:
        for tool in ['npm','go','git']:print(tool+': '+('disponível' if shutil.which(tool) else 'indisponível'))
        return
    print('Executando testes técnicos do projeto…')
    result=run_checks()
    for name,status,_ in result:print(name+': '+status)
    path=save_report(result)
    print('Relatório privado: '+str(path))
    print('Abrir no navegador: '+path.as_uri())
    if args.open:
        try:
            if not webbrowser.open(path.as_uri(),new=2):
                print('Nao foi possivel abrir automaticamente. Use o endereco acima.')
        except (OSError,webbrowser.Error):
            print('Nao foi possivel abrir automaticamente. Use o endereco acima.')
if __name__=='__main__':main()
