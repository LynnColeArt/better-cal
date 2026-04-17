# Documentation Map

This project keeps source-informed audit notes separate from source-neutral implementation specs.

## Directories

- `internal/`: source-informed notes for reviewers who are allowed to inspect the reference implementation.
- `spec/`: source-neutral contracts for the whiteroom reimplementation team.
- `../contracts/`: machine-readable registries, schemas, manifests, and fixture payloads.

Implementation work should use `spec/` and accepted fixture artifacts. The `internal/` documents are useful for producing specs, but they are not implementation inputs.
