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

	kubernetesContext := config.New(ctx, "kubernetes").Get("context")

	ctx.Export("serviceName", service.Metadata.Name())
	ctx.Export("portForward", pulumi.String(portForwardCommand(kubernetesContext, appName)))
	return nil
}

// portForwardCommand returns the kubectl command that reaches the Service from
// the local machine, pinned to the configured Kubernetes context.
func portForwardCommand(kubernetesContext, serviceName string) string {
	if kubernetesContext == "" {
		return fmt.Sprintf("kubectl port-forward service/%s 8080:80", serviceName)
	}
	return fmt.Sprintf("kubectl --context %s port-forward service/%s 8080:80", kubernetesContext, serviceName)
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
