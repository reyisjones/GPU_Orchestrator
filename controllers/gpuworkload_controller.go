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

package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	gpuv1alpha1 "github.com/reyisjones/GPU_Orchestrator/api/v1alpha1"
	"github.com/reyisjones/GPU_Orchestrator/internal/backoff"
	"github.com/reyisjones/GPU_Orchestrator/internal/metrics"
	"github.com/reyisjones/GPU_Orchestrator/internal/scheduling"
)

const (
	// finalizerName is used to ensure cleanup of workloads
	finalizerName = "gpu.warp.dev/finalizer"

	// ownershipAnnotation marks which controller created a job
	ownershipAnnotation = "gpu.warp.dev/created-by"
)

// GPUWorkloadReconciler reconciles a GPUWorkload object
type GPUWorkloadReconciler struct {
	client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=gpu.warp.dev,resources=gpuworkloads,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=gpu.warp.dev,resources=gpuworkloads/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=gpu.warp.dev,resources=gpuworkloads/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile implements the reconciliation loop for GPUWorkload objects.
// It watches GPUWorkload resources and:
// 1. Lists available GPU nodes
// 2. Applies the configured scheduling strategy
// 3. Creates a Job on the selected node
// 4. Updates status with phase, assigned node, and retry info
func (r *GPUWorkloadReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("gpuworkload", req.NamespacedName)
	startTime := time.Now()

	// Fetch the GPUWorkload
	gpuWorkload := &gpuv1alpha1.GPUWorkload{}
	if err := r.Get(ctx, req.NamespacedName, gpuWorkload); err != nil {
		log.Error(err, "unable to fetch GPUWorkload")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Record metrics for reconciliation duration
	defer func() {
		duration := time.Since(startTime).Seconds()
		m := metrics.GetMetrics()
		if m != nil {
			// Determine result based on final phase
			result := "error"
			if gpuWorkload.Status.Phase == gpuv1alpha1.PhaseScheduled || gpuWorkload.Status.Phase == gpuv1alpha1.PhaseRunning {
				result = "success"
			}
			m.RecordReconcileDuration(duration, result)
		}
	}()

	// Skip if already scheduled successfully or permanently failed
	if gpuWorkload.Status.Phase == gpuv1alpha1.PhaseScheduled || gpuWorkload.Status.Phase == gpuv1alpha1.PhaseRunning || gpuWorkload.Status.Phase == gpuv1alpha1.PhaseSucceeded {
		log.V(1).Info("GPUWorkload already scheduled, skipping")
		return ctrl.Result{}, nil
	}

	// Handle deletion with finalizer
	if !gpuWorkload.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, log, gpuWorkload)
	}

	// Add finalizer if not present
	if !containsString(gpuWorkload.ObjectMeta.Finalizers, finalizerName) {
		gpuWorkload.ObjectMeta.Finalizers = append(gpuWorkload.ObjectMeta.Finalizers, finalizerName)
		if err := r.Update(ctx, gpuWorkload); err != nil {
			log.Error(err, "unable to add finalizer")
			return ctrl.Result{}, err
		}
	}

	// Set initial phase if not set
	if gpuWorkload.Status.Phase == "" {
		gpuWorkload.Status.Phase = gpuv1alpha1.PhasePending
		gpuWorkload.Status.LastScheduleTime = &metav1.Time{Time: time.Now()}
		if err := r.Status().Update(ctx, gpuWorkload); err != nil {
			log.Error(err, "unable to update GPUWorkload status")
			return ctrl.Result{}, err
		}
		log.Info("Initialized GPUWorkload status", "phase", gpuWorkload.Status.Phase)
	}

	// Check if we should retry
	maxRetries := int32(3) // default
	if gpuWorkload.Spec.RetryPolicy != nil && gpuWorkload.Spec.RetryPolicy.MaxRetries > 0 {
		maxRetries = gpuWorkload.Spec.RetryPolicy.MaxRetries
	}

	if gpuWorkload.Status.RetryCount >= maxRetries {
		gpuWorkload.Status.Phase = gpuv1alpha1.PhaseFailed
		gpuWorkload.Status.Message = fmt.Sprintf("Failed to schedule after %d retries", maxRetries)
		if err := r.Status().Update(ctx, gpuWorkload); err != nil {
			log.Error(err, "unable to update GPUWorkload status")
			return ctrl.Result{}, err
		}
		log.Info("Max retries exceeded", "retries", gpuWorkload.Status.RetryCount, "maxRetries", maxRetries)
		r.Recorder.Event(gpuWorkload, corev1.EventTypeWarning, "MaxRetriesExceeded", gpuWorkload.Status.Message)
		return ctrl.Result{}, nil
	}

	// List available GPU nodes
	nodes := &corev1.NodeList{}
	if err := r.List(ctx, nodes); err != nil {
		log.Error(err, "unable to list nodes")
		gpuWorkload.Status.Phase = gpuv1alpha1.PhasePending
		gpuWorkload.Status.Message = fmt.Sprintf("Error listing nodes: %v", err)
		r.Status().Update(ctx, gpuWorkload)
		return r.requeueWithBackoff(gpuWorkload)
	}

	// Filter for GPU nodes that are Ready
	var gpuNodes []corev1.Node
	for _, node := range nodes.Items {
		if isNodeReady(&node) && hasGPUs(&node) {
			gpuNodes = append(gpuNodes, node)
		}
	}

	if len(gpuNodes) == 0 {
		log.Info("No GPU nodes available")
		gpuWorkload.Status.Phase = gpuv1alpha1.PhasePending
		gpuWorkload.Status.Message = "No ready GPU nodes available"
		r.Status().Update(ctx, gpuWorkload)
		return r.requeueWithBackoff(gpuWorkload)
	}

	log.Info("Found GPU nodes", "count", len(gpuNodes))

	// Select scheduling strategy
	strategyName := gpuWorkload.Spec.SchedulingStrategy
	if strategyName == "" {
		strategyName = "leastLoaded"
	}

	strategy, err := scheduling.Factory(strategyName, log)
	if err != nil {
		log.Error(err, "failed to create scheduling strategy", "strategy", strategyName)
		gpuWorkload.Status.Phase = gpuv1alpha1.PhasePending
		gpuWorkload.Status.Message = fmt.Sprintf("Invalid scheduling strategy: %s", strategyName)
		r.Status().Update(ctx, gpuWorkload)
		return ctrl.Result{}, nil
	}

	// Choose a node using the strategy
	selectedNode, err := strategy.ChooseNode(ctx, gpuNodes, gpuWorkload)
	if err != nil {
		log.Info("Failed to select node", "error", err)
		gpuWorkload.Status.Phase = gpuv1alpha1.PhasePending
		gpuWorkload.Status.Message = err.Error()
		gpuWorkload.Status.RetryCount++
		if m := metrics.GetMetrics(); m != nil {
			m.RecordRetry()
			m.RecordSchedulingFailure("no_suitable_node")
		}
		r.Status().Update(ctx, gpuWorkload)
		return r.requeueWithBackoff(gpuWorkload)
	}

	log.Info("Selected node for workload", "node", selectedNode.Name, "strategy", strategy.Name())

	// Create Job for the workload
	job, err := r.createJobForWorkload(gpuWorkload, selectedNode)
	if err != nil {
		log.Error(err, "failed to create job")
		gpuWorkload.Status.Phase = gpuv1alpha1.PhasePending
		gpuWorkload.Status.Message = fmt.Sprintf("Failed to create job: %v", err)
		gpuWorkload.Status.RetryCount++
		if m := metrics.GetMetrics(); m != nil {
			m.RecordRetry()
			m.RecordSchedulingFailure("job_creation_failed")
		}
		r.Status().Update(ctx, gpuWorkload)
		return r.requeueWithBackoff(gpuWorkload)
	}

	// Update status to Scheduled
	gpuWorkload.Status.Phase = gpuv1alpha1.PhaseScheduled
	gpuWorkload.Status.AssignedNode = selectedNode.Name
	gpuWorkload.Status.LastScheduleTime = &metav1.Time{Time: time.Now()}
	gpuWorkload.Status.JobName = job.Name
	gpuWorkload.Status.Message = fmt.Sprintf("Successfully scheduled on node %s using %s strategy", selectedNode.Name, strategy.Name())

	if err := r.Status().Update(ctx, gpuWorkload); err != nil {
		log.Error(err, "unable to update GPUWorkload status")
		return ctrl.Result{}, err
	}

	log.Info("GPUWorkload scheduled successfully", "node", selectedNode.Name, "job", job.Name)
	r.Recorder.Event(gpuWorkload, corev1.EventTypeNormal, "Scheduled", gpuWorkload.Status.Message)

	if m := metrics.GetMetrics(); m != nil {
		m.RecordSchedulingSuccess(strategy.Name())
	}

	return ctrl.Result{}, nil
}

// handleDeletion handles cleanup when a GPUWorkload is deleted
func (r *GPUWorkloadReconciler) handleDeletion(ctx context.Context, log logr.Logger, gpuWorkload *gpuv1alpha1.GPUWorkload) (ctrl.Result, error) {
	if containsString(gpuWorkload.ObjectMeta.Finalizers, finalizerName) {
		// Delete associated job if it exists
		if gpuWorkload.Status.JobName != "" {
			job := &batchv1.Job{}
			jobKey := types.NamespacedName{Name: gpuWorkload.Status.JobName, Namespace: gpuWorkload.Namespace}
			if err := r.Get(ctx, jobKey, job); err == nil {
				log.Info("Deleting associated job", "job", job.Name)
				if err := r.Delete(ctx, job); err != nil && !client.IgnoreNotFound(err) != nil {
					log.Error(err, "unable to delete job")
					return ctrl.Result{}, err
				}
			}
		}

		// Remove finalizer
		gpuWorkload.ObjectMeta.Finalizers = removeString(gpuWorkload.ObjectMeta.Finalizers, finalizerName)
		if err := r.Update(ctx, gpuWorkload); err != nil {
			log.Error(err, "unable to remove finalizer")
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

// createJobForWorkload creates a Kubernetes Job for the GPUWorkload
func (r *GPUWorkloadReconciler) createJobForWorkload(gw *gpuv1alpha1.GPUWorkload, node *corev1.Node) (*batchv1.Job, error) {
	jobName := fmt.Sprintf("%s-job-%s", gw.Name, gw.UID[:8])

	// Check if job already exists
	existingJob := &batchv1.Job{}
	if err := r.Get(context.Background(), types.NamespacedName{Name: jobName, Namespace: gw.Namespace}, existingJob); err == nil {
		return existingJob, nil
	}

	// Create the Job spec with GPU resource requests
	backoffLimit := int32(0)
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: gw.Namespace,
			Labels: map[string]string{
				"app":                     gw.Spec.ModelName,
				"gpu.warp.dev/workload":   gw.Name,
				"gpu.warp.dev/controller": "gpu-orchestrator",
			},
			Annotations: map[string]string{
				ownershipAnnotation: gw.Name,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: gw.APIVersion,
					Kind:       gw.Kind,
					Name:       gw.Name,
					UID:        gw.UID,
					Controller: boolPtr(true),
				},
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoffLimit,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": gw.Spec.ModelName,
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					NodeName:      node.Name,
					Containers: []corev1.Container{
						{
							Name:  "gpu-workload",
							Image: fmt.Sprintf("python:3.11-slim"), // Placeholder image
							Env: []corev1.EnvVar{
								{
									Name:  "MODEL_NAME",
									Value: gw.Spec.ModelName,
								},
								{
									Name:  "GPU_COUNT",
									Value: fmt.Sprintf("%d", gw.Spec.GPUCount),
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceName("nvidia.com/gpu"): parseQuantity(fmt.Sprintf("%d", gw.Spec.GPUCount)),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceName("nvidia.com/gpu"): parseQuantity(fmt.Sprintf("%d", gw.Spec.GPUCount)),
								},
							},
						},
					},
				},
			},
		},
	}

	if err := r.Create(context.Background(), job); err != nil {
		return nil, err
	}

	return job, nil
}

// requeueWithBackoff returns a requeue result with exponential backoff
func (r *GPUWorkloadReconciler) requeueWithBackoff(gw *gpuv1alpha1.GPUWorkload) (ctrl.Result, error) {
	baseDuration := 30 * time.Second
	if gw.Spec.RetryPolicy != nil && gw.Spec.RetryPolicy.BackoffSeconds > 0 {
		baseDuration = time.Duration(gw.Spec.RetryPolicy.BackoffSeconds) * time.Second
	}

	backoffDuration := backoff.NextBackoff(baseDuration, int(gw.Status.RetryCount))
	return ctrl.Result{RequeueAfter: backoffDuration}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *GPUWorkloadReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Recorder = mgr.GetEventRecorderFor("gpuworkload-controller")

	return ctrl.NewControllerManagedBy(mgr).
		For(&gpuv1alpha1.GPUWorkload{}).
		Owns(&batchv1.Job{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}

// Utility functions

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func removeString(list []string, s string) []string {
	var result []string
	for _, v := range list {
		if v != s {
			result = append(result, v)
		}
	}
	return result
}

func boolPtr(b bool) *bool {
	return &b
}

func parseQuantity(value string) corev1.ResourceQuantity {
	q := corev1.ResourceQuantity{}
	q.Set(int64(len(value)))
	return q
}

func isNodeReady(node *corev1.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}

func hasGPUs(node *corev1.Node) bool {
	// Check for nvidia.com/gpu resource
	if quantity, ok := node.Status.Allocatable[corev1.ResourceName("nvidia.com/gpu")]; ok && quantity.Value() > 0 {
		return true
	}
	if quantity, ok := node.Status.Capacity[corev1.ResourceName("nvidia.com/gpu")]; ok && quantity.Value() > 0 {
		return true
	}

	// Check for GPU label
	if node.Labels != nil {
		if _, exists := node.Labels["nvidia.com/gpu"]; exists {
			return true
		}
	}

	return false
}
