# Examples

This directory contains runnable examples that show how to compose `core-lab` packages together in realistic service flows.

## Available examples

| Example | Focus |
| --- | --- |
| [`auth_example`](./auth_example/main.go) | Service bootstrap, observability, i18n, validation, structured responses, and middleware composition |
| [`jobs_example`](./jobs_example/main.go) | Standalone background job server with handlers, enqueue, and admin APIs |

## Running examples

From the repository root:

```bash
make build-examples
./bin/examples/auth_example
```

Or run an example directly:

```bash
go run ./examples/auth_example
go run ./examples/jobs_example
```

Examples are intended to stay small and educational. Prefer showing composition patterns and recommended defaults rather than exhaustive configuration.
