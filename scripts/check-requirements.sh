set -Eeuo pipefail

readonly MIN_BASH_VERSION="5.3.15"
readonly MIN_MAKE_VERSION="4.4.1"
readonly MIN_GO_VERSION="1.26.6"
readonly MIN_DOCKER_VERSION="29.8.1"
readonly MIN_DOCKER_BUILDX_VERSION="0.37.1"
readonly MIN_KIND_VERSION="0.33.0"
readonly MIN_KUBECTL_VERSION="1.37.0"
readonly MIN_PULUMI_VERSION="3.264.0"
readonly MIN_GIT_VERSION="2.55.0"
readonly MIN_CURL_VERSION="8.21.0"
readonly MIN_JQ_VERSION="1.8.2"

failure_count=0

function extract_version() {
  local output="$1"

  if [[ "${output}" =~ ([0-9]+(\.[0-9]+)+) ]];
  then
    printf '%s\n' "${BASH_REMATCH[1]}"
    return 0
  fi

  return 1
}

function version_is_at_least() {
  local actual="$1"
  local minimum="$2"
  local index actual_part minimum_part
  local -a actual_parts minimum_parts

  IFS='.' read -r -a actual_parts <<<"${actual}"
  IFS='.' read -r -a minimum_parts <<<"${minimum}"

  for ((index = 0; index < ${#actual_parts[@]} || index < ${#minimum_parts[@]}; index++));
  do
    actual_part="${actual_parts[index]:-0}"
    minimum_part="${minimum_parts[index]:-0}"

    if ((10#${actual_part} > 10#${minimum_part}));
    then
      return 0
    fi

    if ((10#${actual_part} < 10#${minimum_part}));
    then
      return 1
    fi
  done

  return 0
}

function record_failure() {
  printf '[FAIL] %s\n' "$1" >&2
  failure_count=$((failure_count + 1))
}

function check_version() {
  local label="$1"
  local command_name="$2"
  local minimum="$3"
  local output actual
  shift 3

  if ! command -v "${command_name}" >/dev/null 2>&1;
  then
    record_failure "${label} is not installed (minimum ${minimum})"
    return
  fi

  if ! output="$("${command_name}" "$@" 2>&1)";
  then
    record_failure "could not determine ${label} version: ${output}"
    return
  fi

  if ! actual="$(extract_version "${output}")";
  then
    record_failure "could not parse ${label} version from: ${output}"
    return
  fi

  if ! version_is_at_least "${actual}" "${minimum}";
  then
    record_failure "${label} ${actual} is older than required ${minimum}"
    return
  fi

  printf '[ OK ] %s %s (minimum %s)\n' "${label}" "${actual}" "${minimum}"
}

function check_docker_daemon() {
  local output server_version

  if ! command -v docker >/dev/null 2>&1;
  then
    return
  fi

  if ! output="$(docker info --format '{{.ServerVersion}}' 2>&1)";
  then
    record_failure "Docker daemon is unavailable: ${output}"
    return
  fi

  if ! server_version="$(extract_version "${output}")";
  then
    record_failure "could not parse Docker daemon version from: ${output}"
    return
  fi

  if ! version_is_at_least "${server_version}" "${MIN_DOCKER_VERSION}";
  then
    record_failure "Docker daemon ${server_version} is older than required ${MIN_DOCKER_VERSION}"
    return
  fi

  printf '[ OK ] Docker daemon %s is reachable (minimum %s)\n' \
    "${server_version}" \
    "${MIN_DOCKER_VERSION}"
}

printf 'Checking Echo Server prerequisites...\n\n'

check_version "Bash" "bash" "${MIN_BASH_VERSION}" --version
check_version "GNU Make" "make" "${MIN_MAKE_VERSION}" --version
check_version "Go" "go" "${MIN_GO_VERSION}" version
check_version "Docker CLI" "docker" "${MIN_DOCKER_VERSION}" --version
check_version "Docker Buildx" "docker" "${MIN_DOCKER_BUILDX_VERSION}" buildx version
check_docker_daemon
check_version "Kind" "kind" "${MIN_KIND_VERSION}" version
check_version "kubectl" "kubectl" "${MIN_KUBECTL_VERSION}" version --client
check_version "Pulumi CLI" "pulumi" "${MIN_PULUMI_VERSION}" version
check_version "Git" "git" "${MIN_GIT_VERSION}" --version
check_version "curl" "curl" "${MIN_CURL_VERSION}" --version
check_version "jq" "jq" "${MIN_JQ_VERSION}" --version

printf '\n'
if ((failure_count > 0));
then
  printf 'Prerequisite check failed with %d problem(s).\n' "${failure_count}" >&2
  exit 1
fi

printf 'All prerequisites are installed, meet the minimum versions, and are usable.\n'
