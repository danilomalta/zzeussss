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
    def resolve(value, name):
        visited = set()
        while isinstance(value, dict) and '$ref' in value:
            reference = value['$ref']
            key = (name, reference)
            if key in visited:
                raise ValueError('cyclic repository reference: ' + name)
            visited.add(key)
            file, separator, pointer = reference.partition('#')
            name = file.removeprefix('./') if file else name
            if not separator or name not in documents:
                raise ValueError('unresolved repository reference: ' + name)
            value = documents[name]
            for part in pointer.lstrip('/').split('/'):
                if part:
                    value = value[part.replace('~1', '/').replace('~0', '~')]
        return value, name

    for path, item in documents['routes.openapi.json']['paths'].items():
        methods, item_document = resolve(item, 'routes.openapi.json')
        for method, raw_operation in methods.items():
            if method not in ('get', 'post', 'put', 'patch', 'delete', 'head', 'options', 'trace'):
                continue
            operation, operation_document = resolve(raw_operation, item_document)
            actual.add((method, path))
            identifier = operation.get('operationId')
            if (identifier is not None and (not isinstance(identifier, str) or not identifier or identifier in identifiers)) or not operation.get('responses'):
                raise ValueError('invalid operation: ' + path)
            if identifier is not None:
                identifiers.add(identifier)
            # OpenAPI allows reusable operation/parameter references and path
            # parameters inherited from the enclosing Path Item Object.
            combined = [resolve(p, item_document)[0] for p in methods.get('parameters', [])]
            combined += [resolve(p, operation_document)[0] for p in operation.get('parameters', [])]
            parameters = {(p.get('in'), p.get('name')): p for p in combined}
            names = {name for (location, name), p in parameters.items()
                     if location == 'path' and p.get('required') is True}
            if names != set(re.findall(r'\{([^}]+)\}', path)):
                raise ValueError('invalid path parameters: ' + path)
    if expected != actual or len(expected) != len(inventory):
        raise ValueError('route map differs from inventory')
    print(f'Contratos JSON, referências e mapa de {len(expected)} rotas conferidos.')


if __name__ == '__main__':
    check(Path(__file__).resolve().parents[1])
