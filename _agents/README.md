# operator-bdd-test — Agent Context Index

Quick-start index for AI agents. Read this file first, then open the
specific summary file for the area you are working on.

## Repository purpose

BDD (Behaviour-Driven Development) test suite for the **OpenShift Serverless
Logic (OSL)** operator. Tests run against a real OpenShift cluster using
[Godog](https://github.com/cucumber/godog) (Go Cucumber framework).

**Product name:** `logic-operator` (Red Hat OSL). Never use `sonataflow-operator`
(community upstream) in OLM steps — all subscription names, CSV names, and
catalog references use `logic-operator`.

## Go workspace layout

```
go.work                         — workspace root (not a module itself)
bddframework/                   — reusable BDD framework module
testbdd/                        — test suite module (imports bddframework)
test/                           — shared test data and YAML helpers (part of testbdd module)
```

Both modules must be vetted/built independently:
```bash
cd bddframework && go vet ./...
cd testbdd      && go vet ./...
```

## Summary files in this folder

| File | What it covers |
|------|---------------|
| `repo-structure.md` | Full directory tree with one-line descriptions |
| `upgrade-tests.md` | The upgrade scenario — design, current state, planned tasks |
| `step-authoring.md` | How to write new Gherkin steps (patterns, namespaces, placeholders) |
| `key-constants.md` | Important constants, namespaces, ConfigMap names, flag names |

## Task plan files (in `tasks/`)

| File | Status | Contents |
|------|--------|---------|
| `upgrade-tests-improvement-plan-tasks-1-8.md` | ✅ Implemented | Config flags, CSV gate, DI/JS version log checks |
| `upgrade-tests-improvement-plan-task-9.md` | 🟡 Original planning doc | Builder-config ConfigMap check (background / design) |
| `upgrade-tests-improvement-plan-task-9-implementation.md` | 🟡 Ready to implement | **Authoritative spec** — exact Go + Gherkin to write |
| `upgrade-tests-improvement-plan-task-10.md` | ✅ Implemented | GitOps workflow migration steps + feature changes |
