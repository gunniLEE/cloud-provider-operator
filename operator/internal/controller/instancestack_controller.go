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

// InstanceStackReconciler reconciles an InstanceStack object
type InstanceStackReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=infrastructure.cloudprovider.io,resources=instancestacks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cloudprovider.io,resources=instancestacks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cloudprovider.io,resources=instancestacks/finalizers,verbs=update

func (r *InstanceStackReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// InstanceStack 객체 가져오기
	instanceStack := &infrastructurev1alpha1.InstanceStack{}
	if err := r.Get(ctx, req.NamespacedName, instanceStack); err != nil {
		log.Error(err, "unable to fetch InstanceStack")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Check if the instanceStack is marked for deletion
	if !instanceStack.ObjectMeta.DeletionTimestamp.IsZero() {
		if containsString(instanceStack.ObjectMeta.Finalizers, "instancestack.finalizers.cloudprovider.io") {
			if err := r.deleteOpenStackResource(ctx, instanceStack); err != nil {
				log.Error(err, "failed to delete OpenStack resource")
				return ctrl.Result{}, err
			}

			instanceStack.ObjectMeta.Finalizers = removeString(instanceStack.ObjectMeta.Finalizers, "instancestack.finalizers.cloudprovider.io")
			if err := r.Update(ctx, instanceStack); err != nil {
				log.Error(err, "failed to remove finalizer from InstanceStack")
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Add finalizer if not present
	if !containsString(instanceStack.ObjectMeta.Finalizers, "instancestack.finalizers.cloudprovider.io") {
		instanceStack.ObjectMeta.Finalizers = append(instanceStack.ObjectMeta.Finalizers, "instancestack.finalizers.cloudprovider.io")
		if err := r.Update(ctx, instanceStack); err != nil {
			log.Error(err, "failed to add finalizer to InstanceStack")
			return ctrl.Result{}, err
		}
	}

	// Pulumi 스택 이름 설정
	stackName := fmt.Sprintf("%s-%s", instanceStack.Namespace, instanceStack.Name)
	projectName := "cloud-provider-operator"

	os.Setenv("PULUMI_CONFIG_PASSPHRASE", "cloud1234")

	stack, err := auto.UpsertStackInlineSource(ctx, stackName, projectName, pulumiProgram(instanceStack),
		auto.SecretsProvider("passphrase"))
	if err != nil {
		log.Error(err, "failed to create or select Pulumi stack")
		return ctrl.Result{}, err
	}

	// OpenStack 인증 정보 설정
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

	upRes, err := stack.Up(ctx)
	if err != nil {
		log.Error(err, "failed to apply Pulumi stack")
		return ctrl.Result{}, err
	}

	ipAddress := upRes.Outputs["instanceIP"].Value.(string)
	log.Info("Successfully created OpenStack instance", "IP", ipAddress)
	return ctrl.Result{}, nil
}

func (r *InstanceStackReconciler) deleteOpenStackResource(ctx context.Context, instanceStack *infrastructurev1alpha1.InstanceStack) error {
	stackName := fmt.Sprintf("%s-%s", instanceStack.Namespace, instanceStack.Name)
	projectName := "cloud-provider-operator"

	stack, err := auto.SelectStackInlineSource(ctx, stackName, projectName, pulumiProgram(instanceStack))
	if err != nil {
		return fmt.Errorf("failed to select Pulumi stack: %w", err)
	}

	_, err = stack.Destroy(ctx)
	if err != nil {
		return fmt.Errorf("failed to destroy Pulumi stack: %w", err)
	}

	err = stack.Workspace().RemoveStack(ctx, stackName)
	if err != nil {
		return fmt.Errorf("failed to remove Pulumi stack: %w", err)
	}

	return nil
}

func pulumiProgram(instanceStack *infrastructurev1alpha1.InstanceStack) pulumi.RunFunc {
	return func(ctx *pulumi.Context) error {
		// OpenStack 인스턴스 생성
		newInstance, err := compute.NewInstance(ctx, instanceStack.Name, &compute.InstanceArgs{
			FlavorName: pulumi.String(instanceStack.Spec.FlavorName),
			ImageName:  pulumi.String(instanceStack.Spec.ImageName),
			Networks: compute.InstanceNetworkArray{
				&compute.InstanceNetworkArgs{
					Uuid: pulumi.String(instanceStack.Spec.NetworkUUID),
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
func (r *InstanceStackReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrastructurev1alpha1.InstanceStack{}).
		Named("instancestack").
		Complete(r)
}

// Helper functions
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
