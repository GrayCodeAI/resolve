# Resolve

Resolve owns offline, Git-backed tickets. Ticket events are immutable Git objects and references. Reflect and Ridge may present or act on tickets, but Resolve owns their durable state.

After the initial commit, work on feature branches. Keep the event format versioned and test two-clone synchronization before changing it. Run `go test -race ./...` before committing.
