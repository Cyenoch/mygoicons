"""Exercise the workflow's actual PR script against a disposable local Git remote."""
import json
import os
import subprocess
import tempfile
import textwrap
from pathlib import Path

workflow = Path(__file__).parents[1] / "workflows/update-icons.yml"
step = workflow.read_text().split("      - name: Create or update review PR\n", 1)[1]
script = textwrap.dedent(step.split("        run: |\n", 1)[1])


def run(*args, cwd, env=None, check=True):
    return subprocess.run(args, cwd=cwd, env=env, check=check,
                          text=True, capture_output=True)


def lock(version, date):
    return json.dumps({"retrievedAt": date, "collections": [
        {"prefix": "lucide", "version": version, "integrity": "sha512-" + version},
        {"prefix": "twemoji", "version": "1.2.5", "integrity": "sha512-twemoji"},
    ]})


with tempfile.TemporaryDirectory(prefix="mygoicons-pr-check-") as tmp:
    root = Path(tmp)
    remote = root / "remote.git"
    remote.mkdir()
    run("git", "init", "--bare", cwd=remote)
    seed = root / "seed"
    run("git", "clone", str(remote), str(seed), cwd=root)
    run("git", "config", "user.name", "Workflow test", cwd=seed)
    run("git", "config", "user.email", "test@example.invalid", cwd=seed)
    run("git", "checkout", "-b", "main", cwd=seed)
    for name in ["sources/lock.json", "lucide/icons_gen.go", "twemoji/icons_gen.go",
                 "examples/native/catalog_gen.go"]:
        target = seed / name
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(lock("1.0.0", "2026-01-01") if name.endswith(".json") else "fixture")
    run("git", "add", ".", cwd=seed)
    run("git", "commit", "-m", "Initial fixture", cwd=seed)
    run("git", "push", "origin", "main", cwd=seed)
    run("git", "symbolic-ref", "HEAD", "refs/heads/main", cwd=remote)

    tools = root / "bin"
    tools.mkdir()
    gh = tools / "gh"
    gh.write_text("""#!/usr/bin/env python3
import json, os, sys
from pathlib import Path
state = Path(os.environ['PR_TEST_STATE'])
data = json.loads(state.read_text()) if state.exists() else {'created': False, 'calls': []}
args = sys.argv[1:]
data['calls'].append(args)
if args[:2] == ['pr', 'list']:
    print('1' if data['created'] else '')
elif args[:2] == ['pr', 'create']:
    if os.environ.get('FAIL_CREATE') == '1':
        state.write_text(json.dumps(data))
        sys.exit(1)
    data['created'] = True
state.write_text(json.dumps(data))
""")
    gh.chmod(0o755)
    (root / "icon-summary.txt").write_text("Reviewed update fixture")
    env = dict(os.environ, PATH=str(tools) + os.pathsep + os.environ["PATH"],
               RUNNER_TEMP=str(root), PR_TEST_STATE=str(root / "pr.json"))

    def attempt(index, version, date, fail_create=False):
        checkout = root / f"attempt-{index}"
        run("git", "clone", str(remote), str(checkout), cwd=root)
        (checkout / "sources/lock.json").write_text(lock(version, date))
        result = run("bash", "-c", script, cwd=checkout,
                     env=dict(env, FAIL_CREATE="1" if fail_create else "0"), check=False)
        if (result.returncode != 0) != fail_create:
            raise AssertionError(result.stdout + result.stderr)
        return run("git", "rev-parse", "refs/heads/automation/icon-collections",
                   cwd=remote).stdout.strip()

    # A failed API call leaves a valid pushed commit; the next run recovers PR creation.
    first = attempt(1, "1.1.0", "2026-01-02", fail_create=True)
    repeated = attempt(2, "1.1.0", "2026-01-03")
    assert repeated == first, "date-only change created another commit"
    again = attempt(3, "1.1.0", "2026-01-04")
    assert again == first, "same release identity was not idempotent"
    next_update = attempt(4, "1.2.0", "2026-01-05")
    assert next_update != first, "new release did not update the branch"
    run("git", "merge-base", "--is-ancestor", first, next_update, cwd=remote)
    state = json.loads((root / "pr.json").read_text())
    creates = [c for c in state["calls"] if c[:2] == ["pr", "create"]]
    assert len(creates) == 2, "PR creation retry was not exercised"
    print("PR script passed: failed API recovery, same-batch deduplication across dates, "
          "existing PR updates, new release commit, and preserved branch history.")
