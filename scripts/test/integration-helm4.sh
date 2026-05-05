#!/usr/bin/env sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
cd "${repo_root}"

resolve_helm4_bin() {
  if [ -n "${HELM4_BIN:-}" ]; then
    printf '%s\n' "${HELM4_BIN}"
    return 0
  fi

  if [ "${HELM4_AUTO_DOWNLOAD:-0}" != "1" ]; then
    return 2
  fi

  helm4_version=${HELM4_VERSION:-4.1.1}
  archive="/tmp/opencode/helm-v${helm4_version}-linux-amd64.tar.gz"
  unpack_dir="/tmp/opencode/helm-v${helm4_version}-linux-amd64"
  bin_path="${unpack_dir}/linux-amd64/helm"

  if [ ! -x "${bin_path}" ]; then
    rm -rf "${unpack_dir}"
    mkdir -p "${unpack_dir}"
    curl -fsSL "https://get.helm.sh/helm-v${helm4_version}-linux-amd64.tar.gz" -o "${archive}"
    tar -xzf "${archive}" -C "${unpack_dir}"
  fi

  printf '%s\n' "${bin_path}"
}

if helm4_bin=$(resolve_helm4_bin); then
  :
else
  if [ "$?" -eq 2 ]; then
    echo "SKIP: set HELM4_BIN to a Helm 4 binary or HELM4_AUTO_DOWNLOAD=1" >&2
    exit 0
  fi
  echo "failed to resolve Helm 4 binary" >&2
  exit 1
fi

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
