#!/usr/bin/env python3
"""Managed Linux backend releases. No customer data is sent over the network."""
import argparse
import contextlib
import fcntl
import hashlib
import json
import os
from pathlib import Path
import platform
import re
import stat
import struct
import subprocess
import sys
import tempfile
import uuid

BINARIES = ("titan-local", "titan-sync", "titan-receive", "titan-peer", "titan-access", "titan-backup")
MAX_BINARY = 128 * 1024 * 1024
VERSION = re.compile(r"[a-zA-Z0-9][a-zA-Z0-9._-]{0,63}\Z")


class InstallationError(Exception):
    pass


def fail(message):
    raise InstallationError(message)


def path_checked(value):
    path = Path(os.path.abspath(value))
    # Do not resolve symlinks away before inspecting them.
    for part in (path, *path.parents):
        try:
            if stat.S_ISLNK(part.lstat().st_mode):
                fail("Caminho com link simbólico recusado.")
        except FileNotFoundError:
            pass
    return path


def directory(path):
    info = path_checked(path).lstat()
    if not stat.S_ISDIR(info.st_mode) or stat.S_IMODE(info.st_mode) != 0o700 or info.st_uid != os.getuid():
        fail("Diretório gerenciado precisa ser privado 0700 e pertencer ao operador.")


def read_file(path, mode=0o600, limit=65536):
    path = path_checked(path)
    info = path.lstat()
    if not stat.S_ISREG(info.st_mode) or stat.S_IMODE(info.st_mode) != mode or info.st_uid != os.getuid() or info.st_size > limit:
        fail("Arquivo ausente, inseguro ou excede o limite.")
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
    with os.fdopen(fd, "rb") as stream:
        opened = os.fstat(stream.fileno())
        if (opened.st_dev, opened.st_ino) != (info.st_dev, info.st_ino):
            fail("Arquivo mudou durante a leitura.")
        body = stream.read(limit + 1)
    if len(body) > limit:
        fail("Arquivo excede o limite.")
    return body


def sync_dir(path):
    fd = os.open(path, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
    try:
        os.fsync(fd)
    finally:
        os.close(fd)


def write_new(path, body, mode=0o600):
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, mode)
    with os.fdopen(fd, "wb") as stream:
        stream.write(body)
        stream.flush()
        os.fsync(stream.fileno())
    sync_dir(path.parent)


def atomic_json(path, value):
    if path.exists():
        read_file(path)
    temporary = path.with_name(".state-" + uuid.uuid4().hex)
    write_new(temporary, json.dumps(value, sort_keys=True, separators=(",", ":")).encode())
    os.replace(temporary, path)
    sync_dir(path.parent)


def pairs_unique(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            fail("JSON ambíguo recusado.")
        result[key] = value
    return result


def json_file(path):
    try:
        return json.loads(read_file(path), object_pairs_hook=pairs_unique)
    except (ValueError, TypeError):
        fail("Estado ou manifesto inválido.")


def arch():
    if sys.platform != "linux" or platform.machine() not in ("x86_64", "aarch64"):
        fail("Esta entrega suporta Linux amd64/arm64.")
    return {"x86_64": "amd64", "aarch64": "arm64"}[platform.machine()]


def binary_bytes(path):
    body = read_file(path, 0o700, MAX_BINARY)
    machine = {"amd64": 62, "arm64": 183}[arch()]
    if len(body) < 64 or body[:6] != b"\x7fELF\x02\x01" or struct.unpack_from("<H", body, 18)[0] != machine:
        fail("Executável ELF da arquitetura local obrigatório.")
    return body


def validate_manifest(value):
    if not isinstance(value, dict) or set(value) != {"format", "version", "revision", "os", "arch", "files"}:
        fail("Manifesto inválido.")
    if value["format"] != 1 or type(value["format"]) is not int or value["os"] != "linux" or value["arch"] != arch():
        fail("Formato ou arquitetura incompatível.")
    if not isinstance(value["version"], str) or not VERSION.fullmatch(value["version"]) or type(value["revision"]) is not int or not 1 <= value["revision"] <= 2**31 - 1:
        fail("Versão/revisão inválida.")
    if not isinstance(value["files"], dict) or set(value["files"]) != set(BINARIES):
        fail("Pacote deve conter exatamente os seis executáveis autorizados.")
    for digest in value["files"].values():
        if not isinstance(digest, str) or not re.fullmatch(r"[0-9a-f]{64}", digest):
            fail("Hash inválido.")
    return value


def verify_bundle(path):
    directory(path)
    manifest = validate_manifest(json_file(path / "manifest.json"))
    if {p.name for p in path.iterdir()} != {*BINARIES, "manifest.json"}:
        fail("Pacote contém arquivos adicionais.")
    for name in BINARIES:
        if hashlib.sha256(binary_bytes(path / name)).hexdigest() != manifest["files"][name]:
            fail("Pacote alterado; nenhum executável foi iniciado.")
    return manifest


def pack(source, destination, version, revision):
    source, destination = path_checked(source), path_checked(destination)
    value = {"format": 1, "version": version, "revision": revision, "os": "linux", "arch": arch(), "files": {n: "0" * 64 for n in BINARIES}}
    validate_manifest(value)
    # Only the named executables are copied, never a source tree or private file.
    contents = {name: binary_bytes(source / name) for name in BINARIES}
    value["files"] = {name: hashlib.sha256(body).hexdigest() for name, body in contents.items()}
    destination.mkdir(mode=0o700)
    for name, body in contents.items():
        write_new(destination / name, body, 0o700)
    write_new(destination / "manifest.json", json.dumps(value, sort_keys=True).encode())
    sync_dir(destination.parent)
    verify_bundle(destination)


class Manager:
    def __init__(self, root):
        self.root = path_checked(root)
        self.lock_fd = None

    @property
    def db(self):
        return self.root / "data" / "store.sqlite"

    @property
    def station(self):
        return self.root / "data" / "station.json"

    @property
    def key(self):
        return self.root / "private" / "backup.key"

    def layout(self):
        directory(self.root)
        for name in ("releases", "data", "private", "backups", "checks"):
            directory(self.root / name)

    @contextlib.contextmanager
    def lock(self, exclusive=True):
        self.layout()
        lock = self.root / "process.lock"
        if not lock.exists():
            try:
                write_new(lock, b"")
            except FileExistsError:
                pass
        read_file(lock)
        fd = os.open(lock, os.O_RDWR | os.O_NOFOLLOW)
        try:
            try:
                fcntl.flock(fd, (fcntl.LOCK_EX if exclusive else fcntl.LOCK_SH) | fcntl.LOCK_NB)
            except BlockingIOError:
                fail("Instalação em uso. Encerre os processos gerenciados antes da manutenção.")
            self.lock_fd = fd
            yield
        finally:
            self.lock_fd = None
            os.close(fd)

    def release(self, version):
        if not isinstance(version, str) or not VERSION.fullmatch(version):
            fail("Referência de versão inválida.")
        path = self.root / "releases" / version
        value = verify_bundle(path)
        if value["version"] != version:
            fail("Referência de versão divergente.")
        return path, value

    def active(self):
        value = json_file(self.root / "active.json")
        if not isinstance(value, dict) or set(value) != {"version", "enabled"} or type(value["enabled"]) is not bool:
            fail("Estado ativo inválido.")
        self.release(value["version"])
        return value

    def stage(self, bundle):
        bundle = path_checked(bundle)
        value = verify_bundle(bundle)
        target = self.root / "releases" / value["version"]
        target.mkdir(mode=0o700)  # Never reuse or overwrite an existing release.
        for name in BINARIES:
            body = binary_bytes(bundle / name)
            if hashlib.sha256(body).hexdigest() != value["files"][name]:
                fail("Pacote mudou durante a cópia.")
            write_new(target / name, body, 0o700)
        write_new(target / "manifest.json", json.dumps(value, sort_keys=True).encode())
        sync_dir(target.parent)
        verify_bundle(target)
        return value

    def invoke(self, version, executable, args, quiet=True, timeout=600, stdin=None):
        path, _ = self.release(version)
        # Children inherit the lock. Killing this Python process cannot release
        # the installation lock while its child is still using the database.
        options = {"pass_fds": (self.lock_fd,), "timeout": timeout, "stdin": stdin}
        if quiet:
            options.update(stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        result = subprocess.run([str(path / executable), *map(str, args)], **options)
        if result.returncode:
            fail("Comando não concluído. Dados preservados; investigue antes de retomar.")

    def checkpoint(self, phase):
        # Tests override this hook to terminate a subprocess at durable borders.
        pass

    def install(self, bundle):
        verify_bundle(path_checked(bundle))
        self.root.mkdir(mode=0o700)
        sync_dir(self.root.parent)
        for name in ("releases", "data", "private", "backups", "checks"):
            (self.root / name).mkdir(mode=0o700)
        sync_dir(self.root)
        with self.lock():
            value = self.stage(bundle)
            self.invoke(value["version"], "titan-backup", ["key-init", "--key", self.key])
            atomic_json(self.root / "active.json", {"version": value["version"], "enabled": True})

    def ready(self):
        if (self.root / "pending.json").exists():
            fail("Atualização interrompida. Execute recover antes de iniciar qualquer comando.")
        value = self.active()
        if not value["enabled"]:
            fail("Instalação desativada; dados preservados. Use enable para reativar.")
        return value

    def scope(self):
        value = json_file(self.station)
        fields = {"tenant_id", "store_id", "device_id", "private_key"}
        if not isinstance(value, dict) or set(value) != fields:
            fail("Aparelho inválido.")
        for name in fields:
            if not isinstance(value[name], str) or not value[name] or len(value[name]) > 128 or value[name].strip() != value[name] or "\x00" in value[name]:
                fail("Aparelho inválido.")
        # Private key is never returned, printed or included in state/manifest.
        return ["--tenant", value["tenant_id"], "--store", value["store_id"], "--device", value["device_id"]]

    def update(self, bundle):
        with self.lock():
            current = self.ready()
            _, old = self.release(current["version"])
            candidate = verify_bundle(path_checked(bundle))
            if candidate["revision"] <= old["revision"]:
                fail("Downgrade ou revisão repetida recusados.")
            read_file(self.db, limit=128 * 1024 * 1024)
            read_file(self.key, limit=32)
            candidate = self.stage(bundle)
            backup_name = "before-" + uuid.uuid4().hex + ".tytbak"
            archive = self.root / "backups" / backup_name
            self.invoke(old["version"], "titan-backup", ["create", "--db", self.db, "--station", self.station, "--key", self.key, "--out", archive])
            # Old executable verifies/restores its own schema. Candidate only
            # opens the disposable copy at this stage, never the customer DB.
            with tempfile.TemporaryDirectory(prefix="probe-", dir=self.root / "checks") as temporary:
                probe = Path(temporary) / "store.sqlite"
                self.invoke(old["version"], "titan-backup", ["restore", "--archive", archive, "--key", self.key, *self.scope(), "--out", probe])
                self.invoke(candidate["version"], "titan-local", ["check", "--db", probe, "--station", self.station])
            pending = {"from": old["version"], "to": candidate["version"], "backup": backup_name, "enabled": current["enabled"]}
            atomic_json(self.root / "pending.json", pending)
            self.checkpoint("prepared")
            self.finish(pending)

    def finish(self, pending):
        if not isinstance(pending, dict) or set(pending) != {"from", "to", "backup", "enabled"} or type(pending["enabled"]) is not bool:
            fail("Registro de atualização inválido.")
        if not isinstance(pending["backup"], str) or not re.fullmatch(r"before-[0-9a-f]{32}\.tytbak", pending["backup"]):
            fail("Referência de backup inválida.")
        _, old = self.release(pending["from"])
        _, new = self.release(pending["to"])
        current = self.active()
        if new["revision"] <= old["revision"] or current["version"] not in (old["version"], new["version"]):
            fail("Transição de versão inválida.")
        archive = self.root / "backups" / pending["backup"]
        self.invoke(old["version"], "titan-backup", ["verify", "--archive", archive, "--key", self.key, *self.scope()])
        # Forward recovery only: migrations already committed are accepted by
        # the candidate. Never restart an older binary against a newer schema.
        self.invoke(new["version"], "titan-local", ["check", "--db", self.db, "--station", self.station])
        self.checkpoint("migrated")
        atomic_json(self.root / "active.json", {"version": new["version"], "enabled": pending["enabled"]})
        self.checkpoint("activated")
        (self.root / "pending.json").unlink()
        sync_dir(self.root)

    def recover(self):
        with self.lock():
            self.finish(json_file(self.root / "pending.json"))

    def set_enabled(self, enabled):
        with self.lock():
            if (self.root / "pending.json").exists():
                fail("Recupere a atualização antes de mudar o estado.")
            current = self.active()
            atomic_json(self.root / "active.json", {"version": current["version"], "enabled": enabled})

    def run(self, args):
        allowed = {
            "titan-local": {"init", "serve", "check"}, "titan-sync": {"send"},
            "titan-receive": {"serve"}, "titan-backup": {"create", "watch"},
            "titan-access": {"recovery-init", "recover"},
            "titan-peer": {"key-init", "export", "trust", "approve", "revoke", "pair-start", "pair-finish"},
        }
        if len(args) < 2 or args[0] not in allowed or args[1] not in allowed[args[0]]:
            fail("Comando não autorizado pelo gerenciador; confira a documentação.")
        # Prevent overriding managed paths, including Go's single-dash syntax.
        for arg in args[2:]:
            reserved = {"db", "station"} | ({"key", "out"} if args[0] == "titan-backup" else set())
            if arg.split("=", 1)[0].lstrip("-") in reserved or arg == "--":
                fail("Banco/aparelho são definidos pela instalação, não pelos argumentos.")
        with self.lock(exclusive=args[1] in ("init", "check")):
            current = self.ready()
            extra = []
            if args[0] == "titan-backup":
                destination = self.root / "backups"
                if args[1] == "create":
                    destination = destination / ("manual-" + uuid.uuid4().hex + ".tytbak")
                extra = ["--key", self.key, "--out", destination]
            self.invoke(current["version"], args[0], [args[1], "--db", self.db, "--station", self.station, *extra, *args[2:]], quiet=False, timeout=None)

    def status(self):
        with self.lock(exclusive=False):
            current = self.active()
            return {"version": current["version"], "enabled": current["enabled"], "update_pending": (self.root / "pending.json").exists(), "initialized": self.db.exists() and self.station.exists()}


def main(argv=None):
    parser = argparse.ArgumentParser(description="Gerenciador local Linux. Não usar no banco da demonstração.")
    sub = parser.add_subparsers(dest="command", required=True)
    packing = sub.add_parser("pack", help="pacote local; hashes não autenticam a origem")
    packing.add_argument("--source", required=True)
    packing.add_argument("--out", required=True)
    packing.add_argument("--version", required=True)
    packing.add_argument("--revision", required=True, type=int)
    for name in ("install", "update", "recover", "status", "disable", "enable", "run"):
        p = sub.add_parser(name)
        p.add_argument("--root", required=True)
        if name in ("install", "update"):
            p.add_argument("--bundle", required=True)
        if name == "run":
            p.add_argument("args", nargs=argparse.REMAINDER)
    options = parser.parse_args(argv)
    try:
        arch()
        if options.command == "pack":
            pack(options.source, options.out, options.version, options.revision)
        else:
            manager = Manager(options.root)
            if options.command in ("install", "update"):
                getattr(manager, options.command)(options.bundle)
            elif options.command in ("disable", "enable"):
                manager.set_enabled(options.command == "enable")
            elif options.command == "run":
                manager.run(options.args)
            elif options.command == "status":
                print(json.dumps(manager.status(), ensure_ascii=False))
                return 0
            else:
                manager.recover()
        print("Operação concluída. Dados e chaves existentes preservados.")
        return 0
    except (InstallationError, OSError, subprocess.SubprocessError):
        # Never print subprocess output, secret-bearing JSON or command args.
        message = sys.exc_info()[1]
        print("Pare: " + (str(message) if isinstance(message, InstallationError) else "Operação não concluída; arquivos preservados. Confira versão, permissões e caminhos."), file=sys.stderr)
        return 1
    except KeyboardInterrupt:
        print("Interrompido. Se houver atualização pendente, execute recover.", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
