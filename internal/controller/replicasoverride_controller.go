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
	"fmt"
	"strconv"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	dynamicscalingv1 "github.com/KubeDynamicScaler/kubedynamicscaler/api/v1"
	"github.com/KubeDynamicScaler/kubedynamicscaler/pkg/config"
	"github.com/KubeDynamicScaler/kubedynamicscaler/pkg/utils"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ReplicasOverrideReconciler reconciles a ReplicasOverride object
type ReplicasOverrideReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Config *config.Manager
}

// +kubebuilder:rbac:groups=kubedynamicscaler.io,resources=replicasoverrides,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubedynamicscaler.io,resources=replicasoverrides/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kubedynamicscaler.io,resources=replicasoverrides/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=get;list;watch;update;patch

// Reconcile handles the reconciliation of ReplicasOverride resources
func (r *ReplicasOverrideReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Try to get the ReplicasOverride
	override := &dynamicscalingv1.ReplicasOverride{}
	err := r.Get(ctx, req.NamespacedName, override)
	if err != nil {
		if client.IgnoreNotFound(err) == nil {
			// Not found error, try to handle as global config
			// Get the deployment from the same namespace as the request
			deployment := &appsv1.Deployment{}
			err := r.Get(ctx, types.NamespacedName{
				Name:      req.Name,
				Namespace: req.Namespace,
			}, deployment)
			if err != nil {
				return ctrl.Result{}, client.IgnoreNotFound(err)
			}

			// Check for ignore rules
			ignoreList := &dynamicscalingv1.GlobalReplicasIgnoreList{}
			if err := r.List(ctx, ignoreList); err != nil {
				log.Error(err, "Failed to list ignore rules")
				return ctrl.Result{}, err
			}

			for _, ignore := range ignoreList.Items {
				if shouldIgnore, _ := utils.ShouldIgnoreDeployment(deployment, &ignore); shouldIgnore {
					return ctrl.Result{}, nil
				}
			}

			// Process deployment with global config
			if err := r.processDeployment(ctx, deployment, nil); err != nil {
				log.Error(err, "Failed to process deployment with global config")
				return ctrl.Result{}, err
			}

			return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
		}
		return ctrl.Result{}, err
	}

	// Initialize status if needed
	if override.Status.AffectedDeployments == nil {
		override.Status.AffectedDeployments = []dynamicscalingv1.AffectedDeployment{}
	}

	// Get list of deployments to process
	deployments := &appsv1.DeploymentList{}
	listOpts := []client.ListOption{}

	// If using DeploymentRef, get only that deployment
	if override.Spec.DeploymentRef != nil {
		deployment := &appsv1.Deployment{}
		targetNamespace := override.Spec.DeploymentRef.Namespace
		if targetNamespace == "" {
			// If namespace is not specified in DeploymentRef, use the override's namespace
			targetNamespace = override.Namespace
		}
		err := r.Get(ctx, types.NamespacedName{
			Name:      override.Spec.DeploymentRef.Name,
			Namespace: targetNamespace,
		}, deployment)
		if err != nil {
			if client.IgnoreNotFound(err) == nil {
				// If deployment is not found, we don't need to do anything
				return ctrl.Result{}, nil
			}
			return ctrl.Result{}, err
		}
		deployments.Items = []appsv1.Deployment{*deployment}
	} else if override.Spec.Selector != nil && len(override.Spec.Selector.MatchLabels) > 0 {
		// If using Selector, get matching deployments
		listOpts = append(listOpts, client.MatchingLabels(override.Spec.Selector.MatchLabels))
		if err := r.List(ctx, deployments, listOpts...); err != nil {
			log.Error(err, "Failed to list deployments")
			return ctrl.Result{}, err
		}
	}

	// Get previously affected deployments that might no longer match
	for _, affected := range override.Status.AffectedDeployments {
		found := false
		for _, deployment := range deployments.Items {
			if deployment.Name == affected.Name && deployment.Namespace == affected.Namespace {
				found = true
				break
			}
		}
		if !found {
			// This deployment was previously affected but no longer matches
			// We need to clean it up
			deployment := &appsv1.Deployment{}
			err := r.Get(ctx, types.NamespacedName{
				Name:      affected.Name,
				Namespace: affected.Namespace,
			}, deployment)
			if err != nil {
				if client.IgnoreNotFound(err) == nil {
					continue
				}
				log.Error(err, "Failed to get previously affected deployment")
				continue
			}

			// Clean up override annotations
			err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
				latest := &appsv1.Deployment{}
				if err := r.Get(ctx, types.NamespacedName{
					Name:      deployment.Name,
					Namespace: deployment.Namespace,
				}, latest); err != nil {
					return err
				}

				// Store original replicas before cleaning up
				originalReplicas := ""
				if val, exists := latest.Annotations[utils.OriginalReplicasAnnotation]; exists {
					originalReplicas = val
				}

				// Remove all controller annotations
				delete(latest.Annotations, utils.OverrideControllerAnnotation)
				delete(latest.Annotations, utils.ManagedAnnotation)
				delete(latest.Annotations, utils.GlobalConfigManagedAnnotation)
				delete(latest.Annotations, utils.ManagementModeAnnotation)
				delete(latest.Annotations, utils.LastUpdateAnnotation)
				delete(latest.Annotations, utils.OriginalReplicasAnnotation)

				// Reset to original replicas if we had stored them
				if originalReplicas != "" {
					replicas, _ := strconv.ParseInt(originalReplicas, 10, 32)
					count := int32(replicas)
					latest.Spec.Replicas = &count
				}

				// Clean up any associated HPA
				hpaList := &autoscalingv2.HorizontalPodAutoscalerList{}
				if err := r.List(ctx, hpaList, client.InNamespace(latest.Namespace)); err != nil {
					return err
				}

				for _, hpa := range hpaList.Items {
					if hpa.Spec.ScaleTargetRef.Kind == "Deployment" &&
						hpa.Spec.ScaleTargetRef.Name == latest.Name &&
						hpa.Spec.ScaleTargetRef.APIVersion == "apps/v1" {

						// Clean up HPA annotations and reset to original values
						err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
							latestHPA := &autoscalingv2.HorizontalPodAutoscaler{}
							if err := r.Get(ctx, types.NamespacedName{
								Name:      hpa.Name,
								Namespace: hpa.Namespace,
							}, latestHPA); err != nil {
								return err
							}

							// Store original values before cleaning
							originalMin := ""
							originalMax := ""
							if val, exists := latestHPA.Annotations[utils.OriginalMinReplicasAnnotation]; exists {
								originalMin = val
							}
							if val, exists := latestHPA.Annotations[utils.OriginalMaxReplicasAnnotation]; exists {
								originalMax = val
							}

							// Remove all controller annotations
							delete(latestHPA.Annotations, utils.OverrideControllerAnnotation)
							delete(latestHPA.Annotations, utils.ManagedAnnotation)
							delete(latestHPA.Annotations, utils.GlobalConfigManagedAnnotation)
							delete(latestHPA.Annotations, utils.HPAManagedAnnotation)
							delete(latestHPA.Annotations, utils.LastHPAUpdateAnnotation)
							delete(latestHPA.Annotations, utils.OriginalMinReplicasAnnotation)
							delete(latestHPA.Annotations, utils.OriginalMaxReplicasAnnotation)

							// Reset to original values if we had stored them
							if originalMin != "" {
								minReplicas, _ := strconv.ParseInt(originalMin, 10, 32)
								min := int32(minReplicas)
								latestHPA.Spec.MinReplicas = &min
							}
							if originalMax != "" {
								maxReplicas, _ := strconv.ParseInt(originalMax, 10, 32)
								latestHPA.Spec.MaxReplicas = int32(maxReplicas)
							}

							return r.Update(ctx, latestHPA)
						})
						if err != nil {
							log.Error(err, "Failed to clean up HPA", "hpa", hpa.Name)
						} else {
							log.Info("Cleaned up HPA that was associated with deployment",
								"hpa", hpa.Name,
								"namespace", hpa.Namespace)
						}
						break
					}
				}

				return r.Update(ctx, latest)
			})
			if err != nil {
				log.Error(err, "Failed to clean up deployment", "deployment", affected.Name)
			} else {
				log.Info("Cleaned up deployment that no longer matches override",
					"deployment", affected.Name,
					"namespace", affected.Namespace)
			}
		}
	}

	// Process each deployment
	affectedDeployments := []dynamicscalingv1.AffectedDeployment{}
	for _, deployment := range deployments.Items {
		// Skip if this deployment should not be processed
		if !shouldProcessDeployment(&deployment, override) {
			continue
		}

		// Check for ignore rules
		ignoreList := &dynamicscalingv1.GlobalReplicasIgnoreList{}
		if err := r.List(ctx, ignoreList); err != nil {
			log.Error(err, "Failed to list ignore rules")
			return ctrl.Result{}, err
		}

		ignored := false
		for _, ignore := range ignoreList.Items {
			if shouldIgnore, _ := utils.ShouldIgnoreDeployment(&deployment, &ignore); shouldIgnore {
				ignored = true
				break
			}
		}

		if ignored {
			continue
		}

		// Get the original replicas before processing
		originalReplicas, _ := strconv.ParseInt(deployment.Annotations[utils.OriginalReplicasAnnotation], 10, 32)

		// Process the deployment
		if err := r.processDeployment(ctx, &deployment, override); err != nil {
			log.Error(err, "Failed to process deployment", "deployment", deployment.Name)
			continue
		}

		// Get the updated deployment to record the current replicas
		updatedDeployment := &appsv1.Deployment{}
		if err := r.Get(ctx, types.NamespacedName{
			Name:      deployment.Name,
			Namespace: deployment.Namespace,
		}, updatedDeployment); err != nil {
			log.Error(err, "Failed to get updated deployment", "deployment", deployment.Name)
			continue
		}

		// Record the affected deployment
		affectedDeployments = append(affectedDeployments, dynamicscalingv1.AffectedDeployment{
			Name:              deployment.Name,
			Namespace:         deployment.Namespace,
			OriginalReplicas:  int32(originalReplicas),
			CurrentReplicas:   *updatedDeployment.Spec.Replicas,
			CurrentPercentage: override.Spec.ReplicasPercentage,
		})
	}

	// Update status with retry
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Get the latest version of ReplicasOverride before attempting to update it
		latestOverride := &dynamicscalingv1.ReplicasOverride{}
		if err := r.Get(ctx, req.NamespacedName, latestOverride); err != nil {
			return err
		}

		// Update status
		latestOverride.Status.AffectedDeployments = affectedDeployments
		latestOverride.Status.LastUpdateTime = &metav1.Time{Time: time.Now()}

		return r.Status().Update(ctx, latestOverride)
	})

	if retryErr != nil {
		log.Error(retryErr, "Failed to update ReplicasOverride status")
		return ctrl.Result{}, retryErr
	}

	// Requeue after 5 minutes
	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

// processDeployment handles the scaling of a single deployment
func (r *ReplicasOverrideReconciler) processDeployment(ctx context.Context, deployment *appsv1.Deployment, override *dynamicscalingv1.ReplicasOverride) error {
	log := log.FromContext(ctx)

	// Check if there's an HPA managing this deployment
	hpaList := &autoscalingv2.HorizontalPodAutoscalerList{}
	if err := r.List(ctx, hpaList, client.InNamespace(deployment.Namespace)); err != nil {
		return err
	}

	var existingHPA *autoscalingv2.HorizontalPodAutoscaler
	for _, hpa := range hpaList.Items {
		if hpa.Spec.ScaleTargetRef.Kind == "Deployment" &&
			hpa.Spec.ScaleTargetRef.Name == deployment.Name &&
			hpa.Spec.ScaleTargetRef.APIVersion == "apps/v1" {
			existingHPA = &hpa
			break
		}
	}

	// Get current annotations or initialize empty map
	if deployment.Annotations == nil {
		deployment.Annotations = make(map[string]string)
	}

	// Store original replicas if not already stored
	if _, exists := deployment.Annotations[utils.OriginalReplicasAnnotation]; !exists {
		if existingHPA != nil {
			// If HPA exists, use its minReplicas as the original replicas
			deployment.Annotations[utils.OriginalReplicasAnnotation] = strconv.FormatInt(int64(*existingHPA.Spec.MinReplicas), 10)
		} else {
			deployment.Annotations[utils.OriginalReplicasAnnotation] = strconv.FormatInt(int64(*deployment.Spec.Replicas), 10)
		}
	}

	// Mark as managed by us
	if override != nil {
		deployment.Annotations[utils.OverrideControllerAnnotation] = "true"
		deployment.Annotations[utils.ManagedAnnotation] = "true"
	} else {
		deployment.Annotations[utils.GlobalConfigManagedAnnotation] = "true"
	}

	// Add management mode annotation for troubleshooting
	if existingHPA != nil {
		deployment.Annotations[utils.ManagementModeAnnotation] = "hpa"
		// Update the deployment first with retry
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			// Get the latest version before attempting to update
			latest := &appsv1.Deployment{}
			if err := r.Get(ctx, types.NamespacedName{Name: deployment.Name, Namespace: deployment.Namespace}, latest); err != nil {
				return err
			}
			// Copy annotations to latest version
			if latest.Annotations == nil {
				latest.Annotations = make(map[string]string)
			}
			latest.Annotations[utils.ManagementModeAnnotation] = "hpa"
			latest.Annotations[utils.GlobalConfigManagedAnnotation] = "true"
			return r.Update(ctx, latest)
		})
		if err != nil {
			return err
		}
		// Then process the HPA
		return r.processHPA(ctx, existingHPA, override)
	} else {
		deployment.Annotations[utils.ManagementModeAnnotation] = "direct"
	}

	// Get global config
	config := r.Config.GetConfig()
	if config == nil {
		return fmt.Errorf("global config not found")
	}

	// Get original replicas
	originalReplicas, _ := strconv.ParseInt(deployment.Annotations[utils.OriginalReplicasAnnotation], 10, 32)
	var percentage int32

	if override != nil {
		// Use override percentage
		percentage = override.Spec.ReplicasPercentage
	} else {
		// Use global percentage
		percentage = config.GlobalPercentage
	}

	// Calculate target replicas based on percentage
	targetReplicas := int32(float64(originalReplicas) * float64(percentage) / 100.0)

	// Apply min/max limits from config
	if targetReplicas < config.MinReplicas {
		targetReplicas = config.MinReplicas
	}
	if targetReplicas > config.MaxReplicas {
		targetReplicas = config.MaxReplicas
	}

	// If HPA exists, let it manage the replicas
	if existingHPA != nil {
		// Only update the HPA
		return r.processHPA(ctx, existingHPA, override)
	}

	// Update replicas only if no HPA exists
	deployment.Spec.Replicas = &targetReplicas
	deployment.Annotations[utils.LastUpdateAnnotation] = time.Now().UTC().Format(time.RFC3339)

	log.Info("Updating deployment replicas",
		"deployment", deployment.Name,
		"namespace", deployment.Namespace,
		"originalReplicas", deployment.Annotations[utils.OriginalReplicasAnnotation],
		"newReplicas", targetReplicas,
		"percentage", percentage,
		"managementMode", deployment.Annotations[utils.ManagementModeAnnotation])

	// Update the deployment
	return r.Update(ctx, deployment)
}

func calculateTargetReplicas(deployment *appsv1.Deployment, percentage int32) int32 {
	originalReplicas, _ := strconv.ParseInt(deployment.Annotations[utils.OriginalReplicasAnnotation], 10, 32)
	return int32(float64(originalReplicas) * float64(percentage) / 100.0)
}

// processHPA handles updating an HPA's min/max replicas
func (r *ReplicasOverrideReconciler) processHPA(ctx context.Context, hpa *autoscalingv2.HorizontalPodAutoscaler, override *dynamicscalingv1.ReplicasOverride) error {
	log := log.FromContext(ctx)

	// Get current annotations or initialize empty map
	if hpa.Annotations == nil {
		hpa.Annotations = make(map[string]string)
	}

	// Store original min/max if not already stored
	if _, exists := hpa.Annotations[utils.OriginalMinReplicasAnnotation]; !exists {
		hpa.Annotations[utils.OriginalMinReplicasAnnotation] = strconv.FormatInt(int64(*hpa.Spec.MinReplicas), 10)
	}
	if _, exists := hpa.Annotations[utils.OriginalMaxReplicasAnnotation]; !exists {
		hpa.Annotations[utils.OriginalMaxReplicasAnnotation] = strconv.FormatInt(int64(hpa.Spec.MaxReplicas), 10)
	}

	// Mark as managed by us
	if override != nil {
		hpa.Annotations[utils.OverrideControllerAnnotation] = "true"
		hpa.Annotations[utils.ManagedAnnotation] = "true"
	} else {
		hpa.Annotations[utils.GlobalConfigManagedAnnotation] = "true"
	}
	hpa.Annotations[utils.HPAManagedAnnotation] = "true"

	// Get global config
	config := r.Config.GetConfig()
	if config == nil {
		return fmt.Errorf("global config not found")
	}

	// Calculate target min/max replicas
	originalMinReplicas, _ := strconv.ParseInt(hpa.Annotations[utils.OriginalMinReplicasAnnotation], 10, 32)
	originalMaxReplicas, _ := strconv.ParseInt(hpa.Annotations[utils.OriginalMaxReplicasAnnotation], 10, 32)

	var targetMinReplicas, targetMaxReplicas int32
	var percentage int32

	if override != nil {
		// Use override percentage
		percentage = override.Spec.ReplicasPercentage
	} else {
		// Use global percentage
		percentage = config.GlobalPercentage
	}

	// Calculate new values based on percentage
	targetMinReplicas = int32(float64(originalMinReplicas) * float64(percentage) / 100.0)
	targetMaxReplicas = int32(float64(originalMaxReplicas) * float64(percentage) / 100.0)

	// Apply min/max limits from config
	if targetMinReplicas < config.MinReplicas {
		targetMinReplicas = config.MinReplicas
	}
	if targetMaxReplicas > config.MaxReplicas {
		targetMaxReplicas = config.MaxReplicas
	}

	// Ensure min <= max
	if targetMinReplicas > targetMaxReplicas {
		targetMinReplicas = targetMaxReplicas
	}

	// Update HPA
	hpa.Spec.MinReplicas = &targetMinReplicas
	hpa.Spec.MaxReplicas = targetMaxReplicas
	hpa.Annotations[utils.LastHPAUpdateAnnotation] = time.Now().UTC().Format(time.RFC3339)

	log.Info("Updating HPA replicas",
		"hpa", hpa.Name,
		"namespace", hpa.Namespace,
		"originalMinReplicas", hpa.Annotations[utils.OriginalMinReplicasAnnotation],
		"originalMaxReplicas", hpa.Annotations[utils.OriginalMaxReplicasAnnotation],
		"newMinReplicas", targetMinReplicas,
		"newMaxReplicas", targetMaxReplicas,
		"percentage", percentage)

	return r.Update(ctx, hpa)
}

// shouldProcessDeployment determines if a deployment should be processed based on the override spec
func shouldProcessDeployment(deployment *appsv1.Deployment, override *dynamicscalingv1.ReplicasOverride) bool {
	// If using DeploymentRef, check if this is the target deployment
	if override.Spec.DeploymentRef != nil {
		if override.Spec.DeploymentRef.Name == deployment.Name {
			if override.Spec.DeploymentRef.Namespace == "" || override.Spec.DeploymentRef.Namespace == deployment.Namespace {
				return true
			}
		}
		return false
	}

	// If using Selector, check if the deployment matches the labels
	if override.Spec.Selector != nil && len(override.Spec.Selector.MatchLabels) > 0 {
		for key, value := range override.Spec.Selector.MatchLabels {
			if deployment.Labels[key] != value {
				return false
			}
		}
		return true
	}

	return false
}

// SetupWithManager sets up the controller with the Manager.
func (r *ReplicasOverrideReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dynamicscalingv1.ReplicasOverride{}).
		Watches(
			&appsv1.Deployment{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
				deployment, ok := obj.(*appsv1.Deployment)
				if !ok {
					return nil
				}

				// Check for ignore rules first
				ignoreList := &dynamicscalingv1.GlobalReplicasIgnoreList{}
				if err := r.List(ctx, ignoreList); err != nil {
					return nil
				}

				for _, ignore := range ignoreList.Items {
					if shouldIgnore, _ := utils.ShouldIgnoreDeployment(deployment, &ignore); shouldIgnore {
						return nil
					}
				}

				// Get all ReplicasOverrides
				overrideList := &dynamicscalingv1.ReplicasOverrideList{}
				if err := r.List(ctx, overrideList); err != nil {
					return nil
				}

				var requests []reconcile.Request
				foundMatch := false

				// Check each override for a match
				for _, override := range overrideList.Items {
					if shouldProcessDeployment(deployment, &override) {
						requests = append(requests, reconcile.Request{
							NamespacedName: types.NamespacedName{
								Name:      override.Name,
								Namespace: override.Namespace,
							},
						})
						foundMatch = true
					}
				}

				// If no specific override matches and deployment is not ignored, use global config
				if !foundMatch {
					// Process with global config
					deployment.Annotations[utils.GlobalConfigManagedAnnotation] = "true"
					if err := r.Update(ctx, deployment); err != nil {
						return nil
					}
				}

				return requests
			}),
		).
		Watches(
			&autoscalingv2.HorizontalPodAutoscaler{},
			handler.EnqueueRequestsFromMapFunc(r.findReplicasOverridesForHPA),
		).
		Complete(r)
}

// findReplicasOverridesForHPA maps an HPA to a list of ReplicasOverride requests
func (r *ReplicasOverrideReconciler) findReplicasOverridesForHPA(ctx context.Context, obj client.Object) []reconcile.Request {
	log := log.FromContext(ctx)
	hpa, ok := obj.(*autoscalingv2.HorizontalPodAutoscaler)
	if !ok {
		log.Error(nil, "Expected an HPA but got something else", "object", obj)
		return nil
	}

	// Get the deployment that this HPA targets
	deployment := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      hpa.Spec.ScaleTargetRef.Name,
		Namespace: hpa.Namespace,
	}, deployment)
	if err != nil {
		log.Error(err, "Failed to get deployment for HPA", "deployment", hpa.Spec.ScaleTargetRef.Name)
		return nil
	}

	// Check for ignore rules first
	ignoreList := &dynamicscalingv1.GlobalReplicasIgnoreList{}
	if err := r.List(ctx, ignoreList); err != nil {
		log.Error(err, "Failed to list ignore rules")
		return nil
	}

	for _, ignore := range ignoreList.Items {
		if shouldIgnore, _ := utils.ShouldIgnoreDeployment(deployment, &ignore); shouldIgnore {
			return nil
		}
	}

	// Get all ReplicasOverrides
	overrideList := &dynamicscalingv1.ReplicasOverrideList{}
	if err := r.List(ctx, overrideList); err != nil {
		log.Error(err, "Failed to list ReplicasOverrides")
		return nil
	}

	var requests []reconcile.Request
	foundMatch := false

	// Check each override for a match
	for _, override := range overrideList.Items {
		if override.Spec.DeploymentRef != nil &&
			override.Spec.DeploymentRef.Name == deployment.Name &&
			override.Spec.DeploymentRef.Namespace == deployment.Namespace {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      override.Name,
					Namespace: override.Namespace,
				},
			})
			foundMatch = true
		} else if override.Spec.Selector != nil && len(override.Spec.Selector.MatchLabels) > 0 {
			// Check if deployment labels match the selector
			match := true
			for key, value := range override.Spec.Selector.MatchLabels {
				if deployment.Labels[key] != value {
					match = false
					break
				}
			}
			if match {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      override.Name,
						Namespace: override.Namespace,
					},
				})
				foundMatch = true
			}
		}
	}

	// If no override matches, create a request for global rules
	if !foundMatch {
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      hpa.Name,
				Namespace: hpa.Namespace,
			},
		})
	}

	return requests
}
