#!/usr/bin/env python3
"""Mandatory live gate on two owned disposable kind clusters and real Agents."""
import hashlib, json, os
from pathlib import Path
import platform, signal, subprocess, tempfile, urllib.request, uuid

ROOT=Path(__file__).resolve().parents[2]
KIND_VERSION="v0.33.0"
KIND_SHA256="aee6151561422756b764a4ae28e7f44cda5af5a9eead3cc9985112b1de8d8e0d"
NODE_IMAGE="kindest/node:v1.37.0@sha256:a1ed56cfb0e7b93589bdf97c8cd566405a265939e3620fc4f5de89adff580ae5"

def validate_receipt(returncode, events, report):
    selected=[e for e in events if e.get("Test")=="TestTwoClusterDaemonSetLive"]
    if returncode!=0 or not any(e.get("Action")=="pass" for e in selected) or any(e.get("Action") in ("skip","fail") for e in selected):
        raise RuntimeError("live test did not run and pass")
    required=("passed","agentInventory","scopedHTTPJWT","firstInvestigationRBAC","firstFindingRule","concurrentManualAck","workerReconciliation","replacementUIDProtected","deletePersistenceRecovery","agentRestart")
    if any(report.get(key) is not True for key in required) or report.get("clusters")!=2 or report.get("nodes",0)<3 or report.get("daemonSets")!=4 or report.get("namespaceScopes")!=2:
        raise RuntimeError("incomplete live evidence receipt")
    if report.get("runtimeAutoResolutionEnabled") is not False or report.get("dashboardBrowserVerified") is not False:
        raise RuntimeError("receipt overstates runtime or browser validation")

def main():
    if platform.system()!="Linux" or platform.machine() not in ("x86_64","amd64"):raise RuntimeError("live gate requires Linux amd64")
    if not os.environ.get("FORTUNA_TEST_POSTGRES_URL"):raise RuntimeError("isolated FORTUNA_TEST_POSTGRES_URL required")
    folder=Path(tempfile.mkdtemp(prefix="fortuna-two-cluster-"));folder.chmod(0o700)
    suffix=uuid.uuid4().hex[:10];names=["fortuna-audit-a-"+suffix,"fortuna-audit-b-"+suffix];image="fortuna-local-integration:"+suffix
    log_path=folder/"integration.log";log=log_path.open("w");log_path.chmod(0o600)
    env=os.environ.copy();env.update(GOWORK="off",GOTOOLCHAIN="go1.26.8",CGO_ENABLED="0",GOMAXPROCS="1",GOFLAGS="-p=1")
    kind=folder/"kind";download=urllib.request.urlopen("https://github.com/kubernetes-sigs/kind/releases/download/"+KIND_VERSION+"/kind-linux-amd64", timeout=60).read()
    if hashlib.sha256(download).hexdigest()!=KIND_SHA256:raise RuntimeError("kind checksum mismatch")
    kind.write_bytes(download);kind.chmod(0o700)
    owned=[];settings={}
    def call(command,**kw):subprocess.run([str(x) for x in command],check=True,stdout=log,stderr=log,timeout=600,**kw)
    def interrupted(signum,frame):raise KeyboardInterrupt("live gate interrupted")
    signal.signal(signal.SIGTERM,interrupted)
    try:
        # Three real nodes exceed the default inotify instance budget on small
        # VM labs. Raise only if needed and restore the exact values afterwards.
        prefix=[] if os.geteuid()==0 else ["sudo","-n"]
        for setting,minimum in {"fs.inotify.max_user_instances":512,"fs.inotify.max_user_watches":262144}.items():
            path=Path("/proc/sys/"+setting.replace(".","/"));old=int(path.read_text());
            if old<minimum:settings[setting]=old;call(prefix+["sysctl","-w",setting+"="+str(minimum)])
        call(["go","build","-trimpath","-o",folder/"fortuna-agent","./cmd"],cwd=ROOT/"agent",env=env)
        (folder/"Dockerfile").write_text('FROM scratch\nCOPY fortuna-agent /agent\nENTRYPOINT ["/agent"]\n')
        (folder/".dockerignore").write_text('*\n!fortuna-agent\n!Dockerfile\n')
        call(["docker","build","-t",image,folder])
        for index,name in enumerate(names):
            config=folder/(name+".yaml");config.write_text("kind: Cluster\napiVersion: kind.x-k8s.io/v1alpha4\nnodes:\n- role: control-plane\n"+("- role: worker\n" if index==0 else ""))
            owned.append(name)
            call([kind,"create","cluster","--name",name,"--image",NODE_IMAGE,"--config",config,"--kubeconfig",folder/(name+".kubeconfig"),"--wait","120s"])
            call([kind,"load","docker-image",image,"--name",name])
        network=json.loads(subprocess.check_output(["docker","network","inspect","kind"],text=True))[0];gateway=next(item["Gateway"] for item in network["IPAM"]["Config"] if ":" not in item["Gateway"])
        env.update(FORTUNA_RUN_LIVE_INTEGRATION="1",FORTUNA_TEST_AGENT_IMAGE=image,FORTUNA_TEST_CORE_HOST_IP=gateway,FORTUNA_TEST_KUBECONFIG_A=str(folder/(names[0]+".kubeconfig")),FORTUNA_TEST_KUBECONFIG_B=str(folder/(names[1]+".kubeconfig")),FORTUNA_TEST_EVIDENCE_DIR=str(folder))
        # JSON readback proves the selected test ran and passed; a removed,
        # renamed, skipped or empty -run selection cannot silently pass this gate.
        result=subprocess.run(["go","test","-json","./internal/api","-run","^TestTwoClusterDaemonSetLive$","-count=1","-timeout=5m"],cwd=ROOT/"core",env=env,stdout=subprocess.PIPE,stderr=log,text=True,timeout=600)
        log.write(result.stdout);log.flush();events=[json.loads(line) for line in result.stdout.splitlines() if line.startswith("{")]
        if result.returncode!=0:raise RuntimeError("live two-cluster gate failed; logs: "+str(log_path))
        report=json.loads((folder/"two-cluster.json").read_text())
        validate_receipt(result.returncode,events,report)
        report.update(sourceSHA=subprocess.check_output(["git","rev-parse","HEAD"],cwd=ROOT,text=True).strip(),dirty=bool(subprocess.check_output(["git","status","--porcelain"],cwd=ROOT,text=True)),agentSHA256=hashlib.sha256((folder/"fortuna-agent").read_bytes()).hexdigest(),kindVersion=KIND_VERSION,nodeImage=NODE_IMAGE)
        (folder/"result.json").write_text(json.dumps(report,indent=2)+"\n");print(json.dumps({"passed":True,"evidence":str(folder),**report}))
    finally:
        for name in reversed(owned):subprocess.run([str(kind),"delete","cluster","--name",name],stdout=log,stderr=log)
        subprocess.run(["docker","image","rm",image],stdout=log,stderr=log)
        for setting,value in settings.items():subprocess.run(prefix+["sysctl","-w",setting+"="+str(value)],stdout=log,stderr=log)
        log.close()

if __name__=="__main__":main()
