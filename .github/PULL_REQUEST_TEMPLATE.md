## What this changes

<!-- What is different after this change for code that embeds agon, or for someone reading a review's session directory. -->

## Why

<!-- The problem this solves. Link the issue or the spec if there is one. -->

## Checklist

- [ ] `go tool lateregate` passes on your machine.
- [ ] A bug fix includes a test that fails without the fix.
- [ ] If this changes a contract (the exported API, the debate protocol,
      or the session format), the spec under `specs/` changes in the same
      commit.
- [ ] Every caller of an exported identifier still compiles, or
      `CHANGELOG.md` names the incompatible change under `Unreleased`.
- [ ] The engine core still imports no model client and no agent runtime
      (`critic/boundary_test.go` passes).
