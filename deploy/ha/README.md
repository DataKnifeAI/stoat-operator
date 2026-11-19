# Stoat HA data-plane operators

App-tier HA is `spec.scaling` in the Stoat CR. Data-plane HA is **bring-your-own
clusters**, provisioned by dedicated operators (not by stoat-operator itself).

## Operators installed (cluster)

| Component | Operator | Namespace | CRD |
|-----------|----------|-----------|-----|
| MongoDB | [Percona Operator for MongoDB](https://docs.percona.com/percona-operator-for-mongodb/) `psmdb-operator` 1.23.x | `percona-system` | `PerconaServerMongoDB` |
| Redis | [OpsTree redis-operator](https://github.com/OT-CONTAINER-KIT/redis-operator) 0.27.x | `redis-operator` | `RedisReplication` (+ optional `RedisSentinel`) |
| RabbitMQ | [RabbitMQ Cluster Operator](https://www.rabbitmq.com/kubernetes/operator/install-operator) | `rabbitmq-system` | `RabbitmqCluster` |
| TLS for RabbitMQ | [cert-manager](https://cert-manager.io/) | `cert-manager` | (required by Cluster Operator ≥2.20) |

**Not used:** official Valkey operator (still “not production”), Spotahome redis-operator (archived), MongoDB Community Operator (weaker backups vs Percona).

Object storage stays external S3 (RustFS) via `dependencies.objectStorage`.

## Stoat instance CRs

Manifests under `stoat-data/` create HA clusters **in the `stoat` namespace**:

- `stoat-mongo` — 3-member Percona replica set `rs0` (no sharding)
- `stoat-redis` — Redis replication (1 primary + 2 replicas); Stoat uses the master Service URL
- `stoat-rabbit` — 3-node RabbitMQ cluster (`rabbitmq.com/legacy-startup-probe` for 4.1.x)

Apply:

```bash
kubectl apply -f deploy/ha/stoat-data/
```

### Create connection URI secrets

After clusters are ready:

```bash
# Mongo (app user from stoat-ha-mongo-app)
MONGO_PASS=$(kubectl -n stoat get secret stoat-ha-mongo-app -o jsonpath='{.data.password}' | base64 -d)
kubectl -n stoat create secret generic stoat-mongo-uri \
  --from-literal=uri="mongodb://stoat:${MONGO_PASS}@stoat-mongo-rs0.stoat.svc.cluster.local/revolt?replicaSet=rs0&authSource=revolt" \
  --dry-run=client -o yaml | kubectl apply -f -

# Redis (OpsTree master Service)
REDIS_PASS=$(kubectl -n stoat get secret stoat-ha-redis -o jsonpath='{.data.password}' | base64 -d)
kubectl -n stoat create secret generic stoat-redis-uri \
  --from-literal=uri="redis://:${REDIS_PASS}@stoat-redis-master.stoat.svc.cluster.local:6379/" \
  --dry-run=client -o yaml | kubectl apply -f -

# Rabbit (copy Cluster Operator default-user connection string)
AMQP=$(kubectl -n stoat get secret stoat-rabbit-default-user -o jsonpath='{.data.connection_string}' | base64 -d)
kubectl -n stoat create secret generic stoat-rabbit-uri \
  --from-literal=uri="$AMQP" \
  --dry-run=client -o yaml | kubectl apply -f -
```

| Secret | Use |
|--------|-----|
| `stoat-mongo-uri` | Full `mongodb://…/?replicaSet=rs0` for Stoat |
| `stoat-redis-uri` | `redis://:pass@stoat-redis-master:6379/` |
| `stoat-rabbit-uri` | AMQP URI (host + user/pass parsed into Revolt.toml) |

Point the Stoat CR at them (operator reads `secretRef` into Revolt.toml / admin env; inline `uri` remains a fallback):

```yaml
spec:
  dependencies:
    embed: true   # still ok until cutover; per-dep ref skips that embed
    mongodb:
      secretRef: { name: stoat-mongo-uri, key: uri }
    redis:
      secretRef: { name: stoat-redis-uri, key: uri }
    rabbitmq:
      secretRef: { name: stoat-rabbit-uri, key: uri }
```

Sample: `config/samples/stoat_ha_external_deps.yaml`.

## Cutover note

Embedded `database` / `redis` / `rabbit` Deployments hold live data today.
Provision the HA clusters first, **migrate Mongo** (`mongodump`/`mongorestore` or
Percona backup), then flip `dependencies.*.secretRef` and scale embedded deps
off. Do not delete embedded PVCs until the new cluster is verified.

Exact patch once URIs exist and Mongo is migrated (does **not** change live
traffic until applied):

```bash
kubectl -n stoat patch stoat stoat --type=merge -p '
spec:
  dependencies:
    mongodb:
      secretRef:
        name: stoat-mongo-uri
        key: uri
    redis:
      secretRef:
        name: stoat-redis-uri
        key: uri
    rabbitmq:
      secretRef:
        name: stoat-rabbit-uri
        key: uri
'
```

Setting those refs causes the operator to stop reconciling the matching embed
Deployments; delete the old `database` / `redis` / `rabbit` Deployments (and
only later their PVCs) after verifying the new clusters.
