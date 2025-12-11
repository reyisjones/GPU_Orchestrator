/*
Copyright 2025 GPU_Orchestrator contributors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package scheduling provides pluggable scheduling strategies for GPU workload placement.
package scheduling

import (
	"context"
	"fmt"
	"math/rand"
	"sort"

	"github.com/go-logr/logr"
	gpuv1alpha1 "github.com/reyisjones/GPU_Orchestrator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// Strategy defines the interface for scheduling strategies.
// Implementations select a suitable node for a GPUWorkload.
type Strategy interface {
	// ChooseNode selects a node from the available list to host the workload.
	// Returns the selected node or an error if no suitable node is found.
	ChooseNode(ctx context.Context, nodes []corev1.Node, gw *gpuv1alpha1.GPUWorkload) (*corev1.Node, error)

	// Name returns the name of the strategy.
	Name() string
}

// LeastLoadedStrategy selects the node with the most available GPU capacity.
// This strategy minimizes fragmentation and spreads workloads across nodes.
type LeastLoadedStrategy struct {
	logger logr.Logger
}

var _ Strategy = &LeastLoadedStrategy{}

// NewLeastLoadedStrategy creates a new LeastLoadedStrategy.
func NewLeastLoadedStrategy(logger logr.Logger) *LeastLoadedStrategy {
	return &LeastLoadedStrategy{logger: logger}
}

// ChooseNode selects the node with the most available GPUs.
func (s *LeastLoadedStrategy) ChooseNode(ctx context.Context, nodes []corev1.Node, gw *gpuv1alpha1.GPUWorkload) (*corev1.Node, error) {
	if len(nodes) == 0 {
		return nil, fmt.Errorf("no suitable nodes available for GPU workload")
	}

	// Find the node with the most available GPUs
	var bestNode *corev1.Node
	maxAvailableGPUs := int64(-1)

	for i, node := range nodes {
		availableGPUs := getAvailableGPUs(&node)
		if availableGPUs >= int64(gw.Spec.GPUCount) && availableGPUs > maxAvailableGPUs {
			maxAvailableGPUs = availableGPUs
			bestNode = &nodes[i]
		}
	}

	if bestNode == nil {
		return nil, fmt.Errorf("no node has enough available GPUs for workload requiring %d GPUs", gw.Spec.GPUCount)
	}

	s.logger.Info("Selected node using LeastLoadedStrategy", "node", bestNode.Name, "availableGPUs", maxAvailableGPUs)
	return bestNode, nil
}

// Name returns the strategy name.
func (s *LeastLoadedStrategy) Name() string {
	return "leastLoaded"
}

// RandomStrategy selects a random node from the available options.
// This strategy is useful for load distribution when all nodes are comparable.
type RandomStrategy struct {
	logger logr.Logger
}

var _ Strategy = &RandomStrategy{}

// NewRandomStrategy creates a new RandomStrategy.
func NewRandomStrategy(logger logr.Logger) *RandomStrategy {
	return &RandomStrategy{logger: logger}
}

// ChooseNode selects a random node with sufficient GPU capacity.
func (s *RandomStrategy) ChooseNode(ctx context.Context, nodes []corev1.Node, gw *gpuv1alpha1.GPUWorkload) (*corev1.Node, error) {
	if len(nodes) == 0 {
		return nil, fmt.Errorf("no suitable nodes available for GPU workload")
	}

	// Filter nodes with sufficient GPU capacity
	var suitableNodes []corev1.Node
	for _, node := range nodes {
		if getAvailableGPUs(&node) >= int64(gw.Spec.GPUCount) {
			suitableNodes = append(suitableNodes, node)
		}
	}

	if len(suitableNodes) == 0 {
		return nil, fmt.Errorf("no node has enough available GPUs for workload requiring %d GPUs", gw.Spec.GPUCount)
	}

	// Select a random node
	selectedIdx := rand.Intn(len(suitableNodes))
	selectedNode := &suitableNodes[selectedIdx]

	s.logger.Info("Selected node using RandomStrategy", "node", selectedNode.Name)
	return selectedNode, nil
}

// Name returns the strategy name.
func (s *RandomStrategy) Name() string {
	return "random"
}

// CostOptimizedStrategy prefers nodes with the "gpu-orchestrator/cheap-node=true" label.
// Falls back to LeastLoadedStrategy if no cost-optimized nodes are available.
type CostOptimizedStrategy struct {
	logger logr.Logger
}

var _ Strategy = &CostOptimizedStrategy{}

// NewCostOptimizedStrategy creates a new CostOptimizedStrategy.
func NewCostOptimizedStrategy(logger logr.Logger) *CostOptimizedStrategy {
	return &CostOptimizedStrategy{logger: logger}
}

// ChooseNode selects a cost-optimized node if available, otherwise uses LeastLoadedStrategy.
func (s *CostOptimizedStrategy) ChooseNode(ctx context.Context, nodes []corev1.Node, gw *gpuv1alpha1.GPUWorkload) (*corev1.Node, error) {
	if len(nodes) == 0 {
		return nil, fmt.Errorf("no suitable nodes available for GPU workload")
	}

	// First, try to find a cost-optimized node
	var cheapNodes []corev1.Node
	for _, node := range nodes {
		if node.Labels != nil {
			if isCheap, exists := node.Labels["gpu-orchestrator/cheap-node"]; exists && isCheap == "true" {
				if getAvailableGPUs(&node) >= int64(gw.Spec.GPUCount) {
					cheapNodes = append(cheapNodes, node)
				}
			}
		}
	}

	// If cheap nodes are available, use least-loaded among them
	if len(cheapNodes) > 0 {
		var bestNode *corev1.Node
		maxAvailableGPUs := int64(-1)

		for i, node := range cheapNodes {
			availableGPUs := getAvailableGPUs(&node)
			if availableGPUs > maxAvailableGPUs {
				maxAvailableGPUs = availableGPUs
				bestNode = &cheapNodes[i]
			}
		}

		s.logger.Info("Selected cost-optimized node", "node", bestNode.Name)
		return bestNode, nil
	}

	// Fall back to least-loaded strategy
	s.logger.Info("No cost-optimized nodes available, falling back to LeastLoadedStrategy")
	fallback := NewLeastLoadedStrategy(s.logger)
	return fallback.ChooseNode(ctx, nodes, gw)
}

// Name returns the strategy name.
func (s *CostOptimizedStrategy) Name() string {
	return "costOptimized"
}

// Factory creates a strategy based on the name.
func Factory(strategyName string, logger logr.Logger) (Strategy, error) {
	switch strategyName {
	case "leastLoaded":
		return NewLeastLoadedStrategy(logger), nil
	case "random":
		return NewRandomStrategy(logger), nil
	case "costOptimized":
		return NewCostOptimizedStrategy(logger), nil
	default:
		// Default to least-loaded
		logger.Info("Unknown strategy, defaulting to leastLoaded", "requested", strategyName)
		return NewLeastLoadedStrategy(logger), nil
	}
}

// getAvailableGPUs returns the number of allocatable GPUs on a node.
// It checks both the allocatable resources and node labels for GPU availability.
//
// Note: This is a simplified implementation. In production, you might want to:
// - Query the resource metrics API for actual usage
// - Account for reserved/allocated GPUs
// - Support multiple GPU vendors (NVIDIA, AMD, etc.)
func getAvailableGPUs(node *corev1.Node) int64 {
	// Try to get from allocatable resources first (most accurate)
	if quantity, ok := node.Status.Allocatable[corev1.ResourceName("nvidia.com/gpu")]; ok {
		return quantity.Value()
	}

	// Fall back to capacity
	if quantity, ok := node.Status.Capacity[corev1.ResourceName("nvidia.com/gpu")]; ok {
		return quantity.Value()
	}

	// Check for GPU label (some clusters use labels instead of resources)
	if node.Labels != nil {
		if gpuLabel, exists := node.Labels["nvidia.com/gpu"]; exists {
			// Try to parse the label value
			var count int64
			fmt.Sscanf(gpuLabel, "%d", &count)
			if count > 0 {
				return count
			}
		}
	}

	return 0
}

// SortNodesByGPUAvailability sorts nodes in descending order by available GPUs.
// This helper can be useful for strategies that need ordered node lists.
func SortNodesByGPUAvailability(nodes []corev1.Node) {
	sort.Slice(nodes, func(i, j int) bool {
		return getAvailableGPUs(&nodes[i]) > getAvailableGPUs(&nodes[j])
	})
}
