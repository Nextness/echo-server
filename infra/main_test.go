package main

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

const (
	deploymentType = "kubernetes:apps/v1:Deployment"
	serviceType    = "kubernetes:core/v1:Service"
)

func TestRunUsesDefaultConfiguration(t *testing.T) {
	mocks := &recordingMocks{}
	err := pulumi.RunErr(
		run,
		pulumi.WithMocks("echo-infra", "test", mocks),
		withConfig(map[string]string{}),
	)
	if err != nil {
		t.Fatalf("Got Pulumi run error = %v, but expected nil", err)
	}

	deployment := mocks.resourceByType(t, deploymentType)
	spec := mapValue(t, deployment.Inputs.Mappable(), "spec")
	expectedReplicas := float64(1)
	if got := spec["replicas"]; got != expectedReplicas {
		t.Errorf("Got replicas = %#v, but expected %#v", got, expectedReplicas)
	}

	container := deploymentContainer(t, spec)
	expectedImage := "echo-server:local"
	if got := container["image"]; got != expectedImage {
		t.Errorf("Got image = %#v, but expected %q", got, expectedImage)
	}
}

func TestRunRegistersDeploymentAndServiceContract(t *testing.T) {
	mocks := &recordingMocks{}
	err := pulumi.RunErr(
		run,
		pulumi.WithMocks("echo-infra", "test", mocks),
		withConfig(map[string]string{
			"echo-infra:image":    "echo-server:test",
			"echo-infra:replicas": "3",
		}),
	)
	if err != nil {
		t.Fatalf("Got Pulumi run error = %v, but expected nil", err)
	}

	deployment := mocks.resourceByType(t, deploymentType)
	expectedLabels := map[string]any{"app.kubernetes.io/name": "echo-server"}
	deploymentMetadata := mapValue(t, deployment.Inputs.Mappable(), "metadata")
	if got := mapValue(t, deploymentMetadata, "labels"); !reflect.DeepEqual(got, expectedLabels) {
		t.Errorf("Got Deployment labels = %#v, but expected %#v", got, expectedLabels)
	}

	deploymentSpec := mapValue(t, deployment.Inputs.Mappable(), "spec")
	expectedReplicas := float64(3)
	if got := deploymentSpec["replicas"]; got != expectedReplicas {
		t.Errorf("Got replicas = %#v, but expected %#v", got, expectedReplicas)
	}

	selector := mapValue(t, deploymentSpec, "selector")
	deploymentLabels := mapValue(t, selector, "matchLabels")
	if !reflect.DeepEqual(deploymentLabels, expectedLabels) {
		t.Errorf("Got Deployment selector matchLabels = %#v, but expected %#v", deploymentLabels, expectedLabels)
	}

	template := mapValue(t, deploymentSpec, "template")
	templateMetadata := mapValue(t, template, "metadata")
	if got := mapValue(t, templateMetadata, "labels"); !reflect.DeepEqual(got, expectedLabels) {
		t.Errorf("Got Pod template labels = %#v, but expected %#v", got, expectedLabels)
	}

	podSpec := mapValue(t, template, "spec")
	expectedAutomountServiceAccountToken := false
	if got := podSpec["automountServiceAccountToken"]; got != expectedAutomountServiceAccountToken {
		t.Errorf("Got automountServiceAccountToken = %#v, but expected %#v", got, expectedAutomountServiceAccountToken)
	}
	expectedTerminationGracePeriodSeconds := float64(15)
	if got := podSpec["terminationGracePeriodSeconds"]; got != expectedTerminationGracePeriodSeconds {
		t.Errorf("Got terminationGracePeriodSeconds = %#v, but expected %#v", got, expectedTerminationGracePeriodSeconds)
	}
	podSecurityContext := mapValue(t, podSpec, "securityContext")
	seccompProfile := mapValue(t, podSecurityContext, "seccompProfile")
	expectedSeccompProfileType := "RuntimeDefault"
	if got := seccompProfile["type"]; got != expectedSeccompProfileType {
		t.Errorf("Got seccompProfile type = %#v, but expected %q", got, expectedSeccompProfileType)
	}

	container := deploymentContainer(t, deploymentSpec)
	expectedImage := "echo-server:test"
	if got := container["image"]; got != expectedImage {
		t.Errorf("Got image = %#v, but expected %q", got, expectedImage)
	}
	expectedImagePullPolicy := "Never"
	if got := container["imagePullPolicy"]; got != expectedImagePullPolicy {
		t.Errorf("Got imagePullPolicy = %#v, but expected %q", got, expectedImagePullPolicy)
	}

	ports := sliceValue(t, container, "ports")
	containerPort := objectValue(t, ports[0], "container port")
	expectedContainerPortName := "http"
	if got := containerPort["name"]; got != expectedContainerPortName {
		t.Errorf("Got container port name = %#v, but expected %q", got, expectedContainerPortName)
	}
	expectedContainerPort := float64(8080)
	if got := containerPort["containerPort"]; got != expectedContainerPort {
		t.Errorf("Got container port = %#v, but expected %#v", got, expectedContainerPort)
	}

	envVar := objectValue(t, sliceValue(t, container, "env")[0], "environment variable")
	expectedEnvName := "PORT"
	if got := envVar["name"]; got != expectedEnvName {
		t.Errorf("Got env name = %#v, but expected %q", got, expectedEnvName)
	}
	expectedEnvValue := "8080"
	if got := envVar["value"]; got != expectedEnvValue {
		t.Errorf("Got env value = %#v, but expected %q", got, expectedEnvValue)
	}

	expectedReadinessPath := "/readyz"
	assertProbe(t, container, "readinessProbe", expectedReadinessPath)
	expectedLivenessPath := "/healthz"
	assertProbe(t, container, "livenessProbe", expectedLivenessPath)

	securityContext := mapValue(t, container, "securityContext")
	securityExpectations := map[string]any{
		"allowPrivilegeEscalation": false,
		"readOnlyRootFilesystem":   true,
		"runAsNonRoot":             true,
		"runAsUser":                float64(65532),
	}
	for key, expected := range securityExpectations {
		if got := securityContext[key]; got != expected {
			t.Errorf("Got securityContext.%s = %#v, but expected %#v", key, got, expected)
		}
	}

	capabilities := mapValue(t, securityContext, "capabilities")
	droppedCapabilities := sliceValue(t, capabilities, "drop")
	expectedDroppedCapability := "ALL"
	if got := droppedCapabilities[0]; got != expectedDroppedCapability {
		t.Errorf("Got dropped capability = %#v, but expected %q", got, expectedDroppedCapability)
	}

	resources := mapValue(t, container, "resources")
	expectedRequests := map[string]any{"cpu": "10m", "memory": "16Mi"}
	if got := mapValue(t, resources, "requests"); !reflect.DeepEqual(got, expectedRequests) {
		t.Errorf("Got resource requests = %#v, but expected %#v", got, expectedRequests)
	}
	expectedLimits := map[string]any{"cpu": "100m", "memory": "64Mi"}
	if got := mapValue(t, resources, "limits"); !reflect.DeepEqual(got, expectedLimits) {
		t.Errorf("Got resource limits = %#v, but expected %#v", got, expectedLimits)
	}

	service := mocks.resourceByType(t, serviceType)
	serviceSpec := mapValue(t, service.Inputs.Mappable(), "spec")
	expectedServiceType := "ClusterIP"
	if got := serviceSpec["type"]; got != expectedServiceType {
		t.Errorf("Got Service type = %#v, but expected %q", got, expectedServiceType)
	}
	serviceLabels := mapValue(t, serviceSpec, "selector")
	expectedServiceLabels := deploymentLabels
	if !reflect.DeepEqual(serviceLabels, expectedServiceLabels) {
		t.Errorf("Got Service selector = %#v, but expected %#v", serviceLabels, expectedServiceLabels)
	}

	servicePorts := sliceValue(t, serviceSpec, "ports")
	servicePort := objectValue(t, servicePorts[0], "service port")
	expectedServicePortName := "http"
	if got := servicePort["name"]; got != expectedServicePortName {
		t.Errorf("Got Service port name = %#v, but expected %q", got, expectedServicePortName)
	}
	expectedServicePort := float64(80)
	if got := servicePort["port"]; got != expectedServicePort {
		t.Errorf("Got Service port = %#v, but expected %#v", got, expectedServicePort)
	}
	expectedTargetPort := "http"
	if got := servicePort["targetPort"]; got != expectedTargetPort {
		t.Errorf("Got Service targetPort = %#v, but expected %q", got, expectedTargetPort)
	}
}

func TestRunRejectsInvalidReplicaConfiguration(t *testing.T) {
	type localStruct struct {
		name          string
		value         string
		expectedError string
	}
	tests := []localStruct{
		{name: "not an integer", value: "not-a-number", expectedError: "must be an integer"},
		{name: "zero", value: "0", expectedError: "must be at least 1"},
		{name: "negative", value: "-1", expectedError: "must be at least 1"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mocks := &recordingMocks{}
			err := pulumi.RunErr(
				run,
				pulumi.WithMocks("echo-infra", "test", mocks),
				withConfig(map[string]string{"echo-infra:replicas": test.value}),
			)
			if err == nil {
				t.Fatalf("Got nil Pulumi run error for replicas %q, but expected an error", test.value)
			}
			if !strings.Contains(err.Error(), test.expectedError) {
				t.Errorf("Got error = %q, but expected it to contain %q", err, test.expectedError)
			}
			expectedResourceCount := 0
			if got := mocks.customResourceCount(); got != expectedResourceCount {
				t.Errorf("Got custom resource count = %d, but expected %d", got, expectedResourceCount)
			}
		})
	}
}

func assertProbe(t *testing.T, container map[string]any, key, expectedPath string) {
	t.Helper()

	probe := mapValue(t, container, key)
	httpGet := mapValue(t, probe, "httpGet")
	if got := httpGet["path"]; got != expectedPath {
		t.Errorf("Got %s path = %#v, but expected %q", key, got, expectedPath)
	}
	expectedPort := "http"
	if got := httpGet["port"]; got != expectedPort {
		t.Errorf("Got %s port = %#v, but expected %q", key, got, expectedPort)
	}
}

func deploymentContainer(t *testing.T, deploymentSpec map[string]any) map[string]any {
	t.Helper()

	template := mapValue(t, deploymentSpec, "template")
	podSpec := mapValue(t, template, "spec")
	containers := sliceValue(t, podSpec, "containers")
	expectedContainerCount := 1
	if len(containers) != expectedContainerCount {
		t.Fatalf("Got container count = %d, but expected %d", len(containers), expectedContainerCount)
	}
	return objectValue(t, containers[0], "container")
}

func mapValue(t *testing.T, values map[string]any, key string) map[string]any {
	t.Helper()
	return objectValue(t, values[key], key)
}

func objectValue(t *testing.T, value any, description string) map[string]any {
	t.Helper()

	result, ok := value.(map[string]any)
	if !ok {
		expectedType := "map[string]any"
		t.Fatalf("Got %s type = %T, but expected %s", description, value, expectedType)
	}
	return result
}

func sliceValue(t *testing.T, values map[string]any, key string) []any {
	t.Helper()

	result, ok := values[key].([]any)
	if !ok {
		expectedType := "[]any"
		t.Fatalf("Got %s type = %T, but expected %s", key, values[key], expectedType)
	}
	expectedMinimumLength := 1
	if len(result) < expectedMinimumLength {
		t.Fatalf("Got %s length = %d, but expected at least %d", key, len(result), expectedMinimumLength)
	}
	return result
}

func withConfig(values map[string]string) pulumi.RunOption {
	return func(info *pulumi.RunInfo) {
		info.Config = values
	}
}

type recordingMocks struct {
	mutex     sync.Mutex
	resources []pulumi.MockResourceArgs
}

func (mocks *recordingMocks) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	mocks.mutex.Lock()
	mocks.resources = append(mocks.resources, args)
	mocks.mutex.Unlock()
	return args.Name + "-id", args.Inputs, nil
}

func (*recordingMocks) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	return args.Args, nil
}

func (mocks *recordingMocks) resourceByType(t *testing.T, resourceType string) pulumi.MockResourceArgs {
	t.Helper()

	mocks.mutex.Lock()
	defer mocks.mutex.Unlock()
	for _, candidate := range mocks.resources {
		if candidate.TypeToken == resourceType {
			return candidate
		}
	}
	t.Fatalf("Got no resource of type %q, but expected one; resources = %s", resourceType, mocks.resourceTypes())
	return pulumi.MockResourceArgs{}
}

func (mocks *recordingMocks) customResourceCount() int {
	mocks.mutex.Lock()
	defer mocks.mutex.Unlock()

	count := 0
	for _, candidate := range mocks.resources {
		if candidate.Custom {
			count++
		}
	}
	return count
}

func (mocks *recordingMocks) resourceTypes() string {
	types := make([]string, 0, len(mocks.resources))
	for _, candidate := range mocks.resources {
		types = append(types, candidate.TypeToken)
	}
	return fmt.Sprintf("%v", types)
}
