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
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	dynamicscalingv1 "github.com/KubeDynamicScaler/kubedynamicscaler/api/v1"
	"github.com/KubeDynamicScaler/kubedynamicscaler/pkg/utils"
)

var _ = Describe("ReplicasOverride Controller", func() {
	const (
		timeout  = time.Second * 30
		interval = time.Millisecond * 500
	)

	Context("When scaling a deployment", func() {
		var (
			deployment *appsv1.Deployment
			override   *dynamicscalingv1.ReplicasOverride
		)

		BeforeEach(func() {
			// Clean up any existing ConfigMap first
			existingConfigMap := &corev1.ConfigMap{}
			err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      "replicas-controller-config",
				Namespace: "kubedynamicscaler-system",
			}, existingConfigMap)
			if err == nil {
				Expect(k8sClient.Delete(ctx, existingConfigMap)).Should(Succeed())
			}

			// Create global config ConfigMap
			globalConfig := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "replicas-controller-config",
					Namespace: "kubedynamicscaler-system",
				},
				Data: map[string]string{
					"config.yaml": `
globalPercentage: 200
minReplicas: 1
maxReplicas: 10
`,
				},
			}
			Expect(k8sClient.Create(ctx, globalConfig)).Should(Succeed())

			// Wait for ConfigMap to be created
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      globalConfig.Name,
					Namespace: globalConfig.Namespace,
				}, globalConfig)
			}, timeout, interval).Should(Succeed())

			// Create a test deployment
			deployment = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment",
					Namespace: "default",
					Labels: map[string]string{
						"app": "test",
					},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: int32Ptr(2),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "test",
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app": "test",
							},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "nginx",
									Image: "nginx:latest",
								},
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, deployment)).Should(Succeed())

			// Wait for deployment to be created
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      deployment.Name,
					Namespace: deployment.Namespace,
				}, deployment)
			}, timeout, interval).Should(Succeed())

			// Create a ReplicasOverride
			override = &dynamicscalingv1.ReplicasOverride{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-override",
					Namespace: "default",
				},
				Spec: dynamicscalingv1.ReplicasOverrideSpec{
					DeploymentRef: &dynamicscalingv1.DeploymentReference{
						Name:      "test-deployment",
						Namespace: "default",
					},
					OverrideType:       "override",
					ReplicasPercentage: 150,
				},
			}
			Expect(k8sClient.Create(ctx, override)).Should(Succeed())

			// Wait for override to be created
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      override.Name,
					Namespace: override.Namespace,
				}, override)
			}, timeout, interval).Should(Succeed())
		})

		AfterEach(func() {
			// Clean up resources
			Expect(k8sClient.Delete(ctx, deployment)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, override)).Should(Succeed())

			// Clean up ConfigMap
			configMap := &corev1.ConfigMap{}
			err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      "replicas-controller-config",
				Namespace: "kubedynamicscaler-system",
			}, configMap)
			if err == nil {
				Expect(k8sClient.Delete(ctx, configMap)).Should(Succeed())
			}
		})

		It("Should scale deployment to 150% when using a ReplicasOverride with 150% percentage", func() {
			// Wait for the deployment to be scaled
			deploymentLookupKey := types.NamespacedName{Name: "test-deployment", Namespace: "default"}
			scaledDeployment := &appsv1.Deployment{}

			Eventually(func() int32 {
				err := k8sClient.Get(ctx, deploymentLookupKey, scaledDeployment)
				if err != nil {
					fmt.Printf("Error getting deployment: %v\n", err)
					return 0
				}
				fmt.Printf("Current replicas: %d\n", *scaledDeployment.Spec.Replicas)
				return *scaledDeployment.Spec.Replicas
			}, timeout, interval).Should(Equal(int32(3)), "Deployment should have 3 replicas (150% of original 2)")

			// Verify annotations
			Expect(scaledDeployment.Annotations).Should(HaveKey(utils.OriginalReplicasAnnotation))
			Expect(scaledDeployment.Annotations[utils.OriginalReplicasAnnotation]).Should(Equal("2"))
			Expect(scaledDeployment.Annotations).Should(HaveKey(utils.ManagedAnnotation))
			Expect(scaledDeployment.Annotations[utils.ManagedAnnotation]).Should(Equal("true"))
			Expect(scaledDeployment.Annotations).Should(HaveKey(utils.LastUpdateAnnotation))

			// Verify override status
			overrideLookupKey := types.NamespacedName{Name: "test-override", Namespace: "default"}
			updatedOverride := &dynamicscalingv1.ReplicasOverride{}

			Eventually(func() int {
				err := k8sClient.Get(ctx, overrideLookupKey, updatedOverride)
				if err != nil {
					fmt.Printf("Error getting override: %v\n", err)
					return 0
				}
				fmt.Printf("Affected deployments: %d\n", len(updatedOverride.Status.AffectedDeployments))
				if len(updatedOverride.Status.AffectedDeployments) > 0 {
					fmt.Printf("First deployment replicas: %d\n", updatedOverride.Status.AffectedDeployments[0].CurrentReplicas)
				}
				return len(updatedOverride.Status.AffectedDeployments)
			}, timeout, interval).Should(Equal(1), "Should have one affected deployment")

			Expect(updatedOverride.Status.AffectedDeployments[0].Name).Should(Equal("test-deployment"))
			Expect(updatedOverride.Status.AffectedDeployments[0].CurrentReplicas).Should(Equal(int32(3)))
		})

		It("Should update HPA limits to 150% when using a ReplicasOverride with 150% percentage", func() {
			// Create an HPA
			hpa := &autoscalingv2.HorizontalPodAutoscaler{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "custom-hpa-name",
					Namespace: "default",
				},
				Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
					ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
						Kind:       "Deployment",
						Name:       "test-deployment",
						APIVersion: "apps/v1",
					},
					MinReplicas: ptr(int32(2)),
					MaxReplicas: 10,
					Metrics: []autoscalingv2.MetricSpec{
						{
							Type: autoscalingv2.ResourceMetricSourceType,
							Resource: &autoscalingv2.ResourceMetricSource{
								Name: corev1.ResourceCPU,
								Target: autoscalingv2.MetricTarget{
									Type:               autoscalingv2.UtilizationMetricType,
									AverageUtilization: ptr(int32(80)),
								},
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, hpa)).Should(Succeed())

			// Wait for HPA to be created
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      hpa.Name,
					Namespace: hpa.Namespace,
				}, hpa)
			}, timeout, interval).Should(Succeed())

			// Wait for the HPA to be updated
			hpaLookupKey := types.NamespacedName{Name: "custom-hpa-name", Namespace: "default"}
			updatedHPA := &autoscalingv2.HorizontalPodAutoscaler{}

			Eventually(func() int32 {
				err := k8sClient.Get(ctx, hpaLookupKey, updatedHPA)
				if err != nil {
					fmt.Printf("Error getting HPA: %v\n", err)
					return 0
				}
				fmt.Printf("Current HPA min replicas: %d, max replicas: %d\n", *updatedHPA.Spec.MinReplicas, updatedHPA.Spec.MaxReplicas)
				return *updatedHPA.Spec.MinReplicas
			}, timeout, interval).Should(Equal(int32(3)), "HPA min replicas should be 3 (150% of original 2)")

			Expect(updatedHPA.Spec.MaxReplicas).Should(Equal(int32(15)), "HPA max replicas should be 15 (150% of original 10)")

			// Verify HPA annotations
			Expect(updatedHPA.Annotations).Should(HaveKey(utils.OriginalMinReplicasAnnotation))
			Expect(updatedHPA.Annotations[utils.OriginalMinReplicasAnnotation]).Should(Equal("2"))
			Expect(updatedHPA.Annotations).Should(HaveKey(utils.OriginalMaxReplicasAnnotation))
			Expect(updatedHPA.Annotations[utils.OriginalMaxReplicasAnnotation]).Should(Equal("10"))
			Expect(updatedHPA.Annotations).Should(HaveKey(utils.HPAManagedAnnotation))
			Expect(updatedHPA.Annotations[utils.HPAManagedAnnotation]).Should(Equal("true"))
			Expect(updatedHPA.Annotations).Should(HaveKey(utils.LastHPAUpdateAnnotation))

			// Clean up HPA
			Expect(k8sClient.Delete(ctx, hpa)).Should(Succeed())
		})

		It("Should scale deployment to 200% when using global configuration with 200% percentage", func() {
			// Create a new deployment without any matching override
			globalDeployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "global-test-deployment",
					Namespace: "default",
					Labels: map[string]string{
						"app": "global-test",
					},
					Annotations: map[string]string{
						utils.GlobalConfigManagedAnnotation: "true",
					},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: int32Ptr(2),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "global-test",
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app": "global-test",
							},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "nginx",
									Image: "nginx:latest",
								},
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, globalDeployment)).Should(Succeed())

			// Wait for deployment to be created
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      globalDeployment.Name,
					Namespace: globalDeployment.Namespace,
				}, globalDeployment)
			}, timeout, interval).Should(Succeed())

			// Wait for the deployment to be scaled according to global rules
			deploymentLookupKey := types.NamespacedName{Name: "global-test-deployment", Namespace: "default"}
			scaledDeployment := &appsv1.Deployment{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, deploymentLookupKey, scaledDeployment)
				if err != nil {
					fmt.Printf("Error getting deployment: %v\n", err)
					return false
				}

				// Check if annotations are set
				if scaledDeployment.Annotations == nil {
					return false
				}

				// Verify that the deployment is managed by the controller
				if scaledDeployment.Annotations[utils.GlobalConfigManagedAnnotation] != "true" {
					return false
				}

				// Verify original replicas are stored
				if scaledDeployment.Annotations[utils.OriginalReplicasAnnotation] != "2" {
					return false
				}

				// Calculate expected replicas based on global config percentage (200%)
				expectedReplicas := int32(4) // Original replicas * 200%
				fmt.Printf("Current replicas: %d, Expected replicas: %d\n", *scaledDeployment.Spec.Replicas, expectedReplicas)
				return *scaledDeployment.Spec.Replicas == expectedReplicas
			}, timeout, interval).Should(BeTrue(), "Deployment should be scaled according to global rules (200% of original 2 replicas)")

			// Clean up the test deployment
			Expect(k8sClient.Delete(ctx, globalDeployment)).Should(Succeed())
		})

		It("Should update HPA limits to 200% when using global configuration with 200% percentage", func() {
			// Create a deployment first
			globalDeployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "global-test-deployment",
					Namespace: "default",
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: int32Ptr(2),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "global-test",
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app": "global-test",
							},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "nginx",
									Image: "nginx:latest",
								},
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, globalDeployment)).Should(Succeed())

			// Create an HPA without any matching override
			globalHPA := &autoscalingv2.HorizontalPodAutoscaler{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "global-test-hpa",
					Namespace: "default",
				},
				Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
					ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
						Kind:       "Deployment",
						Name:       "global-test-deployment",
						APIVersion: "apps/v1",
					},
					MinReplicas: ptr(int32(2)),
					MaxReplicas: 10,
				},
			}
			Expect(k8sClient.Create(ctx, globalHPA)).Should(Succeed())

			// Wait for HPA to be created
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      globalHPA.Name,
					Namespace: globalHPA.Namespace,
				}, globalHPA)
			}, timeout, interval).Should(Succeed())

			// Wait for the HPA to be updated according to global rules
			hpaLookupKey := types.NamespacedName{Name: "global-test-hpa", Namespace: "default"}
			updatedHPA := &autoscalingv2.HorizontalPodAutoscaler{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, hpaLookupKey, updatedHPA)
				if err != nil {
					fmt.Printf("Error getting HPA: %v\n", err)
					return false
				}

				// Check if annotations are set
				if updatedHPA.Annotations == nil {
					return false
				}

				// Verify that the HPA is managed by the controller
				if updatedHPA.Annotations[utils.GlobalConfigManagedAnnotation] != "true" {
					return false
				}

				// Verify original values are stored
				if updatedHPA.Annotations[utils.OriginalMinReplicasAnnotation] != "2" ||
					updatedHPA.Annotations[utils.OriginalMaxReplicasAnnotation] != "10" {
					return false
				}

				// Verify HPA limits were updated according to global config
				expectedMinReplicas := int32(4)  // Original min replicas * 200%
				expectedMaxReplicas := int32(20) // Original max replicas * 200%
				return *updatedHPA.Spec.MinReplicas == expectedMinReplicas && updatedHPA.Spec.MaxReplicas == expectedMaxReplicas
			}, timeout, interval).Should(BeTrue(), "HPA should be managed by global rules")

			// Clean up the test resources
			Expect(k8sClient.Delete(ctx, globalHPA)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, globalDeployment)).Should(Succeed())
		})
	})
})
