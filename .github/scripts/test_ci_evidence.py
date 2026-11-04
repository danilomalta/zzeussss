import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location('ci_evidence', Path(__file__).with_name('ci_evidence.py'))
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


class EvidenceTest(unittest.TestCase):
    def test_excludes_output_subtest_credentials_and_unrelated_files(self):
        with tempfile.TemporaryDirectory() as root:
            source = Path(root) / 'source'; source.mkdir()
            destination = Path(root) / 'safe'
            secret = 'synthetic-private-value'
            (source / 'backend.jsonl').write_text(json.dumps({'Action': 'fail', 'Package': 'module/pkg', 'Test': 'TestBoundary/' + secret, 'Output': secret, 'Elapsed': 2.0}) + '\n')
            (source / 'private.log').write_text(secret)
            module.collect(source, destination, 'a' * 40)
            contents = ''.join(p.read_text() for p in destination.iterdir())
            self.assertNotIn(secret, contents)
            self.assertIn('fail', contents)
            self.assertIn('TestBoundary', contents)
            self.assertEqual(len(list(destination.iterdir())), 2)
            self.assertEqual(json.loads((destination / 'manifest.json').read_text())['commit'], 'a' * 40)

    def test_requires_exact_commit(self):
        with self.assertRaises(ValueError):
            module.collect(Path('/unused'), Path('/unused'), 'main')

    def test_invalid_json_is_not_silently_accepted(self):
        with tempfile.TemporaryDirectory() as root:
            source = Path(root) / 'source'; source.mkdir()
            (source / 'bad.jsonl').write_text('invalid')
            with self.assertRaises(ValueError):
                module.collect(source, Path(root) / 'safe', 'a' * 40)
