package controller

import (
	"context"
	"fmt"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	infrastructurev1alpha1 "github.com/gunniLee/cloud-provider-operator/api/v1alpha1"

	"github.com/pulumi/pulumi-openstack/sdk/v4/go/openstack/compute"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// InstanceReconciler reconciles an Instance object
type InstanceReconciler struct {
	client.Client                 // kubernetes API 서버와 상호작용하기 위한 클라이언트
	Scheme        *runtime.Scheme // 컨트롤러가 사용할 스키마
}

// +kubebuilder:rbac:groups=infrastructure.cloudprovider.io,resources=instances,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cloudprovider.io,resources=instances/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cloudprovider.io,resources=instances/finalizers,verbs=update

func (r *InstanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Reconcile 메서드는 InstanceReconciler 구조체의 메서드로, context.Context와 ctrl.Request를 인자로 받음
	// 이 메서드는 Kubernetes 컨트롤러의 핵심으로, 리소스의 상태를 감시하고 필요한 작업을 수행함
	// 반환값은 ctrl.Result와 error임

	// log 객체를 생성하여 현재의 context에서 로그를 기록할 수 있도록 함. 이는 로그 메시지를 기록하는 데 사용
	log := log.FromContext(ctx)

	// Instance 객체 가져오기
	// Instance 객체를 위한 포인터를 생성. 이 객체는 Kubernetes API 서버에서 가져올 Instance 리소스를 저장할 것임
	instance := &infrastructurev1alpha1.Instance{}

	if err := r.Get(ctx, req.NamespacedName, instance); err != nil { // r.Get 메서드를 사용하여 Instance 객체를 Kubernetes API 서버에서 가져옴
		log.Error(err, "unable to fetch Instance")       // req.NamespacedName은 요청된 리소스의 네임스페이스와 이름을 포함
		return ctrl.Result{}, client.IgnoreNotFound(err) // 만약 객체를 가져오는 데 실패하면 에러를 반환
	}

	// Check if the instance is marked for deletion
	if !instance.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is being deleted
		if containsString(instance.ObjectMeta.Finalizers, "instance.finalizers.cloudprovider.io") {
			// Finalizer is present, delete the OpenStack resource
			if err := r.deleteOpenStackResource(ctx, instance); err != nil {
				log.Error(err, "failed to delete OpenStack resource")
				return ctrl.Result{}, err
			}

			// Remove finalizer
			instance.ObjectMeta.Finalizers = removeString(instance.ObjectMeta.Finalizers, "instance.finalizers.cloudprovider.io")
			if err := r.Update(ctx, instance); err != nil {
				log.Error(err, "failed to remove finalizer from Instance")
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Add finalizer if not present
	if !containsString(instance.ObjectMeta.Finalizers, "instance.finalizers.cloudprovider.io") {
		instance.ObjectMeta.Finalizers = append(instance.ObjectMeta.Finalizers, "instance.finalizers.cloudprovider.io")
		if err := r.Update(ctx, instance); err != nil {
			log.Error(err, "failed to add finalizer to Instance")
			return ctrl.Result{}, err
		}
	}

	// Pulumi 스택 이름 설정 (Kubernetes 네임스페이스 + 인스턴스 이름)
	stackName := fmt.Sprintf("%s-%s", instance.Namespace, instance.Name)
	projectName := "cloud-provider-operator"

	// Pulumi 암호 설정
	os.Setenv("PULUMI_CONFIG_PASSPHRASE", "cloud1234")

	// Pulumi 스택을 가져오거나 새로 생성
	stack, err := auto.UpsertStackInlineSource(ctx, stackName, projectName, pulumiProgram(instance),
		auto.SecretsProvider("passphrase")) // passphrase 사용
	if err != nil {
		log.Error(err, "failed to create or select Pulumi stack")
		return ctrl.Result{}, err
	}

	// OpenStack 인증 정보 Pulumi Config에 저장
	authURL := os.Getenv("OPENSTACK_AUTH_URL")
	userName := os.Getenv("OPENSTACK_USERNAME")
	password := os.Getenv("OPENSTACK_PASSWORD")
	tenantName := os.Getenv("OPENSTACK_TENANT_NAME")
	region := os.Getenv("OPENSTACK_REGION")
	insecure := os.Getenv("OPENSTACK_INSECURE")

	if authURL == "" || userName == "" || password == "" || tenantName == "" || region == "" || insecure == "" {
		log.Error(nil, "Required environment variables are not set")
		return ctrl.Result{}, fmt.Errorf("missing required environment variables")
	}

	stack.SetConfig(ctx, "openstack:authUrl", auto.ConfigValue{Value: authURL})
	stack.SetConfig(ctx, "openstack:userName", auto.ConfigValue{Value: userName})
	stack.SetConfig(ctx, "openstack:password", auto.ConfigValue{Value: password, Secret: true})
	stack.SetConfig(ctx, "openstack:tenantName", auto.ConfigValue{Value: tenantName})
	stack.SetConfig(ctx, "openstack:region", auto.ConfigValue{Value: region})
	stack.SetConfig(ctx, "openstack:insecure", auto.ConfigValue{Value: insecure})

	// Pulumi 스택 실행 (Apply)
	upRes, err := stack.Up(ctx)
	if err != nil {
		log.Error(err, "failed to apply Pulumi stack")
		return ctrl.Result{}, err
	}

	// 생성된 인스턴스의 IP 주소 가져오기
	ipAddress := upRes.Outputs["instanceIP"].Value.(string)
	// Kubernetes CRD Status 업데이트

	log.Info("Successfully created OpenStack instance", "IP", ipAddress)
	return ctrl.Result{}, nil
}

// deleteOpenStackResource deletes the OpenStack resource associated with the instance
func (r *InstanceReconciler) deleteOpenStackResource(ctx context.Context, instance *infrastructurev1alpha1.Instance) error {
	// Pulumi 스택 이름 설정
	stackName := fmt.Sprintf("%s-%s", instance.Namespace, instance.Name)
	projectName := "cloud-provider-operator"

	// Pulumi 스택 가져오기
	stack, err := auto.SelectStackInlineSource(ctx, stackName, projectName, pulumiProgram(instance))
	if err != nil {
		return fmt.Errorf("failed to select Pulumi stack: %w", err)
	}

	// Pulumi 스택 파괴
	_, err = stack.Destroy(ctx)
	if err != nil {
		return fmt.Errorf("failed to destroy Pulumi stack: %w", err)
	}

	// 스택 삭제 (선택 사항)
	err = stack.Workspace().RemoveStack(ctx, stackName)
	if err != nil {
		return fmt.Errorf("failed to remove Pulumi stack: %w", err)
	}

	return nil
}

// pulumiProgram: Pulumi에서 실행할 OpenStack 인스턴스 생성 로직
func pulumiProgram(instance *infrastructurev1alpha1.Instance) pulumi.RunFunc {
	return func(ctx *pulumi.Context) error {
		// OpenStack 인스턴스 생성
		newInstance, err := compute.NewInstance(ctx, instance.Name, &compute.InstanceArgs{
			FlavorName: pulumi.String(instance.Spec.FlavorName),
			ImageName:  pulumi.String(instance.Spec.ImageName),
			Networks: compute.InstanceNetworkArray{
				&compute.InstanceNetworkArgs{
					Uuid: pulumi.String(instance.Spec.NetworkUUID),
				},
			},
		})
		if err != nil {
			return err
		}

		// 생성된 인스턴스의 IP를 Export
		ctx.Export("instanceIP", newInstance.AccessIpV4)
		return nil
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *InstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrastructurev1alpha1.Instance{}).
		Named("instance").
		Complete(r)
}

// Helper functions to manage finalizers
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) []string {
	var result []string
	for _, item := range slice {
		if item != s {
			result = append(result, item)
		}
	}
	return result
}
