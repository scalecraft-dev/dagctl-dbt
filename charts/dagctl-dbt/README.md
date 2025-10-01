# dagctl-dbt Helm Chart

A Helm chart for deploying the dagctl-dbt operator - a Kubernetes-native orchestrator for dbt projects.

## Installation

### Quick Start

```bash
# Install the operator
helm install dagctl-dbt ./charts/dagctl-dbt

# Or from a git repository
helm install dagctl-dbt \
  https://github.com/scalecraft-dev/dagctl-dbt/releases/download/v0.1.0/dagctl-dbt-0.1.0.tgz
```

### With Custom Values

```bash
helm install dagctl-dbt ./charts/dagctl-dbt \
  --set image.tag=v0.1.0 \
  --set resources.limits.memory=1Gi
```

### Using a values file

```bash
helm install dagctl-dbt ./charts/dagctl-dbt -f my-values.yaml
```

## Configuration

### Common Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `replicaCount` | Number of operator replicas | `1` |
| `image.repository` | Operator image repository | `scalecraft/dagctl-dbt-operator` |
| `image.tag` | Operator image tag | `""` (uses appVersion) |
| `image.pullPolicy` | Image pull policy | `IfNotPresent` |
| `resources.limits.cpu` | CPU limit | `500m` |
| `resources.limits.memory` | Memory limit | `512Mi` |
| `resources.requests.cpu` | CPU request | `250m` |
| `resources.requests.memory` | Memory request | `256Mi` |

### Security Context

| Parameter | Description | Default |
|-----------|-------------|---------|
| `podSecurityContext.runAsNonRoot` | Run as non-root user | `true` |
| `podSecurityContext.runAsUser` | User ID | `65532` |
| `securityContext.allowPrivilegeEscalation` | Allow privilege escalation | `false` |
| `securityContext.readOnlyRootFilesystem` | Read-only root filesystem | `true` |

### Service Account

| Parameter | Description | Default |
|-----------|-------------|---------|
| `serviceAccount.create` | Create service account | `true` |
| `serviceAccount.name` | Service account name | `""` (generated) |
| `serviceAccount.annotations` | Service account annotations | `{}` |

## Usage

### Creating a DbtProject

After installing the operator, create a DbtProject resource:

```yaml
apiVersion: orchestration.scalecraft.io/v1alpha1
kind: DbtProject
metadata:
  name: analytics-prod
spec:
  git:
    repository: https://github.com/your-org/dbt-project.git
    ref: main
    path: /
  
  image: scalecraft/dagctl-dbt-demo:latest
  
  schedule: "0 */6 * * * *"  # Every 6 hours
  
  commands:
    - run
  
  env:
    - name: DBT_PROFILES_DIR
      value: /root/.dbt
  
  profilesConfigMap: dbt-profiles
  
  resources:
    requests:
      memory: "256Mi"
      cpu: "250m"
    limits:
      memory: "512Mi"
      cpu: "500m"
```

### Creating a Manual Run

Trigger a manual dbt run:

```yaml
apiVersion: orchestration.scalecraft.io/v1alpha1
kind: DbtRun
metadata:
  name: manual-run-$(date +%s)
spec:
  projectRef:
    name: analytics-prod
  type: Manual
  commands:
    - run
    - --select
    - +my_model+
```

## ArgoCD Integration

To deploy with ArgoCD:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: dagctl-dbt-operator
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/scalecraft-dev/dagctl-dbt.git
    targetRevision: main
    path: charts/dagctl-dbt
    helm:
      values: |
        image:
          tag: v0.1.0
        resources:
          limits:
            memory: 1Gi
  destination:
    server: https://kubernetes.default.svc
    namespace: dagctl-dbt
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
      - CreateNamespace=true
```

## Uninstallation

```bash
helm uninstall dagctl-dbt
```

Note: This will NOT delete CRDs or any DbtProject/DbtRun resources you created.

## Development

### Testing the Chart

```bash
# Lint the chart
helm lint ./charts/dagctl-dbt

# Dry run installation
helm install dagctl-dbt ./charts/dagctl-dbt --dry-run --debug

# Template rendering
helm template dagctl-dbt ./charts/dagctl-dbt
```

## Support

- GitHub: https://github.com/scalecraft-dev/dagctl-dbt
- Issues: https://github.com/scalecraft-dev/dagctl-dbt/issues
