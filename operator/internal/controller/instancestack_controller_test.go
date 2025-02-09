package controller

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	infrastructurev1alpha1 "github.com/gunniLee/cloud-provider-operator/api/v1alpha1"
)

var _ = Describe("InstanceStack Controller", func() {
	const (
		InstanceStackName      = "test-instancestack"
		InstanceStackNamespace = "default"
		timeout               = time.Second * 10
		interval             = time.Millisecond * 250
	)

	Context("When creating InstanceStack", func() {
		It("Should create successfully", func() {
			By("Creating a new InstanceStack")
			ctx := context.Background()
			instanceStack := &infrastructurev1alpha1.InstanceStack{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "infrastructure.cloudprovider.io/v1alpha1",
					Kind:       "InstanceStack",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      InstanceStackName,
					Namespace: InstanceStackNamespace,
				},
				Spec: infrastructurev1alpha1.InstanceStackSpec{
					FlavorName:  "test-flavor",
					ImageName:   "test-image",
					NetworkUUID: "test-network-uuid",
				},
			}
			Expect(k8sClient.Create(ctx, instanceStack)).Should(Succeed())

			// 생성된 InstanceStack 확인
			instanceStackLookupKey := types.NamespacedName{
				Name:      InstanceStackName,
				Namespace: InstanceStackNamespace,
			}
			createdInstanceStack := &infrastructurev1alpha1.InstanceStack{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, instanceStackLookupKey, createdInstanceStack)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			// 생성된 InstanceStack의 Spec 확인
			Expect(createdInstanceStack.Spec.FlavorName).Should(Equal("test-flavor"))
			Expect(createdInstanceStack.Spec.ImageName).Should(Equal("test-image"))
			Expect(createdInstanceStack.Spec.NetworkUUID).Should(Equal("test-network-uuid"))
		})
	})

	Context("When deleting InstanceStack", func() {
		It("Should delete successfully", func() {
			By("Deleting the InstanceStack")
			ctx := context.Background()
			instanceStackLookupKey := types.NamespacedName{
				Name:      InstanceStackName,
				Namespace: InstanceStackNamespace,
			}
			createdInstanceStack := &infrastructurev1alpha1.InstanceStack{}

			// 먼저 InstanceStack을 가져옴
			Expect(k8sClient.Get(ctx, instanceStackLookupKey, createdInstanceStack)).Should(Succeed())

			// InstanceStack 삭제
			Expect(k8sClient.Delete(ctx, createdInstanceStack)).Should(Succeed())

			// InstanceStack이 삭제되었는지 확인
			Eventually(func() bool {
				err := k8sClient.Get(ctx, instanceStackLookupKey, createdInstanceStack)
				return err != nil
			}, timeout, interval).Should(BeTrue())
		})
	})
}) 