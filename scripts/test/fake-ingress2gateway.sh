#!/usr/bin/env sh
set -eu

if [ -z "${ING2GW_ARGS_FILE:-}" ]; then
  echo "ING2GW_ARGS_FILE is required" >&2
  exit 1
fi

printf '%s\n' "$@" >"${ING2GW_ARGS_FILE}"

if [ -n "${ING2GW_INPUT_FILE:-}" ]; then
  input_file=""
  prev=""
  for arg in "$@"; do
    if [ "${prev}" = "--input-file" ]; then
      input_file="${arg}"
      break
    fi
    prev="${arg}"
  done

  if [ -n "${input_file}" ]; then
    cp "${input_file}" "${ING2GW_INPUT_FILE}"
  fi
fi

cat <<'EOF'
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: converted-from-fake
spec:
  hostnames:
    - sample.local
EOF
