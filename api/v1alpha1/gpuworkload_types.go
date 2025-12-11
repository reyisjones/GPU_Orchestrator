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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GPUWorkloadSpec defines the desired state of a GPU workload.
type GPUWorkloadSpec struct {
	// ModelName is the name of the model or workload (e.g., "llama2", "stable-diffusion").
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=255
	ModelName string `json:"modelName"`

	// GPUCount is the number of GPUs required for this workload.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=8
	GPUCount int32 `json:"gpuCount"`

	// Priority defines the priority level of the workload: "low", "normal", or "high".
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum=low;normal;high
	// +kubebuilder:default=normal
	Priority string `json:"priority,omitempty"`

	// SchedulingStrategy defines which scheduling algorithm to use.
	// Options: "leastLoaded", "random", "costOptimized"
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum=leastLoaded;random;costOptimized
	// +kubebuilder:default=leastLoaded
	SchedulingStrategy string `json:"schedulingStrategy,omitempty"`

	// RetryPolicy defines the retry behavior for failed scheduling attempts.
	// +kubebuilder:validation:Optional
	RetryPolicy *RetryPolicy `json:"retryPolicy,omitempty"`
}

// RetryPolicy defines how the controller should retry scheduling a GPUWorkload.
type RetryPolicy struct {
	// MaxRetries is the maximum number of times to retry scheduling.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=10
	// +kubebuilder:default=3
	MaxRetries int32 `json:"maxRetries,omitempty"`

	// BackoffSeconds is the base delay in seconds for exponential backoff.
	// The actual delay will be backoffSeconds * 2^attempt + jitter.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=300
	// +kubebuilder:default=30
	BackoffSeconds int32 `json:"backoffSeconds,omitempty"`
}

// GPUWorkloadPhase is the phase of a GPUWorkload.
type GPUWorkloadPhase string

const (
	// PhasePending indicates the workload is waiting for scheduling.
	PhasePending GPUWorkloadPhase = "Pending"

	// PhaseScheduling indicates the workload is being scheduled.
	PhaseScheduling GPUWorkloadPhase = "Scheduling"

	// PhaseScheduled indicates the workload has been scheduled.
	PhaseScheduled GPUWorkloadPhase = "Scheduled"

	// PhaseRunning indicates the workload is running.
	PhaseRunning GPUWorkloadPhase = "Running"

	// PhaseFailed indicates the workload failed to schedule or execute.
	PhaseFailed GPUWorkloadPhase = "Failed"

	// PhaseSucceeded indicates the workload completed successfully.
	PhaseSucceeded GPUWorkloadPhase = "Succeeded"
)

// GPUWorkloadStatus defines the observed state of a GPU workload.
type GPUWorkloadStatus struct {
	// Phase is the current phase of the workload.
	// +kubebuilder:validation:Optional
	Phase GPUWorkloadPhase `json:"phase,omitempty"`

	// AssignedNode is the name of the node where the workload is scheduled.
	// +kubebuilder:validation:Optional
	AssignedNode string `json:"assignedNode,omitempty"`

	// LastScheduleTime is the timestamp of the last scheduling attempt.
	// +kubebuilder:validation:Optional
	LastScheduleTime *metav1.Time `json:"lastScheduleTime,omitempty"`

	// RetryCount is the current number of retries attempted.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=0
	RetryCount int32 `json:"retryCount,omitempty"`

	// Message is a human-readable message about the last scheduling attempt.
	// +kubebuilder:validation:Optional
	Message string `json:"message,omitempty"`

	// JobName is the name of the Kubernetes Job created for this workload (if any).
	// +kubebuilder:validation:Optional
	JobName string `json:"jobName,omitempty"`
}

// GPUWorkload is the Schema for the gpuworkloads API.
// It represents a request to schedule a GPU-intensive workload on a suitable Kubernetes node.
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=gpuw;plural=gpuworkloads
// +kubebuilder:printcolumn:name="Model",type=string,JSONPath=`.spec.modelName`
// +kubebuilder:printcolumn:name="GPUs",type=integer,JSONPath=`.spec.gpuCount`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Node",type=string,JSONPath=`.status.assignedNode`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type GPUWorkload struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GPUWorkloadSpec   `json:"spec,omitempty"`
	Status GPUWorkloadStatus `json:"status,omitempty"`
}

// GPUWorkloadList contains a list of GPUWorkload objects.
// +kubebuilder:object:root=true
type GPUWorkloadList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []GPUWorkload `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GPUWorkload{}, &GPUWorkloadList{})
}
