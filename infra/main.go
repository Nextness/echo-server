package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

// TODO: Validate that replicas cannot be negative. Either fail clearly or use Pulumi configuration validation before creating resources.

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "")

		image := cfg.Get("image")
		if image == "" {
			image = "echo-server:local"
		}

		replicas := cfg.GetInt("replicas")
		if replicas == 0 {
			replicas = 1
		}

		service, err := deployEcho(ctx, AppArgs{
			Image:    image,
			Replicas: replicas,
		})
		if err != nil {
			return err
		}

		ctx.Export("serviceName", service.Metadata.Name())
		ctx.Export("portForward", pulumi.String("kubectl port-forward service/echo-server 8080:80"))
		return nil
	})
}
