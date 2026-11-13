"""Print only fixed, non-business diagnostic labels for installed acceptance."""
import json
import re
import sys

TESTS = {'TestPilotInstalledCashPartialReceivingRestore', 'TestPilotInstalledCashClosingCustodyRestore'}
FIXED = (
    'SQLite existente deve ser arquivo regular privado (0600)', 'banco não é arquivo regular 0600',
    'arquivo privado não é regular 0600',
    'explicit absolute installed binaries/controller required', 'installed acceptance input unavailable',
    'installed server spawn failed', 'installed server readiness failed', 'installed server exit failed',
    'installed server drain failed', 'lost final response was not exercised',
    'recovery repeated fiscal/commercial finalization POST', 'receiver observation login unavailable',
    'receiver observation login refused', 'original drawer missing', 'fixture station export failed',
    'fixture station restore failed', 'restored login unavailable', 'invalid authentication retry policy',
    'restored login refused after bounded wait', 'invalid restored authentication response',
    'installed workflow changed after CLI restore', 'installed outcome mismatch',
)

def diagnostics(path):
    with open(path, encoding='utf-8') as source:
      for line in source:
          event = json.loads(line)
          name = event.get('Test', '').split('/')[0]
          if name not in TESTS:
              continue
          if event.get('Action') in ('pass', 'skip', 'fail'):
              print(json.dumps({'test': name, 'action': event['Action'], 'elapsed': event.get('Elapsed')}))
          output = event.get('Output', '')
          for label in FIXED:
              if label in output:
                  print(json.dumps({'test': name, 'diagnostic': label}))
          phase = re.search(r'Screen (?:failure )?phase: ([\w -]{1,80})', output)
          if phase:
              value = phase[1].split(' detail')[0]
              # Stage labels are controller actions, never field values.
              print(json.dumps({'test': name, 'phase': value}, ensure_ascii=True))
          detail = re.search(r'detail: (screen missing: [a-z -]+|desktop [a-z_0-9-]+|assertion or inspection failed|application page unavailable|inspection timeout|screen action failed|disposable application did not close gracefully)', output)
          if detail:
              print(json.dumps({'test': name, 'diagnostic': detail[1]}))

if __name__ == '__main__':
    diagnostics(sys.argv[1])
