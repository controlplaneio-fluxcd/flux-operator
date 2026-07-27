// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package controller

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"testing"

	"github.com/fluxcd/pkg/apis/meta"
	authutils "github.com/fluxcd/pkg/auth/utils"
	"github.com/fluxcd/pkg/runtime/conditions"
	kauth "github.com/google/go-containerregistry/pkg/authn/kubernetes"
	"github.com/google/go-containerregistry/pkg/crane"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/testutils"
)

func TestResourceSetInputProviderReconciler_OCIArtifactTag_LifeCycle(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetInputProviderReconciler(t)
	rsetReconciler := getResourceSetReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	objDef := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
  name: test
  namespace: "%[1]s"
  labels:
    app: test
spec:
  type: OCIArtifactTag
  url: "oci://ghcr.io/stefanprodan/podinfo"
  filter:
    semver: "6.0.x"
    limit: 1
`, ns.Name)

	setDef := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: test
  namespace: "%[1]s"
spec:
  inputsFrom:
    - selector:
        matchLabels:
          app: test
  resources:
    - apiVersion: v1
      kind: ConfigMap
      metadata:
        name: test-<< inputs.id >>
        namespace: << inputs.provider.namespace >>
      data:
        tag: << inputs.tag | quote >>
        digest: << inputs.digest | quote >>
`, ns.Name)

	obj := &fluxcdv1.ResourceSetInputProvider{}
	err = yaml.Unmarshal([]byte(objDef), obj)
	g.Expect(err).ToNot(HaveOccurred())

	// Create the ResourceSetInputProvider.
	err = testEnv.Create(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	// Initialize the ResourceSetInputProvider.
	r, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.Requeue).To(BeTrue())

	// Retrieve the inputs.
	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.Requeue).To(BeFalse())

	// Check if the ResourceSetInputProvider was marked as ready.
	result := &fluxcdv1.ResourceSetInputProvider{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).ToNot(HaveOccurred())

	testutils.LogObjectStatus(t, result)
	g.Expect(conditions.GetReason(result, meta.ReadyCondition)).To(BeIdenticalTo(meta.ReconciliationSucceededReason))
	g.Expect(result.Status.LastExportedRevision).To(BeIdenticalTo("sha256:b7d3334b3411cccf4c9c08b328ec7ae141fcda58e45e1e3d098f59791c033ced"))

	// Create a ResourceSet referencing the ResourceSetInputProvider.
	rset := &fluxcdv1.ResourceSet{}
	err = yaml.Unmarshal([]byte(setDef), rset)
	g.Expect(err).ToNot(HaveOccurred())
	err = testEnv.Create(ctx, rset)
	g.Expect(err).ToNot(HaveOccurred())

	// Initialize the ResourceSet.
	_, err = rsetReconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(rset),
	})
	g.Expect(err).ToNot(HaveOccurred())

	// Reconcile the ResourceSet.
	_, err = rsetReconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(rset),
	})
	g.Expect(err).ToNot(HaveOccurred())

	// Check if the ResourceSet generated the ConfigMap.
	resultCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-48955639",
			Namespace: ns.Name,
		},
	}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(resultCM), resultCM)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(resultCM.Data).To(HaveKeyWithValue("tag", "6.0.4"))
	g.Expect(resultCM.Data).To(HaveKeyWithValue("digest", "sha256:d4ec9861522d4961b2acac5a070ef4f92d732480dff2062c2f3a1dcf9a5d1e91"))
}

func TestResourceSetInputProviderReconciler_buildOCIOptions(t *testing.T) {
	g := NewWithT(t)

	r := getResourceSetInputProviderReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test-build-oci-options")
	g.Expect(err).NotTo(HaveOccurred())

	// Create a ServiceAccount for the test.
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sa",
			Namespace: ns.Name,
		},
		ImagePullSecrets: []corev1.LocalObjectReference{{Name: "pull-secret"}},
	}
	err = testEnv.Create(ctx, sa)
	g.Expect(err).NotTo(HaveOccurred())

	// Create a pull secret for the test.
	anotherAuthSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pull-secret",
			Namespace: ns.Name,
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{
			".dockerconfigjson": []byte(`{
	"auths": {
		"another-example.com": {
			"username": "another-user",
			"password": "another-pass"
		}
	}
}`),
		},
	}
	err = testEnv.Create(ctx, anotherAuthSecret)
	g.Expect(err).NotTo(HaveOccurred())

	for _, tt := range []struct {
		provider string
		cloud    bool
	}{
		{
			provider: fluxcdv1.InputProviderACRArtifactTag,
			cloud:    true,
		},
		{
			provider: fluxcdv1.InputProviderECRArtifactTag,
			cloud:    true,
		},
		{
			provider: fluxcdv1.InputProviderGARArtifactTag,
			cloud:    true,
		},
		{
			provider: fluxcdv1.InputProviderOCIArtifactTag,
		},
	} {
		t.Run(tt.provider, func(t *testing.T) {
			g := NewWithT(t)

			obj := &fluxcdv1.ResourceSetInputProvider{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: ns.Name,
				},
				Spec: fluxcdv1.ResourceSetInputProviderSpec{
					Type:               tt.provider,
					ServiceAccountName: "test-sa",
				},
			}

			const repo = "example.com/stefanprodan/podinfo"

			tlsConfig := &tls.Config{
				ServerName: "server.example.com",
			}

			authSecret := &corev1.Secret{
				Type: corev1.SecretTypeDockerConfigJson,
				Data: map[string][]byte{
					".dockerconfigjson": []byte(`{
	"auths": {
		"example.com": {
			"username": "user",
			"password": "pass"
		}
	}
}`),
				},
			}

			opts, err := r.buildOCIOptions(ctx, obj, repo, tlsConfig, authSecret)
			g.Expect(err).NotTo(HaveOccurred())

			o := crane.GetOptions(opts...)

			// The cloud providers authenticate with the workload identity of
			// the operator, so the pull secrets do not apply to them.
			if tt.cloud {
				pullKeychain, err := kauth.NewFromPullSecrets(ctx,
					[]corev1.Secret{*authSecret, *anotherAuthSecret})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(o.Keychain).NotTo(Equal(pullKeychain))
				return
			}

			// Validate TLS config.
			g.Expect(o.Transport).NotTo(BeNil())
			g.Expect(o.Transport.(*http.Transport)).NotTo(BeNil())
			g.Expect(o.Transport.(*http.Transport).TLSClientConfig.ServerName).To(Equal("server.example.com"))

			// Validate secret data.
			g.Expect(o.Keychain).NotTo(BeNil())
			keychain, err := kauth.NewFromPullSecrets(ctx,
				[]corev1.Secret{*authSecret, *anotherAuthSecret})
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(o.Keychain).To(Equal(keychain))
		})
	}
}

func TestResourceSetInputProviderReconciler_OCIRepositoryPath_RuntimeGuard(t *testing.T) {
	reconciler := getResourceSetInputProviderReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for _, provider := range []string{
		fluxcdv1.InputProviderOCIArtifactTag,
		fluxcdv1.InputProviderACRArtifactTag,
		fluxcdv1.InputProviderECRArtifactTag,
		fluxcdv1.InputProviderGARArtifactTag,
	} {
		t.Run(provider, func(t *testing.T) {
			g := NewWithT(t)

			// Reconcile with a registry host and no repository path should stall.
			obj := &fluxcdv1.ResourceSetInputProvider{
				Spec: fluxcdv1.ResourceSetInputProviderSpec{
					Type: provider,
					URL:  "oci://ghcr.io",
				},
			}

			r, err := reconciler.reconcile(ctx, obj, nil)
			g.Expect(r).To(Equal(reconcile.Result{}))
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(conditions.IsStalled(obj)).To(BeTrue())
			g.Expect(conditions.GetReason(obj, meta.StalledCondition)).To(Equal(fluxcdv1.ReasonInvalidSpec))
			g.Expect(conditions.GetMessage(obj, meta.StalledCondition)).To(
				ContainSubstring("spec.url must include the repository path"))
		})
	}
}

func TestResourceSetInputProviderReconciler_OCICloudRegistryHost(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for _, tt := range []struct {
		provider string
		repo     string
		err      string
	}{
		{
			provider: fluxcdv1.InputProviderACRArtifactTag,
			repo:     "registry.azurecr.io.example.com/podinfo",
			err:      "invalid Azure registry",
		},
		{
			provider: fluxcdv1.InputProviderECRArtifactTag,
			repo:     "012345678901.dkr.ecr.us-east-1.amazonaws.com.example.com/podinfo",
			err:      "invalid AWS registry",
		},
		{
			provider: fluxcdv1.InputProviderGARArtifactTag,
			repo:     "europe-docker.pkg.dev.example.com/project/podinfo",
			err:      "invalid GCP registry",
		},
	} {
		t.Run(tt.provider, func(t *testing.T) {
			g := NewWithT(t)

			// The credentials are resolved lazily, so the registry host is
			// validated when the registry client calls the authenticator.
			authenticator, err := authutils.GetArtifactRegistryCredentials(ctx,
				inputProviderToCloudProvider[tt.provider], tt.repo)
			g.Expect(err).NotTo(HaveOccurred())

			config, err := authenticator.Authorization()
			g.Expect(err).To(HaveOccurred())
			g.Expect(err.Error()).To(ContainSubstring(tt.err))
			g.Expect(config).To(BeNil())
		})
	}
}

func TestResourceSetInputProviderReconciler_InvalidOCIURL(t *testing.T) {
	g := NewWithT(t)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test-invalid-oci-url")
	g.Expect(err).ToNot(HaveOccurred())

	for _, provider := range []string{
		fluxcdv1.InputProviderOCIArtifactTag,
		fluxcdv1.InputProviderACRArtifactTag,
		fluxcdv1.InputProviderECRArtifactTag,
		fluxcdv1.InputProviderGARArtifactTag,
	} {
		for _, tt := range []struct {
			name string
			url  string
			err  string
		}{
			{
				name: "missing scheme",
				url:  "ghcr.io/stefanprodan/podinfo",
				err:  "spec.url must start with 'oci://' when spec.type is an OCI provider",
			},
			{
				name: "missing repository path",
				url:  "oci://ghcr.io",
				err:  "spec.url must include the repository path after the registry host when spec.type is an OCI provider",
			},
		} {
			t.Run(fmt.Sprintf("%s/%s", provider, tt.name), func(t *testing.T) {
				g := NewWithT(t)

				obj := &fluxcdv1.ResourceSetInputProvider{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: ns.Name,
					},
					Spec: fluxcdv1.ResourceSetInputProviderSpec{
						Type: provider,
						URL:  tt.url,
					},
				}

				err = testEnv.Create(ctx, obj)
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.err))
			})
		}
	}
}

func TestResourceSetInputProviderReconciler_OCIArtifactTag_InsecureOption(t *testing.T) {
	g := NewWithT(t)

	r := getResourceSetInputProviderReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test-oci-insecure")
	g.Expect(err).NotTo(HaveOccurred())

	obj := &fluxcdv1.ResourceSetInputProvider{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-insecure",
			Namespace: ns.Name,
		},
		Spec: fluxcdv1.ResourceSetInputProviderSpec{
			Type:     fluxcdv1.InputProviderOCIArtifactTag,
			Insecure: true,
		},
	}

	const repo = "example.com/stefanprodan/podinfo"

	opts, err := r.buildOCIOptions(ctx, obj, repo, nil, nil)
	g.Expect(err).NotTo(HaveOccurred())

	o := crane.GetOptions(opts...)

	// Validate that insecure transport is configured.
	g.Expect(o.Transport).NotTo(BeNil())
	transport, ok := o.Transport.(*http.Transport)
	g.Expect(ok).To(BeTrue())
	g.Expect(transport.TLSClientConfig).NotTo(BeNil())
	g.Expect(transport.TLSClientConfig.InsecureSkipVerify).To(BeTrue())
}

func TestResourceSetInputProviderReconciler_OCIArtifactTag_CertSecretTakesPrecedenceOverInsecure(t *testing.T) {
	g := NewWithT(t)

	r := getResourceSetInputProviderReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test-oci-cert-precedence")
	g.Expect(err).NotTo(HaveOccurred())

	obj := &fluxcdv1.ResourceSetInputProvider{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cert-precedence",
			Namespace: ns.Name,
		},
		Spec: fluxcdv1.ResourceSetInputProviderSpec{
			Type:     fluxcdv1.InputProviderOCIArtifactTag,
			Insecure: true,
		},
	}

	const repo = "example.com/stefanprodan/podinfo"

	// Pass a TLS config alongside insecure — cert config should take precedence,
	// matching source-controller behavior.
	tlsConfig := &tls.Config{
		ServerName: "server.example.com",
	}

	opts, err := r.buildOCIOptions(ctx, obj, repo, tlsConfig, nil)
	g.Expect(err).NotTo(HaveOccurred())

	o := crane.GetOptions(opts...)

	// Validate that the custom TLS config is used, not insecure.
	g.Expect(o.Transport).NotTo(BeNil())
	transport, ok := o.Transport.(*http.Transport)
	g.Expect(ok).To(BeTrue())
	g.Expect(transport.TLSClientConfig).NotTo(BeNil())
	g.Expect(transport.TLSClientConfig.ServerName).To(Equal("server.example.com"))
	g.Expect(transport.TLSClientConfig.InsecureSkipVerify).To(BeFalse())
}

func TestResourceSetInputProviderReconciler_OCIArtifactTag_InsecureOnCloudProvider(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetInputProviderReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	obj := &fluxcdv1.ResourceSetInputProvider{
		Spec: fluxcdv1.ResourceSetInputProviderSpec{
			Type:     fluxcdv1.InputProviderACRArtifactTag,
			URL:      "oci://myregistry.azurecr.io/myrepo",
			Insecure: true,
		},
	}

	r, err := reconciler.reconcile(ctx, obj, nil)
	g.Expect(r).To(Equal(reconcile.Result{}))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(conditions.IsReady(obj)).To(BeFalse())
	g.Expect(conditions.IsStalled(obj)).To(BeTrue())
	g.Expect(conditions.GetReason(obj, meta.ReadyCondition)).To(Equal(fluxcdv1.ReasonInvalidSpec))
	g.Expect(conditions.GetMessage(obj, meta.StalledCondition)).To(ContainSubstring("spec.insecure can only be set when spec.type is 'ExternalService' or 'OCIArtifactTag'"))
}
