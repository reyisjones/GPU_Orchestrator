# Quick Start Guide

Get GPU_Orchestrator running in 5 minutes.

## Prerequisites

✅ Kubernetes 1.24+ cluster with kubectl configured  
✅ GPU nodes with NVIDIA drivers and `nvidia.com/gpu` resource  
✅ kustomize (for deployment)

## 1. Create Local Test Cluster (Optional)

```bash
# Using kind
kind create cluster --name gpu-test --image kindest/node:v1.28.0

# Or using minikube
minikube start --cpus=4 --memory=8192
```

## 2. Deploy gpu-orchestrator

### Option A: Using provided script
```bash
chmod +x scripts/deploy.sh
./scripts/deploy.sh gpu-orchestrator-system
```

### Option B: Manual deployment
```bash
# Apply CRD
kubectl apply -f config/crd/bases/gpu.warp.dev_gpuworkloads.yaml

# Apply RBAC
kubectl apply -k config/rbac/

# Build and deploy controller
docker build -t gpu-orchestrator:latest .
kind load docker-image gpu-orchestrator:latest
kubectl apply -k config/manager/
```

## 3. Verify Installation

```bash
# Check controller is running
kubectl get deployment -n gpu-orchestrator-system
kubectl logs -n gpu-orchestrator-system deployment/gpu-orchestrator-controller-manager

# Check CRD is available
kubectl get crd gpuworkloads.gpu.warp.dev
```

## 4. Label GPU Nodes (Test)

In a real cluster, nodes should have `nvidia.com/gpu` resources.  
For testing, you can label a node:

```bash
kubectl label nodes <node-name> nvidia.com/gpu=2
```

## 5. Create a GPU Workload

```bash
kubectl apply -f examples/gpuworkload-sample.yaml
```

## 6. Monitor the Workload

```bash
# Watch workloads
kubectl get gpuworkloads -w

# Get details
kubectl describe gpuworkload llama2-inference

# Check the created job
kubectl get jobs
```

## 7. View Metrics

```bash
# Port-forward metrics service
kubectl port-forward -n gpu-orchestrator-system \
  svc/gpu-orchestrator-controller-manager-metrics 8080:8080

# In another terminal
curl http://localhost:8080/metrics | grep warp_
```

## Common Commands

```bash
# List all workloads
kubectl get gpuworkloads

# Get detailed info
kubectl describe gpuworkload <name>

# Check status
kubectl get gpuworkloads -o custom-columns=NAME:.metadata.name,PHASE:.status.phase,NODE:.status.assignedNode

# Watch in real-time
kubectl get gpuworkloads -w

# Delete a workload
kubectl delete gpuworkload <name>

# View controller logs
kubectl logs -f -n gpu-orchestrator-system deployment/gpu-orchestrator-controller-manager
```

## Create Your Own Workload

```yaml
apiVersion: gpu.warp.dev/v1alpha1
kind: GPUWorkload
metadata:
  name: my-workload
spec:
  modelName: my-model
  gpuCount: 2
  priority: high
  schedulingStrategy: leastLoaded
  retryPolicy:
    maxRetries: 3
    backoffSeconds: 30
```

## Cleanup

### Remove one workload
```bash
kubectl delete gpuworkload <name>
```

### Remove entire controller
```bash
chmod +x scripts/uninstall.sh
./scripts/uninstall.sh gpu-orchestrator-system
```

## Troubleshooting

### Workload stuck in "Pending"
```bash
# Check controller logs
kubectl logs -n gpu-orchestrator-system deployment/gpu-orchestrator-controller-manager

# Check if GPU nodes exist
kubectl get nodes -L nvidia.com/gpu

# Ensure nodes are Ready
kubectl get nodes
```

### Metrics not showing
```bash
# Check if metrics service exists
kubectl get svc -n gpu-orchestrator-system

# Port-forward and test
kubectl port-forward -n gpu-orchestrator-system \
  svc/gpu-orchestrator-controller-manager-metrics 8080:8080

# In another terminal
curl http://localhost:8080/metrics
```

### Controller not starting
```bash
# Check deployment status
kubectl describe deploy -n gpu-orchestrator-system gpu-orchestrator-controller-manager

# Check events
kubectl get events -n gpu-orchestrator-system

# Check logs with more context
kubectl logs -n gpu-orchestrator-system deployment/gpu-orchestrator-controller-manager -f
```

## Next Steps

- 📚 Read [Architecture](docs/architecture.md) for deep dive
- 🔧 Check [Development Guide](docs/DEVELOPMENT.md) for local development
- 📖 View full [README](README.md) for comprehensive documentation
- 🚀 Explore [Advanced Examples](examples/advanced-examples.yaml)

## Support

For issues and feature requests, visit the [GitHub repository](https://github.com/reyisjones/gpu-orchestrator).
