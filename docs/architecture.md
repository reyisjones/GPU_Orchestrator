# gpu-orchestrator Architecture

## Overview

gpu-orchestrator is a Kubernetes controller that implements a custom GPU workload scheduling system. It uses the controller-runtime framework and Kubernetes native APIs to provide declarative, scalable GPU workload management.

## High-Level Architecture

```
┌──────────────────────────────────────────────────────────────┐
│                    Kubernetes API Server                     │
│                                                              │
│  ┌─────────────┐  ┌──────────┐  ┌──────────────────────────┐ │
│  │ GPUWorkload │  │   Node   │  │ Job / Pod Workloads      │ │
│  │ Custom Res. │  │ Resources│  │                          │ │
│  └─────────────┘  └──────────┘  └──────────────────────────┘ │
└──────────────────────────────────────────────────────────────┘
          ▲                 ▲                    ▲
          │                 │                    │
          │ Watches         │ Lists & Queries    │ Creates
          │ Updates         │ GPU Capacity       │ Deploys
          │                 │                    │
┌──────────┴─────────────────────────────────────────────────┐
│                                                            │
│     gpu-orchestrator Controller Manager (Deployment)       │
│                                                            │
│  ┌────────────────────────────────────────────────────────┐│
│  │         GPUWorkloadReconciler                          ││
│  │                                                        ││
│  │  1. Watch GPUWorkload objects (Create/Update/Delete)   ││
│  │  2. Filter Ready nodes with GPU capacity               ││
│  │  3. Apply Scheduling Strategy                          ││
│  │  4. Create Kubernetes Job on selected node             ││
│  │  5. Update GPUWorkload status                          ││
│  │  6. Record Prometheus metrics                          ││
│  │  7. Handle retries with exponential backoff            ││
│  └────────────────────────────────────────────────────────┘│
│                                                            │
└────────────────────────────────────────────────────────────┘
```

## Component Breakdown

### 1. **GPUWorkload Custom Resource Definition (CRD)**

**Location**: `config/crd/bases/gpu.warp.dev_gpuworkloads.yaml`

The GPUWorkload CRD is the primary API surface. It allows users to declare GPU workload requirements declaratively:

```yaml
apiVersion: gpu.warp.dev/v1alpha1
kind: GPUWorkload
metadata:
  name: my-inference-workload
spec:
  modelName: llama2          # Name of the model/workload
  gpuCount: 2                # GPUs required
  priority: high             # Workload priority
  schedulingStrategy: leastLoaded  # Which strategy to use
  retryPolicy:
    maxRetries: 3            # Max scheduling retries
    backoffSeconds: 30       # Base backoff delay
status:
  phase: Scheduled           # Current state
  assignedNode: gpu-node-01  # Where it's scheduled
  jobName: my-inference-job-abc123
  message: "Successfully scheduled on node..."
```

**Key Features**:
- Full OpenAPI v3 schema validation
- Status subresource for separating spec/status
- Print columns for `kubectl get` output
- Enum constraints for predefined fields

### 2. **GPUWorkloadReconciler**

**Location**: `controllers/gpuworkload_controller.go`

The heart of the system. Implements the reconciliation loop using controller-runtime.

**Reconciliation Flow**:

```
┌─────────────────────────────────────────┐
│ Reconcile() called on GPUWorkload event │
└────────────────┬────────────────────────┘
                 ▼
         ┌───────────────┐
         │ Fetch object  │
         └───────┬───────┘
                 ▼
     ┌───────────────────────┐
     │ Check deletion        │ ◄─── Handle cleanup with finalizers
     └───────┬───────────────┘
             ▼
     ┌───────────────────────┐
     │ Check phase/retry     │
     │ limits                │
     └───────┬───────────────┘
             ▼
     ┌───────────────────────────────┐
     │ List all Nodes in cluster     │
     └───────┬───────────────────────┘
             ▼
     ┌────────────────────────────────┐
     │ Filter: Ready + Has GPUs       │
     │ (nvidia.com/gpu resource)      │
     └───────┬────────────────────────┘
             ▼
     ┌────────────────────────────────┐
     │ Select Strategy (Factory)      │
     │ - LeastLoaded                  │
     │ - Random                       │
     │ - CostOptimized                │
     └───────┬────────────────────────┘
             ▼
     ┌────────────────────────────────┐
     │ ChooseNode() from strategy     │
     │ against filtered GPU nodes     │
     └───────┬────────────────────────┘
             ▼
   ┌─────────────────────────────┐
   │ Create Kubernetes Job       │
   │ - GPU resource requests     │
   │ - NodeName affinity         │
   │ - Environment variables     │
   └───────┬─────────────────────┘
           ▼
   ┌──────────────────────────┐
   │ Update GPUWorkload.Status│
   │ - phase = Scheduled      │
   │ - assignedNode           │
   │ - jobName                │
   └───────┬──────────────────┘
           ▼
   ┌──────────────────────────┐
   │ Record Prometheus metrics│
   │ - scheduling success     │
   │ - reconcile duration     │
   └──────────────────────────┘
```

**Idempotency**:
- Checks if Job already exists before creating
- Respects retry limits and phases
- Safe to be invoked multiple times

**Retry Logic**:
- Uses exponential backoff with jitter
- Prevents thundering herd
- Configurable per workload

### 3. **Scheduling Strategies**

**Location**: `internal/scheduling/strategy.go`

Plugin-style scheduling algorithms. Implements the **Strategy Pattern**.

#### Strategy Interface
```go
type Strategy interface {
    ChooseNode(ctx context.Context, nodes []corev1.Node, gw *gpuv1alpha1.GPUWorkload) (*corev1.Node, error)
    Name() string
}
```

#### Implemented Strategies

**a) LeastLoadedStrategy**
- Selects node with most available GPUs
- Minimizes fragmentation
- Default strategy
- Ideal for balanced workload distribution

**b) RandomStrategy**
- Randomly selects from suitable nodes
- Useful when all nodes are equivalent
- Provides natural load balancing

**c) CostOptimizedStrategy**
- Prefers nodes labeled `gpu-orchestrator/cheap-node=true`
- Falls back to LeastLoaded if no cheap nodes available
- Useful for cost-conscious deployments

**GPU Detection Logic**:
1. Check allocatable `nvidia.com/gpu` resource
2. Check capacity `nvidia.com/gpu` resource
3. Check node labels for GPU count
4. Return 0 if no GPUs found

### 4. **Backoff Helper**

**Location**: `internal/backoff/backoff.go`

Exponential backoff calculation with jitter.

**Formula**:
```
backoff = base * 2^attempt + jitter(0-10%)
```

**Properties**:
- Prevents synchronized retries
- Caps at 5 minutes maximum
- Thread-safe with math/rand
- Configurable base duration per workload

### 5. **Prometheus Metrics**

**Location**: `internal/metrics/metrics.go`

Observable controller behavior through standard Prometheus metrics.

**Metrics**:

| Metric | Type | Labels | Purpose |
|--------|------|--------|---------|
| `warp_gpuworkload_scheduled_total` | Counter | strategy | Successful schedulings |
| `warp_gpuworkload_failed_total` | Counter | reason | Failed scheduling attempts |
| `warp_gpuworkload_retries_total` | Counter | - | Total retry count |
| `warp_gpuworkload_reconcile_duration_seconds` | Histogram | result | Reconciliation timing |

**Exposed on**: Port 8080 (`:8080/metrics`)

### 6. **RBAC Configuration**

**Location**: `config/rbac/`

Principle of Least Privilege (PoLP) implementation:

- **Namespaced Role**: GPUWorkload, Job, Event permissions in gpu-orchestrator-system
- **ClusterRole**: Node and Pod listing (requires cluster-wide access)
- **RoleBinding/ClusterRoleBinding**: Connect ServiceAccount to roles

### 7. **Manager Deployment**

**Location**: `config/manager/manager.yaml`

Production-ready Kubernetes Deployment:

**Security**:
- Non-root user (65532)
- Read-only root filesystem
- Dropped all Linux capabilities
- Security context enforced

**Reliability**:
- Leader election (prevents multiple active controllers)
- Health probes (liveness/readiness)
- Graceful shutdown (terminationGracePeriodSeconds: 10)

**Observability**:
- Structured logging (JSON format via Zap)
- Prometheus metrics endpoint
- Health check endpoints

## Data Flow Example

**Scenario**: User creates a GPUWorkload for LLaMA-2 inference requiring 2 GPUs.

```
1. kubectl apply -f llama2-gpuworkload.yaml
   └─> GPUWorkload object stored in etcd
   
2. API Server publishes "GPUWorkload Created" event
   └─> Informer in controller watches this
   
3. Controller's Reconcile() is invoked
   └─> Fetch GPUWorkload object
   └─> List all Nodes
   └─> Filter for Ready nodes with nvidia.com/gpu
   └─> Invoke LeastLoadedStrategy.ChooseNode()
   └─> Selects gpu-node-01 (has 4 GPUs available)
   └─> Create Job (gpu-affinity to gpu-node-01)
   └─> Kubernetes Scheduler places Job on gpu-node-01
   └─> Kubelet pulls image & starts container
   └─> Container has access to 2 GPU devices
   
4. Controller updates GPUWorkload.Status
   └─> phase = Scheduled
   └─> assignedNode = gpu-node-01
   └─> jobName = llama2-inference-job-xyz
   
5. Prometheus metrics recorded
   └─> warp_gpuworkload_scheduled_total{strategy="leastLoaded"} ++
   └─> warp_gpuworkload_reconcile_duration_seconds observe
```

## Interaction with Kubernetes Components

```
┌────────────────────────────────────────────────────────────┐
│                      Kubernetes                            │
├────────────────────────────────────────────────────────────┤
│                                                            │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │ API Server   │  │ Scheduler    │  │ Kubelet      │      │
│  │              │  │              │  │              │      │
│  │ - Stores     │  │ - Places     │  │ - Runs       │      │
│  │   CRDs       │  │   Pods/Jobs  │  │   containers │      │
│  └──────────────┘  └──────────────┘  └──────────────┘      │
│         ▲                  ▲                ▲              │
│         │                  │                │              │
│         │ CREATE Job       │ Watches Job    │ Executes     │
│         │ GET Nodes        │ Schedules      │              │
│         │                  │                │              │
│  ┌────────────────────────────────────────────┐            │
│  │    gpu-orchestrator Controller             │            │
│  │    (Running in Pod)                        │            │
│  └────────────────────────────────────────────┘            │
│                                                            │
└────────────────────────────────────────────────────────────┘
```

## Extension Points

Users can extend gpu-orchestrator by:

1. **Adding Custom Strategies**: Implement `Strategy` interface
2. **Custom Metrics**: Register additional Prometheus metrics
3. **Webhook Validation**: Add ValidatingWebhook for GPUWorkload
4. **Mutation**: Add MutatingWebhook for defaults/transformations
5. **Multiple Schedulers**: Deploy multiple gpu-orchestrator instances with different configurations

## Security Considerations

1. **RBAC**: Minimal permissions following PoLP
2. **Pod Security**: Non-root, read-only filesystem
3. **Network Policies**: Can restrict controller traffic
4. **Secret Management**: ServiceAccount tokens for API authentication
5. **Audit Logging**: All API calls logged by API Server

## Performance Characteristics

- **Reconciliation Time**: O(n) where n = number of nodes (listing + filtering)
- **Scaling**: Single controller handles 1000+ nodes comfortably
- **Memory**: ~50-100MB typical usage
- **CPU**: 100m request, 500m limit (conservative)
- **HA**: Leader election supported for multi-replica deployments

## Future Enhancements

- [ ] Webhook validation for GPUWorkload spec
- [ ] Custom metrics per strategy
- [ ] Multiple GPU vendors (AMD, Intel, etc.)
- [ ] GPU reservation/pre-allocation
- [ ] Integration with cluster autoscaling
- [ ] Priority-based preemption
- [ ] GPU memory management
- [ ] Workload profiling and recommendations
