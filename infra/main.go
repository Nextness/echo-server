package main

import (
	"errors"
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "")

		image := cfg.Get("image")
		if image == "" {
			image = "echo-server:local"
		}

		replicas, err := cfg.TryInt("replicas")
		switch {
		case errors.Is(err, config.ErrMissingVar):
			replaces = 1
		case err != nil:
			return fmt.Errorf("echo-infra:repliaces must be an integer: %w", err)
		case replicas < 1:
			return fmt.Errorf("echo-infra:repliacas must be at least 1, but got %d", replicas)
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
