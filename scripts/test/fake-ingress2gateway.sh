#!/usr/bin/env sh
set -eu

if [ -z "${ING2GW_ARGS_FILE:-}" ]; then
  echo "ING2GW_ARGS_FILE is required" >&2
  exit 1
fi

printf '%s\n' "$@" >"${ING2GW_ARGS_FILE}"

cat <<'EOF'
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: converted-from-fake
spec:
  hostnames:
    - sample.local
EOF
