# Troubleshooting

Run commands from the repository root.

## Docker permission denied

Confirm that the current user can access Docker:

```bash
docker info
```

If access to `/var/run/docker.sock` is denied, fix the host's non-root Docker configuration and begin a new login session before using Kind.

## Pulumi reports that `kind-echo` does not exist

The Kind cluster has not been created or its kubeconfig context is unavailable:

```bash
kind get clusters
kubectl config get-contexts
kubectl --context kind-echo cluster-info
```

Create the `echo` cluster before running `pulumi preview` or `pulumi up`.

## Pulumi local backend does not exist

Create the directory before logging in:

```bash
mkdir -p .pulumi-state
pulumi login "file://${PWD}/.pulumi-state"
```

## Pulumi reports `incorrect passphrase`

The supplied `PULUMI_CONFIG_PASSPHRASE`, or the value entered at the prompt, does not match the encryption salt in the selected stack configuration. Use the shared exercise passphrase documented in the [Passphrase section](../SETUP.md#passphrase). Do not edit `encryptionsalt` without deliberately migrating the stack's secrets provider.

## Pod reports `ErrImageNeverPull`

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

## Pulumi reports no changes after rebuilding

An image tag is only a string in the Deployment specification. Rebuilding the same tag does not cause a rollout. Use a unique image tag as shown in [Updating the local image](./OPERATIONS.md#updating-the-local-image), or explicitly restart the Deployment after loading the replacement image:

```bash
kubectl --context kind-echo rollout restart deployment/echo-server
kubectl --context kind-echo rollout status deployment/echo-server --timeout=120s
```

## The Pod does not become ready

Inspect its status, Kubernetes events, and application logs:

```bash
kubectl --context kind-echo describe pod \
  --selector app.kubernetes.io/name=echo-server
kubectl --context kind-echo get events \
  --sort-by=.metadata.creationTimestamp
kubectl --context kind-echo logs deployment/echo-server
```

## Port `8080` is already in use

Use another local port without changing Kubernetes:

```bash
kubectl --context kind-echo port-forward service/echo-server 18080:80
```

## The stack already exists

Select it rather than initializing it again:

```bash
pulumi -C infra stack select local
```
