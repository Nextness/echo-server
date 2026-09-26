set -Eeuo pipefail

repository_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"

mapfile -t go_files < <(git -C "${repository_root}" ls-files '*.go')
if (( ${#go_files[@]} == 0 )); then
  echo "No tracked Go files found." >&2
  exit 1
fi

unformatted="$(cd "${repository_root}" && gofmt -l "${go_files[@]}")"

if [[ -n "${unformatted}" ]]; then
  echo "Go files need formatting:" >&2
  echo "${unformatted}" >&2
  exit 1
fi

echo "All tracked Go files are formatted."
