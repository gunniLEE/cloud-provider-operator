package main

import (
	"github.com/pulumi/pulumi-openstack/sdk/v4/go/openstack/compute"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// Create an OpenStack resource (Compute Instance)
		instance, err := compute.NewInstance(ctx, "pulumi-instance", &compute.InstanceArgs{
			FlavorName: pulumi.String("4C8G"),
			ImageName:  pulumi.String("ubuntu-22.04-qemu.qcow2"),
			Networks: compute.InstanceNetworkArray{
				&compute.InstanceNetworkArgs{
					// 네트워크 ID를 명시적으로 지정
					Uuid: pulumi.String("0a7e0885-9deb-45c6-bfeb-d28821d8d3d3"),
				},
			},
		})
		if err != nil {
			return err
		}

		// Export the IP of the instance
		ctx.Export("instanceIP", instance.AccessIpV4)
		return nil
	})
}
