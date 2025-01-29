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
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=infrastructure.cloudprovider.io,resources=instances,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cloudprovider.io,resources=instances/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cloudprovider.io,resources=instances/finalizers,verbs=update

func (r *InstanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Instance 객체 가져오기
	instance := &infrastructurev1alpha1.Instance{}
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		log.Error(err, "unable to fetch Instance")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Pulumi 스택 이름 설정 (Kubernetes 네임스페이스 + 인스턴스 이름)
	stackName := fmt.Sprintf("%s-%s", instance.Namespace, instance.Name)
	projectName := "cloud-provider-operator"

	// Pulumi 암호 설정
	os.Setenv("PULUMI_CONFIG_PASSPHRASE", "your-secure-passphrase")

	// Pulumi 스택을 가져오거나 새로 생성
	stack, err := auto.UpsertStackInlineSource(ctx, stackName, projectName, pulumiProgram(instance),
		auto.SecretsProvider("passphrase")) // passphrase 사용
	if err != nil {
		log.Error(err, "failed to create or select Pulumi stack")
		return ctrl.Result{}, err
	}

	// OpenStack 인증 정보 Pulumi Config에 저장
	stack.SetConfig(ctx, "openstack:authUrl", auto.ConfigValue{Value: "https://172.168.30.10:15000/v3"})
	stack.SetConfig(ctx, "openstack:userName", auto.ConfigValue{Value: "admin"})
	stack.SetConfig(ctx, "openstack:password", auto.ConfigValue{Value: "cloud1234", Secret: true})
	stack.SetConfig(ctx, "openstack:tenantName", auto.ConfigValue{Value: "admin"})
	stack.SetConfig(ctx, "openstack:region", auto.ConfigValue{Value: "RegionOne"})
	stack.SetConfig(ctx, "openstack:insecure", auto.ConfigValue{Value: "true"})

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
