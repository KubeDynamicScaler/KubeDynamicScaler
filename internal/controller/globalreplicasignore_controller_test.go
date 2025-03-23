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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	dynamicscalingv1 "github.com/KubeDynamicScaler/kubedynamicscaler/api/v1"
)

var _ = Describe("GlobalReplicasIgnore Controller", func() {
	const (
		timeout  = time.Second * 10
		interval = time.Millisecond * 250
	)

	Context("When ignoring deployments", func() {
		It("Should ignore deployments based on namespace", func() {
			ctx := context.Background()

			// Create test namespace
			testNamespace := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-namespace",
				},
			}
			Expect(k8sClient.Create(ctx, testNamespace)).Should(Succeed())

			// Create test deployments in different namespaces
			deployment1 := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment-1",
					Namespace: "test-namespace",
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: int32Ptr(2),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "test-deployment-1",
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app": "test-deployment-1",
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
			Expect(k8sClient.Create(ctx, deployment1)).Should(Succeed())

			deployment2 := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment-2",
					Namespace: "default",
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: int32Ptr(2),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "test-deployment-2",
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app": "test-deployment-2",
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
			Expect(k8sClient.Create(ctx, deployment2)).Should(Succeed())

			// Create GlobalReplicasIgnore
			ignore := &dynamicscalingv1.GlobalReplicasIgnore{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ignore-namespace",
					Namespace: "default",
				},
				Spec: dynamicscalingv1.GlobalReplicasIgnoreSpec{
					IgnoreNamespaces: []string{"test-namespace"},
				},
			}
			Expect(k8sClient.Create(ctx, ignore)).Should(Succeed())

			// Wait for the ignore status to be updated
			ignoreLookupKey := types.NamespacedName{Name: "test-ignore-namespace", Namespace: "default"}
			updatedIgnore := &dynamicscalingv1.GlobalReplicasIgnore{}

			Eventually(func() int {
				err := k8sClient.Get(ctx, ignoreLookupKey, updatedIgnore)
				if err != nil {
					return 0
				}
				return len(updatedIgnore.Status.IgnoredDeployments)
			}, timeout, interval).Should(Equal(1))

			Expect(updatedIgnore.Status.IgnoredDeployments[0].Name).Should(Equal("test-deployment-1"))
			Expect(updatedIgnore.Status.IgnoredDeployments[0].Namespace).Should(Equal("test-namespace"))
		})

		It("Should ignore deployments based on labels", func() {
			ctx := context.Background()

			// Create test deployments with different labels
			deployment1 := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment-label-1",
					Namespace: "default",
					Labels: map[string]string{
						"ignore": "true",
					},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: int32Ptr(2),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "test-deployment-label-1",
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app": "test-deployment-label-1",
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
			Expect(k8sClient.Create(ctx, deployment1)).Should(Succeed())

			deployment2 := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment-label-2",
					Namespace: "default",
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: int32Ptr(2),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "test-deployment-label-2",
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app": "test-deployment-label-2",
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
			Expect(k8sClient.Create(ctx, deployment2)).Should(Succeed())

			// Create GlobalReplicasIgnore
			ignore := &dynamicscalingv1.GlobalReplicasIgnore{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ignore-label",
					Namespace: "default",
				},
				Spec: dynamicscalingv1.GlobalReplicasIgnoreSpec{
					IgnoreLabels: map[string]string{
						"ignore": "true",
					},
				},
			}
			Expect(k8sClient.Create(ctx, ignore)).Should(Succeed())

			// Wait for the ignore status to be updated
			ignoreLookupKey := types.NamespacedName{Name: "test-ignore-label", Namespace: "default"}
			updatedIgnore := &dynamicscalingv1.GlobalReplicasIgnore{}

			Eventually(func() int {
				err := k8sClient.Get(ctx, ignoreLookupKey, updatedIgnore)
				if err != nil {
					return 0
				}
				return len(updatedIgnore.Status.IgnoredDeployments)
			}, timeout, interval).Should(Equal(1))

			Expect(updatedIgnore.Status.IgnoredDeployments[0].Name).Should(Equal("test-deployment-label-1"))
			Expect(updatedIgnore.Status.IgnoredDeployments[0].Namespace).Should(Equal("default"))
		})

		It("Should ignore specific resources", func() {
			ctx := context.Background()

			// Create test deployments
			deployment1 := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment-resource-1",
					Namespace: "default",
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: int32Ptr(2),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "test-deployment-resource-1",
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app": "test-deployment-resource-1",
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
			Expect(k8sClient.Create(ctx, deployment1)).Should(Succeed())

			deployment2 := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment-resource-2",
					Namespace: "default",
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: int32Ptr(2),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "test-deployment-resource-2",
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app": "test-deployment-resource-2",
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
			Expect(k8sClient.Create(ctx, deployment2)).Should(Succeed())

			// Create GlobalReplicasIgnore
			ignore := &dynamicscalingv1.GlobalReplicasIgnore{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ignore-resource",
					Namespace: "default",
				},
				Spec: dynamicscalingv1.GlobalReplicasIgnoreSpec{
					IgnoreResources: []dynamicscalingv1.IgnoredResource{
						{
							Kind:      "Deployment",
							Name:      "test-deployment-resource-1",
							Namespace: "default",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, ignore)).Should(Succeed())

			// Wait for the ignore status to be updated
			ignoreLookupKey := types.NamespacedName{Name: "test-ignore-resource", Namespace: "default"}
			updatedIgnore := &dynamicscalingv1.GlobalReplicasIgnore{}

			Eventually(func() int {
				err := k8sClient.Get(ctx, ignoreLookupKey, updatedIgnore)
				if err != nil {
					return 0
				}
				return len(updatedIgnore.Status.IgnoredDeployments)
			}, timeout, interval).Should(Equal(1))

			Expect(updatedIgnore.Status.IgnoredDeployments[0].Name).Should(Equal("test-deployment-resource-1"))
			Expect(updatedIgnore.Status.IgnoredDeployments[0].Namespace).Should(Equal("default"))
		})
	})
})
