# ingress-modernizr

`ingress-modernizr` is a Helm post-renderer that consumes Kubernetes manifests from `stdin`, rewrites any `Ingress` or `IngressClass` resources into their Gateway API equivalents (a `Gateway`, `HTTPRoute`, and `GatewayClass`), and prints the updated manifest stream to `stdout`. Everything else is passed through untouched, making it simple to modernize legacy charts without editing the chart sources.

## Getting Started

```bash
# Build the renderer
go build -o ingress-modernizr ./...

# Run it as a Helm post-renderer
helm template my-release ./chart \
  --post-renderer ./ingress-modernizr \
  --post-renderer-args="--default-gateway-class=internal --gateway-suffix=-gw"
```

The renderer accepts the following flags:

| Flag | Description | Default |
| --- | --- | --- |
| `--default-gateway-class` | GatewayClass name to use when an Ingress did not specify one. | `standard` |
| `--gateway-suffix` | Suffix appended to generated Gateway names. | `-gateway` |

## How It Works

1. Every manifest document is parsed from `stdin`.
2. `IngressClass` objects become `GatewayClass` objects that carry over metadata and controller settings.
3. Each `Ingress`:
   - Turns into a namespaced `Gateway` (one per Ingress) with listeners derived from host and TLS settings.
   - Produces a matching `HTTPRoute` that preserves rules, hostnames, and default backends.
   - Copies metadata and adds `ingress-modernizr` markers so they can be traced back to the source Ingress.
4. Non-Ingress resources are emitted unchanged.

The tool exits with an error if an Ingress cannot be converted (for example, when a backend uses a named Service port that cannot be mapped to a numeric port). This fail-fast behavior prevents Helm from applying partially converted resources.
****
> Helm post renderer tool that replaces all Ingress-related resources with Kubernetes Gateway API equivalents.

⚠️ **VERY EXPERIMENTAL. DO NOT USE IN PRODUCTION.** ⚠️

---

## What is this?

`ingress-modernizr` is an experimental Helm post-renderer that attempts to automatically convert Kubernetes Ingress resources into [Gateway API](https://gateway-api.sigs.k8s.io/) resources. This is an early-stage experiment aiming to explore the feasibility of automated migration.

## Status

* 🚧 Alpha quality
* 🧪 Designed for experimentation and discussion
* ❌ Not safe or suitable for production use
* 🔒 No guarantees of correctness, stability, or support

## Goals

* Help explore automated migration paths from Ingress to Gateway API
* Simplify experimentation with Helm charts that still use Ingress
* Encourage discussion and collaboration around next-gen Kubernetes networking

## Usage

Run Helm with the post-renderer:

```bash
helm install my-app ./my-chart \
  --post-renderer ./ingress-modernizr
```

**Note:** This assumes you have built or downloaded the `ingress-modernizr` binary and made it executable.

## Contributing

PRs and discussions are welcome. If you have ideas, improvements, or just want to explore this migration path together, jump in.

We welcome contributions under the spirit of experimentation and learning.

## License

[MIT](LICENSE)

---

**Again, do NOT use this in production.** This is an experiment. It might break things. It might delete things. There are no safeguards. You have been warned.

---
