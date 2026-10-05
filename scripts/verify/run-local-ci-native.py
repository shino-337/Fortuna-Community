#!/usr/bin/env python3
"""Execute CI workflow run steps sequentially without a large act runner image."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import platform
import re
import signal
import shlex
import shutil
import subprocess
import sys
import tempfile
import time
import urllib.request
import uuid

import yaml

GROUPS = ("hygiene", "scripts", "helm", "go-test", "cluster-identity-postgres", "dashboard")


def capture(command, cwd=None, env=None):
    return subprocess.check_output(command, cwd=cwd, env=env, text=True).strip()


def source_identity(repo):
    sha = capture(["git", "rev-parse", "HEAD"], repo)
    changes = subprocess.check_output(["git", "status", "--porcelain=v1", "-z", "--untracked-files=all"], cwd=repo)
    digest = hashlib.sha256(sha.encode())
    digest.update(changes)
    digest.update(subprocess.check_output(["git", "diff", "HEAD", "--binary"], cwd=repo))
    untracked = subprocess.check_output(["git", "ls-files", "--others", "--exclude-standard", "-z"], cwd=repo)
    for name in sorted(filter(None, untracked.split(b"\0"))):
        digest.update(name)
        candidate = repo / os.fsdecode(name)
        if candidate.is_symlink():
            digest.update(os.readlink(candidate).encode())
        elif candidate.is_file():
            digest.update(candidate.read_bytes())
    return {"sha": sha, "dirty": bool(changes), "fingerprint": digest.hexdigest()}


def step_runs(job, module=None):
    for step in job["steps"]:
        if set(step) - {"name", "uses", "with", "run", "working-directory", "env", "if", "shell"}:
            raise ValueError("Unsupported workflow step configuration")
        condition = step.get("if")
        if condition:
            if condition != "matrix.module == 'core'" or module is None:
                raise ValueError("Unsupported local conditional: " + condition)
            if module != "core":
                continue
        if "uses" in step:
            if set(step) - {"name", "uses", "with"}:
                raise ValueError("Unsupported local action configuration")
            if not re.fullmatch(r"(actions/(checkout|setup-go|setup-node|setup-python)|azure/setup-helm)@v[0-9]+", step["uses"]):
                raise ValueError("Unsupported local action: " + step["uses"])
            continue
        if "run" not in step:
            raise ValueError("Unsupported workflow step")
        if step.get("shell", "bash") != "bash" or "with" in step:
            raise ValueError("Unsupported local shell/run configuration")
        text = step["run"]
        directory = step.get("working-directory", ".")
        if module:
            directory = directory.replace("${{ matrix.module }}", module)
        if "${{" in text or "${{" in directory or any("${{" in str(value) for value in step.get("env", {}).values()):
            raise ValueError("Unresolved workflow expression")
        yield step.get("name", "Run"), text, directory, step.get("env", {})


def validate_workflow(workflow):
    # setup-* and checkout are emulated locally; global/job execution controls
    # cannot be silently ignored if the hosted workflow changes later.
    if set(workflow) - {"name", "on", True, "permissions", "concurrency", "jobs"}:
        raise ValueError("Unsupported workflow-level execution configuration")
    jobs = workflow["jobs"]
    if set(jobs) != set(GROUPS):
        raise ValueError("Workflow job groups changed; update the native runner before claiming all-pass")
    for group, job in jobs.items():
        allowed = {"name", "runs-on", "env", "steps"}
        if group == "go-test": allowed.add("strategy")
        if group == "cluster-identity-postgres": allowed.add("services")
        if set(job) - allowed or job.get("runs-on") != "ubuntu-latest":
            raise ValueError("Unsupported job execution configuration: " + group)
        if any("${{" in str(value) for value in job.get("env", {}).values()):
            raise ValueError("Unresolved job environment expression")
    if jobs["go-test"]["strategy"] != {"fail-fast": False, "matrix": {"module": ["core", "agent", "api"]}}:
        raise ValueError("Go matrix changed; update the native runner")
    if set(jobs["cluster-identity-postgres"]["services"]) != {"postgres"}:
        raise ValueError("Unsupported PostgreSQL workflow services")
    for group, job in jobs.items():
        for module in (["core", "agent", "api"] if group == "go-test" else [None]):
            list(step_runs(job, module))
    return jobs


def setup_version(job, action, field):
    matches = [step["with"][field] for step in job["steps"] if step.get("uses", "").startswith("actions/" + action + "@")]
    if len(matches) != 1:
        raise ValueError("Expected one " + action + " version")
    return str(matches[0])


def helm_version(job):
    matches = [step["with"]["version"] for step in job["steps"] if step.get("uses", "").startswith("azure/setup-helm@")]
    if len(matches) != 1:
        raise ValueError("Expected one setup-helm version")
    return str(matches[0])


def publishable(selection, source, finished_source, passed):
    return selection == "all" and passed and not source["dirty"] and source == finished_source


def ensure_node(major, environment):
    """Keep the host Node installation untouched; verify official archive hashes."""
    configured = os.environ.get("LOCAL_CI_NODE_BIN")
    candidates = [Path(configured).resolve() / "node"] if configured else [Path(shutil.which("node") or "/missing/node")]
    for candidate in candidates:
        if candidate.is_file() and capture([str(candidate), "--version"]).split(".")[0] == "v" + major:
            environment["PATH"] = str(candidate.parent) + os.pathsep + environment["PATH"]
            return
    if configured:
        raise ValueError("LOCAL_CI_NODE_BIN does not provide the workflow Node major " + major)
    if platform.system() != "Linux" or platform.machine() not in ("x86_64", "amd64"):
        raise ValueError("Configure LOCAL_CI_NODE_BIN for this platform")
    cache = Path(os.environ.get("LOCAL_CI_CACHE_DIR", str(Path.home() / ".cache/fortuna-local-ci")))
    cache.mkdir(parents=True, exist_ok=True)
    base = "https://nodejs.org/dist/latest-v" + major + ".x/"
    with urllib.request.urlopen(base + "SHASUMS256.txt", timeout=60) as response:
        sums = response.read().decode()
    matches = re.findall(r"^([a-f0-9]{64})\s+(node-v" + re.escape(major) + r"\.[0-9]+\.[0-9]+-linux-x64\.tar\.xz)$", sums, re.M)
    if len(matches) != 1:
        raise ValueError("Cannot resolve official workflow Node archive")
    expected, archive = matches[0]
    extracted = archive.removesuffix(".tar.xz")
    node = cache / extracted / "bin/node"
    if not node.exists():
        with tempfile.TemporaryDirectory(prefix="node-download-", dir=cache) as folder:
            download = Path(folder) / archive
            urllib.request.urlretrieve(base + archive, download)
            if hashlib.sha256(download.read_bytes()).hexdigest() != expected:
                raise ValueError("Node archive checksum mismatch")
            subprocess.run(["tar", "-xJf", str(download), "-C", folder], check=True)
            if (cache / extracted).exists():
                raise ValueError("Node cache changed concurrently; retry the run")
            os.rename(Path(folder) / extracted, cache / extracted)
    if capture([str(node), "--version"]).split(".")[0] != "v" + major:
        raise ValueError("Invalid cached Node version")
    environment["PATH"] = str(node.parent) + os.pathsep + environment["PATH"]


class Runner:
    def __init__(self, repo, selection, output):
        self.repo, self.selection, self.output = repo, selection, output
        self.workflow_path = repo / ".github/workflows/ci.yml"
        self.workflow_bytes = self.workflow_path.read_bytes()
        self.jobs = validate_workflow(yaml.safe_load(self.workflow_bytes))
        self.source = source_identity(repo)
        self.results = []
        self.env = os.environ.copy()
        # Integration tests must never inherit a live database URL from the shell.
        self.env.pop("FORTUNA_TEST_POSTGRES_URL", None)
        self.env["CI"] = "true"
        self.env["GOMAXPROCS"] = os.environ.get("LOCAL_CI_GOMAXPROCS", "1")
        # Inherited -run/-tags flags must not turn the full suite into a false pass.
        self.env["GOFLAGS"] = "-p=1"

    def execute(self, command, log, cwd=None, env=None):
        print("+ " + shlex.join(command), flush=True)
        log.write("+ " + shlex.join(command) + "\n"); log.flush()
        process = subprocess.Popen(command, cwd=cwd or self.repo, env=env or self.env, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True, start_new_session=True)
        try:
            for line in process.stdout:
                print(line, end="", flush=True); log.write(line); log.flush()
            if process.wait():
                raise subprocess.CalledProcessError(process.returncode, command)
        except BaseException:
            if process.poll() is None:
                os.killpg(process.pid, signal.SIGTERM)
                try: process.wait(timeout=10)
                except subprocess.TimeoutExpired: os.killpg(process.pid, signal.SIGKILL); process.wait()
            raise

    def run_steps(self, job, log, module=None, extra=None):
        environment = self.env.copy()
        environment.update({k: str(v) for k, v in job.get("env", {}).items()})
        for name, text, directory, variables in step_runs(job, module):
            current = environment.copy()
            current.update({k: str(v) for k, v in variables.items()})
            current.update(extra or {})
            print(name, flush=True)
            self.execute(["bash", "-euo", "pipefail", "-c", text], log, self.repo / directory, current)

    def postgres(self, job, log):
        service = job["services"]["postgres"]
        if service["image"] != "apache/age:release_PG16_1.6.0@sha256:16aa423d20a31aed36a3313244bf7aa00731325862f20ed584510e381f2feaed" or service["env"] != {"POSTGRES_USER": "postgres", "POSTGRES_PASSWORD": "postgres", "POSTGRES_DB": "fortuna_test"}:
            raise ValueError("PostgreSQL workflow service changed; refusing an unverified local database configuration")
        container = None
        name = "fortuna-local-ci-pg-" + uuid.uuid4().hex[:12]
        try:
            command = ["docker", "run", "--rm", "-d", "--name", name, "-p", "127.0.0.1::5432"]
            for key, value in service["env"].items(): command += ["-e", key + "=" + str(value)]
            container = capture(command + [service["image"]])
            if not re.fullmatch(r"[a-f0-9]{64}", container):
                raise ValueError("Unexpected temporary PostgreSQL container ID")
            # TCP stays unavailable during initdb's temporary socket-only server.
            for _ in range(90):
                result = subprocess.run(["docker", "exec", container, "pg_isready", "-h", "127.0.0.1", "-U", "postgres", "-d", "fortuna_test"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
                if result.returncode == 0: break
                time.sleep(1)
            else: raise RuntimeError("Temporary PostgreSQL readiness timed out")
            binding = capture(["docker", "port", container, "5432/tcp"])
            if not re.fullmatch(r"127\.0\.0\.1:[0-9]+", binding): raise ValueError("CI PostgreSQL must be loopback-only")
            port = binding.split(":")[1]
            url = "postgres://postgres:postgres@127.0.0.1:" + port + "/fortuna_test?sslmode=disable"
            self.run_steps(job, log, extra={"FORTUNA_TEST_POSTGRES_URL": url})
        finally:
            if container and re.fullmatch(r"[a-f0-9]{64}", container):
                subprocess.run(["docker", "stop", "--time", "10", container], check=True, stdout=subprocess.DEVNULL)
            else:
                # A signal may arrive after daemon creation but before docker
                # run returns its ID. The unique name is still scoped to us.
                subprocess.run(["docker", "stop", "--time", "10", name], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)

    def run_group(self, group):
        job = self.jobs[group]
        modules = job["strategy"]["matrix"]["module"] if group == "go-test" else [None]
        for module in modules:
            label = group + ("/" + module if module else "")
            record = {"job": label, "passed": False, "log": label.replace("/", "-") + ".log"}
            self.results.append(record)
            with (self.output / record["log"]).open("w") as log:
                if group in ("go-test", "cluster-identity-postgres"):
                    self.env["GOTOOLCHAIN"] = "go" + setup_version(job, "setup-go", "go-version")
                    self.execute(["go", "version"], log)
                if group == "helm":
                    # Not downloaded: an installed helm must match the workflow's pinned version.
                    wanted = helm_version(job)
                    installed = capture(["helm", "version", "--template", "{{.Version}}"], env=self.env)
                    if installed != wanted:
                        raise ValueError("helm " + installed + " found; the workflow pins " + wanted)
                    self.env["RUNNER_TEMP"] = str(self.output)
                if group == "dashboard":
                    ensure_node(setup_version(job, "setup-node", "node-version"), self.env)
                    self.execute(["node", "--version"], log)
                if group == "scripts":
                    version = setup_version(job, "setup-python", "python-version")
                    runs = list(step_runs(job))
                    if any(directory != "." or variables for _, _, directory, variables in runs):
                        raise ValueError("Scripts job working directory/env changed")
                    text = "apt-get update -qq\napt-get install -y --no-install-recommends git openssl shellcheck >/dev/null\n" + "\n".join(run for _, run, _, _ in runs)
                    name = "fortuna-local-ci-scripts-" + uuid.uuid4().hex[:12]
                    try:
                        self.execute(["docker", "run", "--rm", "--name", name, "--cpus", "1", "--memory", "512m", "--mount", "type=bind,src=" + str(self.repo) + ",dst=/repo,readonly", "-w", "/repo", "python:" + version + "-slim", "bash", "-euo", "pipefail", "-c", text], log)
                    finally:
                        # Only this run's uniquely named container can be stopped.
                        subprocess.run(["docker", "stop", "--time", "10", name], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
                elif group == "cluster-identity-postgres":
                    self.postgres(job, log)
                else:
                    self.run_steps(job, log, module)
            record["passed"] = True

    def run(self):
        passed = False
        try:
            groups = GROUPS if self.selection == "all" else (self.selection,)
            for group in groups: self.run_group(group)
            passed = True
        finally:
            finished = source_identity(self.repo)
            workflow_unchanged = self.workflow_path.read_bytes() == self.workflow_bytes
            stable = finished == self.source and workflow_unchanged
            report = {"version": 1, "backend": "native", "selection": self.selection, "source": self.source, "finished_source": finished,
                      "workflow_sha256": hashlib.sha256(self.workflow_bytes).hexdigest(), "passed": passed and stable,
                      "publishable": publishable(self.selection, self.source, finished, passed and stable), "jobs": self.results}
            for record in self.results:
                artifact = self.output / record["log"]
                if artifact.exists(): record["sha256"] = hashlib.sha256(artifact.read_bytes()).hexdigest()
            (self.output / "results.json").write_text(json.dumps(report, indent=2) + "\n")
            print("Local CI evidence: " + str(self.output), flush=True)
            if not stable: raise RuntimeError("Source/workflow changed during CI; results are not a valid gate")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("selection", choices=("list", "all") + GROUPS, default="all", nargs="?")
    args = parser.parse_args()
    if args.selection == "list":
        print("\n".join(GROUPS)); return 0
    def interrupted(signum, frame):
        raise KeyboardInterrupt("Local CI interrupted by signal " + str(signum))
    signal.signal(signal.SIGTERM, interrupted)
    os.umask(0o077)
    repo = Path(__file__).resolve().parents[2]
    root = os.environ.get("LOCAL_CI_OUTPUT_DIR")
    if root:
        parent = Path(root).resolve()
        if parent == repo or repo in parent.parents: raise ValueError("CI evidence must be outside the worktree")
        parent.mkdir(parents=True, exist_ok=True)
    else: parent = None
    output = Path(tempfile.mkdtemp(prefix="fortuna-local-ci-", dir=parent))
    try:
        Runner(repo, args.selection, output).run()
        return 0
    except (Exception, KeyboardInterrupt) as error:
        print("Local CI failed: " + str(error), file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
