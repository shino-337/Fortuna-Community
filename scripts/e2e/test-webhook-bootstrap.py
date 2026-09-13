#!/usr/bin/env python3
"""Exercise CA/hostname validation with real OpenSSL and a fake Kubernetes API."""
import base64
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]


class WebhookBootstrapTest(unittest.TestCase):
    def run_case(self, hostname):
        with tempfile.TemporaryDirectory() as folder:
            tmp = Path(folder)
            cert = tmp / 'cert.pem'
            subprocess.run(['openssl', 'req', '-x509', '-newkey', 'rsa:2048', '-nodes',
                            '-days', '1', '-subj', '/CN=webhook-test',
                            '-addext', f'subjectAltName=DNS:{hostname}',
                            '-keyout', str(tmp / 'key.pem'), '-out', str(cert)],
                           check=True, capture_output=True)
            fake = tmp / 'kubectl'
            fake.write_text('''#!/usr/bin/env python3
import json, os, sys
args = sys.argv[1:]
with open(os.environ['CALL_LOG'], 'a') as f:
    f.write(json.dumps(args) + '\\n')
if 'get' in args and 'secret' in args:
    print(os.environ['TEST_CERT'])
elif 'patch' in args:
    patch = json.loads(args[args.index('-p') + 1])
    assert patch[0]['path'] == '/webhooks/0/clientConfig/caBundle'
    assert patch[0]['value'] == os.environ['TEST_CERT']
    print('apiVersion: admissionregistration.k8s.io/v1')
elif 'apply' in args:
    if args[-1] == '-':
        assert sys.stdin.read().strip()
else:
    sys.exit(2)
''')
            fake.chmod(0o755)
            log = tmp / 'calls'
            env = dict(os.environ, PATH=str(tmp) + os.pathsep + os.environ['PATH'],
                       CALL_LOG=str(log), TEST_CERT=base64.b64encode(cert.read_bytes()).decode(),
                       NAMESPACE='fortuna')
            result = subprocess.run(['bash', str(ROOT / 'scripts/deploy/enable-webhook.sh')],
                                    env=env, capture_output=True, text=True)
            calls = [json.loads(line) for line in log.read_text().splitlines()]
            return result, calls

    def test_valid_certificate_applies_service_and_ca(self):
        result, calls = self.run_case('fortuna-webhook.fortuna.svc')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(sum('apply' in call for call in calls), 2)
        self.assertEqual(sum('patch' in call for call in calls), 1)

    def test_old_core_only_certificate_does_not_mutate_cluster(self):
        result, calls = self.run_case('fortuna-core.fortuna.svc.cluster.local')
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse(any('apply' in call or 'patch' in call for call in calls))


if __name__ == '__main__':
    unittest.main()
