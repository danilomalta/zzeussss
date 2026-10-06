import contextlib
import hashlib
import http.client
import importlib.util
import io
import json
import multiprocessing
import os
from pathlib import Path
import shutil
import socket
import sqlite3
import struct
import subprocess
import tempfile
import time
import unittest

TOOLS = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location("titan_install", TOOLS / "titan_install.py")
installer = importlib.util.module_from_spec(spec)
spec.loader.exec_module(installer)


def crash_update(root, bundle, phase):
    class Crash(installer.Manager):
        def checkpoint(self, current):
            if current == phase:
                os._exit(73)
    Crash(root).update(bundle)


class PackageTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.base = Path(self.temp.name)
        self.source = self.base / "source"
        self.source.mkdir(mode=0o700)
        body = bytearray(64)
        body[:6] = b"\x7fELF\x02\x01"
        struct.pack_into("<H", body, 18, {"amd64": 62, "arm64": 183}[installer.arch()])
        for name in installer.BINARIES:
            installer.write_new(self.source / name, body, 0o700)

    def bundle(self):
        target = self.base / "bundle"
        installer.pack(self.source, target, "test-1", 1)
        return target

    def test_package_copies_only_whitelist_and_never_overwrites(self):
        installer.write_new(self.source / "secret.key", b"do-not-copy")
        bundle = self.bundle()
        self.assertNotIn("secret.key", [p.name for p in bundle.iterdir()])
        self.assertEqual(installer.verify_bundle(bundle)["revision"], 1)
        with self.assertRaises(FileExistsError):
            installer.pack(self.source, bundle, "test-1", 1)

    def test_tampered_hash_extra_files_and_unsafe_modes_are_rejected(self):
        bundle = self.bundle()
        path = bundle / "titan-local"
        original = path.read_bytes()
        path.write_bytes(original + b"tampered")
        with self.assertRaises(installer.InstallationError): installer.verify_bundle(bundle)
        path.write_bytes(original)
        path.chmod(0o755)
        with self.assertRaises(installer.InstallationError): installer.verify_bundle(bundle)
        path.chmod(0o700)
        installer.write_new(bundle / "extra", b"x")
        with self.assertRaises(installer.InstallationError): installer.verify_bundle(bundle)

    def test_ambiguous_json_and_path_traversal_rejected(self):
        path = self.base / "json"
        installer.write_new(path, b'{"version":"a","version":"b"}')
        with self.assertRaises(installer.InstallationError): installer.json_file(path)
        value = {"format": 1, "version": "../escape", "revision": 1, "os": "linux", "arch": installer.arch(), "files": {n: "0" * 64 for n in installer.BINARIES}}
        with self.assertRaises(installer.InstallationError): installer.validate_manifest(value)
        value["version"] = "good"
        value["revision"] = True
        with self.assertRaises(installer.InstallationError): installer.validate_manifest(value)

    def test_symlink_binary_parent_and_foreign_architecture_rejected(self):
        link = self.base / "alias"
        link.symlink_to(self.source, target_is_directory=True)
        with self.assertRaises(installer.InstallationError): installer.path_checked(link / "titan-local")
        body = bytearray((self.source / "titan-local").read_bytes())
        struct.pack_into("<H", body, 18, 0)
        (self.source / "titan-local").write_bytes(body)
        with self.assertRaises(installer.InstallationError): self.bundle()

    def test_run_rejects_managed_path_overrides_before_opening_database(self):
        manager = installer.Manager(self.base / "missing")
        for args in (["titan-local", "serve", "-db=other"], ["titan-sync", "send", "--station", "other"], ["titan-local", "serve", "--"], ["titan-backup", "restore"]):
            with self.assertRaises(installer.InstallationError): manager.run(args)
        self.assertFalse(manager.root.exists())

    def test_errors_do_not_render_raw_os_errors_or_arguments(self):
        out = io.StringIO()
        with contextlib.redirect_stderr(out):
            result = installer.main(["status", "--root", str(self.base / "secret-token-not-real")])
        self.assertEqual(result, 1)
        self.assertNotIn("secret-token", out.getvalue())


class RealInstallationTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.build_dir = tempfile.TemporaryDirectory()
        cls.addClassCleanup(cls.build_dir.cleanup)
        cls.binaries = Path(os.environ.get("TITAN_INSTALL_BIN_DIR", cls.build_dir.name))
        if not os.environ.get("TITAN_INSTALL_BIN_DIR"):
            go = shutil.which("go")
            if not go:
                raise RuntimeError("Go é obrigatório para testar executáveis reais do instalador")
            env = dict(os.environ, CGO_ENABLED="0")
            for name in installer.BINARIES:
                result = subprocess.run([go, "build", "-buildvcs=false", "-o", str(cls.binaries / name), "./cmd/" + name], cwd=TOOLS.parent / "backend", env=env, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
                if result.returncode:
                    raise RuntimeError("Compilação de executável real falhou")
                (cls.binaries / name).chmod(0o700)

    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.base = Path(self.temp.name)
        self.bundle1 = self.base / "bundle1"
        self.bundle2 = self.base / "bundle2"
        installer.pack(self.binaries, self.bundle1, "test-1", 1)
        installer.pack(self.binaries, self.bundle2, "test-2", 2)
        self.manager = installer.Manager(self.base / "installation")
        self.manager.install(self.bundle1)
        with self.manager.lock():
            with tempfile.TemporaryFile() as password:
                password.write(b"senha-so-para-teste-2026\n")
                password.seek(0)
                self.manager.invoke("test-1", "titan-local", ["init", "--db", self.manager.db, "--station", self.manager.station, "--empresa", "Empresa teste", "--loja", "Loja teste", "--dono", "Dono teste"], stdin=password)
        self.station_bytes = self.manager.station.read_bytes()
        self.key_bytes = self.manager.key.read_bytes()

    def rows(self):
        with sqlite3.connect(self.manager.db) as db:
            return db.execute("SELECT id, name FROM tenants").fetchall()

    def test_actual_install_update_backup_and_check_preserve_station_key_and_company(self):
        before = self.rows()
        self.manager.update(self.bundle2)
        self.assertEqual(self.manager.status()["version"], "test-2")
        self.assertEqual(self.rows(), before)
        self.assertEqual(self.manager.station.read_bytes(), self.station_bytes)
        self.assertEqual(self.manager.key.read_bytes(), self.key_bytes)
        self.assertEqual(len(list((self.manager.root / "backups").glob("*.tytbak"))), 1)
        self.assertFalse((self.manager.root / "pending.json").exists())
        self.assertNotIn(b"private_key", (self.manager.root / "active.json").read_bytes())
        self.manager.run(["titan-local", "check"])

    def test_real_process_exit_recovered_at_all_durable_borders(self):
        # Each border is tested on an independent installation.
        for phase in ("prepared", "migrated", "activated"):
            with self.subTest(phase=phase):
                if phase != "prepared":
                    self.temp.cleanup()
                    self.setUp()
                before = self.rows()
                child = multiprocessing.get_context("fork").Process(target=crash_update, args=(self.manager.root, self.bundle2, phase))
                child.start()
                child.join(30)
                self.assertFalse(child.is_alive())
                self.assertEqual(child.exitcode, 73)
                self.assertTrue(self.manager.status()["update_pending"])
                with self.assertRaises(installer.InstallationError): self.manager.run(["titan-local", "check"])
                self.manager.recover()
                self.assertEqual(self.manager.status()["version"], "test-2")
                self.assertFalse(self.manager.status()["update_pending"])
                self.assertEqual(self.rows(), before)

    def test_active_process_blocks_update_and_deactivation(self):
        with self.manager.lock(exclusive=False):
            with self.assertRaises(installer.InstallationError): installer.Manager(self.manager.root).update(self.bundle2)
            with self.assertRaises(installer.InstallationError): installer.Manager(self.manager.root).set_enabled(False)
        self.assertFalse((self.manager.root / "releases" / "test-2").exists())

    def test_child_keeps_inherited_lock_after_manager_closes_its_descriptor(self):
        # A server inherits the descriptor; closing/killing its manager must
        # not unlock the installation while this child is still alive.
        with self.manager.lock(exclusive=False):
            release, _ = self.manager.release("test-1")
            with socket.socket() as reservation:
                reservation.bind(("127.0.0.1", 0))
                port = reservation.getsockname()[1]
            child = subprocess.Popen([str(release / "titan-local"), "serve", "--db", str(self.manager.db), "--station", str(self.manager.station), "--port", str(port)], pass_fds=(self.manager.lock_fd,), stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        try:
            deadline = time.monotonic() + 5
            while True:
                self.assertIsNone(child.poll())
                connection = http.client.HTTPConnection("127.0.0.1", port, timeout=1)
                try:
                    connection.request("GET", "/local/v1/health")
                    response = connection.getresponse()
                    self.assertEqual(response.status, 200)
                    self.assertEqual(json.loads(response.read()), {"status": "local"})
                    break
                except ConnectionRefusedError:
                    if time.monotonic() >= deadline:
                        raise
                    time.sleep(0.02)
                finally:
                    connection.close()
            with self.assertRaises(installer.InstallationError): self.manager.update(self.bundle2)
        finally:
            child.kill()
            child.wait(timeout=5)
        with self.manager.lock():
            self.assertFalse((self.manager.root / "pending.json").exists())

    def test_failed_candidate_probe_keeps_old_version_and_commercial_data(self):
        class ProbeFailure(installer.Manager):
            def invoke(self, version, executable, args, **kwargs):
                if version == "test-2" and executable == "titan-local":
                    raise installer.InstallationError("candidate failed")
                return super().invoke(version, executable, args, **kwargs)
        before = self.rows()
        with self.assertRaises(installer.InstallationError): ProbeFailure(self.manager.root).update(self.bundle2)
        self.assertEqual(self.manager.status()["version"], "test-1")
        self.assertFalse((self.manager.root / "pending.json").exists())
        self.assertEqual(self.rows(), before)
        self.assertEqual(len(list((self.manager.root / "backups").glob("*.tytbak"))), 1)

    def test_disable_preserves_all_data_and_enable_restores_access(self):
        before = self.rows()
        self.manager.set_enabled(False)
        with self.assertRaises(installer.InstallationError): self.manager.run(["titan-local", "check"])
        self.assertEqual(self.rows(), before)
        self.assertEqual(self.manager.station.read_bytes(), self.station_bytes)
        self.assertEqual(self.manager.key.read_bytes(), self.key_bytes)
        self.manager.set_enabled(True)
        self.manager.run(["titan-local", "check"])

    def test_downgrade_repeated_install_and_revoked_station_do_not_activate(self):
        with self.assertRaises(installer.InstallationError): self.manager.update(self.bundle1)
        with self.assertRaises(FileExistsError): self.manager.install(self.bundle1)
        with sqlite3.connect(self.manager.db) as db:
            db.execute("UPDATE device_pairings SET status='revoked'")
        with self.assertRaises(installer.InstallationError): self.manager.update(self.bundle2)
        self.assertEqual(self.manager.status()["version"], "test-1")
        self.assertFalse((self.manager.root / "pending.json").exists())

    def test_recovery_never_launches_old_version_or_replaces_real_database(self):
        class Stop(installer.Manager):
            def checkpoint(self, phase):
                if phase == "migrated":
                    raise installer.InstallationError("test interruption")
        with self.assertRaises(installer.InstallationError): Stop(self.manager.root).update(self.bundle2)
        self.assertTrue((self.manager.root / "pending.json").exists())
        # Corrupt only the candidate executable. Recovery must stay blocked,
        # rather than falling back to old binaries or overwriting the database.
        executable = self.manager.root / "releases" / "test-2" / "titan-local"
        corrupted = executable.with_name("corrupted")
        installer.write_new(corrupted, executable.read_bytes() + b"corrupt", 0o700)
        os.replace(corrupted, executable)
        before = hashlib.sha256(self.manager.db.read_bytes()).digest()
        with self.assertRaises(installer.InstallationError): self.manager.recover()
        self.assertEqual(hashlib.sha256(self.manager.db.read_bytes()).digest(), before)
        self.assertTrue((self.manager.root / "pending.json").exists())
        with self.assertRaises(installer.InstallationError): self.manager.run(["titan-local", "serve"])


if __name__ == "__main__":
    unittest.main()
