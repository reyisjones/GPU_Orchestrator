#!/bin/bash
# Quick start script to deploy GPU_Orchestrator to a local cluster

set -e

NAMESPACE=${1:-gpu-orchestrator-system}
REGISTRY=${2:-docker.io}
IMAGE_NAME=${3:-gpu-orchestrator}
IMAGE_TAG=${4:-latest}

echo "🚀 Deploying gpu-orchestrator to namespace: $NAMESPACE"
echo "📦 Using image: $REGISTRY/$IMAGE_NAME:$IMAGE_TAG"

# Check if kubectl is available
if ! command -v kubectl &> /dev/null; then
    echo "❌ kubectl not found. Please install kubectl."
    exit 1
fi

# Check cluster connectivity
if ! kubectl cluster-info &> /dev/null; then
    echo "❌ Cannot connect to Kubernetes cluster."
    exit 1
fi

echo "✅ Connected to Kubernetes cluster"

# Create namespace if it doesn't exist
if ! kubectl get namespace "$NAMESPACE" &> /dev/null; then
    echo "📝 Creating namespace: $NAMESPACE"
    kubectl create namespace "$NAMESPACE"
fi

# Apply CRD
echo "📋 Applying CRD..."
kubectl apply -f config/crd/bases/gpu.warp.dev_gpuworkloads.yaml

# Apply RBAC
echo "🔐 Applying RBAC..."
kubectl apply -k config/rbac/

# Apply manager deployment
echo "🎯 Applying manager deployment..."
kubectl set image deployment/gpu-orchestrator-controller-manager \
    manager="$REGISTRY/$IMAGE_NAME:$IMAGE_TAG" \
    -n "$NAMESPACE" || true
kubectl apply -k config/manager/

# Wait for deployment
echo "⏳ Waiting for deployment to be ready..."
kubectl rollout status deployment/gpu-orchestrator-controller-manager \
    -n "$NAMESPACE" \
    --timeout=300s

echo "✅ gpu-orchestrator deployed successfully!"
echo ""
echo "📊 View controller logs:"
echo "   kubectl logs -f deployment/gpu-orchestrator-controller-manager -n $NAMESPACE"
echo ""
echo "📈 View metrics:"
echo "   kubectl port-forward svc/gpu-orchestrator-controller-manager-metrics 8080:8080 -n $NAMESPACE"
echo "   curl http://localhost:8080/metrics"
echo ""
echo "🧪 Try a sample workload:"
echo "   kubectl apply -f examples/gpuworkload-sample.yaml"
echo "   kubectl get gpuworkloads"
echo "   kubectl describe gpuworkload llama2-inference"
