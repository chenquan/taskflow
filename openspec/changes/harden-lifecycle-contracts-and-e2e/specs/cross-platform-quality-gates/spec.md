## ADDED Requirements

### Requirement: Run cross-platform continuous integration
The repository MUST run Go tests and vet plus strict OpenSpec validation on Linux, macOS, and Windows. Race tests MUST run on Linux and macOS. CI MUST install the pinned supported OpenSpec version for real adapter verification.

#### Scenario: Pull request validation
- **WHEN** a change is pushed or proposed to the repository
- **THEN** the operating-system matrix runs portable lifecycle tests, vet, strict specification validation, and the supported race jobs

### Requirement: Enforce meaningful safety coverage
CI MUST generate a combined coverage profile, require at least 80 percent statement coverage overall, and include direct tests for Git, OpenSpec, lock, filesystem, configuration, planning, process, and application safety packages.

#### Scenario: Coverage falls below the gate
- **WHEN** combined statement coverage is less than 80 percent
- **THEN** the coverage check exits non-zero and CI fails

### Requirement: Verify portable snapshot releases
The repository MUST define a GoReleaser snapshot build for Linux, macOS, and Windows on amd64 and arm64 where supported, inject the CLI version through linker flags, and never publish from the validation workflow.

#### Scenario: Snapshot release validation
- **WHEN** the release configuration is checked in CI
- **THEN** GoReleaser builds local snapshot artifacts for the declared targets without publishing them
