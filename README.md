# Echo Server

This repository contains a small Echo Server written in Go. It accepts requests on any path and returns the request headers, query parameters, raw body, and path as JSON.

The application is packaged as a minimal non-root container and deployed to a local Kind Kubernetes cluster with Pulumi. CI creates an ephemeral cluster for smoke tests.

## Architecture

The Pod is reached in two different ways depending on where the client runs:

```text
Client outside the cluster (local development and CI)
  -> kubectl port-forward (resolves the Service to a Pod, tunnels directly)
  -> Deployment Pod :8080
  -> stateless Go http.Handler

Client inside the cluster (other Pods)
  -> ClusterIP Service :80
  -> Deployment Pod :8080
  -> stateless Go http.Handler
```

The current automated checks run outside the cluster and use port forwarding, so they do not exercise ClusterIP routing.

Repository structure:

- `internal/echo`: translates an HTTP request into the response contract;
- `cmd/echo/config.go`: parses and validates process configuration;
- `cmd/echo/main.go`: composes the handler, HTTP server, signals, and graceful shutdown;
- `infra`: declares the Kubernetes Deployment and Service with Pulumi;
- `scripts/`: contains scripts relevant to the project;
- GitHub Actions creates an ephemeral Kind cluster and verifies the deployed service end to end.

## Response contract

Every normal request path is handled. A successful request returns `200 OK` and `Content-Type: application/json` with this shape:

```json
{
  "headers": {
    "Content-Type": ["application/json"],
    "X-Demo": ["one", "two"]
  },
  "params": {
    "tag": ["go", "kubernetes"]
  },
  "body": "{\"message\":\"hello\"}",
  "path": "/anything/here"
}
```

**Details**

- Header and query-parameter values are arrays so repeated values are preserved;
- `Host` is included even though Go stores it separately from `Request.Header`;
- `body` is a string containing the payload as UTF-8 text; JSON input is not parsed and reserialized. Invalid UTF-8 bytes are replaced with the Unicode replacement character (U+FFFD), so binary payloads are not preserved byte-for-byte;
- `path` excludes the query string because query parameters are returned separately;
- `OPTIONS *` reaches the echo handler because the server's automatic general OPTIONS response is disabled, so it returns the same JSON envelope with `path` set to `*`;
- `HEAD` is answered by the same handler, but HTTP requires an empty response body, so only the status and headers are returned;
- Go canonicalizes header names, so a header such as `X-CI` is normally returned as `X-Ci`;
- Request bodies are bounded by `MAX_BODY_BYTES`;
- An oversized body returns `413 Request Entity Too Large`;
- Other body-read failures return `400 Bad Request`;
- Both `413` and `400` responses use the same JSON envelope, leave `body` empty, and add an `error` field.

## Configuration

| Environment variable | Default   | Validation                       | Purpose                   |
| -------------------- | --------: | -------------------------------- | ------------------------- |
| `PORT`               | `8080`    | Integer from `1` through `65535` | HTTP listen port          |
| `MAX_BODY_BYTES`     | `1048576` | Positive integer                 | Maximum request-body size |

**Note**: Invalid configuration prevents the server from starting.

## Relevant Commands

```bash
make help               # list all available Make commands
make check-requirements # report missing or outdated prerequisites (does not fail)
make test               # runs all formatting checks, vet checks, race-enabled unit tests, and Pulumi tests
make test-e2e           # deploys locally and validates the response through kubectl port-forward, optionally add E2E_ARGS=--delete
                        #   to delete the resources created in the e2e test
make build-echo-server  # builds a local executable at build/echo-server
make run-infra          # applies the infrastructure; requires the Kind cluster described in SETUP.md
make clean              # deletes build/
```

For more details on how to properly set up and run this project, read [SETUP.md](./SETUP.md).

## Container and Kubernetes safeguards

- Multi-stage build producing a statically linked Linux binary;
- `scratch` runtime image with no shell or package manager;
- Numeric non-root user `65532:65532`;
- Read-only root filesystem and disabled privilege escalation;
- All Linux capabilities dropped and `RuntimeDefault` seccomp enabled;
- Service account token automount disabled;
- Explicit CPU and memory requests and limits;
- Readiness and liveness probes through the named `http` port;
- HTTP server read, write, idle, and header timeouts;
- Ten-second graceful HTTP shutdown within Kubernetes' fifteen-second termination grace period.

The service has no external dependencies, so `/readyz` and `/healthz` deliberately use the normal catch-all echo handler. A successful HTTP response proves that the process can serve requests without adding special health-check behavior.

## Infrastructure

Pulumi creates two Kubernetes resources:

- A one-replica `Deployment` running `echo-server:local` by default;
- A `ClusterIP` `Service` exposing port `80` and targeting the container's named `http` port on `8080`.

The image uses `imagePullPolicy: Never` because it is copied directly into Kind with `kind load docker-image` (therefore no registry is required). Pulumi uses a repository-local filesystem backend in `.pulumi-state/` and explicitly targets the `kind-echo` kubeconfig context.

## Testing strategy

- Handler tests cover methods, paths, raw and Unicode bodies, repeated headers and query parameters, body limits, read errors, HEAD semantics, and concurrent requests;
- Configuration tests cover defaults, custom values, boundaries, and invalid values;
- Server tests cover invalid configuration, listener errors, and graceful cancellation;
- Pulumi mock tests validate configuration, labels, image settings, ports, probes, security settings, and the Service contract;
- `go test -race` checks application code for data races;
- GitHub Actions deploys the image to a real Kind cluster (which is ephemeral by design) and validates the response contract through `kubectl port-forward`, which resolves the Service to a Pod.

CI runs on pushes to `main` and when pull requests targeting `main` are opened, updated, or reopened. Each run tests, builds, and loads the image into Kind. Pull requests run a Pulumi preview; pushes to `main` deploy and smoke-test the application.

## Deliberate tradeoffs

This exercise intentionally avoids components that do not improve the required local workflow:

- No HTTP framework: the standard library is enough;
- No image registry: `kind load` is simpler for a local-only cluster;
- No Ingress by default: port forwarding is sufficient and keeps the Kubernetes resources portable for such a simple project;
- No Helm chart: Pulumi already owns the Deployment and Service;
- No custom Pulumi component: two resources do not justify abstraction layers;
- No HPA, PDB, database, authentication, TLS, DNS, or observability stack: these are outside the requested scope.
