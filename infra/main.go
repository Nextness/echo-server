package main

import (
	"errors"
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(run)
}

func run(ctx *pulumi.Context) error {
	args, err := loadAppArgs(ctx)
	if err != nil {
		return err
	}

	service, err := deployEcho(ctx, args)
	if err != nil {
		return err
	}

	ctx.Export("serviceName", service.Metadata.Name())
	ctx.Export("portForward", pulumi.String("kubectl port-forward service/echo-server 8080:80"))
	return nil
}

func loadAppArgs(ctx *pulumi.Context) (AppArgs, error) {
	cfg := config.New(ctx, "")

	image := cfg.Get("image")
	if image == "" {
		image = "echo-server:local"
	}

	replicas, err := cfg.TryInt("replicas")
	switch {
	case errors.Is(err, config.ErrMissingVar):
		replicas = 1
	case err != nil:
		return AppArgs{}, fmt.Errorf("echo-infra:replicas must be an integer: %w", err)
	case replicas < 1:
		return AppArgs{}, fmt.Errorf("echo-infra:replicas must be at least 1, but got %d", replicas)
	}

	return AppArgs{
		Image:    image,
		Replicas: replicas,
	}, nil
}
