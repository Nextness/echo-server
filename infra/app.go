package main

import (
	"strconv"

	appsv1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/apps/v1"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

const (
	// appName is the name shared by the Echo Server Kubernetes resources.
	appName = "echo-server"
	// containerPort is the port the Echo Server container listens on.
	containerPort = 8080
)

// AppArgs configures the Echo Server deployment.
type AppArgs struct {
	Image    string
	Replicas int
}

// deployEcho creates the Echo Server Deployment and ClusterIP Service.
func deployEcho(ctx *pulumi.Context, args AppArgs) (*corev1.Service, error) {
	labels := pulumi.StringMap{
		"app.kubernetes.io/name": pulumi.String(appName),
	}

	deployment, err := appsv1.NewDeployment(ctx, appName, &appsv1.DeploymentArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:   pulumi.String(appName),
			Labels: labels,
		},
		Spec: appsv1.DeploymentSpecArgs{
			Replicas: pulumi.Int(args.Replicas),
			Selector: &metav1.LabelSelectorArgs{MatchLabels: labels},
			Template: corev1.PodTemplateSpecArgs{
				Metadata: &metav1.ObjectMetaArgs{Labels: labels},
				Spec: corev1.PodSpecArgs{
					AutomountServiceAccountToken:  pulumi.Bool(false),
					TerminationGracePeriodSeconds: pulumi.Int(15),
					SecurityContext: &corev1.PodSecurityContextArgs{
						SeccompProfile: &corev1.SeccompProfileArgs{
							Type: pulumi.String("RuntimeDefault"),
						},
					},
					Containers: corev1.ContainerArray{
						corev1.ContainerArgs{
							Name:            pulumi.String("echo-server"),
							Image:           pulumi.String(args.Image),
							ImagePullPolicy: pulumi.String("Never"),
							Ports: corev1.ContainerPortArray{
								corev1.ContainerPortArgs{
									Name:          pulumi.String("http"),
									ContainerPort: pulumi.Int(containerPort),
								},
							},
							Env: corev1.EnvVarArray{
								corev1.EnvVarArgs{
									Name:  pulumi.String("PORT"),
									Value: pulumi.String(strconv.Itoa(containerPort)),
								},
							},
							ReadinessProbe: httpProbe("/readyz", 2, 5),
							LivenessProbe:  httpProbe("/healthz", 5, 10),
							Resources: &corev1.ResourceRequirementsArgs{
								Requests: pulumi.StringMap{
									"cpu":    pulumi.String("10m"),
									"memory": pulumi.String("16Mi"),
								},
								Limits: pulumi.StringMap{
									"cpu":    pulumi.String("100m"),
									"memory": pulumi.String("64Mi"),
								},
							},
							SecurityContext: &corev1.SecurityContextArgs{
								AllowPrivilegeEscalation: pulumi.Bool(false),
								ReadOnlyRootFilesystem:   pulumi.Bool(true),
								RunAsNonRoot:             pulumi.Bool(true),
								RunAsUser:                pulumi.Int(65532),
								Capabilities: &corev1.CapabilitiesArgs{
									Drop: pulumi.StringArray{pulumi.String("ALL")},
								},
							},
						},
					},
				},
			},
		},
	})

	if err != nil {
		return nil, err
	}

	service, err := corev1.NewService(ctx, appName, &corev1.ServiceArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:   pulumi.String(appName),
			Labels: labels,
		},
		Spec: corev1.ServiceSpecArgs{
			Type:     pulumi.String("ClusterIP"),
			Selector: labels,
			Ports: corev1.ServicePortArray{
				corev1.ServicePortArgs{
					Name:       pulumi.String("http"),
					Port:       pulumi.Int(80),
					TargetPort: pulumi.String("http"),
				},
			},
		},
	}, pulumi.DependsOn([]pulumi.Resource{deployment}))
	if err != nil {
		return nil, err
	}

	return service, nil
}

// httpProbe returns an HTTP probe for path with the given initial delay
// and period in seconds.
func httpProbe(path string, initialDelay, period int) *corev1.ProbeArgs {
	return &corev1.ProbeArgs{
		HttpGet: &corev1.HTTPGetActionArgs{
			Path: pulumi.String(path),
			Port: pulumi.String("http"),
		},
		InitialDelaySeconds: pulumi.Int(initialDelay),
		PeriodSeconds:       pulumi.Int(period),
		TimeoutSeconds:      pulumi.Int(2),
		FailureThreshold:    pulumi.Int(3),
	}
}
