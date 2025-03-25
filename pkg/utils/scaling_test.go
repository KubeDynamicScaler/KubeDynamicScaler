package utils

import (
	"testing"

	dynamicscalingv1 "github.com/KubeDynamicScaler/kubedynamicscaler/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// int32Ptr returns a pointer to an int32 value
func int32Ptr(v int32) *int32 {
	return &v
}

func TestCalculateNewReplicas(t *testing.T) {
	tests := []struct {
		name        string
		replicas    int32
		percent     int32
		minReplicas *int32
		maxReplicas *int32
		want        int32
	}{
		{
			name:        "100% keeps same replicas",
			replicas:    4,
			percent:     100,
			minReplicas: int32Ptr(2),
			maxReplicas: int32Ptr(5),
			want:        4,
		},
		{
			name:        "150% increases replicas but respects max",
			replicas:    4,
			percent:     150,
			minReplicas: int32Ptr(2),
			maxReplicas: int32Ptr(5),
			want:        5,
		},
		{
			name:        "50% decreases replicas but respects min",
			replicas:    4,
			percent:     50,
			minReplicas: int32Ptr(2),
			maxReplicas: int32Ptr(5),
			want:        2,
		},
		{
			name:        "75% rounds correctly",
			replicas:    4,
			percent:     75,
			minReplicas: int32Ptr(2),
			maxReplicas: int32Ptr(5),
			want:        3,
		},
		{
			name:        "respects min replicas (case 1)",
			replicas:    3,
			percent:     10,
			minReplicas: int32Ptr(2),
			maxReplicas: int32Ptr(5),
			want:        2,
		},
		{
			name:        "respects max replicas (case 2)",
			replicas:    2,
			percent:     400,
			minReplicas: int32Ptr(2),
			maxReplicas: int32Ptr(5),
			want:        5,
		},
		{
			name:        "respects min replicas with small percentage",
			replicas:    5,
			percent:     20,
			minReplicas: int32Ptr(2),
			maxReplicas: int32Ptr(5),
			want:        2,
		},
		{
			name:        "respects max replicas with large percentage",
			replicas:    3,
			percent:     500,
			minReplicas: int32Ptr(2),
			maxReplicas: int32Ptr(5),
			want:        5,
		},
		{
			name:        "no limits specified",
			replicas:    4,
			percent:     150,
			minReplicas: nil,
			maxReplicas: nil,
			want:        6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deployment := &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Replicas: &tt.replicas,
				},
			}

			override := &dynamicscalingv1.ReplicasOverride{
				Spec: dynamicscalingv1.ReplicasOverrideSpec{
					ReplicasPercentage: tt.percent,
					MinReplicas:        tt.minReplicas,
					MaxReplicas:        tt.maxReplicas,
				},
			}

			got := CalculateNewReplicas(deployment, override)
			if got != tt.want {
				t.Errorf("CalculateNewReplicas() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCalculateHPALimits(t *testing.T) {
	tests := []struct {
		name        string
		minRep      *int32
		maxRep      int32
		percent     int32
		minReplicas *int32
		maxReplicas *int32
		wantMin     int32
		wantMax     int32
	}{
		{
			name:        "100% keeps same limits",
			minRep:      ptr(int32(2)),
			maxRep:      10,
			percent:     100,
			minReplicas: int32Ptr(2),
			maxReplicas: int32Ptr(5),
			wantMin:     2,
			wantMax:     5,
		},
		{
			name:        "150% increases limits but respects max",
			minRep:      ptr(int32(2)),
			maxRep:      10,
			percent:     150,
			minReplicas: int32Ptr(2),
			maxReplicas: int32Ptr(5),
			wantMin:     2,
			wantMax:     5,
		},
		{
			name:        "50% decreases limits but respects min",
			minRep:      ptr(int32(2)),
			maxRep:      10,
			percent:     50,
			minReplicas: int32Ptr(2),
			maxReplicas: int32Ptr(5),
			wantMin:     2,
			wantMax:     5,
		},
		{
			name:        "nil minReplicas defaults to 1",
			minRep:      nil,
			maxRep:      10,
			percent:     150,
			minReplicas: int32Ptr(2),
			maxReplicas: int32Ptr(5),
			wantMin:     2,
			wantMax:     5,
		},
		{
			name:        "respects min limit (case 1)",
			minRep:      ptr(int32(3)),
			maxRep:      10,
			percent:     10,
			minReplicas: int32Ptr(2),
			maxReplicas: int32Ptr(5),
			wantMin:     2,
			wantMax:     5,
		},
		{
			name:        "respects max limit (case 2)",
			minRep:      ptr(int32(2)),
			maxRep:      10,
			percent:     400,
			minReplicas: int32Ptr(2),
			maxReplicas: int32Ptr(5),
			wantMin:     2,
			wantMax:     5,
		},
		{
			name:        "respects min limit with small percentage",
			minRep:      ptr(int32(5)),
			maxRep:      20,
			percent:     20,
			minReplicas: int32Ptr(2),
			maxReplicas: int32Ptr(5),
			wantMin:     2,
			wantMax:     5,
		},
		{
			name:        "respects max limit with large percentage",
			minRep:      ptr(int32(3)),
			maxRep:      15,
			percent:     500,
			minReplicas: int32Ptr(2),
			maxReplicas: int32Ptr(5),
			wantMin:     2,
			wantMax:     5,
		},
		{
			name:        "no limits specified",
			minRep:      ptr(int32(2)),
			maxRep:      10,
			percent:     150,
			minReplicas: nil,
			maxReplicas: nil,
			wantMin:     3,
			wantMax:     15,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hpa := &autoscalingv2.HorizontalPodAutoscaler{
				Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
					MinReplicas: tt.minRep,
					MaxReplicas: tt.maxRep,
				},
			}

			override := &dynamicscalingv1.ReplicasOverride{
				Spec: dynamicscalingv1.ReplicasOverrideSpec{
					ReplicasPercentage: tt.percent,
					MinReplicas:        tt.minReplicas,
					MaxReplicas:        tt.maxReplicas,
				},
			}

			gotMin, gotMax := CalculateHPALimits(hpa, override)
			if gotMin != tt.wantMin {
				t.Errorf("CalculateHPALimits() minReplicas = %v, want %v", gotMin, tt.wantMin)
			}
			if gotMax != tt.wantMax {
				t.Errorf("CalculateHPALimits() maxReplicas = %v, want %v", gotMax, tt.wantMax)
			}
		})
	}
}

func TestShouldIgnoreDeployment(t *testing.T) {
	tests := []struct {
		name       string
		deployment *appsv1.Deployment
		ignore     *dynamicscalingv1.GlobalReplicasIgnore
		want       bool
		wantReason string
	}{
		{
			name: "ignore by namespace",
			deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment",
					Namespace: "kube-system",
				},
			},
			ignore: &dynamicscalingv1.GlobalReplicasIgnore{
				Spec: dynamicscalingv1.GlobalReplicasIgnoreSpec{
					IgnoreNamespaces: []string{"kube-system"},
				},
			},
			want:       true,
			wantReason: "Namespace is in ignore list",
		},
		{
			name: "ignore by resource name",
			deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "critical-app",
					Namespace: "production",
				},
			},
			ignore: &dynamicscalingv1.GlobalReplicasIgnore{
				Spec: dynamicscalingv1.GlobalReplicasIgnoreSpec{
					IgnoreResources: []dynamicscalingv1.IgnoredResource{
						{
							Kind:      "Deployment",
							Name:      "critical-app",
							Namespace: "production",
						},
					},
				},
			},
			want:       true,
			wantReason: "Deployment is in ignore list",
		},
		{
			name: "ignore by label",
			deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment",
					Namespace: "default",
					Labels: map[string]string{
						"scaling.disabled": "true",
					},
				},
			},
			ignore: &dynamicscalingv1.GlobalReplicasIgnore{
				Spec: dynamicscalingv1.GlobalReplicasIgnoreSpec{
					IgnoreLabels: map[string]string{
						"scaling.disabled": "true",
					},
				},
			},
			want:       true,
			wantReason: "Deployment has ignored label",
		},
		{
			name: "do not ignore",
			deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment",
					Namespace: "default",
				},
			},
			ignore: &dynamicscalingv1.GlobalReplicasIgnore{
				Spec: dynamicscalingv1.GlobalReplicasIgnoreSpec{
					IgnoreNamespaces: []string{"kube-system"},
				},
			},
			want:       false,
			wantReason: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotReason := ShouldIgnoreDeployment(tt.deployment, tt.ignore)
			if got != tt.want {
				t.Errorf("ShouldIgnoreDeployment() = %v, want %v", got, tt.want)
			}
			if gotReason != tt.wantReason {
				t.Errorf("ShouldIgnoreDeployment() reason = %v, want %v", gotReason, tt.wantReason)
			}
		})
	}
}

func ptr[T any](v T) *T {
	return &v
}

func TestInitializeAnnotations(t *testing.T) {
	replicas := int32(3)
	deployment := &appsv1.Deployment{
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
		},
	}

	// Test initialization of annotations
	InitializeAnnotations(deployment)

	// Check if all required annotations are set
	if val, exists := deployment.Annotations[OriginalReplicasAnnotation]; !exists {
		t.Error("OriginalReplicasAnnotation not set")
	} else if val != "3" {
		t.Errorf("OriginalReplicasAnnotation = %v, want %v", val, "3")
	}

	if val, exists := deployment.Annotations[ManagedAnnotation]; !exists {
		t.Error("ManagedAnnotation not set")
	} else if val != "true" {
		t.Errorf("ManagedAnnotation = %v, want %v", val, "true")
	}

	if _, exists := deployment.Annotations[LastUpdateAnnotation]; !exists {
		t.Error("LastUpdateAnnotation not set")
	}

	// Test that original replicas are not overwritten
	replicas = 5
	InitializeAnnotations(deployment)
	if val := deployment.Annotations[OriginalReplicasAnnotation]; val != "3" {
		t.Errorf("OriginalReplicasAnnotation was overwritten, got %v, want %v", val, "3")
	}
}

func TestInitializeHPAAnnotations(t *testing.T) {
	minReplicas := int32(2)
	maxReplicas := int32(10)
	hpa := &autoscalingv2.HorizontalPodAutoscaler{
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			MinReplicas: &minReplicas,
			MaxReplicas: maxReplicas,
		},
	}

	// Test initialization of annotations
	InitializeHPAAnnotations(hpa)

	// Check if all required annotations are set
	if val, exists := hpa.Annotations[OriginalMinReplicasAnnotation]; !exists {
		t.Error("OriginalMinReplicasAnnotation not set")
	} else if val != "2" {
		t.Errorf("OriginalMinReplicasAnnotation = %v, want %v", val, "2")
	}

	if val, exists := hpa.Annotations[OriginalMaxReplicasAnnotation]; !exists {
		t.Error("OriginalMaxReplicasAnnotation not set")
	} else if val != "10" {
		t.Errorf("OriginalMaxReplicasAnnotation = %v, want %v", val, "10")
	}

	if val, exists := hpa.Annotations[HPAManagedAnnotation]; !exists {
		t.Error("HPAManagedAnnotation not set")
	} else if val != "true" {
		t.Errorf("HPAManagedAnnotation = %v, want %v", val, "true")
	}

	if _, exists := hpa.Annotations[LastHPAUpdateAnnotation]; !exists {
		t.Error("LastHPAUpdateAnnotation not set")
	}

	// Test that original values are not overwritten
	minReplicas = 5
	maxReplicas = 15
	InitializeHPAAnnotations(hpa)
	if val := hpa.Annotations[OriginalMinReplicasAnnotation]; val != "2" {
		t.Errorf("OriginalMinReplicasAnnotation was overwritten, got %v, want %v", val, "2")
	}
	if val := hpa.Annotations[OriginalMaxReplicasAnnotation]; val != "10" {
		t.Errorf("OriginalMaxReplicasAnnotation was overwritten, got %v, want %v", val, "10")
	}
}

func TestGetOriginalReplicas(t *testing.T) {
	replicas := int32(3)
	deployment := &appsv1.Deployment{
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
		},
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				OriginalReplicasAnnotation: "5",
			},
		},
	}

	// Test getting original replicas from annotation
	got := GetOriginalReplicas(deployment)
	if got != 5 {
		t.Errorf("GetOriginalReplicas() = %v, want %v", got, 5)
	}

	// Test fallback to current replicas when annotation is missing
	deployment.Annotations = nil
	got = GetOriginalReplicas(deployment)
	if got != 3 {
		t.Errorf("GetOriginalReplicas() = %v, want %v", got, 3)
	}
}

func TestGetOriginalHPALimits(t *testing.T) {
	minReplicas := int32(2)
	maxReplicas := int32(10)
	hpa := &autoscalingv2.HorizontalPodAutoscaler{
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			MinReplicas: &minReplicas,
			MaxReplicas: maxReplicas,
		},
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				OriginalMinReplicasAnnotation: "3",
				OriginalMaxReplicasAnnotation: "15",
			},
		},
	}

	// Test getting original limits from annotations
	gotMin, gotMax := GetOriginalHPALimits(hpa)
	if gotMin != 3 || gotMax != 15 {
		t.Errorf("GetOriginalHPALimits() = (%v, %v), want (3, 15)", gotMin, gotMax)
	}

	// Test fallback to current values when annotations are missing
	hpa.Annotations = nil
	gotMin, gotMax = GetOriginalHPALimits(hpa)
	if gotMin != 2 || gotMax != 10 {
		t.Errorf("GetOriginalHPALimits() = (%v, %v), want (2, 10)", gotMin, gotMax)
	}

	// Test default min replicas when spec.MinReplicas is nil
	hpa.Spec.MinReplicas = nil
	gotMin, gotMax = GetOriginalHPALimits(hpa)
	if gotMin != 1 || gotMax != 10 {
		t.Errorf("GetOriginalHPALimits() = (%v, %v), want (1, 10)", gotMin, gotMax)
	}
}
