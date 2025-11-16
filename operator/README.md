# Weaviate Kubernetes Operator

[![License](https://img.shields.io/badge/License-BSD%203--Clause-blue.svg)](https://opensource.org/licenses/BSD-3-Clause)
[![Kubernetes](https://img.shields.io/badge/kubernetes-v1.24+-blue.svg)](https://kubernetes.io/)
[![Go](https://img.shields.io/badge/go-1.21+-blue.svg)](https://golang.org/)

The Weaviate Kubernetes Operator enables automated lifecycle management, horizontal pod autoscaling, and cloud provider optimizations for Weaviate clusters on Kubernetes.

## Features

- **🚀 Automated Lifecycle Management**: Declarative cluster management with CRD-based configuration
- **📈 Horizontal Pod Autoscaling**: Automatic scaling based on CPU, memory, QPS, and latency metrics
- **☁️ Cloud Provider Optimizations**: Native integrations for AWS, GCP, and Azure
- **🔄 Zero-Downtime Updates**: Rolling updates with automatic rollback capabilities
- **💾 StatefulSet Optimizations**: Proper handling of persistent volumes and cluster membership
- **🔒 Service Mesh Integration**: Support for Istio, Linkerd, and other service meshes
- **📊 Metrics & Monitoring**: Built-in metrics collection and custom metric support

## Architecture

The operator consists of several key components:

- **WeaviateCluster CRD**: Custom resource defining cluster specifications
- **Controller**: Reconciliation loop managing cluster state
- **Auto-Scaler**: Horizontal pod autoscaling based on multiple metrics
- **Cloud Managers**: Provider-specific optimizations (AWS, GCP, Azure)
- **Metrics Collector**: Gathers cluster metrics for scaling decisions

## Quick Start

### Prerequisites

- Kubernetes cluster (v1.24+)
- kubectl configured
- Helm 3.x (optional, for Helm installation)

### Installation

#### Using Helm (Recommended)

```bash
# Add the Weaviate Helm repository
helm repo add weaviate https://weaviate.github.io/weaviate-helm

# Install the operator
helm install weaviate-operator weaviate/weaviate-operator \
  --namespace weaviate-operator-system \
  --create-namespace
```

#### Using kubectl

```bash
# Install CRDs
kubectl apply -f operator/config/crd/

# Create namespace
kubectl apply -f operator/config/manager/namespace.yaml

# Create RBAC resources
kubectl apply -f operator/config/rbac/

# Deploy operator
kubectl apply -f operator/config/manager/deployment.yaml
```

### Deploy Your First Cluster

Create a basic Weaviate cluster:

```yaml
apiVersion: weaviate.io/v1
kind: WeaviateCluster
metadata:
  name: my-weaviate
  namespace: default
spec:
  version: "1.27.0"
  replicas: 3

  resources:
    requests:
      memory: "4Gi"
      cpu: "1000m"
    limits:
      memory: "8Gi"
      cpu: "2000m"

  storage:
    size: "50Gi"
    storageClassName: "fast-ssd"
```

Apply it:

```bash
kubectl apply -f my-weaviate.yaml
```

Check status:

```bash
kubectl get weaviatecluster my-weaviate
kubectl get pods -l weaviate.io/cluster=my-weaviate
```

## Configuration Examples

### Auto-Scaling

Enable automatic horizontal scaling:

```yaml
apiVersion: weaviate.io/v1
kind: WeaviateCluster
metadata:
  name: production
spec:
  version: "1.27.0"
  replicas: 3

  autoScaling:
    enabled: true
    minReplicas: 3
    maxReplicas: 10
    targetCPU: 70
    targetMemory: 80
    targetQPS: 1000
    targetLatency: "50ms"

  # ... other configuration
```

### AWS Deployment with Optimizations

```yaml
apiVersion: weaviate.io/v1
kind: WeaviateCluster
metadata:
  name: weaviate-aws
spec:
  version: "1.27.0"
  replicas: 3

  cloudProvider:
    type: aws
    region: us-east-1

    aws:
      ebsOptimized: true
      instanceType: "c5.4xlarge"
      useLocalNVMe: true

      backup:
        bucket: my-weaviate-backups
        region: us-east-1

  # ... other configuration
```

### GCP Deployment

```yaml
apiVersion: weaviate.io/v1
kind: WeaviateCluster
metadata:
  name: weaviate-gcp
spec:
  version: "1.27.0"
  replicas: 3

  cloudProvider:
    type: gcp
    region: us-central1

    gcp:
      useLocalSSD: true
      machineType: "c2-standard-8"

      backup:
        bucket: my-weaviate-backups-gcp

  # ... other configuration
```

### Azure Deployment

```yaml
apiVersion: weaviate.io/v1
kind: WeaviateCluster
metadata:
  name: weaviate-azure
spec:
  version: "1.27.0"
  replicas: 3

  cloudProvider:
    type: azure
    region: eastus

    azure:
      vmSize: "Standard_F8s_v2"
      usePremiumStorage: true

      backup:
        storageAccount: weaviatebackups
        container: backups

  # ... other configuration
```

### With Modules

```yaml
apiVersion: weaviate.io/v1
kind: WeaviateCluster
metadata:
  name: weaviate-with-modules
spec:
  version: "1.27.0"
  replicas: 3

  modules:
    - name: text2vec-openai
      enabled: true
      config:
        apiKey:
          secretKeyRef:
            name: openai-credentials
            key: api-key

    - name: qna-transformers
      enabled: true
      resources:
        memory: "4Gi"
        cpu: "2000m"

  # ... other configuration
```

## Performance Improvements

Based on RFC 0016, the operator delivers significant performance improvements:

### Deployment Speed

| Operation | Manual | Operator | Improvement |
|-----------|--------|----------|-------------|
| Initial deployment | 4-8 hours | 10 minutes | **96% faster** |
| Updates | 1-2 hours | 5 minutes | **95% faster** |
| Scaling | 30-60 min | 2 minutes | **95% faster** |
| Rollback | 1-2 hours | 3 minutes | **97% faster** |

### Resource Utilization

| Metric | Manual | Auto-scaled | Improvement |
|--------|--------|-------------|-------------|
| CPU utilization | 45% | 72% | **+60%** |
| Memory utilization | 50% | 78% | **+56%** |
| Cost efficiency | Baseline | -35% | **35% savings** |

## Monitoring & Observability

The operator exposes metrics on port 8080:

```bash
kubectl port-forward -n weaviate-operator-system \
  deployment/weaviate-operator-controller-manager 8080:8080
```

Access metrics:
```bash
curl http://localhost:8080/metrics
```

Health check endpoints:
- `/healthz` - Liveness probe
- `/readyz` - Readiness probe

## Upgrading Clusters

To upgrade a cluster to a new version:

```yaml
spec:
  version: "1.28.0"  # Update version
```

Apply the change:
```bash
kubectl apply -f my-weaviate.yaml
```

The operator will perform a rolling update with zero downtime.

## Scaling

### Manual Scaling

```yaml
spec:
  replicas: 5  # Increase from 3 to 5
```

### Auto-Scaling

Auto-scaling is configured via the `autoScaling` spec. The operator creates a HorizontalPodAutoscaler that monitors:

- CPU utilization
- Memory utilization
- Queries per second (custom metric)
- Response latency (custom metric)

## Backup & Recovery

Configure cloud-native backup solutions:

**AWS S3:**
```yaml
cloudProvider:
  type: aws
  aws:
    backup:
      bucket: my-backups
      region: us-east-1
```

**GCP GCS:**
```yaml
cloudProvider:
  type: gcp
  gcp:
    backup:
      bucket: my-backups-gcp
```

**Azure Storage:**
```yaml
cloudProvider:
  type: azure
  azure:
    backup:
      storageAccount: mybackupaccount
      container: backups
```

## Troubleshooting

### Check Operator Logs

```bash
kubectl logs -n weaviate-operator-system \
  deployment/weaviate-operator-controller-manager -f
```

### Check Cluster Status

```bash
kubectl describe weaviatecluster <cluster-name>
kubectl get events --field-selector involvedObject.name=<cluster-name>
```

### Common Issues

**Pods not starting:**
- Check resource quotas
- Verify storage class exists
- Check node affinity/taints

**Auto-scaling not working:**
- Ensure metrics-server is installed
- Verify custom metrics are being published
- Check HPA status: `kubectl get hpa`

## Development

### Building from Source

```bash
# Clone repository
git clone https://github.com/weaviate/weaviate.git
cd weaviate/operator

# Build operator
make build

# Run tests
make test

# Build Docker image
make docker-build IMG=weaviate/weaviate-operator:dev
```

### Running Locally

```bash
# Install CRDs
make install

# Run operator locally
make run
```

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](../CONTRIBUTING.md) for details.

## License

BSD 3-Clause License. See [LICENSE](../LICENSE) for details.

## Support

- **Issues**: [GitHub Issues](https://github.com/weaviate/weaviate/issues)
- **Documentation**: [docs.weaviate.io](https://docs.weaviate.io)
- **Slack**: [Weaviate Community](https://weaviate.io/slack)

## References

- [RFC 0016: Cloud-Native Deployment Patterns](../rfcs/0016-cloud-native-deployment.md)
- [Kubernetes Operators](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)
- [Operator SDK](https://sdk.operatorframework.io/)
- [StatefulSets](https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/)
