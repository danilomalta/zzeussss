"""Publish structural test evidence only; never copy test stdout or env values."""
import argparse
import hashlib
import json
from pathlib import Path
import re


def collect(source, destination, commit):
    if not re.fullmatch(r'[0-9a-f]{40}', commit):
        raise ValueError('Exact commit SHA required')
    destination.mkdir(parents=True, exist_ok=True)
    artifacts = []
    for path in sorted(source.glob('*.jsonl')):
        events = []
        for line in path.read_text(encoding='utf-8').splitlines():
            event = json.loads(line)
            # Output and subtest names can contain customer data or credentials.
            # Keep package/top-level test names and numeric timing, not raw logs.
            item = {key: event[key] for key in ('Action', 'Package', 'Elapsed') if key in event}
            if 'Test' in event:
                item['Test'] = event['Test'].split('/')[0]
            events.append(item)
        target = destination / path.name
        target.write_text(''.join(json.dumps(e) + '\n' for e in events), encoding='utf-8')
        artifacts.append({'name': target.name, 'sha256': hashlib.sha256(target.read_bytes()).hexdigest(), 'events': len(events)})
    (destination / 'manifest.json').write_text(json.dumps({'commit': commit, 'raw_output_included': False, 'artifacts': artifacts}, indent=2) + '\n', encoding='utf-8')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('source', type=Path)
    parser.add_argument('destination', type=Path)
    parser.add_argument('commit')
    args = parser.parse_args()
    collect(args.source, args.destination, args.commit)
