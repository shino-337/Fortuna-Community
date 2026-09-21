#!/usr/bin/env python3
import hashlib
import json
from pathlib import Path
import stat
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
TOOL = ROOT / "scripts/deploy/agent-certificate-tool.py"

class Certificates(unittest.TestCase):
    def run_tool(self, *args, ok=True):
        result = subprocess.run([sys.executable, str(TOOL), *map(str,args)],capture_output=True,text=True)
        self.assertEqual(result.returncode == 0,ok,result.stderr)
        self.assertNotIn("PRIVATE KEY",result.stdout)
        return result

    def test_certificate_isolation_rotation_revocation(self):
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp);ca=root/"ca.crt";key=root/"ca.key"
            subprocess.run(["openssl","req","-x509","-newkey","rsa:2048","-nodes","-keyout",str(key),"-out",str(ca),"-days","10","-subj","/CN=test-ca","-addext","basicConstraints=critical,CA:TRUE"],check=True,capture_output=True)
            common=["--ca-cert",ca,"--ca-key",key,"--ttl-days","1","--node","node-a"]
            one=json.loads(self.run_tool("issue","--cluster-id","a","--output-dir",root/"one",*common).stdout)
            two=json.loads(self.run_tool("issue","--cluster-id","b","--output-dir",root/"two","--existing-registry",one["registry"],*common).stdout)
            three=json.loads(self.run_tool("issue","--cluster-id","a","--output-dir",root/"three","--existing-registry",two["registry"],*common).stdout)
            entries=json.loads(Path(three["registry"]).read_text())["credentials"]
            self.assertEqual([e["cluster_id"] for e in entries],["a","b","a"])
            self.assertEqual(len({e["certificate_sha256"] for e in entries}),3)
            self.assertEqual(len({e["agent_id"] for e in entries}),1)
            for summary,entry in zip([one,two,three],entries):
                directory=Path(summary["issued"][0]["directory"])
                der=subprocess.run(["openssl","x509","-in",str(directory/"tls.crt"),"-outform","DER"],check=True,capture_output=True).stdout
                self.assertEqual(hashlib.sha256(der).hexdigest(),entry["certificate_sha256"])
                self.assertEqual(stat.S_IMODE((directory/"tls.key").stat().st_mode),0o600)
                verify=subprocess.run(["openssl","verify","-purpose","sslclient","-CAfile",str(ca),str(directory/"tls.crt")],capture_output=True)
                self.assertEqual(verify.returncode,0)
                self.assertNotIn("PRIVATE KEY",Path(summary["registry"]).read_text())
            self.run_tool("revoke","--registry",three["registry"],"--credential-id",entries[0]["id"])
            revoked=json.loads(Path(three["registry"]).read_text())["credentials"]
            self.assertEqual([e["revoked"] for e in revoked],[True,False,False])
            self.run_tool("issue","--cluster-id","a","--output-dir",root/"one",*common,ok=False)
            self.run_tool("issue","--cluster-id","a","--output-dir",root/"long",*common,"--ttl-days","30",ok=False)
            self.assertFalse((root/"long").exists())

if __name__ == "__main__":
    unittest.main()
