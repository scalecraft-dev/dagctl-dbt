# dagctl-dbt

Kubernetes operator for orchestrating dbt data pipelines. Part of the dagctl ecosystem - native K8s scheduling, GitOps-friendly, and open source alternative to dbt Cloud.

## Features

- **Kubernetes Native**: Built using controller-runtime and Kubebuilder
- **GitOps Ready**: Declarative configuration through Custom Resource Definitions
- **Flexible Scheduling**: Cron-based scheduling with customizable intervals  
- **Git Integration**: Clone repositories with support for SSH and HTTPS authentication
- **Resource Management**: Configure CPU/memory limits and requests
- **History Management**: Automatic cleanup of completed jobs
- **Multi-environment Support**: Run different dbt profiles and targets

## Architecture

The operator manages two main Custom Resource Definitions (CRDs):

1. **DbtProject**: Defines a dbt project with git repository, schedule, and execution configuration
2. **DbtRun**: Represents a single execution of a dbt project (created automatically or manually)

## Installation

### Prerequisites

- Kubernetes cluster (1.21+)
- kubectl configured to access your cluster
- Helm 3.x

### Via Helm Chart (Recommended)

```bash
# Install from source
helm install dagctl-dbt charts/dagctl-dbt \
  --namespace dagctl-dbt \
  --create-namespace

# Or install with custom values
helm install dagctl-dbt charts/dagctl-dbt \
  --namespace dagctl-dbt \
  --create-namespace \
  --set image.tag=0.1.0 \
  --set resources.limits.memory=512Mi
```

### Via kubectl

```bash
# Install CRDs
make install

# Deploy operator
make deploy IMG=scalecraft/dagctl-dbt-operator:0.1.0
```

## Usage

### Create a DbtProject

```yaml
apiVersion: orchestration.scalecraft.io/v1alpha1
kind: DbtProject
metadata:
  name: analytics-dbt
  namespace: default
spec:
  git:
    repository: https://github.com/example/analytics-dbt.git
    ref: main
    path: transform
  
  schedule: "0 0 */6 * * *"  # Run every 6 hours (6 fields: seconds minutes hours day month weekday)
  
  image: scalecraft/dagctl-dbt-demo:latest
  
  profilesConfigMap: dbt-profiles  # ConfigMap with dbt profiles.yml
  
  commands:
    - run
    - --models
    - staging
  
  resources:
    requests:
      memory: "512Mi"
      cpu: "500m"
    limits:
      memory: "2Gi"
      cpu: "2000m"
```

Apply the configuration:

```bash
kubectl apply -f config/samples/orchestration_v1alpha1_dbtproject.yaml
```

### Manual Runs

Create a manual run by creating a DbtRun resource:

```yaml
apiVersion: orchestration.scalecraft.io/v1alpha1
kind: DbtRun
metadata:
  name: manual-run
spec:
  projectRef:
    name: analytics-dbt
  type: Manual
  commands:
    - test
```

### Monitor Status

```bash
# List projects
kubectl get dbtprojects

# Check run status
kubectl get dbtruns

# View logs
kubectl logs -l job-name=<run-name>-job
```

## Configuration

### Git Authentication

#### SSH Authentication

Create a secret with your SSH private key:

```bash
kubectl create secret generic git-ssh-key \
  --from-file=ssh-privatekey=~/.ssh/id_rsa \
  --from-file=known_hosts=~/.ssh/known_hosts
```

Reference in your DbtProject:

```yaml
spec:
  git:
    repository: git@github.com:example/repo.git
    sshKeySecret: git-ssh-key
```

#### HTTPS Authentication

Create a secret with username and token:

```bash
kubectl create secret generic git-auth \
  --from-literal=username=myuser \
  --from-literal=password=ghp_xxxxxxxxxxxx
```

Reference in your DbtProject:

```yaml
spec:
  git:
    repository: https://github.com/example/repo.git
    authSecret: git-auth
```

### dbt Profiles

Create a ConfigMap with your dbt profiles.yml:

```bash
kubectl create configmap dbt-profiles --from-file=profiles.yml
```

Example profiles.yml:

```yaml
default:
  outputs:
    prod:
      type: postgres
      host: postgres.default.svc.cluster.local
      port: 5432
      user: dbt
      password: "{{ env_var('DBT_PASSWORD') }}"
      dbname: analytics
      schema: public
      threads: 4
  target: prod
```

### Supported dbt Adapters

Use the appropriate dbt image for your data warehouse:

- **DuckDB**: `scalecraft/dagctl-dbt-demo:latest` (example)
- **PostgreSQL**: `ghcr.io/dbt-labs/dbt-postgres:1.7.0`
- **BigQuery**: `ghcr.io/dbt-labs/dbt-bigquery:1.7.0`  
- **Snowflake**: `ghcr.io/dbt-labs/dbt-snowflake:1.7.0`
- **Redshift**: `ghcr.io/dbt-labs/dbt-redshift:1.7.0`
- **Databricks**: `ghcr.io/dbt-labs/dbt-databricks:1.7.0`

Or build your own image with your dbt project and required adapter.

## Development

### Prerequisites

- Go 1.22+
- Kubebuilder 3.x
- Docker
- Kind or Minikube (for local testing)

### Local Development

```bash
# Install dependencies
go mod download

# Generate CRDs
make manifests

# Run locally
make run

# Run tests
make test
```

### Building

```bash
# Build binary
make build

# Build Docker image
make docker-build IMG=myrepo/dagctl-dbt:latest

# Push Docker image
make docker-push IMG=myrepo/dagctl-dbt:latest
```

## Roadmap

### Phase 1: Core Features (Current)
- [x] Basic scheduling and execution
- [x] Git integration
- [x] Resource management
- [ ] Webhook triggers
- [ ] Slack/email notifications
- [ ] Artifact storage (manifests, docs)

### Phase 2: Advanced Features
- [ ] DAG visualization
- [ ] Web UI
- [ ] Multi-tenancy improvements
- [ ] Advanced scheduling (dependencies between projects)
- [ ] Integration with data catalogs
- [ ] Metrics and monitoring (Prometheus)
- [ ] dbt Cloud parity features (lineage, docs serving)

### Phase 3: Enterprise Features
- [ ] RBAC enhancements
- [ ] Audit logging
- [ ] Cost tracking
- [ ] SLA management
- [ ] Disaster recovery
- [ ] Integration with dbt Mesh

## Contributing

Contributions are welcome! Please read our [Contributing Guide](CONTRIBUTING.md) for details.

## License

Apache License 2.0

## Support

- Issues: [GitHub Issues](https://github.com/scottypate/dagctl-dbt/issues)
- Documentation: See [examples](config/samples/)

## Comparison

| Feature | dagctl-dbt | dbt Cloud | Airflow | Prefect |
|---------|------------|-----------|---------|---------|
| Kubernetes Native | ✅ | ❌ | Partial | Partial |
| GitOps | ✅ | ❌ | ❌ | ❌ |
| Self-hosted | ✅ | ❌ | ✅ | ✅ |
| Cost | Free | $100/user/month | Free | Free/Paid |
| Scheduling | ✅ | ✅ | ✅ | ✅ |
| Web UI | Roadmap | ✅ | ✅ | ✅ |
| Multi-cloud | ✅ | ✅ | ✅ | ✅ |
| dbt-native | ✅ | ✅ | Via plugins | Via plugins |
