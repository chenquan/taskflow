## 1. Configuration and Domain Contracts

- [x] 1.1 Redefine version-1 execution configuration and make init serialize explicit change-creation intent.
- [x] 1.2 Validate enabled/default development tools, configured executables, and direct launch mode.
- [x] 1.3 Add typed action, status, validation-report, and readiness domain models with backward-compatible state decoding.
- [x] 1.4 Add configuration regression tests for removed fields, missing intent, and invalid tool policy.

## 2. Planning, Preflight, and Recovery

- [x] 2.1 Extend planning with a directory action and conditional fetch/OpenSpec actions in deterministic order.
- [x] 2.2 Extend the Git adapter with HEAD, dirty-file count, branch occupancy, upstream, and common-directory inspection.
- [x] 2.3 Implement all-repository mutation-free start preflight and typed conflict diagnostics.
- [x] 2.4 Persist and reconcile typed action outcomes after every executed or skipped action.
- [x] 2.5 Add unit and integration tests for preflight preservation, partial failure, and resume.

## 3. OpenSpec and Validation Reports

- [x] 3.1 Implement structured OpenSpec status and strict-validation JSON adapters.
- [x] 3.2 Implement task progress parsing and explicit not-configured behavior.
- [x] 3.3 Add full and repository-scoped dependency-closure validation.
- [x] 3.4 Persist atomic validation reports fingerprinted by normalized configuration and selected HEADs.
- [x] 3.5 Add adapter and application tests for valid, invalid, malformed, failed, timed-out, and scoped validation.

## 4. Status, Finish, Doctor, and Tools

- [x] 4.1 Return typed aggregate status with Git, OpenSpec, dependency, validation, and session facts.
- [x] 4.2 Implement strictly non-mutating finish readiness using fresh validation fingerprints and deterministic recommendations.
- [x] 4.3 Honor enabled tools and configured executables and report child exit facts under exit code 1.
- [x] 4.4 Extend doctor with version/capability, branch occupancy, and target readiness probes.
- [x] 4.5 Bind repository-scoped validate and add command/report regression tests.

## 5. Portable Lifecycle Tests and Delivery Gates

- [x] 5.1 Replace shell fixtures with a portable Go fixture executable and bounded controls.
- [x] 5.2 Add three-repository lifecycle tests including Unicode/space paths and byte-for-byte invariants.
- [x] 5.3 Add branch/target conflicts, disabled OpenSpec, custom tools, child failures, and stale-report E2E scenarios.
- [x] 5.4 Add direct Git, OpenSpec, lock, filesystem, and process adapter tests.
- [x] 5.5 Add a pinned real-OpenSpec integration test.
- [x] 5.6 Add combined coverage enforcement with an 80 percent minimum.
- [x] 5.7 Add Linux/macOS/Windows CI and supported race jobs.
- [x] 5.8 Add GoReleaser snapshot configuration with version linker flags.

## 6. Documentation and Verification

- [x] 6.1 Update the implementation plan for the breaking v1 schema, validation fingerprints, and child exit contract.
- [x] 6.2 Run formatting, full tests, race tests, vet, coverage, real OpenSpec integration, snapshot build, strict OpenSpec validation, and diff checks.
- [x] 6.3 Verify every modified requirement and scenario against implementation evidence.
