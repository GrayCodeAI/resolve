# Resolve

Resolve is an offline Git-backed ticket engine for GrayCode. Each ticket change is an immutable event in a `refs/resolve/events/<ticket-id>/<event-id>` Git reference. Separate replicas can write events offline, then exchange them through a Git remote. No service or database is required.

## Try it

```sh
go build -o resolve ./cmd/resolve
./resolve new --title 'Fix preflight' --body 'Detect missing tool capability'
./resolve list
./resolve show <ticket-id>
./resolve comment --text 'Reproduced locally' <ticket-id>
./resolve status --set in_progress <ticket-id>
./resolve sync --remote origin push
./resolve sync --remote origin pull
```

All commands accept `--repo <path>` before positional arguments. The JSON output is intended for Reflect, Ridge, and a future Rally workspace. Sync requires an ordinary configured Git remote. Resolve data is not added to the source branch or working tree.

## Current boundary

Resolve owns ticket identity and state. Reflect owns coding subtasks and agent execution. Ridge can show and claim tickets; Record can cite ticket IDs in checkpoints; Rally can link messages to tickets. These products communicate through ticket IDs and JSON, with no Go import dependency yet.

The initial version supports create, comment, status, list, show, and Git push/pull of ticket events. Events are Git content-addressed, but actor names are self-reported; signatures, authorization, attachments, import bridges, and a web UI are future work. Concurrent updates converge deterministically by logical clock and event ID; a simultaneous claim is visible in history but only one claimant wins the projected state.

## License

MIT — see [LICENSE](LICENSE) for details. Contributions are welcome under the terms in [CONTRIBUTING.md](CONTRIBUTING.md); report security issues privately as described in [SECURITY.md](SECURITY.md).

© 2026 GrayCode AI
