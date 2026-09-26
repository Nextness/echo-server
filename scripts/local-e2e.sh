set -Eeuo pipefail

repository_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"

readonly cluster_name="echo"
readonly kube_context="kind-${cluster_name}"
readonly node_image="kindest/node:v1.37.0@sha256:a1ed56cfb0e7b93589bdf97c8cd566405a265939e3620fc4f5de89adff580ae5"
readonly image_ref="echo-server:local"
readonly stack_name="local"
readonly backend_directory="${repository_root}/.pulumi-state"
readonly backend_url="file://${backend_directory}"
readonly local_port="18080"

port_forward_pid=""
port_forward_log=""

function usage() {
  printf '%s\n' \
    'Usage: scripts/local-e2e.sh [--delete]' \
    '' \
    'Without arguments, create the local Kind environment, deploy the application,' \
    'and verify its response contract.' \
    '' \
    '  --delete  Destroy the Pulumi resources and stack, delete the Kind cluster,' \
    '            remove the local Pulumi backend, and remove echo-server images.' \
    '  --help    Show this help message.'
}

function cluster_exists() {
  kind get clusters | grep -Fxq "${cluster_name}"
}

function require_commands() {
  local command_name

  for command_name in "$@";
  do
    if ! command -v "${command_name}" >/dev/null 2>&1;
    then
      printf 'Required command not found: %s\n' "${command_name}" >&2
      exit 1
    fi
  done
}

function stop_port_forward() {
  if [[ -n "${port_forward_pid}" ]] && kill -0 "${port_forward_pid}" 2>/dev/null;
  then
    kill "${port_forward_pid}" 2>/dev/null || true
    wait "${port_forward_pid}" 2>/dev/null || true
  fi

  if [[ -n "${port_forward_log}" && -f "${port_forward_log}" ]];
  then
    rm -- "${port_forward_log}"
  fi
}

function delete_environment() {
  local -a project_image_refs

  require_commands docker kind kubectl pulumi
  export PULUMI_CONFIG_PASSPHRASE="${PULUMI_CONFIG_PASSPHRASE:-local-ci-only}"

  if [[ -d "${backend_directory}" ]];
  then
    pulumi login "${backend_url}"

    if pulumi -C "${repository_root}/infra" stack select "${stack_name}" >/dev/null 2>&1;
    then
      if ! cluster_exists;
      then
        printf 'Cannot gracefully destroy stack %s because Kind cluster %s does not exist.\n' \
          "${stack_name}" \
          "${cluster_name}" >&2
        exit 1
      fi

      pulumi -C "${repository_root}/infra" destroy --yes --stack "${stack_name}"
      pulumi -C "${repository_root}/infra" stack rm "${stack_name}" \
        --yes \
        --preserve-config \
        --remove-backups
    else
      printf 'Pulumi stack %s does not exist; skipping stack deletion.\n' "${stack_name}"
    fi

    pulumi logout "${backend_url}"
    rm -rf -- "${backend_directory}"
  else
    printf 'Pulumi backend does not exist; skipping backend deletion.\n'
  fi

  if cluster_exists;
  then
    kind delete cluster --name "${cluster_name}"
  else
    printf 'Kind cluster %s does not exist; skipping cluster deletion.\n' "${cluster_name}"
  fi

  mapfile -t project_image_refs < <(
    docker image ls \
      --filter 'reference=echo-server:*' \
      --format '{{.Repository}}:{{.Tag}}'
  )

  if ((${#project_image_refs[@]} > 0));
  then
    docker image rm "${project_image_refs[@]}"
  else
    printf 'No echo-server images exist; skipping image deletion.\n'
  fi

  printf 'Local Echo Server environment deleted.\n'
}

function wait_for_port_forward() {
  local attempt

  for ((attempt = 1; attempt <= 30; attempt++));
  do
    if ! kill -0 "${port_forward_pid}" 2>/dev/null;
    then
      printf 'kubectl port-forward stopped unexpectedly:\n' >&2
      cat "${port_forward_log}" >&2
      exit 1
    fi

    if curl --silent --fail "http://127.0.0.1:${local_port}/readyz" >/dev/null;
    then
      return
    fi

    sleep 1
  done

  printf 'Application did not become reachable through port-forward after 30 seconds:\n' >&2
  cat "${port_forward_log}" >&2
  exit 1
}

function create_and_test_environment() {
  local response

  bash "${repository_root}/scripts/check-requirements.sh"
  export PULUMI_CONFIG_PASSPHRASE="${PULUMI_CONFIG_PASSPHRASE:-local-ci-only}"

  if cluster_exists;
  then
    printf 'Using existing Kind cluster %s.\n' "${cluster_name}"
  else
    kind create cluster \
      --name "${cluster_name}" \
      --image "${node_image}"
  fi

  kubectl --context "${kube_context}" cluster-info
  kubectl --context "${kube_context}" wait \
    --for=condition=Ready \
    nodes \
    --all \
    --timeout=120s
  kubectl --context "${kube_context}" get nodes -o wide

  docker buildx build --load --tag "${image_ref}" "${repository_root}"
  docker image inspect "${image_ref}" >/dev/null
  kind load docker-image "${image_ref}" --name "${cluster_name}"
  docker exec "${cluster_name}-control-plane" crictl images | grep -F "echo-server"

  mkdir -p "${backend_directory}"
  pulumi login "${backend_url}"
  pulumi -C "${repository_root}/infra" stack select "${stack_name}" --create
  pulumi -C "${repository_root}/infra" config --stack "${stack_name}"
  pulumi -C "${repository_root}/infra" preview --diff --stack "${stack_name}"
  pulumi -C "${repository_root}/infra" up --yes --stack "${stack_name}"

  kubectl --context "${kube_context}" rollout status \
    deployment/echo-server \
    --timeout=120s
  kubectl --context "${kube_context}" wait \
    --for=condition=Ready \
    pod \
    --selector app.kubernetes.io/name=echo-server \
    --timeout=120s
  kubectl --context "${kube_context}" get deployment,service,pod -o wide
  kubectl --context "${kube_context}" logs deployment/echo-server

  port_forward_log="$(mktemp)"
  kubectl --context "${kube_context}" port-forward \
    service/echo-server \
    "${local_port}:80" \
    >"${port_forward_log}" 2>&1 &
  port_forward_pid=$!
  trap stop_port_forward EXIT

  wait_for_port_forward

  response="$(curl --silent --show-error --fail-with-body \
    --request POST \
    --header 'Content-Type: application/json' \
    --header 'X-Demo: value' \
    --data '{"message":"hello"}' \
    "http://127.0.0.1:${local_port}/demo?tag=go&tag=kind")"

  jq -e '
    .path == "/demo" and
    .body == "{\"message\":\"hello\"}" and
    .params.tag == ["go", "kind"] and
    .headers["X-Demo"] == ["value"]
  ' <<<"${response}" >/dev/null

  jq . <<<"${response}"
  printf 'Local end-to-end test passed. Run bash scripts/local-e2e.sh --delete to remove the environment.\n'
}

case "${1:-}" in
  "")
    create_and_test_environment
    ;;
  --delete)
    if (($# != 1));
    then
      usage >&2
      exit 2
    fi
    delete_environment
    ;;
  --help|-h)
    usage
    ;;
  *)
    usage >&2
    exit 2
    ;;
esac
