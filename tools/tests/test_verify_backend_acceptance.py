import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location(
    'acceptance_gate', Path(__file__).resolve().parents[1] / 'verify_backend_acceptance.py')
gate = importlib.util.module_from_spec(spec)
spec.loader.exec_module(gate)


class AcceptanceGateTests(unittest.TestCase):
    def report(self, events):
        directory = tempfile.TemporaryDirectory(prefix='titan-acceptance-gate-')
        self.addCleanup(directory.cleanup)
        path = Path(directory.name) / 'report.jsonl'
        path.write_text(''.join(json.dumps(event) + '\n' for event in events), encoding='utf-8')
        return path

    def success(self):
        return ([{'Action': 'pass', 'Package': package, 'Test': test}
                 for package, test in gate.REQUIRED]
                + [{'Action': 'pass', 'Package': package}
                   for package, _ in gate.REQUIRED])

    def test_complete_report_is_accepted(self):
        result = gate.verify(self.report(self.success()))
        self.assertEqual(result['required_integrations'], 25)
        self.assertEqual(result['status'], 'PASS')

    def test_every_required_test_must_pass(self):
        for pair in gate.REQUIRED:
            with self.subTest(test=pair), self.assertRaises(ValueError):
                gate.verify(self.report([e for e in self.success()
                    if (e.get('Package'), e.get('Test')) != pair]))

    def test_test_pass_without_package_pass_is_rejected(self):
        with self.assertRaises(ValueError):
            gate.verify(self.report([e for e in self.success() if e.get('Test')]))

    def test_required_skip_is_rejected(self):
        events = self.success()
        events[0]['Action'] = 'skip'
        with self.assertRaises(ValueError):
            gate.verify(self.report(events))

    def test_unrelated_failure_or_unexpected_skip_is_rejected(self):
        for action in ('fail', 'skip'):
            with self.subTest(action=action), self.assertRaises(ValueError):
                gate.verify(self.report(self.success() + [
                    {'Action': action, 'Package': 'other', 'Test': 'OtherTest'}]))

    def test_explicit_auxiliaries_may_skip(self):
        events = self.success() + [{'Action': 'skip', 'Package': p, 'Test': t}
                                  for p, t in gate.ALLOWED_SKIPS]
        self.assertEqual(gate.verify(self.report(events))['skipped'], 3)

    def test_real_focus_opt_in_skip_is_allowed_but_failure_is_rejected(self):
        package = 'titansystem-backend/internal/fiscalproduction'
        test = 'TestFocusSandboxRealNFCe'
        events = self.success() + [{'Action': 'skip', 'Package': package, 'Test': test}]
        self.assertEqual(gate.verify(self.report(events))['status'], 'PASS')
        events[-1]['Action'] = 'fail'
        with self.assertRaises(ValueError):
            gate.verify(self.report(events))

    def test_focus_exception_does_not_allow_other_skips(self):
        for package, test in [
            ('other', 'TestFocusSandboxRealNFCe'),
            ('titansystem-backend/internal/fiscalproduction', 'TestFocusFaultTimeoutThenConsultationNeverDuplicates'),
        ]:
            with self.subTest(test=test), self.assertRaises(ValueError):
                gate.verify(self.report(self.success() + [
                    {'Action': 'skip', 'Package': package, 'Test': test}]))

    def test_empty_and_malformed_reports_are_rejected(self):
        with self.assertRaises(ValueError):
            gate.verify(self.report([]))
        path = self.report([])
        path.write_text('not JSON\n', encoding='utf-8')
        with self.assertRaises(ValueError):
            gate.verify(path)


if __name__ == '__main__':
    unittest.main()
