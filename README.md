# ingress-modernizr

A Helm post-renderer that automatically converts Kubernetes Ingress resources into Gateway API resources using [ingress2gateway](https://github.com/kubernetes-sigs/ingress2gateway).

`ingress-modernizr` injects itself between Helm's template output and the final apply step.
It runs `ingress2gateway` under the hood, replaces all Ingress resources, and returns a transformed manifest set containing the Gateway API equivalents (e.g., Gateway, HTTPRoute, service attachments, etc.).

This makes it possible to progressively modernize charts and clusters without modifying the charts themselves.

## ⚠️ Extremely Experimental

- This tool is VERY EXPERIMENTAL and an experiment in itself.
- Do NOT use in production.
- No warranties, guarantees, or expectations of correctness.
- APIs may break at any time.
- The output may destroy clusters, summon demons, or both.
- PRs, discussions, and contributions are welcome.

## Why?

NGINX Ingress is being retired. Gateway API is the strategic direction across major Kubernetes vendors.
Migrating existing Helm charts can be painful, especially when:
- Dozens of charts still ship Ingress resources.
- You need Gateway API today.
- You want to modernize without forking upstream charts.
- ingress-modernizr serves as a drop-in modernization layer.

## What It Does

1. Helm renders all templates.
1. Helm pipes the rendered YAML into ingress-modernizr.
1. `ingress-modernizr`:
    - Reads the entire manifest set.
    - Removes all Ingress resources.
    - Invokes ingress2gateway print with all user-supplied flags.
    - Receives Gateway API resources from ingress2gateway.
    - Appends them to the manifest set.
1. The transformed output is sent back to Helm.
1. Helm applies the Gateway API resources instead of Ingress.

Everything else (Deployments, Services, CRDs, RBAC, etc.) remains untouched.

## Installation 

### Build from source:

```bash
git clone https://github.com/deadmilkman/ingress-modernizr
cd ingress-modernizr
make build    # or `go build ./cmd/ingress-modernizr`

```

### Or install using Go:

```bash
go install github.com/deadmilkman/ingress-modernizr@latest
```

Make sure `ingress2gateway` is also available in your `$PATH`:

```bash
go install sigs.k8s.io/ingress2gateway@latest
```

You can override the binary path via:

```bash
export INGRESS2GATEWAY_BIN=/custom/path/ingress2gateway
```

## Usage With Helm

Basic example

```bash
helm upgrade --install myapp ./chart \
  --post-renderer ./ingress-modernizr \
  --post-renderer-args="--providers=ingress-nginx"
```

Example with namespace + other flags

Everything after `--post-renderer-args` is passed directly to `ingress2gateway`:

```bash
helm upgrade --install myapp ./chart \
  --post-renderer ./ingress-modernizr \
  --post-renderer-args="--namespace=apps --providers=ingress-nginx --kubeconfig=/my/kubeconfig"
```


### Provider is mandatory

`ingress2gateway` requires you to specify a provider:

- --providers=ingress-nginx
- --providers=gce
- --providers=traefik
- etc.

If you forget it, the tool will error out.

## Debugging

Inspect what Helm is giving the post-renderer

```bash
helm template myapp ./chart > before.yaml
```

Run ingress-modernizr manually

```bash
cat before.yaml \
  | ingress-modernizr --providers=ingress-nginx \
  > after.yaml
```

Now inspect after.yaml:

- No Ingress resources remain.
- Gateway, HTTPRoute, and related objects appear.

Example End-to-End

```bash
helm template demo ./demo-chart \
  | ingress-modernizr --providers=ingress-nginx \
  | kubectl apply -f -
```

## Contributing

Contributions, issues, discussions, and PRs are welcome.

## License

[MIT](https://mit-license.org/)