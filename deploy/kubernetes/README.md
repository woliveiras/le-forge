# LeForge on Kubernetes

```bash
# 1. PostgreSQL: install an operator (e.g. CloudNativePG) and create cluster "leforge-postgres",
#    or use a managed PostgreSQL; put its DSN in secret.yaml.
# 2. Edit secret.yaml (or replace it with an ExternalSecret), configmap.yaml, and the image tags.
kubectl apply -k deploy/kubernetes
# 3. Expose leforge-forged:8080 through your ingress (TLS). Do NOT route /internal/ publicly.
# 4. Agent runtimes for graph nodes that use runtime.image:
forge k8s render -f examples/graphs/software-factory.yaml --namespace leforge | kubectl apply -f -
```

See [docs/deployment.md](../../docs/deployment.md) and
[docs/production-hardening.md](../../docs/production-hardening.md).

Notes:
- The service-account tokens are not mounted: forged does not talk to the Kubernetes API.
- `isolation.mode: docker` is not available in this layout (no Docker socket); tools run in
  forged's restricted container. Per-replica tool pods are an extension point
  (`engine.SandboxFactory`). Use a sandboxed RuntimeClass (gVisor, Kata) for forged if agents get
  `shell.exec`.
- The workspace volume must be ReadWriteMany when forged has more than one replica.
