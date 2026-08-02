# Kubernetes sketches (Milestone 12)

Teaching manifests for API, worker, and migrate Job. Dependencies (Postgres, MinIO, Redpanda) are assumed to exist in-cluster or via Compose — these files focus on **probes, ConfigMap/Secret, and Job semantics**.

## Apply order

```bash
# build images (tags match manifests)
docker build -f deploy/docker/Dockerfile.api -t bugsathi/api:latest .
docker build -f deploy/docker/Dockerfile.worker -t bugsathi/worker:latest .
docker build -f deploy/docker/Dockerfile.migrate -t bugsathi/migrate:latest .

kubectl apply -f deploy/k8s/namespace.yaml
kubectl apply -f deploy/k8s/configmap.yaml
# prefer: kubectl create secret ... (see secret.example.yaml comments)
kubectl apply -f deploy/k8s/secret.example.yaml
kubectl apply -f deploy/k8s/migrate-job.yaml
kubectl wait --for=condition=complete job/bugsathi-migrate -n bugsathi --timeout=120s
kubectl apply -f deploy/k8s/api-deployment.yaml
kubectl apply -f deploy/k8s/worker-deployment.yaml
```

## Probe contract

| Probe | Path | Meaning |
|-------|------|---------|
| Liveness | `/healthz` | Process up (cheap) |
| Readiness | `/readyz` | Dependencies reachable (Postgres) |

Compose production file uses the same paths on container healthchecks.
