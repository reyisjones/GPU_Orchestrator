# Development Guide

## Local Development Setup

### Prerequisites

- **Go 1.22+**: [Download](https://golang.org/dl/)
- **kubectl**: [Install](https://kubernetes.io/docs/tasks/tools/)
- **Docker**: [Install](https://www.docker.com/products/docker-desktop)
- **Kubernetes Cluster**: 
  - **Kind**: `kind create cluster --name gpu-orchestrator --image kindest/node:v1.28.0`
  - **Minikube**: `minikube start --cpus=4 --memory=8192`
  - **Docker Desktop**: Enable Kubernetes in settings

### Getting Started

1. **Clone the repository**
   ```bash
   git clone https://github.com/reyisjones/gpu-orchestrator.git
   cd gpu-orchestrator
   ```

2. **Install dependencies**
   ```bash
   go mod download
   go mod tidy
   ```

3. **Run tests**
   ```bash
   make test
   ```

4. **Build the binary**
   ```bash
   make build
   ```

5. **Run controller locally (against kubeconfig)**
   ```bash
   make run
   # Or with flags:
   go run ./cmd/manager/main.go --leader-elect=false --metrics-bind-address=:8080
   ```

## Project Layout

```
gpu-orchestrator/
├── api/v1alpha1/              # CRD type definitions
├── controllers/               # Reconciler logic
├── internal/
│   ├── backoff/               # Exponential backoff utilities
│   ├── metrics/               # Prometheus metrics
│   └── scheduling/            # Scheduling strategies
├── config/                    # Kubernetes manifests
├── examples/                  # Sample GPUWorkloads
├── docs/                      # Documentation
├── scripts/                   # Helper scripts
└── main.go                    # Entry point
```

## Common Tasks

### Add a New Scheduling Strategy

1. **Create the strategy** in `internal/scheduling/strategy.go`:
   ```go
   type MyCustomStrategy struct {
       logger logr.Logger
   }
   
   func (s *MyCustomStrategy) ChooseNode(ctx context.Context, nodes []corev1.Node, gw *gpuv1alpha1.GPUWorkload) (*corev1.Node, error) {
       // Implementation
   }
   
   func (s *MyCustomStrategy) Name() string {
       return "myCustom"
   }
   ```

2. **Register in Factory**:
   ```go
   func Factory(strategyName string, logger logr.Logger) (Strategy, error) {
       // ...
       case "myCustom":
           return NewMyCustomStrategy(logger), nil
   }
   ```

3. **Add tests** in `internal/scheduling/strategy_test.go`

4. **Use in spec**:
   ```yaml
   spec:
     schedulingStrategy: myCustom
   ```

### Add a New Metric

1. **Define in** `internal/metrics/metrics.go`:
   ```go
   myNewMetric := prometheus.NewGauge(
       prometheus.GaugeOpts{
           Name: "warp_my_new_metric",
           Help: "Description of the metric",
       },
   )
   ```

2. **Register**:
   ```go
   metrics.Registry.MustRegister(myNewMetric)
   ```

3. **Record in reconciler**:
   ```go
   m := metrics.GetMetrics()
   if m != nil {
       // Record metric
   }
   ```

### Test the Controller

#### Unit Tests
```bash
# Run all tests
make test

# Run specific test
go test -run TestName ./package/...

# With verbose output
go test -v ./...

# With coverage
go test ./... -cover -coverprofile=coverage.out
go tool cover -html=coverage.out
```

#### Integration Tests
```bash
# Run with envtest (Kubernetes test environment)
KUBEBUILDER_ASSETS="..." go test -race ./controllers/...
```

#### Manual Testing
```bash
# 1. Start the controller
make run

# 2. In another terminal, create a workload
kubectl apply -f examples/gpuworkload-sample.yaml

# 3. Watch it being scheduled
kubectl get gpuworkloads -w
kubectl describe gpuworkload llama2-inference

# 4. Check logs
kubectl logs <pod-name>

# 5. View metrics
curl http://localhost:8080/metrics | grep warp_
```

## Code Style

### Formatting
```bash
go fmt ./...
```

### Linting
```bash
# Install golangci-lint if not present
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin

# Run linter
make lint
```

### Code Organization

- **Interfaces**: Define in package root (strategy.go, etc.)
- **Implementations**: In same package or subpackage
- **Tests**: In same package with `_test.go` suffix
- **Comments**: Export all public symbols
- **Error handling**: Wrap errors with context

## Debugging

### Enable debug logging
```go
// In main.go or controller
logger.V(1).Info("Debug message", "key", value)
```

### Use pprof for profiling
```bash
go run ./cmd/manager/main.go --metrics-bind-address=:6060
curl http://localhost:6060/debug/pprof/heap
```

### Inspect API objects
```bash
# Check CRD schema
kubectl explain gpuworkload.spec
kubectl explain gpuworkload.status

# Get raw YAML
kubectl get gpuworkload -o yaml
```

## Building and Publishing

### Build Docker image
```bash
make docker-build IMG=myregistry/gpu-orchestrator:v0.1.0
```

### Push image
```bash
make docker-push IMG=myregistry/gpu-orchestrator:v0.1.0
```

### Update deployment to use new image
```bash
kubectl set image deployment/gpu-orchestrator-controller-manager \
    manager=myregistry/gpu-orchestrator:v0.1.0 \
    -n gpu-orchestrator-system
```

## Performance Tips

1. **Watch Selectors**: Use field selectors to reduce watch payload
2. **Batch Operations**: Group multiple updates into single reconciliation
3. **Caching**: Cache node GPU capacity to reduce allocatable queries
4. **Indexing**: Add index for frequently queried fields

## Troubleshooting

### Controller not starting
```bash
# Check logs
kubectl logs deployment/gpu-orchestrator-controller-manager -n gpu-orchestrator-system

# Check events
kubectl describe deployment gpu-orchestrator-controller-manager -n gpu-orchestrator-system

# Verify RBAC permissions
kubectl auth can-i get gpuworkloads --as=system:serviceaccount:gpu-orchestrator-system:gpu-orchestrator-controller-manager
```

### Workload not scheduling
```bash
# Check workload status
kubectl describe gpuworkload <name>

# Check if GPU nodes exist
kubectl get nodes -L nvidia.com/gpu

# Check node details
kubectl describe node <gpu-node-name>

# Verify job was created
kubectl get jobs
```

### Metrics not showing
```bash
# Port-forward metrics service
kubectl port-forward svc/gpu-orchestrator-controller-manager-metrics 8080:8080 -n gpu-orchestrator-system

# Check metrics endpoint
curl http://localhost:8080/metrics
```

## Resources

- [controller-runtime documentation](https://pkg.go.dev/sigs.k8s.io/controller-runtime)
- [Kubebuilder book](https://book.kubebuilder.io/)
- [Kubernetes API conventions](https://kubernetes.io/docs/concepts/overview/kubernetes-api/)
- [CRD best practices](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/)

## Contributing

See [CONTRIBUTING.md](../CONTRIBUTING.md) for contribution guidelines.
