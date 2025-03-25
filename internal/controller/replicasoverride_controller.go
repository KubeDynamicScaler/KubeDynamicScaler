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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch

// Reconcile handles the reconciliation of ReplicasOverride resources
func (r *ReplicasOverrideReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// 1. First, get the list of ignored deployments
	ignoreList := &dynamicscalingv1.GlobalReplicasIgnoreList{}
	if err := r.List(ctx, ignoreList); err != nil {
		log.Error(err, "Failed to list ignore rules")
		return ctrl.Result{}, err
	}

	// Create a map of ignored deployments for quick access
	ignoredDeployments := make(map[string]bool)
	for _, ignore := range ignoreList.Items {
		// Verifies by namespace
		for _, namespace := range ignore.Spec.IgnoreNamespaces {
			deployments := &appsv1.DeploymentList{}
			if err := r.List(ctx, deployments, client.InNamespace(namespace)); err != nil {
				continue
			}
			for _, deployment := range deployments.Items {
				ignoredDeployments[deployment.Namespace+"/"+deployment.Name] = true
			}
		}

		// Verifies by specific resources
		for _, resource := range ignore.Spec.IgnoreResources {
			if resource.Kind == "Deployment" {
				namespace := resource.Namespace
				if namespace == "" {
					namespace = "default"
				}
				ignoredDeployments[namespace+"/"+resource.Name] = true
			}
		}

		// Verifies by labels
		if len(ignore.Spec.IgnoreLabels) > 0 {
			deployments := &appsv1.DeploymentList{}
			if err := r.List(ctx, deployments, client.MatchingLabels(ignore.Spec.IgnoreLabels)); err != nil {
				continue
			}
			for _, deployment := range deployments.Items {
				ignoredDeployments[deployment.Namespace+"/"+deployment.Name] = true
			}
		}
	}

	// 2. List all namespaces except the ignored ones
	namespaces := &corev1.NamespaceList{}
	if err := r.List(ctx, namespaces); err != nil {
		log.Error(err, "Failed to list namespaces")
		return ctrl.Result{}, err
	}

	// Create a map of ignored namespaces for quick access
	ignoredNamespaces := make(map[string]bool)
	for _, ignore := range ignoreList.Items {
		for _, namespace := range ignore.Spec.IgnoreNamespaces {
			ignoredNamespaces[namespace] = true
		}
	}

	// 3. For each namespace not ignored, list and process the deployments
	for _, namespace := range namespaces.Items {
		// Skips if the namespace is in the ignored list
		if ignoredNamespaces[namespace.Name] {
			continue
		}

		// List all deployments in the namespace
		deployments := &appsv1.DeploymentList{}
		if err := r.List(ctx, deployments, client.InNamespace(namespace.Name)); err != nil {
			log.Error(err, "Failed to list deployments in namespace", "namespace", namespace.Name)
			continue
		}

		// 4. For each deployment, check if it should be processed
		for _, deployment := range deployments.Items {
			// Skips if it's in the ignored list
			if ignoredDeployments[deployment.Namespace+"/"+deployment.Name] {
				continue
			}

			// 5. Check if there's a specific override
			var override *dynamicscalingv1.ReplicasOverride
			overrideList := &dynamicscalingv1.ReplicasOverrideList{}
			if err := r.List(ctx, overrideList, client.InNamespace(deployment.Namespace)); err != nil {
				log.Error(err, "Failed to list overrides")
				continue
			}

			// Search for an override that matches the deployment
			for _, o := range overrideList.Items {
				if o.Spec.DeploymentRef != nil {
					if o.Spec.DeploymentRef.Name == deployment.Name &&
						(o.Spec.DeploymentRef.Namespace == "" || o.Spec.DeploymentRef.Namespace == deployment.Namespace) {
						override = &o
						break
					}
				} else if o.Spec.Selector != nil && len(o.Spec.Selector.MatchLabels) > 0 {
					matches := true
					for key, value := range o.Spec.Selector.MatchLabels {
						if deployment.Labels[key] != value {
							matches = false
							break
						}
					}
					if matches {
						override = &o
						break
					}
				}
			}

			// 6. Process the deployment with the override or global configuration
			if err := r.processDeployment(ctx, &deployment, override); err != nil {
				log.Error(err, "Failed to process deployment",
					"deployment", deployment.Name,
					"namespace", deployment.Namespace,
					"hasOverride", override != nil)
				continue
			}

			// Update the override status with the affected deployment
			if override != nil {
				// Check if the deployment already exists in the status
				deploymentExists := false
				for _, affected := range override.Status.AffectedDeployments {
					if affected.Name == deployment.Name && affected.Namespace == deployment.Namespace {
						deploymentExists = true
						affected.CurrentReplicas = *deployment.Spec.Replicas
						break
					}
				}

				// If it doesn't exist, add to the status
				if !deploymentExists {
					override.Status.AffectedDeployments = append(override.Status.AffectedDeployments, dynamicscalingv1.AffectedDeployment{
						Name:            deployment.Name,
						Namespace:       deployment.Namespace,
						CurrentReplicas: *deployment.Spec.Replicas,
					})
				}

				// Update the override status
				if err := r.Status().Update(ctx, override); err != nil {
					log.Error(err, "Failed to update override status",
						"override", override.Name,
						"namespace", override.Namespace)
				}
			}
		}
	}

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
			latest.Annotations[utils.OriginalReplicasAnnotation] = deployment.Annotations[utils.OriginalReplicasAnnotation]
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

	// Check if update is needed
	if deployment.Spec.Replicas != nil && *deployment.Spec.Replicas == targetReplicas {
		log.Info("Deployment already at desired replicas, skipping update",
			"deployment", deployment.Name,
			"namespace", deployment.Namespace,
			"replicas", targetReplicas)
		return nil
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
	// If no override is provided, this is a global config request
	if override == nil {
		return true
	}

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
			client.Object(&appsv1.Deployment{}),
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

				// If no specific override matches and deployment is not ignored, trigger reconciliation
				// with an empty ReplicasOverride name to handle global config
				if !foundMatch {
					requests = append(requests, reconcile.Request{
						NamespacedName: types.NamespacedName{
							Name:      "", // Empty name to indicate global config processing
							Namespace: deployment.Namespace,
						},
					})
				}

				return requests
			}),
		).
		Watches(
			client.Object(&autoscalingv2.HorizontalPodAutoscaler{}),
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
				hpa, ok := obj.(*autoscalingv2.HorizontalPodAutoscaler)
				if !ok {
					return nil
				}

				// Get the deployment that this HPA targets
				deployment := &appsv1.Deployment{}
				err := r.Get(ctx, types.NamespacedName{
					Name:      hpa.Spec.ScaleTargetRef.Name,
					Namespace: hpa.Namespace,
				}, deployment)
				if err != nil {
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

				// If no override matches, trigger reconciliation with an empty name
				if !foundMatch {
					requests = append(requests, reconcile.Request{
						NamespacedName: types.NamespacedName{
							Name:      "", // Empty name to indicate global config processing
							Namespace: hpa.Namespace,
						},
					})
				}

				return requests
			}),
		).
		Watches(
			client.Object(&corev1.ConfigMap{}),
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
				configMap := obj.(*corev1.ConfigMap)
				if configMap.Name == "replicas-controller-config" && configMap.Namespace == "kubedynamicscaler-system" {
					// When the ConfigMap changes, we need to reconcile all deployments
					deployments := &appsv1.DeploymentList{}
					if err := r.List(ctx, deployments); err != nil {
						return nil
					}

					var requests []reconcile.Request
					for _, deployment := range deployments.Items {
						// Skip deployments that should be ignored
						ignoreList := &dynamicscalingv1.GlobalReplicasIgnoreList{}
						if err := r.List(ctx, ignoreList); err != nil {
							continue
						}

						shouldProcess := true
						for _, ignore := range ignoreList.Items {
							if shouldIgnore, _ := utils.ShouldIgnoreDeployment(&deployment, &ignore); shouldIgnore {
								shouldProcess = false
								break
							}
						}

						if shouldProcess {
							requests = append(requests, reconcile.Request{
								NamespacedName: types.NamespacedName{
									Name:      "", // Empty name to indicate global config processing
									Namespace: deployment.Namespace,
								},
							})
						}
					}
					return requests
				}
				return nil
			}),
		).
		Complete(r)
}

// updateDeploymentAnnotations updates deployment annotations with retry logic
func (r *ReplicasOverrideReconciler) updateDeploymentAnnotations(ctx context.Context, deployment *appsv1.Deployment, annotations map[string]string) error {
	log := log.FromContext(ctx)
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		// Get the latest version of the deployment
		latestDeployment := &appsv1.Deployment{}
		if err := r.Get(ctx, types.NamespacedName{
			Name:      deployment.Name,
			Namespace: deployment.Namespace,
		}, latestDeployment); err != nil {
			return err
		}

		// Update annotations
		if latestDeployment.Annotations == nil {
			latestDeployment.Annotations = make(map[string]string)
		}
		for key, value := range annotations {
			latestDeployment.Annotations[key] = value
		}

		// Try to update
		if err := r.Update(ctx, latestDeployment); err != nil {
			if errors.IsConflict(err) {
				log.Info("Conflict while updating deployment annotations, retrying...", "attempt", i+1)
				continue
			}
			return err
		}
		return nil
	}
	return fmt.Errorf("failed to update deployment annotations after %d retries", maxRetries)
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
