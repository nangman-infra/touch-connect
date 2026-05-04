# tc-worker

`tc-worker` is the execution endpoint runtime.

Responsibilities:

- register itself as an endpoint
- advertise capabilities
- receive or claim messages
- run local CLI, shell, process, or skill-backed work
- send readback, checkpoint, artifact, completion, and failure updates

It owns execution, not the source of truth.

Detailed implementation docs are maintained as local living contracts and are intentionally not tracked in the public Git repository.
