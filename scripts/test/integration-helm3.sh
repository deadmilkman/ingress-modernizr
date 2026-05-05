#!/usr/bin/env sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
cd "${repo_root}"

go build -o ./dist/ingress-modernizr main.go

out_file=$(mktemp)
args_file=$(mktemp)
trap 'rm -f "${out_file}" "${args_file}"' EXIT

INGRESS2GATEWAY_BIN="${repo_root}/scripts/test/fake-ingress2gateway.sh" \
ING2GW_ARGS_FILE="${args_file}" \
helm template sample ./samples/sample \
  --post-renderer ./dist/ingress-modernizr \
  --post-renderer-args=--providers=ingress-nginx \
  >"${out_file}"

if grep -q '^kind: Ingress$' "${out_file}"; then
  echo "unexpected Ingress kind found in Helm 3 post-render output" >&2
  exit 1
fi

if ! grep -q '^kind: HTTPRoute$' "${out_file}"; then
  echo "expected HTTPRoute kind not found in Helm 3 post-render output" >&2
  exit 1
fi

if ! grep -q '^kind: Service$' "${out_file}"; then
  echo "expected Service kind not found in Helm 3 post-render output" >&2
  exit 1
fi

if ! grep -q '^--providers=ingress-nginx$' "${args_file}"; then
  echo "expected provider arg not forwarded to ingress2gateway" >&2
  exit 1
fi

echo "Helm 3 integration check passed"
