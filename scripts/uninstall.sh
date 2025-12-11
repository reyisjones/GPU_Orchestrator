#!/bin/bash
# Script to uninstall gpu-orchestrator from a Kubernetes cluster

set -e

NAMESPACE=${1:-gpu-orchestrator-system}

echo "🗑️  Removing gpu-orchestrator from namespace: $NAMESPACE"

# Delete manager
echo "🎯 Removing manager deployment..."
kubectl delete deployment gpu-orchestrator-controller-manager -n "$NAMESPACE" --ignore-not-found

# Delete services
echo "🔌 Removing services..."
kubectl delete service gpu-orchestrator-controller-manager-metrics -n "$NAMESPACE" --ignore-not-found
kubectl delete service gpu-orchestrator-webhook-service -n "$NAMESPACE" --ignore-not-found

# Delete RBAC
echo "🔐 Removing RBAC..."
kubectl delete rolebinding gpu-orchestrator-controller-manager -n "$NAMESPACE" --ignore-not-found
kubectl delete role gpu-orchestrator-controller-manager -n "$NAMESPACE" --ignore-not-found
kubectl delete clusterrolebinding gpu-orchestrator-controller-manager --ignore-not-found
kubectl delete clusterrole gpu-orchestrator-controller-manager --ignore-not-found
kubectl delete serviceaccount gpu-orchestrator-controller-manager -n "$NAMESPACE" --ignore-not-found

# Delete CRD
echo "📋 Removing CRD..."
kubectl delete crd gpuworkloads.gpu.warp.dev --ignore-not-found

# Delete namespace
echo "🗂️  Removing namespace..."
kubectl delete namespace "$NAMESPACE" --ignore-not-found

echo "✅ gpu-orchestrator removed successfully!"
