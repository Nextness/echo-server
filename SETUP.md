# Local setup and operations

This document describes the setup required to run the project locally.

Run commands from the repository root unless a section says otherwise.

## Prerequisites

Install these tools before starting:

- Go
- Docker Engine
- Docker Buildx plugin
- Kind
- kubectl
- Pulumi CLI
- GNU Make, Git, Bash, curl, and jq

**Note**: The shell script uses Bash features such as arrays and `mapfile`. On Windows, use WSL2 rather than attempting to run it from Command Prompt or PowerShell.

### Tested versions

The following table lists the versions used to run the complete project. If you are unsure whether your dependencies meet the requirements, run `make check-requirements`. This command checks Bash, GNU Make, Go, the Docker CLI, Buildx and daemon, Kind, kubectl, Pulumi, Git, curl, and jq. It prints every missing, outdated, or unusable requirement and warns when the project may not work as intended; it always exits successfully, so review its output before continuing. Installed command-line tool versions may be equal to or newer than the corresponding versions in the table below.

If you don't have `make` installed in your system, you can use bash directly with the following command `bash scripts/check-requirements.sh`.

| Component                      | Version    |
| ------------------------------ | ---------- |
| Bash                           | `5.3.15`   |
| GNU Make                       | `4.4.1`    |
| Go                             | `1.27.1`   |
| Docker Engine                  | `29.8.1`   |
| Docker Buildx                  | `v0.37.1`  |
| Kind                           | `v0.33.0`  |
| Kubernetes node                | `v1.37.0`  |
| kubectl                        | `v1.37.1`  |
| Pulumi CLI                     | `3.265.0`  |
| Pulumi Go SDK                  | `v3.265.0` |
| Pulumi Kubernetes SDK/provider | `v4.34.2`  |
| Git                            | `2.55.0`   |
| curl                           | `8.21.0`   |
| jq                             | `1.8.2`    |

**Note**: The Kubernetes node image is pinned to: `kindest/node:v1.37.0@sha256:a1ed56cfb0e7b93589bdf97c8cd566405a265939e3620fc4f5de89adff580ae5`. Kind `v0.33.0` does not publish a `v1.37.1` node image, so the node stays on `v1.37.0` while `kubectl v1.37.1` remains within the supported one-minor version skew.

**Note**: Kind must be able to use Docker as the current user. If the script reports permission denied for `/var/run/docker.sock`, configure non-root Docker access and start a new login session before continuing. Avoid mixing `sudo docker` with non-sudo Kind and kubectl commands because that can create resources and configuration under different users.

### Optional local checks

You may run formatting checks, vet, and all tests with race detection and coverage to make sure the project is up to date. Alternatively, you may run only the formatting check.

```bash
make test
# or
make fmt-check
```

## 1. Create the Kind cluster

Pulumi deploys resources into an existing Kubernetes cluster, in other words it does not create the Kind cluster itself. Therefore, we must create the cluster with the reviewed, immutable node image:

```bash
kind create cluster \
  --name echo \
  --image kindest/node:v1.37.0@sha256:a1ed56cfb0e7b93589bdf97c8cd566405a265939e3620fc4f5de89adff580ae5
```

The arguments have these effects:

- `kind create cluster` runs Kubernetes nodes as Docker containers;
- `--name echo` names the cluster and creates the kubeconfig context `kind-echo`;
- `--image` selects Kubernetes `v1.37.0` instead of a floating Kind default;
- The digest pins the exact node-image contents for reproducibility.

Once we create the cluster, we can verify the cluster and kubeconfig:

```bash
kind get clusters
kubectl config current-context
kubectl --context kind-echo cluster-info
kubectl --context kind-echo wait --for=condition=Ready \
  nodes --all --timeout=120s
kubectl --context kind-echo get nodes -o wide
```

Expected results:

- `kind get clusters` includes `echo`.
- `kubectl config current-context` prints `kind-echo`.
- The wait command succeeds.
- The control-plane node is reported as `Ready`.

The wait command may return immediately when the node is already ready as it waits for a condition, not for a fixed duration.

## 2. Build and load the application image

Using the `Dockerfile` provided in the project, we need to create the image we will use for the application to run in the cluster:

```bash
docker buildx build --load --tag echo-server:local .
```

Verify the image in the host Docker daemon:

```bash
docker image inspect echo-server:local
```

Optionally, run the image locally in a dedicated terminal:

```bash
docker run --rm --name echo-server -p 8080:8080 echo-server:local
```

From another terminal, verify that it echoes the request headers, body, path, and query parameters:

```bash
curl --fail-with-body \
  --request POST \
  --header 'Content-Type: application/json' \
  --header 'X-Demo: value' \
  --data '{"message":"hello"}' \
  'http://localhost:8080/demo?tag=go&tag=kind'
```

Press `Ctrl+C` in the first terminal to stop and remove the standalone container before continuing.

The image must be loaded before deployment because the Kubernetes container uses `imagePullPolicy: Never`. Kubernetes will not download it from a registry.

Load the image into Kind:

```bash
kind load docker-image echo-server:local --name echo
```

Verify the copy in Kind's container runtime:

```bash
docker exec echo-control-plane crictl images | grep echo-server
```

## 3. Configure the local Pulumi backend

The local backend is a directory of Pulumi checkpoint data. Create it before logging in:

```bash
mkdir -p .pulumi-state
pulumi login "file://${PWD}/.pulumi-state"
```

The backend is ignored by Git and does not require a Pulumi Cloud account.

### Passphrase

Pulumi's local passphrase protects encrypted stack configuration. For this exercise, the committed local stack configuration was generated with the shared passphrase `local-ci-only` so that local development and ephemeral CI can use the same configuration. Enter it when Pulumi prompts for the passphrase. The GitHub Actions workflow sets the same value directly as an environment variable, so no repository secret is required.

The committed `encryptionsalt` in `infra/Pulumi.local.yaml` is not itself a password. Do not delete or edit it to rotate the passphrase for an existing stack. A production repository should use a unique, undisclosed passphrase and a supported Pulumi secrets-provider migration process.

## 4. Preview and deploy

Select or create the committed local stack:

```bash
pulumi -C infra stack select local --create
```

Review its non-secret configuration:

```bash
pulumi -C infra config --stack local
```

The expected values are:

```text
kubernetes:context   kind-echo
echo-infra:image     echo-server:local
echo-infra:replicas  1
```

Preview the changes before applying them:

```bash
pulumi -C infra preview --diff --stack local
```

The preview should contain one Kubernetes Deployment and one ClusterIP Service. Apply it:

```bash
make run-infra
```

The Make target reruns the prerequisite check and then executes `pulumi -C infra up --yes --stack local`. Pulumi exports the Service name and the normal port-forward command after a successful update.

## 5. Verify the deployment

Wait for the Deployment rollout and availability:

```bash
kubectl --context kind-echo rollout status \
  deployment/echo-server \
  --timeout=120s

kubectl --context kind-echo wait \
  --for=condition=Available \
  deployment/echo-server \
  --timeout=120s
```

Inspect the resulting resources and logs:

```bash
kubectl --context kind-echo get deployment,service,pod -o wide
kubectl --context kind-echo get pods \
  --selector app.kubernetes.io/name=echo-server
kubectl --context kind-echo logs deployment/echo-server
```

The expected configuration is:

- Deployment and Service named `echo-server`.
- One ready replica by default.
- ClusterIP Service port `80` targeting the named container port `http` on `8080`.
- Container image matching the value configured in Pulumi.

## 6. Access and test the service

The default Service is intentionally private to the cluster. In a dedicated terminal, use `kubectl port-forward` to resolve the Service to one of its Pods and tunnel local port `8080` directly to that Pod:

```bash
kubectl --context kind-echo port-forward \
  service/echo-server \
  8080:80
```

The command remains in the foreground. Keep it running while testing and press `Ctrl+C` when finished.

From another terminal, exercise the complete response contract:

```bash
curl --fail-with-body \
  --request POST \
  --header 'Content-Type: application/json' \
  --header 'X-Demo: value' \
  --data '{"message":"hello"}' \
  'http://127.0.0.1:8080/demo?tag=go&tag=kind' | jq
```

The response should contain these values, in addition to automatically supplied headers:

```json
{
  "headers": {
    "X-Demo": ["value"]
  },
  "params": {
    "tag": ["go", "kind"]
  },
  "body": "{\"message\":\"hello\"}",
  "path": "/demo"
}
```

This request travels through the local port-forward and directly to a Deployment Pod that the Service selected. It verifies the deployed Pod, the handler, and that the Service resolves to a ready endpoint, but it does not exercise ClusterIP routing or Service load balancing. Testing those would require an in-cluster client or the optional `NodePort` path below.

If port `8080` is already occupied, change only the host side of the mapping:

```bash
kubectl --context kind-echo port-forward service/echo-server 18080:80
```

Then send the request to `http://127.0.0.1:18080`.

## Ongoing operations

### Updating the local image

Rebuilding the same `echo-server:local` tag does not change the Deployment specification, so an existing Pod may continue running the previous image. The clearest local iteration workflow is to use a new tag:

```bash
image_ref="echo-server:dev-$(date +%s)"
docker buildx build --load --tag "${image_ref}" .
kind load docker-image "${image_ref}" --name echo
pulumi -C infra config set image "${image_ref}" --stack local
pulumi -C infra up --yes --stack local
kubectl --context kind-echo rollout status deployment/echo-server --timeout=120s
```

To return to the default tag:

```bash
pulumi -C infra config set image echo-server:local --stack local
pulumi -C infra up --yes --stack local
```

## Optional improvement

### Persistent local endpoint

Port forwarding is the recommended default because it keeps the Kubernetes resources portable and exposes the service only while the command runs.

For local development on a Linux host, Pulumi can expose the Service through a persistent `NodePort`. This option is not enabled in the repository. Change the Service specification in `infra/app.go` to:

```go
Spec: corev1.ServiceSpecArgs{
    Type: pulumi.String("NodePort"),
    Selector: labels,
    Ports: corev1.ServicePortArray{
        corev1.ServicePortArgs{
            Name:       pulumi.String("http"),
            Port:       pulumi.Int(80),
            TargetPort: pulumi.String("http"),
            NodePort:   pulumi.Int(30080),
        },
    },
},
```

Apply the Pulumi change:

```bash
pulumi -C infra up --yes --stack local
```

Retrieve the Kind node IP and access the Service without `kubectl port-forward`:

```bash
node_ip="$(
  kubectl --context kind-echo get node echo-control-plane \
    -o jsonpath='{.status.addresses[?(@.type=="InternalIP")].address}'
)"

curl --fail-with-body "http://${node_ip}:30080/demo?tag=go&tag=kind" | jq
```

The endpoint remains available while the Service and Kind cluster are running. Unlike port forwarding, this path enters through the node and is routed by kube-proxy to the Service's ClusterIP, so it does exercise ClusterIP routing. This relies on the Linux host being able to route to the Docker bridge address used by the Kind node. Environments that cannot reach that address should continue using port forwarding. Binding specifically to `127.0.0.1` would still require a Kind host-port mapping outside the current Pulumi-managed Kubernetes resources. A real cloud environment would normally use an Ingress or `LoadBalancer` Service instead of this local NodePort arrangement.

**Note**: This is a local-development convenience rather than the recommended approach. It avoids keeping a port-forward process running, but requires resolving the Kind node IP and **only works** when the host can route to the Docker bridge network.

## GitHub Actions

The workflow creates a new Kind cluster and local Pulumi backend for each job:

- Pull requests run tests, build and load the image, and execute `pulumi preview`.
- Pushes to `main` run `pulumi up`, wait for the Deployment, and validate the response contract.
- Manual runs can select either `preview` or `apply`.

The CI environment and deployed service disappear when the runner is destroyed; this workflow is deployment validation, not a persistent environment.

### Passphrase in CI

The workflow sets `PULUMI_CONFIG_PASSPHRASE` to the documented exercise passphrase directly instead of using a repository secret. The stack contains no encrypted values and the value is already public, so there is nothing to protect. This also means fresh repository copies and pull requests from forks can run the Pulumi preview and apply steps without any repository configuration.

## Troubleshooting

### Docker permission denied

Confirm that the current user can access Docker:

```bash
docker info
```

If access to `/var/run/docker.sock` is denied, fix the host's non-root Docker configuration and begin a new login session before using Kind.

### Pulumi reports that `kind-echo` does not exist

The Kind cluster has not been created or its kubeconfig context is unavailable:

```bash
kind get clusters
kubectl config get-contexts
kubectl --context kind-echo cluster-info
```

Create the `echo` cluster before running `pulumi preview` or `pulumi up`.

### Pulumi local backend does not exist

Create the directory before logging in:

```bash
mkdir -p .pulumi-state
pulumi login "file://${PWD}/.pulumi-state"
```

### Pulumi reports `incorrect passphrase`

The supplied `PULUMI_CONFIG_PASSPHRASE`, or the value entered at the prompt, does not match the encryption salt in the selected stack configuration. Use the shared exercise passphrase documented in the Passphrase section. Do not edit `encryptionsalt` without deliberately migrating the stack's secrets provider.

### Pod reports `ErrImageNeverPull`

The image exists in the host Docker daemon but has not been copied into Kind, or Pulumi references a different tag. Compare the configured and loaded images:

```bash
pulumi -C infra config get image --stack local
docker image ls --filter 'reference=echo-server:*'
docker exec echo-control-plane crictl images | grep echo-server
```

Load the exact configured reference:

```bash
kind load docker-image echo-server:local --name echo
```

If Pulumi uses a different tag, substitute that complete tag in the load command.

### Pulumi reports no changes after rebuilding

An image tag is only a string in the Deployment specification. Rebuilding the same tag does not cause a rollout. Use a unique image tag as shown in “Updating the local image,” or explicitly restart the Deployment after loading the replacement image:

```bash
kubectl --context kind-echo rollout restart deployment/echo-server
kubectl --context kind-echo rollout status deployment/echo-server --timeout=120s
```

### The Pod does not become ready

Inspect its status, Kubernetes events, and application logs:

```bash
kubectl --context kind-echo describe pod \
  --selector app.kubernetes.io/name=echo-server
kubectl --context kind-echo get events \
  --sort-by=.metadata.creationTimestamp
kubectl --context kind-echo logs deployment/echo-server
```

### Port `8080` is already in use

Use another local port without changing Kubernetes:

```bash
kubectl --context kind-echo port-forward service/echo-server 18080:80
```

### The stack already exists

Select it rather than initializing it again:

```bash
pulumi -C infra stack select local
```

## Optional destruction and cleanup

### Graceful teardown

Reverse the setup order so Pulumi can delete Kubernetes resources while the cluster is still reachable.

First, stop the active `kubectl port-forward` with `Ctrl+C`. Then destroy the Deployment and Service:

```bash
pulumi -C infra destroy --yes --stack local
```

Kubernetes sends `SIGTERM` when deleting the Pod. The application allows up to ten seconds for active HTTP requests to finish, while Kubernetes waits fifteen seconds before forcing termination.

Confirm that the application resources are gone:

```bash
kubectl --context kind-echo get deployment,service,pod
```

Remove the Pulumi stack record and backups while preserving the tracked `infra/Pulumi.local.yaml` configuration:

```bash
pulumi -C infra stack rm local \
  --yes \
  --preserve-config \
  --remove-backups
```

Delete the Kind cluster, confirm its removal, and log out of the backend:

```bash
kind delete cluster --name echo
kind get clusters
kubectl config get-contexts
pulumi logout "file://${PWD}/.pulumi-state"
```

Kind normally removes the `kind-echo` kubeconfig context with the cluster. Deleting the cluster before `pulumi destroy` would leave Pulumi state referring to resources it can no longer reach.

After removing the stack and logging out, remove the local backend directory if complete cleanup is desired:

```bash
rm -rf -- .pulumi-state
```

### Optional Docker image cleanup

List application images created for this project:

```bash
docker image ls --filter 'reference=echo-server:*'
```

Remove every matching application tag without touching images from other projects:

```bash
mapfile -t project_image_refs < <(
  docker image ls \
    --filter 'reference=echo-server:*' \
    --format '{{.Repository}}:{{.Tag}}'
)

if (( ${#project_image_refs[@]} > 0 ));
then
  docker image rm "${project_image_refs[@]}"
fi
```

Deleting the Kind cluster removes its internal containerd image store, including the copy loaded with `kind load`. The commands above remove the separate host Docker tags.

The Kind node image is infrastructure rather than an image generated by this application. It can optionally be removed after deleting the cluster, but the next cluster creation will download it again:

```bash
docker image rm \
  kindest/node@sha256:a1ed56cfb0e7b93589bdf97c8cd566405a265939e3620fc4f5de89adff580ae5
```

Do not use `docker system prune --all --volumes` for project cleanup. It is host-wide and can remove unrelated images, build cache, networks, and persistent data.

# End to end Automation

To automate the complete local workflow after installing the prerequisites, run:

```bash
make test-e2e
```

The script creates or reuses the `echo` Kind cluster, builds and loads the application image under a unique tag (for example, `echo-server:e2e-20260926213000`), configures Pulumi with that exact reference, deploys it, and validates the response contract through `kubectl port-forward` to a Pod selected by the Service. It also checks that the Deployment references the built tag and restores the previous Pulumi image configuration when it finishes, so rebuilding the same tag cannot make the smoke test pass against a stale Pod.

Remove the project environment with:

```bash
make test-e2e E2E_ARGS=--delete
```
