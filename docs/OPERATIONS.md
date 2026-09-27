# Ongoing operations

Run commands from the repository root with the local environment already deployed.

## Updating the local image

Rebuilding the same `echo-server:local` tag does not change the Deployment specification, so an existing Pod may continue running the previous image. The clearest local iteration workflow is to use a new tag:

```bash
image_ref="echo-server:dev-$(date +%s)"
docker buildx build --load --tag "${image_ref}" .
kind load docker-image "${image_ref}" --name echo
pulumi -C infra config set image "${image_ref}" --stack local
pulumi -C infra up --yes --stack local
kubectl --context kind-echo rollout status deployment/echo-server --timeout=120s
```

To return to the default tag, build and load it before updating Pulumi. This also works when the environment was created with `make test-e2e`, which only loads a uniquely tagged E2E image:

```bash
docker buildx build --load --tag echo-server:local .
kind load docker-image echo-server:local --name echo
pulumi -C infra config set image echo-server:local --stack local
pulumi -C infra up --yes --stack local
kubectl --context kind-echo rollout status deployment/echo-server --timeout=120s
```

If the Deployment already uses `echo-server:local`, follow the [restart instructions](./TROUBLESHOOTING.md#pulumi-reports-no-changes-after-rebuilding) after rebuilding and loading it.

## GitHub Actions

Every workflow run executes `scripts/ci.sh` to run tests, build the image, and load it into a new Kind cluster. Pulumi uses a local backend for that run. The trigger determines the remaining steps:

- Opening, updating, or reopening a pull request targeting `main` runs `pulumi preview`.
- Pushes to `main` run `pulumi up`, wait for the Deployment, and validate the response contract.
- Manual runs can select either `preview` or `apply`.

Pushing to a feature branch triggers CI only when it updates an open pull request targeting `main`. The workflow does not run automatically on every branch push.

The CI environment and deployed service disappear when the runner is destroyed; this workflow is deployment validation, not a persistent environment.

## Passphrase in CI

The workflow sets `PULUMI_CONFIG_PASSPHRASE` to the [documented exercise passphrase](../SETUP.md#passphrase) directly instead of using a repository secret. The stack contains no encrypted values and the value is already public, so there is nothing to protect. No repository secret configuration is required, including for previews of pull requests from forks.
