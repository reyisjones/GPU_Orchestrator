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

package scheduling

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gpuv1alpha1 "github.com/reyisjones/GPU_Orchestrator/api/v1alpha1"
)

func createMockNode(name string, gpuCount int64) corev1.Node {
	quantity := *resource.NewQuantity(gpuCount, resource.DecimalSI)
	node := corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Status: corev1.NodeStatus{
			Allocatable: corev1.ResourceList{
				corev1.ResourceName("nvidia.com/gpu"): quantity,
			},
			Capacity: corev1.ResourceList{
				corev1.ResourceName("nvidia.com/gpu"): quantity,
			},
			Conditions: []corev1.NodeCondition{
				{
					Type:   corev1.NodeReady,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}
	return node
}

func createMockGPUWorkload(gpuCount int32) *gpuv1alpha1.GPUWorkload {
	return &gpuv1alpha1.GPUWorkload{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workload",
			Namespace: "default",
		},
		Spec: gpuv1alpha1.GPUWorkloadSpec{
			ModelName: "test-model",
			GPUCount:  gpuCount,
		},
	}
}

func TestLeastLoadedStrategy_ChoosesNodeWithMostGPUs(t *testing.T) {
	logger := logr.Discard()
	strategy := NewLeastLoadedStrategy(logger)

	nodes := []corev1.Node{
		createMockNode("node1", 2),
		createMockNode("node2", 4),
		createMockNode("node3", 1),
	}

	workload := createMockGPUWorkload(1)

	selected, err := strategy.ChooseNode(context.Background(), nodes, workload)
	if err != nil {
		t.Fatalf("ChooseNode() error = %v", err)
	}

	if selected.Name != "node2" {
		t.Errorf("Expected node2 to be selected (most GPUs), got %s", selected.Name)
	}
}

func TestLeastLoadedStrategy_EmptyNodeList(t *testing.T) {
	logger := logr.Discard()
	strategy := NewLeastLoadedStrategy(logger)

	nodes := []corev1.Node{}
	workload := createMockGPUWorkload(1)

	_, err := strategy.ChooseNode(context.Background(), nodes, workload)
	if err == nil {
		t.Error("Expected error for empty node list")
	}
}

func TestLeastLoadedStrategy_InsufficientGPUs(t *testing.T) {
	logger := logr.Discard()
	strategy := NewLeastLoadedStrategy(logger)

	nodes := []corev1.Node{
		createMockNode("node1", 1),
		createMockNode("node2", 2),
	}

	workload := createMockGPUWorkload(4) // Requires 4 GPUs

	_, err := strategy.ChooseNode(context.Background(), nodes, workload)
	if err == nil {
		t.Error("Expected error when no node has enough GPUs")
	}
}

func TestRandomStrategy_ChoosesFromSuitableNodes(t *testing.T) {
	logger := logr.Discard()
	strategy := NewRandomStrategy(logger)

	nodes := []corev1.Node{
		createMockNode("node1", 2),
		createMockNode("node2", 3),
		createMockNode("node3", 4),
	}

	workload := createMockGPUWorkload(2)

	// Run multiple times to ensure randomness doesn't pick unsuitable nodes
	for i := 0; i < 10; i++ {
		selected, err := strategy.ChooseNode(context.Background(), nodes, workload)
		if err != nil {
			t.Fatalf("Iteration %d: ChooseNode() error = %v", i, err)
		}

		// All nodes have >= 2 GPUs, so any should be acceptable
		if selected == nil {
			t.Fatalf("Iteration %d: selected node is nil", i)
		}
	}
}

func TestRandomStrategy_EmptyNodeList(t *testing.T) {
	logger := logr.Discard()
	strategy := NewRandomStrategy(logger)

	nodes := []corev1.Node{}
	workload := createMockGPUWorkload(1)

	_, err := strategy.ChooseNode(context.Background(), nodes, workload)
	if err == nil {
		t.Error("Expected error for empty node list")
	}
}

func TestCostOptimizedStrategy_PrefersLabeledNodes(t *testing.T) {
	logger := logr.Discard()
	strategy := NewCostOptimizedStrategy(logger)

	// Create nodes with and without cost label
	node1 := createMockNode("cheap-node", 4)
	node1.Labels = map[string]string{"gpu-orchestrator/cheap-node": "true"}

	node2 := createMockNode("expensive-node", 8)
	node2.Labels = map[string]string{"gpu-orchestrator/cheap-node": "false"}

	nodes := []corev1.Node{node2, node1}
	workload := createMockGPUWorkload(2)

	selected, err := strategy.ChooseNode(context.Background(), nodes, workload)
	if err != nil {
		t.Fatalf("ChooseNode() error = %v", err)
	}

	if selected.Name != "cheap-node" {
		t.Errorf("Expected cheap-node to be selected, got %s", selected.Name)
	}
}

func TestCostOptimizedStrategy_FallsBackToLeastLoaded(t *testing.T) {
	logger := logr.Discard()
	strategy := NewCostOptimizedStrategy(logger)

	// Create nodes without cost label
	nodes := []corev1.Node{
		createMockNode("node1", 2),
		createMockNode("node2", 4),
	}

	workload := createMockGPUWorkload(1)

	selected, err := strategy.ChooseNode(context.Background(), nodes, workload)
	if err != nil {
		t.Fatalf("ChooseNode() error = %v", err)
	}

	// Should fall back to least-loaded (node2 has most GPUs)
	if selected.Name != "node2" {
		t.Errorf("Expected node2 (most GPUs, fallback), got %s", selected.Name)
	}
}

func TestFactory_CreatesCorrectStrategy(t *testing.T) {
	logger := logr.Discard()

	tests := []struct {
		name         string
		strategyName string
		expectedType string
	}{
		{"leastLoaded", "leastLoaded", "*scheduling.LeastLoadedStrategy"},
		{"random", "random", "*scheduling.RandomStrategy"},
		{"costOptimized", "costOptimized", "*scheduling.CostOptimizedStrategy"},
		{"unknown defaults to leastLoaded", "unknown", "*scheduling.LeastLoadedStrategy"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy, err := Factory(tt.strategyName, logger)
			if err != nil {
				t.Fatalf("Factory() error = %v", err)
			}
			if strategy == nil {
				t.Error("Factory() returned nil strategy")
			}
		})
	}
}

func TestSortNodesByGPUAvailability(t *testing.T) {
	nodes := []corev1.Node{
		createMockNode("node1", 1),
		createMockNode("node2", 4),
		createMockNode("node3", 2),
	}

	SortNodesByGPUAvailability(nodes)

	expectedOrder := []string{"node2", "node3", "node1"}
	for i, expectedName := range expectedOrder {
		if nodes[i].Name != expectedName {
			t.Errorf("After sort, position %d: expected %s, got %s", i, expectedName, nodes[i].Name)
		}
	}
}

func BenchmarkLeastLoadedStrategy(b *testing.B) {
	logger := logr.Discard()
	strategy := NewLeastLoadedStrategy(logger)

	nodes := make([]corev1.Node, 100)
	for i := 0; i < 100; i++ {
		nodes[i] = createMockNode("node"+string(rune(i)), int64((i%4)+1))
	}

	workload := createMockGPUWorkload(2)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		strategy.ChooseNode(ctx, nodes, workload)
	}
}

func BenchmarkRandomStrategy(b *testing.B) {
	logger := logr.Discard()
	strategy := NewRandomStrategy(logger)

	nodes := make([]corev1.Node, 100)
	for i := 0; i < 100; i++ {
		nodes[i] = createMockNode("node"+string(rune(i)), int64((i%4)+1))
	}

	workload := createMockGPUWorkload(2)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		strategy.ChooseNode(ctx, nodes, workload)
	}
}
