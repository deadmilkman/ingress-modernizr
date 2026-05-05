#!/usr/bin/env sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
cd "${repo_root}"

helm4_bin=${HELM4_BIN:-helm}
version_out=$(${helm4_bin} version --short)
case "${version_out}" in
  v4.*)
    ;;
  *)
    echo "HELM4_BIN must point to a Helm 4 binary (got: ${version_out})" >&2
    exit 1
    ;;
esac

make plugin-layout

env_root=$(mktemp -d)
out_file=$(mktemp)
args_file=$(mktemp)
trap 'rm -rf "${env_root}"; rm -f "${out_file}" "${args_file}"' EXIT

config_home="${env_root}/config"
cache_home="${env_root}/cache"
data_home="${env_root}/data"
mkdir -p "${config_home}" "${cache_home}" "${data_home}"

HELM_CONFIG_HOME="${config_home}" \
HELM_CACHE_HOME="${cache_home}" \
HELM_DATA_HOME="${data_home}" \
${helm4_bin} plugin install ./dist/plugin/helm4

HELM_CONFIG_HOME="${config_home}" \
HELM_CACHE_HOME="${cache_home}" \
HELM_DATA_HOME="${data_home}" \
INGRESS2GATEWAY_BIN="${repo_root}/scripts/test/fake-ingress2gateway.sh" \
ING2GW_ARGS_FILE="${args_file}" \
${helm4_bin} template sample ./samples/sample \
  --post-renderer ingress-modernizr \
  --post-renderer-args=--providers=ingress-nginx \
  >"${out_file}"

if grep -q '^kind: Ingress$' "${out_file}"; then
  echo "unexpected Ingress kind found in Helm 4 plugin post-render output" >&2
  exit 1
fi

if ! grep -q '^kind: HTTPRoute$' "${out_file}"; then
  echo "expected HTTPRoute kind not found in Helm 4 plugin post-render output" >&2
  exit 1
fi

if ! grep -q '^kind: Service$' "${out_file}"; then
  echo "expected Service kind not found in Helm 4 plugin post-render output" >&2
  exit 1
fi

if ! grep -q '^--providers=ingress-nginx$' "${args_file}"; then
  echo "expected provider arg not forwarded under Helm 4 plugin" >&2
  exit 1
fi

echo "Helm 4 integration check passed"
