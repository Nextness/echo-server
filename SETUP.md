# Local setup

This document describes the setup required to run the project locally.

Run commands from the repository root unless a section says otherwise.

Related documents: [Operations](./docs/OPERATIONS.md), [Cleanup](./docs/CLEANUP.md), and [Troubleshooting](./docs/TROUBLESHOOTING.md).

## Prerequisites

The following table lists the versions used to run the project. If you are unsure whether your dependencies meet the requirements, run `make check-requirements`.

`make check-requirements` prints every missing, outdated, or unusable requirement and warns when the project may not work as intended. It always exits successfully, so review its output before continuing.

If you don't have `make` installed in your system, you can use bash directly with the following command `bash scripts/check-requirements.sh`.

| Component                      | Validated Version |
| :----------------------------- | :---------------- |
| Bash                           | `5.3.15`          |
| GNU Make                       | `4.4.1`           |
| Go                             | `1.27.1`          |
| Docker Engine                  | `29.8.1`          |
| Docker Buildx                  | `v0.37.1`         |
| Kind                           | `v0.33.0`         |
| Kubernetes node                | `v1.37.0`         |
| kubectl                        | `v1.37.1`         |
| Pulumi CLI                     | `3.265.0`         |
| Pulumi Go SDK                  | `v3.265.0`        |
| Pulumi Kubernetes SDK/provider | `v4.34.2`         |
| Git                            | `2.55.0`          |
| curl                           | `8.21.0`          |
| jq                             | `1.8.2`           |

> [!NOTE]
> - The Kubernetes node image is pinned to: `kindest/node:v1.37.0@sha256:a1ed56cfb0e7b93589bdf97c8cd566405a265939e3620fc4f5de89adff580ae5`. Kind `v0.33.0` does not publish a `v1.37.1` node image, so the node stays on `v1.37.0` while `kubectl v1.37.1` remains within the supported one-minor version skew.
> - Kind must be able to use Docker as the current user. If the script reports permission denied for `/var/run/docker.sock`, configure non-root Docker access and start a new login session before continuing.
> - Avoid mixing `sudo docker` with non-sudo Kind and kubectl commands because that can create resources and configuration under different users.

<details>

<summary>Optional local checks</summary>

You may run formatting checks, vet, and all tests with race detection and coverage to make sure the project is up to date. Alternatively, you may run only the formatting check.

```bash
make test
# or
make fmt-check
```

</details>

## 1. Create the Kind cluster

Pulumi deploys resources into an existing Kubernetes cluster, in other words it does not create the Kind cluster itself. Therefore, we must create the cluster with the reviewed, immutable node image:

```bash
kind create cluster \
  --name echo \
  --image kindest/node:v1.37.0@sha256:a1ed56cfb0e7b93589bdf97c8cd566405a265939e3620fc4f5de89adff580ae5
```

<details>

<summary>The arguments have these effects</summary>

- `kind create cluster` runs Kubernetes nodes as Docker containers;
- `--name echo` names the cluster and creates the kubeconfig context `kind-echo`;
- `--image` selects Kubernetes `v1.37.0` instead of a floating Kind default;
- The digest pins the exact node-image contents for reproducibility.

</details>

Once we create the cluster, we can verify the cluster and kubeconfig:

```bash
kind get clusters
kubectl config current-context
kubectl --context kind-echo cluster-info
kubectl --context kind-echo wait --for=condition=Ready \
  nodes --all --timeout=120s
kubectl --context kind-echo get nodes -o wide
```

<details>

<summary>Expected results</summary>

- `kind get clusters` includes `echo`.
- `kubectl config current-context` prints `kind-echo`.
- The wait command succeeds.
- The control-plane node is reported as `Ready`.

The wait command may return immediately when the node is already ready as it waits for a condition, not for a fixed duration.

</details>

## 2. Build and load the application image

Using the `Dockerfile` provided in the project, we need to create the image we will use for the application to run in the cluster:

```bash
docker buildx build --load --tag echo-server:local .
```

Verify the image in the host Docker daemon:

```bash
docker image inspect echo-server:local
```

<details>

<summary>Optionally, run the image locally in a dedicated terminal</summary>

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

</details>

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

The committed `encryptionsalt` in `infra/Pulumi.local.yaml` is not itself a password. Do not delete or edit it to rotate the passphrase for an existing stack. **A production repository should use a unique, undisclosed passphrase and a supported Pulumi secrets-provider migration process**.

## 4. Preview and deploy

Select or create the committed local stack:

```bash
pulumi -C infra stack select local --create
```

Review its non-secret configuration:

```bash
pulumi -C infra config --stack local
```

<details>

<summary>The expected values are</summary>

```text
kubernetes:context   kind-echo
echo-infra:image     echo-server:local
echo-infra:replicas  1
```

</details>

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

<details>

<summary>The expected configuration is</summary>

- Deployment and Service named `echo-server`.
- One ready replica by default.
- ClusterIP Service port `80` targeting the named container port `http` on `8080`.
- Container image matching the value configured in Pulumi.

</details>

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

This request travels through the local port-forward and directly to a Deployment Pod that the Service selected. It verifies the deployed Pod, the handler, and that the Service resolves to a ready endpoint, but it does not exercise ClusterIP routing or Service load balancing. Testing those would require an in-cluster client.

If port `8080` is already occupied, change only the host side of the mapping:

```bash
kubectl --context kind-echo port-forward service/echo-server 18080:80
```

Then send the request to `http://127.0.0.1:18080`.

## End-to-end automation

To automate the complete local workflow after installing the prerequisites, run:

```bash
make test-e2e
```

After using this automated workflow, remove the project environment with:

```bash
make test-e2e E2E_ARGS=--delete
```

This also restores the image setting saved before the first E2E run. Use this command instead of manual teardown after running E2E; see [Cleanup](./docs/CLEANUP.md) for details and deletion scope.
