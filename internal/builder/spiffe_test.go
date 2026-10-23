// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package builder

import (
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

func TestSPIFFE_WorkloadAPI(t *testing.T) {
	g := NewWithT(t)
	instance := spiffeTestInstance(nil, &fluxcdv1.SPIFFE{
		WorkloadAPI: fluxcdv1.WorkloadAPI{
			Endpoint:  "unix:///spiffe-workload-api/agent.sock",
			CSIDriver: "csi.spiffe.io",
		},
	})

	result := buildSPIFFE(t, instance)

	for _, component := range instance.GetComponents() {
		deploy := findObject(result.Objects, deploymentKind, component)
		g.Expect(deploy).NotTo(BeNil(), component)
		g.Expect(containerEnv(deploy)).To(ContainElement(envVar{
			Name:  "SPIFFE_ENDPOINT_SOCKET",
			Value: "unix:///spiffe-workload-api/agent.sock",
		}), component)
		g.Expect(deployVolumes(deploy)).To(ContainElement(spiffeVolume()), component)
		g.Expect(containerVolumeMounts(deploy)).To(ContainElement(spiffeVolumeMount()), component)
		g.Expect(containerArgs(deploy)).NotTo(ContainElement(ContainSubstring("--authorized-")))
	}
}

func TestSPIFFE_WorkloadAPI_WithoutCSIDriver(t *testing.T) {
	g := NewWithT(t)
	instance := spiffeTestInstance(nil, &fluxcdv1.SPIFFE{
		WorkloadAPI: fluxcdv1.WorkloadAPI{
			Endpoint: "unix:///custom/agent.sock",
		},
	})

	result := buildSPIFFE(t, instance)

	deploy := findObject(result.Objects, deploymentKind, fluxcdv1.FluxSourceController)
	g.Expect(containerEnv(deploy)).To(ContainElement(envVar{
		Name:  "SPIFFE_ENDPOINT_SOCKET",
		Value: "unix:///custom/agent.sock",
	}))
	g.Expect(deployVolumes(deploy)).NotTo(ContainElement(HaveKeyWithValue("name", spiffeVolumeName)))
}

func TestSPIFFE_MTLS(t *testing.T) {
	g := NewWithT(t)
	instance := spiffeTestInstance([]string{
		fluxcdv1.FluxSourceController,
		fluxcdv1.FluxKustomizeController,
		fluxcdv1.FluxHelmController,
		fluxcdv1.FluxNotificationController,
		fluxcdv1.FluxSourceWatcher,
	}, &fluxcdv1.SPIFFE{
		WorkloadAPI: fluxcdv1.WorkloadAPI{Endpoint: "unix:///spiffe-workload-api/agent.sock"},
		MTLS: &fluxcdv1.MTLS{
			Enabled:             true,
			ComponentIDTemplate: "spiffe://example.org/fluxcd/{component}",
			Receivers:           []string{"spiffe://example.org/gateway/receiver"},
			ExtraArtifactClients: []string{
				"spiffe://example.org/fluxcd/standalone-web",
			},
			ExtraArtifactServers: []string{
				"spiffe://example.org/fluxcd/external-source",
			},
			ExtraNotifiers: []string{
				"spiffe://example.org/fluxcd/standalone-web",
			},
		},
	})

	result := buildSPIFFE(t, instance)

	sourceArgs := containerArgs(findObject(result.Objects, deploymentKind, fluxcdv1.FluxSourceController))
	g.Expect(sourceArgs).To(ContainElements(
		"--authorized-artifact-client-spiffe-id=spiffe://example.org/fluxcd/kustomize-controller",
		"--authorized-artifact-client-spiffe-id=spiffe://example.org/fluxcd/helm-controller",
		"--authorized-artifact-client-spiffe-id=spiffe://example.org/fluxcd/source-watcher",
		"--authorized-artifact-client-spiffe-id=spiffe://example.org/fluxcd/flux-operator",
		"--authorized-artifact-client-spiffe-id=spiffe://example.org/fluxcd/standalone-web",
		"--authorized-event-server-spiffe-id=spiffe://example.org/fluxcd/notification-controller",
	))

	watcherArgs := containerArgs(findObject(result.Objects, deploymentKind, fluxcdv1.FluxSourceWatcher))
	g.Expect(watcherArgs).To(ContainElements(
		"--authorized-artifact-client-spiffe-id=spiffe://example.org/fluxcd/kustomize-controller",
		"--authorized-artifact-client-spiffe-id=spiffe://example.org/fluxcd/source-watcher",
		"--authorized-artifact-client-spiffe-id=spiffe://example.org/fluxcd/flux-operator",
		"--authorized-artifact-server-spiffe-id=spiffe://example.org/fluxcd/source-controller",
		"--authorized-artifact-server-spiffe-id=spiffe://example.org/fluxcd/source-watcher",
		"--authorized-artifact-server-spiffe-id=spiffe://example.org/fluxcd/external-source",
		"--authorized-event-server-spiffe-id=spiffe://example.org/fluxcd/notification-controller",
	))

	for _, component := range []string{fluxcdv1.FluxKustomizeController, fluxcdv1.FluxHelmController} {
		args := containerArgs(findObject(result.Objects, deploymentKind, component))
		g.Expect(args).To(ContainElements(
			"--authorized-artifact-server-spiffe-id=spiffe://example.org/fluxcd/source-controller",
			"--authorized-artifact-server-spiffe-id=spiffe://example.org/fluxcd/source-watcher",
			"--authorized-artifact-server-spiffe-id=spiffe://example.org/fluxcd/external-source",
			"--authorized-event-server-spiffe-id=spiffe://example.org/fluxcd/notification-controller",
		), component)
	}

	notificationArgs := containerArgs(findObject(result.Objects, deploymentKind, fluxcdv1.FluxNotificationController))
	g.Expect(notificationArgs).To(ContainElements(
		"--authorized-notifier-spiffe-id=spiffe://example.org/fluxcd/source-controller",
		"--authorized-notifier-spiffe-id=spiffe://example.org/fluxcd/kustomize-controller",
		"--authorized-notifier-spiffe-id=spiffe://example.org/fluxcd/helm-controller",
		"--authorized-notifier-spiffe-id=spiffe://example.org/fluxcd/notification-controller",
		"--authorized-notifier-spiffe-id=spiffe://example.org/fluxcd/source-watcher",
		"--authorized-notifier-spiffe-id=spiffe://example.org/fluxcd/flux-operator",
		"--authorized-notifier-spiffe-id=spiffe://example.org/fluxcd/standalone-web",
		"--authorized-receiver-spiffe-id=spiffe://example.org/gateway/receiver",
	))

	for _, name := range []string{
		fluxcdv1.FluxSourceController,
		fluxcdv1.FluxSourceWatcher,
		fluxcdv1.FluxNotificationController,
	} {
		assertServicePort(t, result, name, "https", 443, "http")
	}
	assertServicePort(t, result, "webhook-receiver", "https", 443, "http-webhook")

	for _, name := range []string{fluxcdv1.FluxSourceController, fluxcdv1.FluxSourceWatcher} {
		deploy := findObject(result.Objects, deploymentKind, name)
		assertReadinessProbe(t, deploy)
	}
}

func TestSPIFFE_MTLS_WithoutOptionalControllers(t *testing.T) {
	g := NewWithT(t)
	instance := spiffeTestInstance([]string{
		fluxcdv1.FluxSourceController,
		fluxcdv1.FluxKustomizeController,
		fluxcdv1.FluxHelmController,
	}, &fluxcdv1.SPIFFE{
		WorkloadAPI: fluxcdv1.WorkloadAPI{Endpoint: "unix:///spiffe-workload-api/agent.sock"},
		MTLS: &fluxcdv1.MTLS{
			Enabled:             true,
			ComponentIDTemplate: "spiffe://example.org/fluxcd/{component}",
		},
	})

	result := buildSPIFFE(t, instance)

	sourceArgs := containerArgs(findObject(result.Objects, deploymentKind, fluxcdv1.FluxSourceController))
	g.Expect(sourceArgs).To(ContainElement("--authorized-artifact-client-spiffe-id=spiffe://example.org/fluxcd/flux-operator"))
	g.Expect(sourceArgs).To(ContainElement("--authorized-artifact-client-spiffe-id=spiffe://example.org/fluxcd/kustomize-controller"))
	g.Expect(sourceArgs).To(ContainElement("--authorized-artifact-client-spiffe-id=spiffe://example.org/fluxcd/helm-controller"))
	g.Expect(sourceArgs).NotTo(ContainElement(ContainSubstring("source-watcher")))
	g.Expect(sourceArgs).NotTo(ContainElement(ContainSubstring("--authorized-event-server-spiffe-id")))
	g.Expect(findObject(result.Objects, deploymentKind, fluxcdv1.FluxSourceWatcher)).To(BeNil())
	g.Expect(findObject(result.Objects, deploymentKind, fluxcdv1.FluxNotificationController)).To(BeNil())

	kustomizeArgs := containerArgs(findObject(result.Objects, deploymentKind, fluxcdv1.FluxKustomizeController))
	g.Expect(kustomizeArgs).To(ContainElement("--authorized-artifact-server-spiffe-id=spiffe://example.org/fluxcd/source-controller"))
	g.Expect(kustomizeArgs).NotTo(ContainElement(ContainSubstring("source-watcher")))

	service := findObject(result.Objects, "Service", "webhook-receiver")
	g.Expect(service).To(BeNil())
}

func TestSPIFFE_BrokerAPI(t *testing.T) {
	g := NewWithT(t)
	instance := spiffeTestInstance(nil, &fluxcdv1.SPIFFE{
		WorkloadAPI: fluxcdv1.WorkloadAPI{Endpoint: "unix:///spiffe-workload-api/agent.sock"},
		BrokerAPI: &fluxcdv1.BrokerAPI{
			Endpoint: "dns:///spiffe-broker.example.com:443",
			IDs:      []string{"spiffe://example.org/broker/"},
		},
	})

	result := buildSPIFFE(t, instance)

	for _, component := range instance.GetComponents() {
		args := containerArgs(findObject(result.Objects, deploymentKind, component))
		g.Expect(args).To(ContainElements(
			"--spiffe-broker-endpoint=dns:///spiffe-broker.example.com:443",
			"--spiffe-broker-id=spiffe://example.org/broker/",
		), component)
	}

	instance = spiffeTestInstance([]string{
		fluxcdv1.FluxSourceController,
		fluxcdv1.FluxSourceWatcher,
		fluxcdv1.FluxKustomizeController,
	}, &fluxcdv1.SPIFFE{
		WorkloadAPI: fluxcdv1.WorkloadAPI{Endpoint: "unix:///spiffe-workload-api/agent.sock"},
		BrokerAPI: &fluxcdv1.BrokerAPI{
			Endpoint: "dns:///spiffe-broker.example.com:443",
			IDs:      []string{"spiffe://example.org/broker/"},
		},
	})

	result = buildSPIFFE(t, instance)
	g.Expect(containerArgs(findObject(result.Objects, deploymentKind, fluxcdv1.FluxSourceWatcher))).
		NotTo(ContainElement(ContainSubstring("--spiffe-broker-endpoint")))
}

func TestSPIFFE_VersionGate(t *testing.T) {
	g := NewWithT(t)
	instance := spiffeTestInstance(nil, &fluxcdv1.SPIFFE{
		WorkloadAPI: fluxcdv1.WorkloadAPI{Endpoint: "unix:///spiffe-workload-api/agent.sock"},
	})

	options := MakeDefaultOptions()
	options.Version = "v2.9.6"
	options.Components = instance.GetComponents()

	err := options.ValidateAndApplySPIFFEConfig(instance)
	g.Expect(err).To(MatchError(ContainSubstring(".spec.spiffe requires Flux >= 2.10.0")))
}

func TestSPIFFE_Sharding(t *testing.T) {
	g := NewWithT(t)
	instance := spiffeTestInstance([]string{
		fluxcdv1.FluxSourceController,
		fluxcdv1.FluxKustomizeController,
		fluxcdv1.FluxNotificationController,
	}, &fluxcdv1.SPIFFE{
		WorkloadAPI: fluxcdv1.WorkloadAPI{Endpoint: "unix:///spiffe-workload-api/agent.sock"},
		MTLS: &fluxcdv1.MTLS{
			Enabled:             true,
			ComponentIDTemplate: "spiffe://example.org/fluxcd/{component}",
		},
	})
	instance.Spec.Sharding = &fluxcdv1.Sharding{
		Shards: []string{"apps"},
	}

	result := buildSPIFFE(t, instance)

	shardSource := findObject(result.Objects, deploymentKind, "source-controller-apps")
	g.Expect(shardSource).NotTo(BeNil())
	g.Expect(containerArgs(shardSource)).To(ContainElement(
		"--authorized-artifact-client-spiffe-id=spiffe://example.org/fluxcd/flux-operator"))
	g.Expect(containerEnv(shardSource)).To(ContainElement(envVar{
		Name:  "SPIFFE_ENDPOINT_SOCKET",
		Value: "unix:///spiffe-workload-api/agent.sock",
	}))
	assertServicePort(t, result, "source-controller-apps", "https", 443, "http")
}

func TestSPIFFE_Idempotent(t *testing.T) {
	g := NewWithT(t)
	instance := spiffeTestInstance([]string{
		fluxcdv1.FluxSourceController,
		fluxcdv1.FluxKustomizeController,
		fluxcdv1.FluxHelmController,
		fluxcdv1.FluxNotificationController,
		fluxcdv1.FluxSourceWatcher,
	}, &fluxcdv1.SPIFFE{
		WorkloadAPI: fluxcdv1.WorkloadAPI{Endpoint: "unix:///spiffe-workload-api/agent.sock"},
		MTLS: &fluxcdv1.MTLS{
			Enabled:             true,
			ComponentIDTemplate: "spiffe://example.org/fluxcd/{component}",
		},
	})

	result := buildSPIFFE(t, instance)

	for _, name := range []string{fluxcdv1.FluxSourceController, fluxcdv1.FluxSourceWatcher} {
		assertReadinessProbe(t, findObject(result.Objects, deploymentKind, name))
	}

	for _, name := range []string{fluxcdv1.FluxSourceController, fluxcdv1.FluxSourceWatcher, fluxcdv1.FluxNotificationController} {
		service := findObject(result.Objects, "Service", name)
		ports, found, err := unstructured.NestedSlice(service.Object, "spec", "ports")
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(found).To(BeTrue())

		var httpsCount int
		for _, port := range ports {
			portMap := port.(map[string]any)
			if portMap["name"] != "https" {
				continue
			}
			httpsCount++
			g.Expect(portMap["port"]).To(Equal(int64(443)))
			g.Expect(portMap["protocol"]).To(Equal("TCP"))
			g.Expect(portMap["targetPort"]).To(Equal("http"))
		}
		g.Expect(httpsCount).To(Equal(1), name)
	}
}

type envVar struct {
	Name  string
	Value string
}

func spiffeTestInstance(components []string, spiffe *fluxcdv1.SPIFFE) *fluxcdv1.FluxInstance {
	var comps []fluxcdv1.Component
	if components != nil {
		comps = make([]fluxcdv1.Component, len(components))
		for i, component := range components {
			comps[i] = fluxcdv1.Component(component)
		}
	}

	return &fluxcdv1.FluxInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fluxcdv1.DefaultInstanceName,
			Namespace: "flux-system",
		},
		Spec: fluxcdv1.FluxInstanceSpec{
			Distribution: fluxcdv1.Distribution{
				Version:  "2.10.x",
				Registry: "ghcr.io/fluxcd",
			},
			Components: comps,
			SPIFFE:     spiffe,
		},
	}
}

func buildSPIFFE(t *testing.T, instance *fluxcdv1.FluxInstance) *Result {
	t.Helper()
	g := NewWithT(t)

	options := MakeDefaultOptions()
	options.Version = "v2.10.0"
	options.Components = instance.GetComponents()
	options.Namespace = instance.GetNamespace()
	options.ClusterDomain = instance.GetCluster().Domain
	if instance.Spec.Sharding != nil {
		options.ShardingKey = instance.Spec.Sharding.Key
		options.Shards = instance.Spec.Sharding.Shards
	}

	if err := options.ValidateAndApplySPIFFEConfig(instance); err != nil {
		t.Fatalf("failed to apply SPIFFE config: %v", err)
	}

	srcDir := filepath.Join("testdata", options.Version)
	dstDir, err := testTempDir(t)
	g.Expect(err).NotTo(HaveOccurred())

	ci, err := ExtractComponentImages(srcDir, options)
	g.Expect(err).NotTo(HaveOccurred())
	options.ComponentImages = ci

	result, err := Build(srcDir, dstDir, options)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result.Objects).NotTo(BeEmpty())
	return result
}

func containerEnv(obj *unstructured.Unstructured) []envVar {
	containers, _, _ := unstructured.NestedSlice(obj.Object, "spec", "template", "spec", "containers")
	if len(containers) == 0 {
		return nil
	}
	container, ok := containers[0].(map[string]any)
	if !ok {
		return nil
	}
	rawEnv, ok := container["env"].([]any)
	if !ok {
		return nil
	}
	env := make([]envVar, 0, len(rawEnv))
	for _, ev := range rawEnv {
		m, ok := ev.(map[string]any)
		if !ok {
			continue
		}
		name, _ := m["name"].(string)
		value, _ := m["value"].(string)
		env = append(env, envVar{Name: name, Value: value})
	}
	return env
}

func deployVolumes(obj *unstructured.Unstructured) []map[string]any {
	raw, _, _ := unstructured.NestedSlice(obj.Object, "spec", "template", "spec", "volumes")
	volumes := make([]map[string]any, 0, len(raw))
	for _, v := range raw {
		if m, ok := v.(map[string]any); ok {
			volumes = append(volumes, m)
		}
	}
	return volumes
}

func spiffeVolume() map[string]any {
	return map[string]any{
		"name": spiffeVolumeName,
		"csi": map[string]any{
			"driver":   "csi.spiffe.io",
			"readOnly": true,
		},
	}
}

func containerVolumeMounts(obj *unstructured.Unstructured) []map[string]any {
	containers, _, _ := unstructured.NestedSlice(obj.Object, "spec", "template", "spec", "containers")
	if len(containers) == 0 {
		return nil
	}
	container, ok := containers[0].(map[string]any)
	if !ok {
		return nil
	}
	raw, ok := container["volumeMounts"].([]any)
	if !ok {
		return nil
	}
	mounts := make([]map[string]any, 0, len(raw))
	for _, v := range raw {
		if m, ok := v.(map[string]any); ok {
			mounts = append(mounts, m)
		}
	}
	return mounts
}

func spiffeVolumeMount() map[string]any {
	return map[string]any{
		"name":      spiffeVolumeName,
		"mountPath": spiffeVolumeMountPath,
		"readOnly":  true,
	}
}

func assertReadinessProbe(t *testing.T, deploy *unstructured.Unstructured) {
	t.Helper()
	g := NewWithT(t)
	g.Expect(deploy).NotTo(BeNil())

	containers, found, err := unstructured.NestedSlice(deploy.Object, "spec", "template", "spec", "containers")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(found).To(BeTrue())
	g.Expect(containers).NotTo(BeEmpty())
	container, ok := containers[0].(map[string]any)
	g.Expect(ok).To(BeTrue())

	probe, ok := container["readinessProbe"].(map[string]any)
	g.Expect(ok).To(BeTrue())
	httpGet, ok := probe["httpGet"].(map[string]any)
	g.Expect(ok).To(BeTrue())
	g.Expect(httpGet["path"]).To(Equal("/readyz"))
	g.Expect(httpGet["port"]).To(Equal("healthz"))
}

func assertServicePort(t *testing.T, result *Result, serviceName, portName string, port int64, targetPort string) {
	t.Helper()
	g := NewWithT(t)
	service := findObject(result.Objects, "Service", serviceName)
	g.Expect(service).NotTo(BeNil(), serviceName)
	ports, found, err := unstructured.NestedSlice(service.Object, "spec", "ports")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(found).To(BeTrue())
	for _, p := range ports {
		portMap := p.(map[string]any)
		if portMap["name"] != portName {
			continue
		}
		g.Expect(portMap["port"]).To(Equal(port))
		g.Expect(portMap["protocol"]).To(Equal("TCP"))
		g.Expect(portMap["targetPort"]).To(Equal(targetPort))
		return
	}
	t.Fatalf("port %s not found in service %s", portName, serviceName)
}
