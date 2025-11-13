# ingress-modernizr
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
