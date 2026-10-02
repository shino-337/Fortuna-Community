#!/usr/bin/env python3
"""Restore an operator backup into an isolated database; never mutate the lab."""
import argparse, hashlib, json, os
from pathlib import Path
import subprocess, tempfile, time, uuid

ROOT=Path(__file__).resolve().parents[2]
IMAGE="apache/age:release_PG16_1.6.0@sha256:16aa423d20a31aed36a3313244bf7aa00731325862f20ed584510e381f2feaed"

def main():
    parser=argparse.ArgumentParser();parser.add_argument("--backup",type=Path,required=True);parser.add_argument("--output",type=Path);args=parser.parse_args()
    backup=args.backup.resolve(strict=True)
    output=(args.output or Path(tempfile.mkdtemp(prefix="fortuna-migration-rehearsal-"))).resolve();output.mkdir(parents=True,exist_ok=True);output.chmod(0o700)
    log_path=output/"migration.log";log=log_path.open("w");log_path.chmod(0o600)
    name="fortuna-rehearsal-"+uuid.uuid4().hex[:12];database="fortuna_rehearsal_"+uuid.uuid4().hex[:12];password=uuid.uuid4().hex
    def call(command,**kw):return subprocess.run(command,check=True,stdout=log,stderr=log,**kw)
    try:
        call(["docker","run","--rm","-d","--name",name,"-p","127.0.0.1::5432","-e","POSTGRES_PASSWORD="+password,"-e","POSTGRES_DB="+database,IMAGE])
        for _ in range(60):
            ready=subprocess.run(["docker","exec",name,"pg_isready","-h","127.0.0.1","-U","postgres","-d",database],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
            if ready.returncode==0:break
            time.sleep(1)
        else:raise RuntimeError("isolated database readiness timed out")
        with backup.open("rb") as source:call(["docker","exec","-i",name,"pg_restore","-U","postgres","-d",database,"--no-owner","--no-acl","--exit-on-error"],stdin=source)
        binding=subprocess.check_output(["docker","port",name,"5432/tcp"],text=True).strip()
        if not binding.startswith("127.0.0.1:"):raise RuntimeError("rehearsal database must bind loopback")
        env=os.environ.copy();env.update(FORTUNA_REHEARSAL_POSTGRES_URL="postgres://postgres:"+password+"@"+binding+"/"+database+"?sslmode=disable",GOWORK="off",GOTOOLCHAIN="go1.26.8",GOMAXPROCS="1",GOFLAGS="-p=1")
        result=subprocess.run(["go","run","./cmd/migration-rehearsal"],cwd=ROOT/"core",env=env,check=True,stdout=subprocess.PIPE,stderr=log,text=True)
        evidence=json.loads(result.stdout);evidence.update(backupSHA256=hashlib.sha256(backup.read_bytes()).hexdigest(),sourceSHA=subprocess.check_output(["git","rev-parse","HEAD"],cwd=ROOT,text=True).strip(),dirty=bool(subprocess.check_output(["git","status","--porcelain"],cwd=ROOT,text=True)),image=IMAGE)
        (output/"result.json").write_text(json.dumps(evidence,indent=2)+"\n");print(json.dumps({"passed":True,"output":str(output),**evidence}))
    finally:
        subprocess.run(["docker","stop","--time","5",name],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL);log.close()

if __name__=="__main__":main()
