import contextlib
import importlib.util
import io
import json
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location('api_contracts', Path(__file__).resolve().parents[1] / 'check_api_contracts.py')
validator = importlib.util.module_from_spec(spec)
spec.loader.exec_module(validator)


class ContractMapTests(unittest.TestCase):
    def fixture(self, operation=None, path_item=None, domain=None):
        directory = tempfile.TemporaryDirectory(prefix='titan-contract-map-')
        self.addCleanup(directory.cleanup)
        root = Path(directory.name)
        (root / 'docs/api').mkdir(parents=True)
        operation = operation or {'operationId': 'readItem', 'responses': {'200': {'description': 'ok'}},
                                  'parameters': [{'name': 'id', 'in': 'path', 'required': True, 'schema': {'type': 'string'}}]}
        paths = {'/local/v1/items/{id}': path_item or {'get': operation}}
        document = {'openapi': '3.0.3', 'paths': paths}
        (root / 'docs/api/routes.openapi.json').write_text(json.dumps(document))
        (root / 'docs/api/route-inventory.json').write_text(json.dumps({'routes': [
            {'method': 'GET', 'path': '/local/v1/items/:id'}]}))
        if domain:
            (root / 'docs/api/domain.openapi.json').write_text(json.dumps(domain))
        return root

    def check(self, root):
        with contextlib.redirect_stdout(io.StringIO()):
            validator.check(root)

    def test_inline_operation(self):
        self.check(self.fixture())

    def test_external_operation_and_reusable_parameter(self):
        operation = {'operationId': 'domainRead', 'responses': {'200': {'description': 'ok'}},
                     'parameters': [{'$ref': '#/components/parameters/Id'}]}
        domain = {'openapi': '3.0.3', 'paths': {'/items/{id}': {'get': operation}},
                  'components': {'parameters': {'Id': {'name': 'id', 'in': 'path', 'required': True, 'schema': {'type': 'string'}}}}}
        self.check(self.fixture(operation={'$ref': './domain.openapi.json#/paths/~1items~1{id}/get'}, domain=domain))

    def test_path_item_inheritance(self):
        self.check(self.fixture(path_item={'parameters': [{'name': 'id', 'in': 'path', 'required': True, 'schema': {'type': 'string'}}],
            'get': {'operationId': 'inheritedRead', 'responses': {'200': {'description': 'ok'}}}}))

    def test_reusable_path_item_with_optional_operation_id(self):
        domain = {'openapi': '3.0.3', 'paths': {'/items/{id}': {
            'parameters': [{'name': 'id', 'in': 'path', 'required': True}],
            'get': {'responses': {'200': {'description': 'ok'}}}}}}
        self.check(self.fixture(path_item={'$ref': './domain.openapi.json#/paths/~1items~1{id}'}, domain=domain))

    def test_duplicate_operation_ids_are_rejected(self):
        operation = {'operationId': 'same', 'responses': {'200': {'description': 'ok'}},
                     'parameters': [{'name': 'id', 'in': 'path', 'required': True}]}
        with self.assertRaisesRegex(ValueError, 'invalid operation'):
            self.check(self.fixture(path_item={'get': operation, 'post': operation}))

    def test_missing_or_optional_path_parameter_is_rejected(self):
        for required in (False, 'true'):
            with self.subTest(required=required), self.assertRaises(ValueError):
                self.check(self.fixture(operation={'operationId': 'badRead', 'responses': {'200': {'description': 'ok'}},
                    'parameters': [{'name': 'id', 'in': 'path', 'required': required}]}))

    def test_operation_override_cannot_hide_required_parameter(self):
        with self.assertRaises(ValueError):
            self.check(self.fixture(path_item={'parameters': [{'name': 'id', 'in': 'path', 'required': True}],
                'get': {'operationId': 'override', 'responses': {'200': {'description': 'ok'}},
                        'parameters': [{'name': 'id', 'in': 'path', 'required': False}]}}))

    def test_missing_operation_reference_is_rejected(self):
        with self.assertRaises(ValueError):
            self.check(self.fixture(operation={'$ref': './missing.openapi.json#/paths/anything'}))

    def test_cyclic_reference_is_rejected(self):
        domain = {'openapi': '3.0.3', 'paths': {}, 'components': {'loop': {'$ref': '#/components/loop'}}}
        with self.assertRaisesRegex(ValueError, 'cyclic'):
            self.check(self.fixture(operation={'$ref': './domain.openapi.json#/components/loop'}, domain=domain))

    def test_missing_inventory_route_is_rejected(self):
        root = self.fixture()
        (root / 'docs/api/route-inventory.json').write_text(json.dumps({'routes': []}))
        with self.assertRaisesRegex(ValueError, 'route map'):
            self.check(root)


if __name__ == '__main__':
    unittest.main()
