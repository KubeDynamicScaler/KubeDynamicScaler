package utils

import (
	"math"
	"strconv"
	"time"

	v1 "github.com/KubeDynamicScaler/kubedynamicscaler/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
)

const (
	// Domain prefix for all annotations
	annotationDomain = "kubedynamicscaler.io"

	// Deployment annotations
	OriginalReplicasAnnotation    = annotationDomain + "/original-replicas"
	OverrideControllerAnnotation  = annotationDomain + "/override-controller"
	LastUpdateAnnotation          = annotationDomain + "/last-update"
	ManagedAnnotation             = annotationDomain + "/managed"
	GlobalConfigManagedAnnotation = annotationDomain + "/global-config-managed"
	ManagementModeAnnotation      = annotationDomain + "/management-mode" // Values: "direct" or "hpa"

	// HPA specific annotations
	HPAManagedAnnotation          = annotationDomain + "/hpa-managed"
	OriginalMinReplicasAnnotation = annotationDomain + "/hpa-original-min"
	OriginalMaxReplicasAnnotation = annotationDomain + "/hpa-original-max"
	LastHPAUpdateAnnotation       = annotationDomain + "/last-hpa-update"
)

// InitializeAnnotations initializes the required annotations for a deployment
func InitializeAnnotations(deployment *appsv1.Deployment) {
	if deployment.Annotations == nil {
		deployment.Annotations = make(map[string]string)
	}

	// Set original replicas if not set
	if _, exists := deployment.Annotations[OriginalReplicasAnnotation]; !exists {
		deployment.Annotations[OriginalReplicasAnnotation] = strconv.FormatInt(int64(*deployment.Spec.Replicas), 10)
	}

	// Mark as managed
	deployment.Annotations[ManagedAnnotation] = "true"
	deployment.Annotations[LastUpdateAnnotation] = time.Now().UTC().Format(time.RFC3339)
}

// InitializeHPAAnnotations initializes the required annotations for an HPA
func InitializeHPAAnnotations(hpa *autoscalingv2.HorizontalPodAutoscaler) {
	if hpa.Annotations == nil {
		hpa.Annotations = make(map[string]string)
	}

	// Set original min/max if not set
	if _, exists := hpa.Annotations[OriginalMinReplicasAnnotation]; !exists && hpa.Spec.MinReplicas != nil {
		hpa.Annotations[OriginalMinReplicasAnnotation] = strconv.FormatInt(int64(*hpa.Spec.MinReplicas), 10)
	}
	if _, exists := hpa.Annotations[OriginalMaxReplicasAnnotation]; !exists {
		hpa.Annotations[OriginalMaxReplicasAnnotation] = strconv.FormatInt(int64(hpa.Spec.MaxReplicas), 10)
	}

	// Mark as managed
	hpa.Annotations[HPAManagedAnnotation] = "true"
	hpa.Annotations[LastHPAUpdateAnnotation] = time.Now().UTC().Format(time.RFC3339)
}

// GetOriginalReplicas gets the original replicas from annotations
func GetOriginalReplicas(deployment *appsv1.Deployment) int32 {
	if val, exists := deployment.Annotations[OriginalReplicasAnnotation]; exists {
		if parsed, err := strconv.ParseInt(val, 10, 32); err == nil {
			return int32(parsed)
		}
	}
	return *deployment.Spec.Replicas
}

// GetOriginalHPALimits gets the original min and max replicas from annotations
func GetOriginalHPALimits(hpa *autoscalingv2.HorizontalPodAutoscaler) (int32, int32) {
	var originalMin, originalMax int32

	if val, exists := hpa.Annotations[OriginalMinReplicasAnnotation]; exists {
		if parsed, err := strconv.ParseInt(val, 10, 32); err == nil {
			originalMin = int32(parsed)
		}
	} else if hpa.Spec.MinReplicas != nil {
		originalMin = *hpa.Spec.MinReplicas
	} else {
		originalMin = 1
	}

	if val, exists := hpa.Annotations[OriginalMaxReplicasAnnotation]; exists {
		if parsed, err := strconv.ParseInt(val, 10, 32); err == nil {
			originalMax = int32(parsed)
		}
	} else {
		originalMax = hpa.Spec.MaxReplicas
	}

	return originalMin, originalMax
}

// CalculateNewReplicas calculates the new number of replicas based on the override type and percentage
func CalculateNewReplicas(deployment *appsv1.Deployment, override *v1.ReplicasOverride) int32 {
	// Get original replicas from annotations
	baseReplicas := GetOriginalReplicas(deployment)

	percentage := float64(override.Spec.ReplicasPercentage)
	newReplicas := float64(baseReplicas) * (percentage / 100.0)

	// Round to nearest integer and ensure it's at least 1
	result := int32(math.Max(1, math.Round(newReplicas)))

	// Cap the result at MaxInt32 to prevent overflow
	if result > math.MaxInt32 {
		result = math.MaxInt32
	}

	return result
}

// CalculateHPALimits calculates new min and max replicas for an HPA based on the override
func CalculateHPALimits(hpa *autoscalingv2.HorizontalPodAutoscaler, override *v1.ReplicasOverride) (int32, int32) {
	percentage := float64(override.Spec.ReplicasPercentage) / 100.0

	// Get original min and max from annotations
	originalMin, originalMax := GetOriginalHPALimits(hpa)

	// Calculate new min and max replicas based on percentage
	newMin := int32(math.Max(1, math.Round(float64(originalMin)*percentage)))
	newMax := int32(math.Max(float64(newMin), math.Round(float64(originalMax)*percentage)))

	return newMin, newMax
}

// ShouldIgnoreDeployment checks if a deployment should be ignored based on the ignore rules
func ShouldIgnoreDeployment(deployment *appsv1.Deployment, ignore *v1.GlobalReplicasIgnore) (bool, string) {
	// Check namespace
	for _, ns := range ignore.Spec.IgnoreNamespaces {
		if deployment.Namespace == ns {
			return true, "Namespace is in ignore list"
		}
	}

	// Check specific resources
	for _, res := range ignore.Spec.IgnoreResources {
		if res.Kind == "Deployment" && res.Name == deployment.Name {
			if res.Namespace == "" || res.Namespace == deployment.Namespace {
				return true, "Deployment is in ignore list"
			}
		}
	}

	// Check labels
	for key, value := range ignore.Spec.IgnoreLabels {
		if deployment.Labels[key] == value {
			return true, "Deployment has ignored label"
		}
	}

	return false, ""
}
