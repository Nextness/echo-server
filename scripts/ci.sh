set -Eeuo pipefail

repository_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"

KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME:-echo}"
IMAGE_REF="${IMAGE_REF:-echo-server:local}"

required_commands=(go docker kind git)
for command_name in "${required_commands[@]}";
do
    if ! command -v "${command_name}" >/dev/null 2>&1;
    then
        echo "required command not found: ${command_name}" >&2
        exit 1
    fi
done

if ! kind get clusters | grep -Fxq "${KIND_CLUSTER_NAME}";
then
  echo "Kind cluster ${KIND_CLUSTER_NAME} does not exist" >&2
  echo "Failed to create kind cluster ${KIND_CLUSTER_NAME}" >&2
  exit 1
fi

mapfile -t go_files < <(git -C "${repository_root}" ls-files '*.go')
unformatted="$(cd "${repository_root}" && gofmt -l "${go_files[@]}")"
if [[ -n "${unformatted}" ]];
then
  echo "Go files need formatting:" >&2
  echo "${unformatted}" >&2
  exit 1
fi

(
  cd "${repository_root}"
  go vet ./...
  go test -race -cover ./...
)

(
  cd "${repository_root}/infra"
  go vet ./...
  go test ./...
)

docker buildx build --pull --load --tag "${IMAGE_REF}" "${repository_root}"
kind load docker-image "${IMAGE_REF}" --name "${KIND_CLUSTER_NAME}"

echo "Loaded ${IMAGE_REF} into Kind cluster ${KIND_CLUSTER_NAME}"
