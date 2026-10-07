"""Validate repository-owned JSON documents, references and route map.

This is a consistency check, not a full OpenAPI semantic validator or proof
that payloads and commercial integrations are complete. No network or DB I/O.
"""
import json
from pathlib import Path
import re


def check(root):
    directory = Path(root) / 'docs/api'
    documents = {p.name: json.loads(p.read_text()) for p in directory.glob('*.openapi.json')}
    for name, document in documents.items():
        if document.get('openapi') != '3.0.3' or not isinstance(document.get('paths'), dict):
            raise ValueError('invalid OpenAPI structure: ' + name)
        def visit(value):
            if isinstance(value, list):
                for item in value:
                    visit(item)
            elif isinstance(value, dict):
                if '$ref' in value:
                    file, separator, pointer = value['$ref'].partition('#')
                    target = document if not file else documents.get(file.removeprefix('./'))
                    if not separator or target is None:
                        raise ValueError('unresolved repository reference: ' + name)
                    for part in pointer.lstrip('/').split('/'):
                        if not part:
                            continue
                        part = part.replace('~1', '/').replace('~0', '~')
                        target = target[part]
                for item in value.values():
                    visit(item)
        visit(document)
    inventory = json.loads((directory / 'route-inventory.json').read_text())['routes']
    expected = {(x['method'].lower(), re.sub(r':([a-z_]+)', r'{\1}', x['path'])) for x in inventory}
    actual = set()
    identifiers = set()
    for path, methods in documents['routes.openapi.json']['paths'].items():
        for method, operation in methods.items():
            actual.add((method, path))
            identifier = operation['operationId']
            if identifier in identifiers or not operation.get('responses'):
                raise ValueError('invalid operation: ' + path)
            identifiers.add(identifier)
            parameters = {p['name'] for p in operation['parameters'] if p.get('in') == 'path' and p.get('required')}
            if parameters != set(re.findall(r'\{([^}]+)\}', path)):
                raise ValueError('invalid path parameters: ' + path)
    if expected != actual or len(expected) != len(inventory):
        raise ValueError('route map differs from inventory')
    print(f'Contratos JSON, referências e mapa de {len(expected)} rotas conferidos.')


if __name__ == '__main__':
    check(Path(__file__).resolve().parents[1])
