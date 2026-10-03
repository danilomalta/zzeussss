"""Instalacao descartavel para demonstrar o PDV com as APIs reais do Titan."""
import argparse
from datetime import datetime, timedelta, timezone
import getpass
import json
import os
from pathlib import Path
import shutil
import socket
import sqlite3
import subprocess
import sys
import tempfile
import time
from urllib.error import HTTPError, URLError
from urllib.request import HTTPRedirectHandler, ProxyHandler, Request, build_opener
import uuid


class DemoError(Exception):
    pass


class NoRedirect(HTTPRedirectHandler):
    def redirect_request(self, request, fp, code, message, headers, new_url):
        fp.close()
        raise DemoError("A API de teste respondeu com redirecionamento. Nenhum destino alternativo foi acessado.")


class API:
    def __init__(self, port):
        self.base = f"http://127.0.0.1:{port}/local/v1"
        self.opener = build_opener(ProxyHandler({}), NoRedirect())

    def request(self, path, token=None, body=None, method=None):
        if not path.startswith("/") or ":" in path or ".." in path:
            raise DemoError("Caminho local invalido.")
        headers = {"Accept": "application/json"}
        if token:
            headers["Authorization"] = f"Bearer {token}"
        data = None
        if body is not None:
            data = json.dumps(body, ensure_ascii=False).encode()
            headers["Content-Type"] = "application/json"
        request = Request(self.base + path, data=data, headers=headers, method=method or ("POST" if data is not None else "GET"))
        try:
            with self.opener.open(request, timeout=10) as response:
                if response.status == 204:
                    return None
                payload = response.read(131073)
                if len(payload) > 131072:
                    raise DemoError("Resposta da API de teste excedeu o limite.")
                result = json.loads(payload)
                if not isinstance(result, dict):
                    raise DemoError("Resposta da API de teste em formato incompativel.")
                return result
        except HTTPError as error:
            # Never dump a response, token or submitted password.
            status = error.code
            error.close()
            raise DemoError(f"API de teste recusou {path}: HTTP {status}. Dados preservados para conferencia.") from None
        except (URLError, TimeoutError, OSError, ValueError):
            raise DemoError(f"Nao foi possivel confirmar {path}. Nenhum reenvio automatico foi feito.") from None


def check_port(port):
    if not 1 <= port <= 65535:
        raise DemoError("Porta deve estar entre 1 e 65535.")
    with socket.socket() as candidate:
        try:
            candidate.bind(("127.0.0.1", port))
        except OSError:
            raise DemoError(f"Porta {port} ocupada. Nenhum processo foi encerrado. Escolha outra com --api-port ou --web-port.") from None


def check_environment(repo, api_port, web_port):
    if api_port == web_port:
        raise DemoError("API e frontend precisam de portas diferentes.")
    for command in ("go", "node"):
        if not shutil.which(command):
            raise DemoError(f"Comando {command} nao encontrado.")
    required = [repo / "backend/go.mod", repo / "frontend-web/node_modules/vite/package.json",
                repo / "frontend-web/scripts/demo-vite.mjs"]
    if not all(path.is_file() for path in required):
        raise DemoError("Execute dentro da copia com o patch aplicado e dependencias npm instaladas.")
    check_port(api_port)
    check_port(web_port)


def private_file(path, data):
    with path.open("x", encoding="utf-8") as file:
        os.chmod(path, 0o600)
        json.dump(data, file, ensure_ascii=False, indent=2)
        file.write("\n")


def command(args, cwd, input_data=None):
    env = dict(os.environ, GOTOOLCHAIN="go1.25.0")
    # CLI inputs may contain the installation password. Never use it in argv.
    result = subprocess.run([str(part) for part in args], cwd=cwd, env=env, input=input_data,
                            stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
    if result.returncode:
        raise DemoError(f"Falhou a etapa {Path(str(args[0])).name}. Saida nao exibida para proteger dados privados.")
    return result.stdout


def wait_ready(api, process, timeout=30):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        if process.poll() is not None:
            raise DemoError("O servidor de teste terminou antes de ficar disponivel. Confira as portas e os arquivos preservados.")
        try:
            if api.request("/health") == {"status": "local"}:
                return
        except DemoError:
            pass
        time.sleep(0.2)
    raise DemoError("O servidor de teste nao confirmou disponibilidade dentro do prazo.")


def stop_owned(process):
    if process is not None and process.poll() is None:
        process.terminate()
        try:
            process.wait(timeout=5)
        except subprocess.TimeoutExpired:
            process.kill()
            process.wait(timeout=5)


def seed(api, identity_id, password, station, contract, profile="varejo"):
    login = api.request("/login", body={"identity_id": identity_id, "password": password})
    token = login.get("token")
    if not isinstance(token, str) or not token:
        raise DemoError("Login de teste nao confirmou sessao.")
    try:
        context = api.request("/me", token)
        expected = {key: station[key] for key in ("tenant_id", "store_id", "device_id")}
        expected["identity_id"] = identity_id
        if not isinstance(context, dict) or any(context.get(key) != value for key, value in expected.items()):
            raise DemoError("A API nao corresponde a instalacao de teste. Nenhum cadastro foi enviado.")
        installed = api.request("/module-contracts", token, contract)
        if installed.get("revision") != 1:
            raise DemoError("Contrato de teste nao confirmado.")
        if profile in ("rh", "contabilidade"):
            return []
        location = api.request("/locations", token, {"kind": "shelf", "name": "Gondola de teste"})["id"]
        if not isinstance(location, str) or not location:
            raise DemoError("Gondola de teste nao confirmada.")
        rows = []
        for sku, name, unit, price, quantity in [
            ("TEST-CAFE", "Cafe de teste", "unit", 899, 20000),
            ("TEST-ARROZ", "Arroz de teste", "kg", 1000, 10000),
        ]:
            product = api.request("/products", token, {"sku": sku, "name": name, "unit": unit, "price_cents": price, "cost_cents": 0})["id"]
            operation = str(uuid.uuid4())
            result = api.request("/stock/operations", token, {"operation_id": operation, "kind": "entry", "product_id": product,
                                "to_location_id": location, "quantity_milli": quantity, "reason": "Carga inicial da demonstracao isolada"})
            if result.get("operation_id") != operation:
                raise DemoError("Entrada de estoque de teste nao confirmada.")
            balance = api.request(f"/stock/balance?product_id={product}&location_id={location}", token)
            if balance.get("quantity_milli") != quantity:
                raise DemoError("Saldo de teste nao corresponde a entrada confirmada.")
            rows.append({"id": product, "sku": sku, "unit": unit, "price_cents": price, "quantity_milli": quantity})
        if profile == "varejo" and api.request("/cash/current", token).get("session", "missing") is not None:
            raise DemoError("A instalacao nova ja possui turno aberto; preserve para conferencia.")
        return rows
    finally:
        api.request("/logout", token, method="POST")


def read_public_context(directory):
    # The only database read is from this newly created test directory, read-only.
    station = json.loads((directory / "station.json").read_text())
    database = sqlite3.connect((directory / "store.sqlite").as_uri() + "?mode=ro", uri=True)
    try:
        owners = database.execute("SELECT identity_id FROM memberships WHERE tenant_id=? AND role='owner' AND status='active'",
                                  (station["tenant_id"],)).fetchall()
    finally:
        database.close()
    if len(owners) != 1:
        raise DemoError("A instalacao nova nao possui um unico dono ativo.")
    # Do not propagate the station private key into metadata or API requests.
    return {key: station[key] for key in ("tenant_id", "store_id", "device_id")}, owners[0][0]


def run(repo, api_port, web_port, profile="varejo"):
    module = {"varejo": "pos", "producao": "production", "rh": "staff", "contabilidade": "accounting"}[profile]
    check_environment(repo, api_port, web_port)
    if not sys.stdin.isatty():
        raise DemoError("Execute em um terminal para escolher a senha sem exibi-la.")
    password = getpass.getpass("Crie a senha DE TESTE (12 a 72 bytes; nao aparece ao digitar): ")
    if not 12 <= len(password.encode()) <= 72 or password.strip() != password or "\n" in password or "\r" in password:
        raise DemoError("Senha de teste invalida: use de 12 a 72 bytes, sem espacos nas extremidades.")
    if password != getpass.getpass("Repita a senha de teste: "):
        raise DemoError("Senhas diferentes. Nenhuma instalacao foi criada.")
    parent = Path.home() / ".local/share/titansystem-tests"
    parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    directory = Path(tempfile.mkdtemp(prefix="pdv-", dir=parent))
    os.chmod(directory, 0o700)
    bin_dir = directory / "bin"
    bin_dir.mkdir(mode=0o700)
    issuer_dir = directory / "issuer-test-only"
    issuer_dir.mkdir(mode=0o700)
    api_process = web_process = None
    api_log = web_log = None
    print(f"Instalacao exclusivamente de teste: {directory}", flush=True)
    try:
        print("Compilando os executaveis da sua copia atual...", flush=True)
        for name in ("titan-local", "titan-license"):
            command(["go", "build", "-o", bin_dir / name, f"./cmd/{name}"], repo / "backend")
        local = bin_dir / "titan-local"
        license_cli = bin_dir / "titan-license"
        database, station_path = directory / "store.sqlite", directory / "station.json"
        command([local, "init", "--db", database, "--station", station_path, "--empresa", "EMPRESA DE TESTE",
                 "--loja", "LOJA DE TESTE", "--dono", "OPERADOR DE TESTE"], repo, password + "\n")
        station, owner = read_public_context(directory)
        private, public, contract_path = issuer_dir / "private.json", directory / "issuer-public.json", directory / "contract.json"
        command([license_cli, "init", "--private-key", private, "--key-id", "issuer-demo-only"], repo)
        command([license_cli, "public", "--private-key", private, "--out", public], repo)
        expiration = (datetime.now(timezone.utc) + timedelta(days=2)).strftime("%Y-%m-%dT%H:%M:%SZ")
        command([license_cli, "sign", "--private-key", private, "--out", contract_path, "--tenant", station["tenant_id"],
                 "--revision", "1", "--expires", expiration, "--modules", module], repo)
        api_log = (directory / "api.log").open("x")
        api_process = subprocess.Popen([str(local), "serve", "--db", str(database), "--station", str(station_path),
                                       "--issuer-keys", str(public), "--port", str(api_port)], stdout=api_log, stderr=api_log)
        api = API(api_port)
        wait_ready(api, api_process)
        print("Conferindo identidade, instalando contrato de teste e cadastrando estoque pelas APIs reais...", flush=True)
        rows = seed(api, owner, password, station, json.loads(contract_path.read_text()), profile)
        password = ""
        private_file(directory / "demo.json", {**station, "owner_id": owner, "api_port": api_port, "web_port": web_port,
                                              "expires": expiration, "products": rows, "profile": profile})
        web_log = (directory / "web.log").open("x")
        web_process = subprocess.Popen(["node", "scripts/demo-vite.mjs", str(web_port), str(api_port)], cwd=repo / "frontend-web",
                                       stdout=web_log, stderr=web_log)
        wait_ready(API(web_port), web_process)
        print(f"\nPRONTO — abra http://127.0.0.1:{web_port}/local/login", flush=True)
        print(f"ID do operador DE TESTE: {owner}", flush=True)
        print("Senha: a que voce acabou de escolher; ela nao foi salva em texto nem impressa.", flush=True)
        print(f"Perfil da instalacao DE TESTE: {profile}. O menu usa apenas os modulos desse contrato.", flush=True)
        if profile == "varejo":
            print("Abra o PDV, informe fundo de 100,00 e selecione Gondola de teste.", flush=True)
            print("Adicione 1 Cafe (8,99) e 0,500 kg de Arroz (5,00). Total: 13,99.", flush=True)
            print("Dinheiro recebido: 20,00. Troco: 6,01. Conclua a venda.", flush=True)
            print("Para fechar sem outras operacoes, declare 113,99. Diferenca esperada: zero.", flush=True)
        print("Mantenha este terminal aberto. Ctrl+C encerra SOMENTE os dois processos criados aqui.", flush=True)
        while api_process.poll() is None and web_process.poll() is None:
            time.sleep(0.5)
        raise DemoError("Um dos processos de demonstracao terminou. Os dados de teste foram preservados.")
    finally:
        password = ""
        stop_owned(web_process)
        stop_owned(api_process)
        if web_log:
            web_log.close()
        if api_log:
            api_log.close()
        print(f"Dados de teste preservados em {directory}", flush=True)


def main():
    options = argparse.ArgumentParser(description="Demonstra o PDV em instalacao nova e isolada.")
    options.add_argument("--api-port", type=int, default=8182)
    options.add_argument("--web-port", type=int, default=3001)
    options.add_argument("--profile", choices=["varejo", "producao", "rh", "contabilidade"], default="varejo")
    options.add_argument("--check", action="store_true", help="verifica ferramentas e portas; nao cria arquivos")
    args = options.parse_args()
    repo = Path(__file__).resolve().parent.parent
    os.umask(0o077)
    try:
        if args.check:
            check_environment(repo, args.api_port, args.web_port)
            print("Ferramentas, dependencias e portas disponiveis. Nenhum banco foi alterado.")
        else:
            run(repo, args.api_port, args.web_port, args.profile)
    except KeyboardInterrupt:
        print("\nDemonstracao encerrada.")
    except (DemoError, OSError, KeyError, ValueError, subprocess.SubprocessError):
        error = sys.exc_info()[1]
        print(f"Pare: {error}" if isinstance(error, DemoError) else "Pare: falha de preparacao. Preserve a instalacao para conferencia.", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
