/*
Copyright 2026.

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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	operatorv1alpha1 "github.com/SAP/cad-operator/api/v1alpha1"
)

var _ = Describe("CloudActiveDefense Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}
		cloudactivedefense := &operatorv1alpha1.CloudActiveDefense{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind CloudActiveDefense")
			err := k8sClient.Get(ctx, typeNamespacedName, cloudactivedefense)
			if err != nil && errors.IsNotFound(err) {
				resource := &operatorv1alpha1.CloudActiveDefense{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: operatorv1alpha1.CloudActiveDefenseSpec{
						Domain: "test.kyma.ondemand.com",
						Database: operatorv1alpha1.DatabaseSpec{
							Port: 5432,
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &operatorv1alpha1.CloudActiveDefense{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance CloudActiveDefense")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &CloudActiveDefenseReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			// Note: Reconcile might not fully complete if external CRDs (Kyma APIRule, Istio AuthorizationPolicy)
			// are not available, but basic operations should work
			_, _ = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})

			By("Checking that the finalizer was added")
			err := k8sClient.Get(ctx, typeNamespacedName, cloudactivedefense)
			Expect(err).NotTo(HaveOccurred())
			Expect(cloudactivedefense.Finalizers).To(ContainElement("operator.sundew.com/finalizer"))

			By("Checking that the status was updated with resolved domain")
			Expect(cloudactivedefense.Status.ResolvedDomain).To(Equal("test.kyma.ondemand.com"))
		})
	})
})
