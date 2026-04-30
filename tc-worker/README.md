# tc-worker

`tc-worker` is the execution endpoint runtime.

Responsibilities:

- register itself as an endpoint
- advertise capabilities
- receive or claim messages
- run local CLI, shell, process, or skill-backed work
- send readback, checkpoint, artifact, completion, and failure updates

It owns execution, not the source of truth.

Detailed implementation docs:

- [docs/implementation-contract.md](docs/implementation-contract.md)
- [docs/definition-of-done.md](docs/definition-of-done.md)
- [docs/implementation-task-list.md](docs/implementation-task-list.md)
