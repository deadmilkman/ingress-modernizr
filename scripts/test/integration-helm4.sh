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
hook_chart_dir=$(mktemp -d)
hook_out_file=$(mktemp)
trap 'rm -rf "${env_root}" "${hook_chart_dir}"; rm -f "${out_file}" "${args_file}" "${hook_out_file}"' EXIT

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

cp "${repo_root}/samples/sample/Chart.yaml" "${hook_chart_dir}/Chart.yaml"
cp "${repo_root}/samples/sample/values.yaml" "${hook_chart_dir}/values.yaml"
mkdir -p "${hook_chart_dir}/templates"
cp -R "${repo_root}/samples/sample/templates/." "${hook_chart_dir}/templates/"

cat >"${hook_chart_dir}/templates/ingress-hook.yaml" <<'EOF'
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: {{ include "sample.fullname" . }}-hook-ingress
  annotations:
    helm.sh/hook: pre-install
spec:
  rules:
    - host: hook.local
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: {{ include "sample.fullname" . }}
                port:
                  number: {{ .Values.service.port }}
EOF

HELM_CONFIG_HOME="${config_home}" \
HELM_CACHE_HOME="${cache_home}" \
HELM_DATA_HOME="${data_home}" \
INGRESS2GATEWAY_BIN="${repo_root}/scripts/test/fake-ingress2gateway.sh" \
ING2GW_ARGS_FILE="${args_file}" \
${helm4_bin} template sample "${hook_chart_dir}" \
  --post-renderer ingress-modernizr \
  --post-renderer-args=--providers=ingress-nginx \
  >"${hook_out_file}"

if grep -q '^kind: Ingress$' "${hook_out_file}"; then
  echo "unexpected Ingress kind found in Helm 4 hook coverage output" >&2
  exit 1
fi

if ! grep -q 'helm.sh/hook: test' "${hook_out_file}"; then
  echo "expected non-Ingress test hook annotation not found" >&2
  exit 1
fi

if grep -q 'sample-hook-ingress' "${hook_out_file}"; then
  echo "unexpected Ingress hook object name found after post-render" >&2
  exit 1
fi

echo "Helm 4 integration check passed"
