# GPU_Orchestrator

A production-grade Kubernetes GPU Workload Operator built with Go and controller-runtime.

## Overview

**GPU_Orchestrator** is a custom Kubernetes controller that manages GPU workload scheduling across GPU-enabled nodes. It provides:

- **GPUWorkload CRD**: Define GPU workload requirements in Kubernetes-native YAML
- **Pluggable Scheduling Strategies**: LeastLoaded, Random, and CostOptimized scheduling algorithms
- **Exponential Backoff Retry Logic**: Automatic retry with configurable policies
- **Prometheus Metrics**: Full observability into scheduling behavior
- **Production-Ready**: Follows controller-runtime best practices and idiomatic Go patterns

## Features

- 🎯 Custom Resource Definition (`GPUWorkload`) for declarative GPU workload management
- 🔀 Multiple scheduling strategies with pluggable architecture
- 📊 Prometheus metrics for monitoring and alerting
- ⏱️ Exponential backoff with jitter for intelligent retry behavior
- 🧪 Comprehensive unit and integration tests
- 📚 Full documentation and examples
- 🔐 RBAC configuration included

## Quick Start

### Prerequisites

- Kubernetes 1.24+ cluster
- `kubectl` configured to access your cluster
- Go 1.22+ (for building from source)
- A GPU-enabled node with NVIDIA drivers and `nvidia.com/gpu` resources available

### Installation

1. **Clone the repository:**
   ```bash
   git clone https://github.com/reyisjones/GPU_Orchestrator.git
   cd GPU_Orchestrator
   ```

2. **Install the CRD and controller:**
   ```bash
   kubectl apply -k config/crd
   kubectl apply -k config/rbac
   kubectl apply -k config/manager
   ```

3. **Verify the controller is running:**
   ```bash
   kubectl get deployment -n gpu-orchestrator-system
   kubectl logs -n gpu-orchestrator-system deployment/gpu-orchestrator-controller-manager
   ```

### Create a Sample GPU Workload

```bash
kubectl apply -f examples/gpuworkload-sample.yaml
```

Monitor the workload:
```bash
kubectl get gpuworkloads
kubectl describe gpuworkload my-model
kubectl logs -l app=my-model
```

## Architecture

The controller implements a standard Kubernetes reconciliation pattern:

```
GPUWorkload CRD (API)
    ↓
Reconciler (watches GPUWorkload objects)
    ↓
Scheduling Strategy (selects best node)
    ↓
Job/Pod Creation (with GPU resource requests)
    ↓
Status Update (phase, assigned node, metrics)
```

For a detailed architecture diagram and component descriptions, see [docs/architecture.md](docs/architecture.md).

## Project Structure

```
gpu-orchestrator/
├── api/v1alpha1/              # CRD types and API group definitions
├── controllers/               # Reconciler logic
├── internal/
│   ├── scheduling/            # Pluggable scheduling strategies
│   ├── metrics/               # Prometheus metrics
│   └── backoff/               # Exponential backoff utilities
├── config/                    # Kubernetes manifests
│   ├── crd/                   # Custom Resource Definition
│   ├── rbac/                  # ServiceAccount, Role, RoleBinding
│   └── manager/               # Controller deployment
├── examples/                  # Sample GPUWorkload manifests
├── docs/                      # Documentation and diagrams
├── tests/                     # Unit and integration tests
└── main.go                    # Entry point
```

## Configuration

### GPUWorkload Spec

```yaml
apiVersion: gpu.warp.dev/v1alpha1
kind: GPUWorkload
metadata:
  name: my-model
spec:
  modelName: "llama2"           # Name of the workload/model
  gpuCount: 2                   # Number of GPUs required
  priority: "high"              # Workload priority
  schedulingStrategy: "leastLoaded"  # Strategy for node selection
  retryPolicy:
    maxRetries: 3               # Maximum retry attempts
    backoffSeconds: 30          # Base backoff delay in seconds
```

### Scheduling Strategies

- **leastLoaded**: Selects node with most available GPU capacity
- **random**: Randomly selects a suitable node
- **costOptimized**: Prefers nodes with `gpu-orchestrator/cheap-node=true` label

## Metrics

The controller exposes Prometheus metrics on port 8080:

- `warp_gpuworkload_scheduled_total{strategy="<name>"}` - Workloads successfully scheduled
- `warp_gpuworkload_failed_total{reason="<reason>"}` - Failed scheduling attempts
- `warp_gpuworkload_retries_total` - Total retry attempts
- `warp_gpuworkload_reconcile_duration_seconds` - Reconciliation duration histogram

View metrics:
```bash
kubectl port-forward -n gpu-orchestrator-system svc/gpu-orchestrator-controller-manager-metrics 8080:8080
curl http://localhost:8080/metrics
```

## Building from Source

### Build the binary:
```bash
make build
```

### Run locally (requires kubeconfig):
```bash
make run
```

### Build Docker image:
```bash
make docker-build
```

### Run tests:
```bash
make test
```

## Development

### Run the controller locally

```bash
go run main.go
```

### Run tests with coverage

```bash
go test ./... -cover -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### Code generation (if updating CRD types)

```bash
make manifests
```

## Testing Strategy

The project includes:

- **Unit Tests**: For strategies, backoff logic, and utility functions
- **Integration Tests**: Using controller-runtime's `envtest` framework
- **Mock Tests**: For scheduling strategy selection

## Assumptions & Design Decisions

1. **GPU Detection**: Uses NVIDIA's standard `nvidia.com/gpu` resource labels and allocatable resources
2. **Job Creation**: Workloads are deployed as Kubernetes Jobs (can be extended for Pods)
3. **Node Selection**: Requires nodes to be Ready and have GPU capacity
4. **Backoff Strategy**: Exponential backoff with jitter prevents thundering herd problem
5. **Metrics**: Exposed via Prometheus on standard controller-runtime metrics endpoint

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch
3. Write tests for new features
4. Submit a pull request

## License

Apache License 2.0 - See [LICENSE](LICENSE) file for details

## Author

A complete implementation demonstrating professional-grade Kubernetes operator development.

## Resources

- [Kubernetes Operator Pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)
- [controller-runtime Documentation](https://pkg.go.dev/sigs.k8s.io/controller-runtime)
- [NVIDIA GPU Support in Kubernetes](https://kubernetes.io/docs/tasks/manage-gpus/scheduling-gpus/)
- [Prometheus Go Client](https://github.com/prometheus/client_golang)
