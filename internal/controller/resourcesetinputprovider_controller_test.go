// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package controller

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/auth/serviceaccounttoken"
	"github.com/fluxcd/pkg/runtime/conditions"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apix "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/inputs"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/schedule"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/testutils"
)

func TestResourceSetInputProviderReconciler_Static(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetInputProviderReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	objDef := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
  name: test-static
  namespace: "%[1]s"
spec:
  type: Static
  url: https://gitlab.com/stefanprodan/podinfo
  defaultValues:
    env: "staging"
    foo: "bar"
`, ns.Name)

	obj := &fluxcdv1.ResourceSetInputProvider{}
	err = yaml.Unmarshal([]byte(objDef), obj)
	g.Expect(err).NotTo(HaveOccurred())

	// Create the ResourceSetInputProvider. Should error out due to CEL validation.
	err = testEnv.Create(ctx, obj)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("spec.url must be empty when spec.type is 'Static'"))

	// Invert CEL validation error to type not Static and URL empty.
	obj.Spec.Type = fluxcdv1.InputProviderGitHubBranch
	obj.Spec.URL = ""
	err = testEnv.Create(ctx, obj)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("spec.url must not be empty when spec.type is not 'Static'"))

	// Fix object and create.
	obj.Spec.Type = fluxcdv1.InputProviderStatic
	err = testEnv.Create(ctx, obj)
	g.Expect(err).NotTo(HaveOccurred())
	exportedInput := fmt.Sprintf(`
id: "%[1]s"
env: staging
foo: bar `, inputs.ID(string(obj.UID)))

	// Initialize the ResourceSetInputProvider.
	r, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(r.Requeue).To(BeTrue())

	// Reconcile and verify exported inputs.
	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(r.Requeue).To(BeFalse())
	result := &fluxcdv1.ResourceSetInputProvider{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(conditions.IsReady(result)).To(BeTrue())
	g.Expect(result.Status.ExportedInputs).To(HaveLen(1))
	b, err := yaml.Marshal(result.Status.ExportedInputs[0])
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(string(b)).To(MatchYAML(exportedInput))
	g.Expect(result.Status.LastExportedRevision).To(HavePrefix("sha256:"))
	g.Expect(result.Status.LastExportedRevision).To(HaveLen(71))
	lastExportedRevision := result.Status.LastExportedRevision

	// Reconcile again and verify that revision did not change.
	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(r.Requeue).To(BeFalse())
	result = &fluxcdv1.ResourceSetInputProvider{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(conditions.IsReady(result)).To(BeTrue())
	g.Expect(result.Status.ExportedInputs).To(HaveLen(1))
	b, err = yaml.Marshal(result.Status.ExportedInputs[0])
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(string(b)).To(MatchYAML(exportedInput))
	g.Expect(result.Status.LastExportedRevision).To(Equal(lastExportedRevision))
}

func TestResourceSetInputProviderReconciler_InvalidDefaultValues(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetInputProviderReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	obj := &fluxcdv1.ResourceSetInputProvider{
		Spec: fluxcdv1.ResourceSetInputProviderSpec{
			DefaultValues: fluxcdv1.ResourceSetInput{
				"foo": &apix.JSON{
					Raw: []byte(`{"bar": "baz"`),
				},
			},
		},
	}

	r, err := reconciler.reconcile(ctx, obj, nil)
	g.Expect(r).To(Equal(reconcile.Result{}))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obj.Status.Conditions).To(HaveLen(2))
	g.Expect(conditions.IsReady(obj)).To(BeFalse())
	g.Expect(conditions.IsStalled(obj)).To(BeTrue())
	g.Expect(conditions.GetReason(obj, meta.ReadyCondition)).To(Equal(fluxcdv1.ReasonInvalidDefaultValues))
	g.Expect(conditions.GetReason(obj, meta.StalledCondition)).To(Equal(fluxcdv1.ReasonInvalidDefaultValues))
	g.Expect(conditions.GetMessage(obj, meta.ReadyCondition)).To(ContainSubstring(msgTerminalError))
	g.Expect(conditions.GetMessage(obj, meta.StalledCondition)).To(ContainSubstring(msgTerminalError))
}

func TestResourceSetInputProviderReconciler_reconcile_InvalidSchedule(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetInputProviderReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	obj := &fluxcdv1.ResourceSetInputProvider{
		Spec: fluxcdv1.ResourceSetInputProviderSpec{
			Schedule: []fluxcdv1.Schedule{{
				Cron: "lalksadlsakd",
			}},
		},
	}

	r, err := reconciler.reconcile(ctx, obj, nil)
	g.Expect(r).To(Equal(reconcile.Result{}))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obj.Status.Conditions).To(HaveLen(2))
	g.Expect(conditions.IsReady(obj)).To(BeFalse())
	g.Expect(conditions.IsStalled(obj)).To(BeTrue())
	g.Expect(conditions.GetReason(obj, meta.ReadyCondition)).To(Equal(fluxcdv1.ReasonInvalidSchedule))
	g.Expect(conditions.GetReason(obj, meta.StalledCondition)).To(Equal(fluxcdv1.ReasonInvalidSchedule))
	g.Expect(conditions.GetMessage(obj, meta.ReadyCondition)).To(ContainSubstring(msgTerminalError))
	g.Expect(conditions.GetMessage(obj, meta.StalledCondition)).To(ContainSubstring(msgTerminalError))
}

func TestResourceSetInputProviderReconciler_SkippedDueToSchedule(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetInputProviderReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test-skipped-due-to-schedule")
	g.Expect(err).NotTo(HaveOccurred())

	objDef := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
  name: test-schedule
  namespace: "%[1]s"
  annotations:
    fluxcd.controlplane.io/reconcileTimeout: 100ms
spec:
  type: Static
  schedule:
  - cron: "0 0 29 2 *" # This cron only happens once every 4 years.
    window: 1s
  defaultValues:
    env: test
`, ns.Name)

	// Create the ResourceSetInputProvider.
	obj := &fluxcdv1.ResourceSetInputProvider{}
	err = yaml.Unmarshal([]byte(objDef), obj)
	g.Expect(err).NotTo(HaveOccurred())
	err = testEnv.Create(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	// Initialize the ResourceSetInputProvider.
	r, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(r.Requeue).To(BeTrue())

	// Reconcile and verify schedule.
	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(r.Requeue).To(BeFalse())

	result := &fluxcdv1.ResourceSetInputProvider{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).NotTo(HaveOccurred())

	sched, err := schedule.Parse("0 0 29 2 *", "UTC")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(sched).NotTo(BeNil())

	// Verify that the next schedule is set correctly.
	expectedRequeueAfter := time.Until(sched.Next(time.Now()))
	g.Expect(r.RequeueAfter).To(BeNumerically("~", expectedRequeueAfter, time.Second))

	testutils.LogObjectStatus(t, result)

	// Verify that the status contains the next schedule.
	g.Expect(result.Status.NextSchedule).NotTo(BeNil())
	g.Expect(result.Status.NextSchedule.Schedule).To(Equal(fluxcdv1.Schedule{
		Cron:     "0 0 29 2 *",
		TimeZone: "UTC",
		Window:   metav1.Duration{Duration: time.Second},
	}))

	// Verify that the status contains the next schedule time.
	untilWhen := time.Until(result.Status.NextSchedule.When.Time)
	g.Expect(untilWhen).To(BeNumerically("~", expectedRequeueAfter, time.Second))

	// Verify that the status ready condition reason is set to SkippedDueToSchedule.
	g.Expect(conditions.IsReady(result)).To(BeTrue())
	g.Expect(conditions.GetReason(result, meta.ReadyCondition)).To(Equal(fluxcdv1.ReasonSkippedDueToSchedule))

	// Verify skipped reconciliation event.
	events := getEvents(obj.Name, obj.Namespace)
	g.Expect(events).To(HaveLen(1))
	g.Expect(events[0].Reason).To(Equal(fluxcdv1.ReasonSkippedDueToSchedule))
	g.Expect(events[0].Message).To(ContainSubstring("Reconciliation skipped, next scheduled at"))
}

func TestResourceSetInputProviderReconciler_ProviderAuthAndSecretsCompatiblity(t *testing.T) {
	g := NewWithT(t)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test-provider-auth-and-secrets-compatibility")
	g.Expect(err).ToNot(HaveOccurred())

	for _, tt := range []struct {
		provider           string
		serviceAccountName bool
		certSecretRef      bool
		secretRef          bool
	}{
		{
			provider:           fluxcdv1.InputProviderStatic,
			serviceAccountName: false,
			certSecretRef:      false,
			secretRef:          false,
		},
		{
			provider:           fluxcdv1.InputProviderGitHubBranch,
			serviceAccountName: false,
			certSecretRef:      true,
			secretRef:          true,
		},
		{
			provider:           fluxcdv1.InputProviderGitHubTag,
			serviceAccountName: false,
			certSecretRef:      true,
			secretRef:          true,
		},
		{
			provider:           fluxcdv1.InputProviderGitHubPullRequest,
			serviceAccountName: false,
			certSecretRef:      true,
			secretRef:          true,
		},
		{
			provider:           fluxcdv1.InputProviderGitLabBranch,
			serviceAccountName: false,
			certSecretRef:      true,
			secretRef:          true,
		},
		{
			provider:           fluxcdv1.InputProviderGitLabTag,
			serviceAccountName: false,
			certSecretRef:      true,
			secretRef:          true,
		},
		{
			provider:           fluxcdv1.InputProviderGitLabMergeRequest,
			serviceAccountName: false,
			certSecretRef:      true,
			secretRef:          true,
		},
		{
			provider:           fluxcdv1.InputProviderAzureDevOpsBranch,
			serviceAccountName: true,
			certSecretRef:      false,
			secretRef:          true,
		},
		{
			provider:           fluxcdv1.InputProviderAzureDevOpsPullRequest,
			serviceAccountName: true,
			certSecretRef:      false,
			secretRef:          true,
		},
		{
			provider:           fluxcdv1.InputProviderAzureDevOpsTag,
			serviceAccountName: true,
			certSecretRef:      false,
			secretRef:          true,
		},
		{
			provider:           fluxcdv1.InputProviderOCIArtifactTag,
			serviceAccountName: true,
			certSecretRef:      true,
			secretRef:          true,
		},
		{
			provider:           fluxcdv1.InputProviderACRArtifactTag,
			serviceAccountName: true,
			certSecretRef:      false,
			secretRef:          false,
		},
		{
			provider:           fluxcdv1.InputProviderECRArtifactTag,
			serviceAccountName: true,
			certSecretRef:      false,
			secretRef:          false,
		},
		{
			provider:           fluxcdv1.InputProviderGARArtifactTag,
			serviceAccountName: true,
			certSecretRef:      false,
			secretRef:          false,
		},
	} {
		t.Run(tt.provider, func(t *testing.T) {
			g := NewWithT(t)

			// Prepare object and spec.
			obj := &fluxcdv1.ResourceSetInputProvider{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: ns.Name,
				},
			}
			spec := fluxcdv1.ResourceSetInputProviderSpec{
				Type: tt.provider,
			}
			urlScheme := "oci"
			if strings.HasPrefix(tt.provider, "Git") || strings.HasPrefix(tt.provider, "AzureDevOps") {
				urlScheme = "https"
			}
			if tt.provider != fluxcdv1.InputProviderStatic {
				spec.URL = fmt.Sprintf("%s://example.com/owner/repo", urlScheme)
			}

			// Validate serviceAccountName.
			const saErr = "cannot specify spec.serviceAccountName when spec.type is not one of AzureDevOps* or *ArtifactTag"
			obj.Spec = spec
			obj.Spec.ServiceAccountName = "test-sa"
			if !tt.serviceAccountName {
				err = testEnv.Create(ctx, obj)
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(saErr))
			} else {
				err = testEnv.Create(ctx, obj, client.DryRunAll, client.FieldOwner(controllerName))
				if err != nil {
					g.Expect(err.Error()).NotTo(ContainSubstring(saErr))
				}
			}

			// Validate certSecretRef.
			const certErr = "cannot specify spec.certSecretRef when spec.type is one of Static, AzureDevOps*, ACRArtifactTag, ECRArtifactTag or GARArtifactTag"
			obj.Spec = spec
			obj.Spec.CertSecretRef = &meta.LocalObjectReference{
				Name: "test-cert-secret",
			}
			if !tt.certSecretRef {
				err = testEnv.Create(ctx, obj)
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(certErr))
			} else {
				err = testEnv.Create(ctx, obj, client.DryRunAll, client.FieldOwner(controllerName))
				if err != nil {
					g.Expect(err.Error()).NotTo(ContainSubstring(certErr))
				}
			}

			// Validate secretRef.
			const secretErr = "cannot specify spec.secretRef when spec.type is one of Static, ACRArtifactTag, ECRArtifactTag or GARArtifactTag"
			obj.Spec = spec
			obj.Spec.SecretRef = &meta.LocalObjectReference{
				Name: "test-secret",
			}
			if !tt.secretRef {
				err = testEnv.Create(ctx, obj)
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(secretErr))
			} else {
				err = testEnv.Create(ctx, obj, client.DryRunAll, client.FieldOwner(controllerName))
				if err != nil {
					g.Expect(err.Error()).NotTo(ContainSubstring(secretErr))
				}
			}
		})
	}
}

func TestResourceSetInputProviderReconciler_CredentialAndAudiencesValidation(t *testing.T) {
	g := NewWithT(t)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test-credential-audiences-validation")
	g.Expect(err).ToNot(HaveOccurred())

	t.Run("credential can only be ServiceAccountToken when type is OCIArtifactTag", func(t *testing.T) {
		g := NewWithT(t)

		for _, provider := range []string{
			fluxcdv1.InputProviderACRArtifactTag,
			fluxcdv1.InputProviderECRArtifactTag,
			fluxcdv1.InputProviderGARArtifactTag,
		} {
			obj := &fluxcdv1.ResourceSetInputProvider{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-credential-" + strings.ToLower(provider),
					Namespace: ns.Name,
				},
				Spec: fluxcdv1.ResourceSetInputProviderSpec{
					Type:       provider,
					URL:        "oci://example.com/owner/repo",
					Credential: serviceaccounttoken.CredentialName,
					Audiences:  []string{"aud1"},
				},
			}
			err := testEnv.Create(ctx, obj)
			g.Expect(err).To(HaveOccurred())
			g.Expect(err.Error()).To(ContainSubstring(
				"spec.credential can be set to 'ServiceAccountToken' only when spec.type is 'OCIArtifactTag'"))
		}
	})

	t.Run("audiences can only be set when credential is ServiceAccountToken", func(t *testing.T) {
		g := NewWithT(t)

		obj := &fluxcdv1.ResourceSetInputProvider{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-audiences-without-credential",
				Namespace: ns.Name,
			},
			Spec: fluxcdv1.ResourceSetInputProviderSpec{
				Type:      fluxcdv1.InputProviderOCIArtifactTag,
				URL:       "oci://example.com/owner/repo",
				Audiences: []string{"aud1"},
			},
		}
		err := testEnv.Create(ctx, obj)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring(
			"spec.audiences can be set only when spec.credential is set to 'ServiceAccountToken'"))
	})

	t.Run("audiences must be set when credential is ServiceAccountToken", func(t *testing.T) {
		g := NewWithT(t)

		obj := &fluxcdv1.ResourceSetInputProvider{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-credential-without-audiences",
				Namespace: ns.Name,
			},
			Spec: fluxcdv1.ResourceSetInputProviderSpec{
				Type:       fluxcdv1.InputProviderOCIArtifactTag,
				URL:        "oci://example.com/owner/repo",
				Credential: serviceaccounttoken.CredentialName,
			},
		}
		err := testEnv.Create(ctx, obj)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring(
			"spec.audiences must be set when spec.credential is set to 'ServiceAccountToken'"))
	})

	t.Run("valid credential and audiences configuration", func(t *testing.T) {
		g := NewWithT(t)

		obj := &fluxcdv1.ResourceSetInputProvider{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-valid-credential-audiences",
				Namespace: ns.Name,
			},
			Spec: fluxcdv1.ResourceSetInputProviderSpec{
				Type:       fluxcdv1.InputProviderOCIArtifactTag,
				URL:        "oci://example.com/owner/repo",
				Credential: serviceaccounttoken.CredentialName,
				Audiences:  []string{"aud1"},
			},
		}
		err := testEnv.Create(ctx, obj, client.DryRunAll, client.FieldOwner(controllerName))
		g.Expect(err).ToNot(HaveOccurred())
	})
}

func TestResourceSetInputProviderReconciler_makeFilters(t *testing.T) {
	r := getResourceSetInputProviderReconciler(t)

	t.Run("no filters", func(t *testing.T) {
		g := NewWithT(t)

		obj := &fluxcdv1.ResourceSetInputProvider{
			Spec: fluxcdv1.ResourceSetInputProviderSpec{
				Type: fluxcdv1.InputProviderStatic,
			},
		}

		filters, err := r.makeFilters(obj)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(filters.Limit).To(Equal(100))
	})

	t.Run("with branch filters", func(t *testing.T) {
		for _, tt := range []struct {
			name    string
			filter  *fluxcdv1.ResourceSetInputFilter
			include string
			exclude string
		}{
			{
				name: "branch",
				filter: &fluxcdv1.ResourceSetInputFilter{
					Limit:         50,
					Labels:        []string{"env=production"},
					IncludeBranch: "^main$",
					ExcludeBranch: "^feature/.*$",
					Semver:        ">=1.0.0 <2.0.0",
				},
				include: "^main$",
				exclude: "^feature/.*$",
			},
			{
				name: "tag",
				filter: &fluxcdv1.ResourceSetInputFilter{
					Limit:      50,
					Labels:     []string{"env=production"},
					IncludeTag: "^v[0-9]+\\.[0-9]+\\.[0-9]+$",
					ExcludeTag: "^v[0-9]+\\.[0-9]+\\.[0-9]+-beta$",
					Semver:     ">=1.0.0 <2.0.0",
				},
				include: "^v[0-9]+\\.[0-9]+\\.[0-9]+$",
				exclude: "^v[0-9]+\\.[0-9]+\\.[0-9]+-beta$",
			},
		} {
			t.Run(tt.name, func(t *testing.T) {
				g := NewWithT(t)

				obj := &fluxcdv1.ResourceSetInputProvider{
					Spec: fluxcdv1.ResourceSetInputProviderSpec{
						Type:   fluxcdv1.InputProviderGitHubBranch,
						Filter: tt.filter,
					},
				}

				filters, err := r.makeFilters(obj)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(filters.Limit).To(Equal(50))
				g.Expect(filters.Labels).To(Equal([]string{"env=production"}))
				g.Expect(filters.Include).NotTo(BeNil())
				g.Expect(filters.Include.String()).To(Equal(tt.include))
				g.Expect(filters.Exclude).NotTo(BeNil())
				g.Expect(filters.Exclude.String()).To(Equal(tt.exclude))
				g.Expect(filters.SemVer).NotTo(BeNil())
				g.Expect(filters.SemVer.String()).To(Equal(">=1.0.0 <2.0.0"))
			})
		}
	})
}

func TestRequeueAfterResourceSetInputProvider(t *testing.T) {
	for _, tt := range []struct {
		name                 string
		interval             time.Duration
		schedules            []fluxcdv1.Schedule
		timeout              time.Duration
		reconcileEnd         string
		expectedRequeueAfter time.Duration
		expectedNextSchedule *fluxcdv1.NextSchedule
	}{
		{
			name:                 "disabled",
			interval:             0,
			reconcileEnd:         "2023-10-01T00:00:00Z",
			expectedRequeueAfter: 0,
		},
		{
			name:                 "no schedule",
			interval:             time.Hour,
			reconcileEnd:         "2023-10-01T00:00:00Z",
			expectedRequeueAfter: time.Hour,
		},
		{
			name:     "schedule without window",
			interval: time.Hour,
			schedules: []fluxcdv1.Schedule{{
				Cron: "0 9 * * *",
			}},
			timeout:              5 * time.Minute,
			reconcileEnd:         "2023-10-01T10:00:00Z",
			expectedRequeueAfter: 23 * time.Hour,
			expectedNextSchedule: &fluxcdv1.NextSchedule{
				Schedule: fluxcdv1.Schedule{
					Cron: "0 9 * * *",
				},
				When: metav1.Time{Time: time.Date(2023, 10, 2, 9, 0, 0, 0, time.UTC)},
			},
		},
		{
			name:     "next interval is within the window",
			interval: time.Hour,
			schedules: []fluxcdv1.Schedule{{
				Cron:   "0 9 * * *",
				Window: metav1.Duration{Duration: 8 * time.Hour},
			}},
			timeout:              5 * time.Minute,
			reconcileEnd:         "2023-10-01T10:00:00Z",
			expectedRequeueAfter: time.Hour,
		},
		{
			name:     "next interval is outside the window",
			interval: time.Hour,
			schedules: []fluxcdv1.Schedule{{
				Cron:   "0 9 * * *",
				Window: metav1.Duration{Duration: 8 * time.Hour},
			}},
			timeout:              5 * time.Minute,
			reconcileEnd:         "2023-10-01T18:00:00Z",
			expectedRequeueAfter: 15 * time.Hour,
			expectedNextSchedule: &fluxcdv1.NextSchedule{
				Schedule: fluxcdv1.Schedule{
					Cron:   "0 9 * * *",
					Window: metav1.Duration{Duration: 8 * time.Hour},
				},
				When: metav1.Time{Time: time.Date(2023, 10, 2, 9, 0, 0, 0, time.UTC)},
			},
		},
		{
			name:     "next interval is later than next schedule",
			interval: time.Hour,
			schedules: []fluxcdv1.Schedule{{
				Cron:   "0 9 * * *",
				Window: metav1.Duration{Duration: 8 * time.Hour},
			}},
			timeout:              5 * time.Minute,
			reconcileEnd:         "2023-10-01T08:30:00Z",
			expectedRequeueAfter: 30 * time.Minute,
			expectedNextSchedule: &fluxcdv1.NextSchedule{
				Schedule: fluxcdv1.Schedule{
					Cron:   "0 9 * * *",
					Window: metav1.Duration{Duration: 8 * time.Hour},
				},
				When: metav1.Time{Time: time.Date(2023, 10, 1, 9, 0, 0, 0, time.UTC)},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			obj := &fluxcdv1.ResourceSetInputProvider{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						fluxcdv1.ReconcileEveryAnnotation: tt.interval.String(),
					},
				},
			}

			scheduler, err := schedule.NewScheduler(tt.schedules, tt.timeout)
			g.Expect(err).NotTo(HaveOccurred())

			reconcileEnd := testutils.ParseTime(t, tt.reconcileEnd)

			res := requeueAfterResourceSetInputProvider(obj, scheduler, reconcileEnd)
			g.Expect(res.RequeueAfter).To(Equal(tt.expectedRequeueAfter))
			g.Expect(obj.Status.NextSchedule).To(Equal(tt.expectedNextSchedule))
		})
	}
}

// getResourceSetInputProviderReconciler returns a new ResourceSetInputProviderReconciler
// configured for testing purposes, with notifications disabled and a test event recorder.
func getResourceSetInputProviderReconciler(t *testing.T) *ResourceSetInputProviderReconciler {
	// Disable notifications for the tests as no pod is running.
	// This is required to avoid the 30s retry loop performed by the HTTP client.
	t.Setenv("NOTIFICATIONS_DISABLED", "yes")
	return &ResourceSetInputProviderReconciler{
		Client:        testClient,
		Scheme:        NewTestScheme(),
		StatusManager: controllerName,
		EventRecorder: testEnv.GetEventRecorderFor(controllerName),
	}
}

func TestResourceSetInputProviderReconciler_SuccessfullyFetchesSecret(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetInputProviderReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	// Create the secret.
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: ns.Name,
		},
		Data: map[string][]byte{
			"foo": []byte("bar"),
		},
	}
	err = testEnv.Create(ctx, secret)
	g.Expect(err).ToNot(HaveOccurred())

	objDef := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
  name: test-secret
  namespace: "%[1]s"
spec:
  type: OCIArtifactTag
  url: oci://ghcr.io/stefanprodan/podinfo
  secretRef:
    name: test-secret
  filter:
    limit: 1
`, ns.Name)

	// Create object.
	obj := &fluxcdv1.ResourceSetInputProvider{}
	err = yaml.Unmarshal([]byte(objDef), obj)
	g.Expect(err).NotTo(HaveOccurred())
	err = testEnv.Create(ctx, obj)
	g.Expect(err).NotTo(HaveOccurred())

	// Initialize the ResourceSetInputProvider.
	r, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(r.Requeue).To(BeTrue())

	// Reconcile.
	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(r.Requeue).To(BeFalse())
}
