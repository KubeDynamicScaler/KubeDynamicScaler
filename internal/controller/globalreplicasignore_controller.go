/*
Copyright 2025.

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

package controller

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dynamicscalingv1 "github.com/KubeDynamicScaler/kubedynamicscaler/api/v1"
	"github.com/KubeDynamicScaler/kubedynamicscaler/pkg/utils"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GlobalReplicasIgnoreReconciler reconciles a GlobalReplicasIgnore object
type GlobalReplicasIgnoreReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=kubedynamicscaler.io,resources=globalreplicasignores,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubedynamicscaler.io,resources=globalreplicasignores/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kubedynamicscaler.io,resources=globalreplicasignores/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *GlobalReplicasIgnoreReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the GlobalReplicasIgnore instance
	ignore := &dynamicscalingv1.GlobalReplicasIgnore{}
	if err := r.Get(ctx, req.NamespacedName, ignore); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Initialize status if needed
	if ignore.Status.IgnoredDeployments == nil {
		ignore.Status.IgnoredDeployments = []dynamicscalingv1.IgnoredDeployment{}
	}

	// Get list of all deployments
	deployments := &appsv1.DeploymentList{}
	if err := r.List(ctx, deployments); err != nil {
		log.Error(err, "Failed to list deployments")
		return ctrl.Result{}, err
	}

	// Process each deployment
	ignoredDeployments := []dynamicscalingv1.IgnoredDeployment{}
	for _, deployment := range deployments.Items {
		shouldIgnore, reason := utils.ShouldIgnoreDeployment(&deployment, ignore)
		if shouldIgnore {
			ignoredDeployments = append(ignoredDeployments, dynamicscalingv1.IgnoredDeployment{
				Name:      deployment.Name,
				Namespace: deployment.Namespace,
				Reason:    reason,
			})
		}
	}

	// Update status
	ignore.Status.IgnoredDeployments = ignoredDeployments
	ignore.Status.LastUpdateTime = &metav1.Time{Time: time.Now()}

	if err := r.Status().Update(ctx, ignore); err != nil {
		log.Error(err, "Failed to update GlobalReplicasIgnore status")
		return ctrl.Result{}, err
	}

	// Requeue after 5 minutes
	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *GlobalReplicasIgnoreReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dynamicscalingv1.GlobalReplicasIgnore{}).
		Complete(r)
}
