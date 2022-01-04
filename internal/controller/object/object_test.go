/*
Copyright 2021 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package object

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/crossplane-contrib/provider-kubernetes/apis/object/v1alpha1"
	kubernetesv1alpha1 "github.com/crossplane-contrib/provider-kubernetes/apis/v1alpha1"
)

const (
	providerName            = "kubernetes-test" 
	providerSecretName      = "kubernetes-test-secret"
	providerSecretNamespace = "kubernetes-test-secret-namespace"

	providerSecretKey  = "kubeconfig"
	providerSecretData = "somethingsecret"

	testObjectName = "test-object"
	testNamespace  = "test-namespace"
)

var (
	errBoom = errors.New("boom")
)

type notKubernetesObject struct {
	resource.Managed
}

type kubernetesObjectModifier func(obj *v1alpha1.Object)

func kubernetesObject(om ...kubernetesObjectModifier) *v1alpha1.Object {
	o := &v1alpha1.Object{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testObjectName,
			Namespace: testNamespace,
		},
		Spec: v1alpha1.ObjectSpec{
			ResourceSpec: xpv1.ResourceSpec{
				ProviderConfigReference: &xpv1.Reference{
					Name: providerName,
				},
			},
			ForProvider: v1alpha1.ObjectParameters{
				Manifest: runtime.RawExtension{Raw: []byte(`{
				    "apiVersion": "v1",
				    "kind": "Namespace",
				    "metadata": {
				        "name": "crossplane-system"
				    }
				}`)},
			},
		},
		Status: v1alpha1.ObjectStatus{},
	}

	for _, m := range om {
		m(o)
	}

	return o
}

type providerConfigModifier func(pc *kubernetesv1alpha1.ProviderConfig)

func providerConfig(pm ...providerConfigModifier) *kubernetesv1alpha1.ProviderConfig {
	pc := &kubernetesv1alpha1.ProviderConfig{
		ObjectMeta: metav1.ObjectMeta{Name: providerName},
		Spec: kubernetesv1alpha1.ProviderConfigSpec{
			Credentials: kubernetesv1alpha1.ProviderCredentials{},
			Identity: &kubernetesv1alpha1.Identity{
				Type: kubernetesv1alpha1.IdentityTypeGoogleApplicationCredentials,
			},
		},
	}

	for _, m := range pm {
		m(pc)
	}

	return pc
}

func Test_connector_Connect(t *testing.T) {
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: providerSecretNamespace, Name: providerSecretName},
		Data:       map[string][]byte{providerSecretKey: []byte(providerSecretData)},
	}

	type args struct {
		client          client.Client
		kcfgExtractorFn func(ctx context.Context, src xpv1.CredentialsSource, c client.Client, ccs xpv1.CommonCredentialSelectors) ([]byte, error)
		gcpExtractorFn  func(ctx context.Context, src xpv1.CredentialsSource, c client.Client, ccs xpv1.CommonCredentialSelectors) ([]byte, error)
		gcpInjectorFn   func(ctx context.Context, rc *rest.Config, credentials []byte, scopes ...string) error
		newRESTConfigFn func(kubeconfig []byte) (*rest.Config, error)
		newKubeClientFn func(config *rest.Config) (client.Client, error)
		usage           resource.Tracker
		mg              resource.Managed
	}
	type want struct {
		err error
	}
	cases := map[string]struct {
		args
		want
	}{
		"NotKubernetesObject": {
			args: args{
				mg: notKubernetesObject{},
			},
			want: want{
				err: errors.New(errNotKubernetesObject),
			},
		},
		"FailedToTrackUsage": {
			args: args{
				usage: resource.TrackerFn(func(ctx context.Context, mg resource.Managed) error { return errBoom }),
				mg:    kubernetesObject(),
			},
			want: want{
				err: errors.Wrap(errBoom, errTrackPCUsage),
			},
		},
		"FailedToGetProvider": {
			args: args{
				client: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
						if key.Name == providerName {
							*obj.(*kubernetesv1alpha1.ProviderConfig) = *providerConfig()
							return errBoom
						}
						return nil
					},
				},
				usage: resource.TrackerFn(func(ctx context.Context, mg resource.Managed) error { return nil }),
				mg:    kubernetesObject(),
			},
			want: want{
				err: errors.Wrap(errBoom, errGetPC),
			},
		},
		"FailedToExtractKubeconfig": {
			args: args{
				client: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
						if key.Name == providerName {
							*obj.(*kubernetesv1alpha1.ProviderConfig) = *providerConfig()
							return nil
						}
						if key.Name == providerSecretName && key.Namespace == providerSecretNamespace {
							*obj.(*corev1.Secret) = secret
							return nil
						}
						return errBoom
					},
				},
				kcfgExtractorFn: func(ctx context.Context, src xpv1.CredentialsSource, c client.Client, ccs xpv1.CommonCredentialSelectors) ([]byte, error) {
					return nil, errBoom
				},
				usage: resource.TrackerFn(func(ctx context.Context, mg resource.Managed) error { return nil }),
				mg:    kubernetesObject(),
			},
			want: want{
				err: errors.Wrap(errBoom, errGetCreds),
			},
		},
		"FailedToCreateRESTConfig": {
			args: args{
				client: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
						if key.Name == providerName {
							*obj.(*kubernetesv1alpha1.ProviderConfig) = *providerConfig()
							return nil
						}
						if key.Name == providerSecretName && key.Namespace == providerSecretNamespace {
							*obj.(*corev1.Secret) = secret
							return nil
						}
						return errBoom
					},
				},
				kcfgExtractorFn: func(ctx context.Context, src xpv1.CredentialsSource, c client.Client, ccs xpv1.CommonCredentialSelectors) ([]byte, error) {
					return nil, nil
				},
				newRESTConfigFn: func(kubeconfig []byte) (config *rest.Config, err error) {
					return nil, errBoom
				},
				usage: resource.TrackerFn(func(ctx context.Context, mg resource.Managed) error { return nil }),
				mg:    kubernetesObject(),
			},
			want: want{
				err: errors.Wrap(errBoom, errFailedToCreateRestConfig),
			},
		},
		"FailedToExtractGoogleAppCreds": {
			args: args{
				client: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
						if key.Name == providerName {
							*obj.(*kubernetesv1alpha1.ProviderConfig) = *providerConfig()
							return nil
						}
						if key.Name == providerSecretName && key.Namespace == providerSecretNamespace {
							*obj.(*corev1.Secret) = secret
							return nil
						}
						return errBoom
					},
				},
				kcfgExtractorFn: func(ctx context.Context, src xpv1.CredentialsSource, c client.Client, ccs xpv1.CommonCredentialSelectors) ([]byte, error) {
					return nil, nil
				},
				newRESTConfigFn: func(kubeconfig []byte) (config *rest.Config, err error) {
					return nil, nil
				},
				gcpExtractorFn: func(ctx context.Context, src xpv1.CredentialsSource, c client.Client, ccs xpv1.CommonCredentialSelectors) ([]byte, error) {
					return nil, errBoom
				},
				usage: resource.TrackerFn(func(ctx context.Context, mg resource.Managed) error { return nil }),
				mg:    kubernetesObject(),
			},
			want: want{
				err: errors.Wrap(errBoom, errFailedToExtractGoogleCredentials),
			},
		},
		"FailedToInjectGoogleAppCreds": {
			args: args{
				client: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
						if key.Name == providerName {
							*obj.(*kubernetesv1alpha1.ProviderConfig) = *providerConfig()
							return nil
						}
						if key.Name == providerSecretName && key.Namespace == providerSecretNamespace {
							*obj.(*corev1.Secret) = secret
							return nil
						}
						return errBoom
					},
				},
				kcfgExtractorFn: func(ctx context.Context, src xpv1.CredentialsSource, c client.Client, ccs xpv1.CommonCredentialSelectors) ([]byte, error) {
					return nil, nil
				},
				newRESTConfigFn: func(kubeconfig []byte) (config *rest.Config, err error) {
					return nil, nil
				},
				gcpExtractorFn: func(ctx context.Context, src xpv1.CredentialsSource, c client.Client, ccs xpv1.CommonCredentialSelectors) ([]byte, error) {
					return nil, nil
				},
				gcpInjectorFn: func(ctx context.Context, rc *rest.Config, credentials []byte, scopes ...string) error {
					return errBoom
				},
				usage: resource.TrackerFn(func(ctx context.Context, mg resource.Managed) error { return nil }),
				mg:    kubernetesObject(),
			},
			want: want{
				err: errors.Wrap(errBoom, errFailedToInjectGoogleCredentials),
			},
		},
		"FailedToCreateNewKubernetesClient": {
			args: args{
				client: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
						if key.Name == providerName {
							*obj.(*kubernetesv1alpha1.ProviderConfig) = *providerConfig()
							return nil
						}
						if key.Name == providerSecretName && key.Namespace == providerSecretNamespace {
							*obj.(*corev1.Secret) = secret
							return nil
						}
						return errBoom
					},
					MockStatusUpdate: func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
						return nil
					},
				},
				kcfgExtractorFn: func(ctx context.Context, src xpv1.CredentialsSource, c client.Client, ccs xpv1.CommonCredentialSelectors) ([]byte, error) {
					return nil, nil
				},
				newRESTConfigFn: func(kubeconfig []byte) (config *rest.Config, err error) {
					return &rest.Config{}, nil
				},

				gcpExtractorFn: func(ctx context.Context, src xpv1.CredentialsSource, c client.Client, ccs xpv1.CommonCredentialSelectors) ([]byte, error) {
					return nil, nil
				},
				gcpInjectorFn: func(ctx context.Context, rc *rest.Config, credentials []byte, scopes ...string) error {
					return nil
				},
				newKubeClientFn: func(config *rest.Config) (c client.Client, err error) {
					return nil, errBoom
				},
				usage: resource.TrackerFn(func(ctx context.Context, mg resource.Managed) error { return nil }),
				mg:    kubernetesObject(),
			},
			want: want{
				err: errors.Wrap(errBoom, errNewKubernetesClient),
			},
		},
		"Success": {
			args: args{
				client: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
						switch t := obj.(type) {
						case *kubernetesv1alpha1.ProviderConfig:
							*t = *providerConfig()
						case *corev1.Secret:
							*t = secret
						default:
							return errBoom
						}
						return nil
					},
					MockStatusUpdate: func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
						return nil
					},
				},
				kcfgExtractorFn: func(ctx context.Context, src xpv1.CredentialsSource, c client.Client, ccs xpv1.CommonCredentialSelectors) ([]byte, error) {
					return nil, nil
				},
				newRESTConfigFn: func(kubeconfig []byte) (config *rest.Config, err error) {
					return &rest.Config{}, nil
				},
				gcpExtractorFn: func(ctx context.Context, src xpv1.CredentialsSource, c client.Client, ccs xpv1.CommonCredentialSelectors) ([]byte, error) {
					return nil, nil
				},
				gcpInjectorFn: func(ctx context.Context, rc *rest.Config, credentials []byte, scopes ...string) error {
					return nil
				},
				newKubeClientFn: func(config *rest.Config) (c client.Client, err error) {
					return &test.MockClient{}, nil
				},
				usage: resource.TrackerFn(func(ctx context.Context, mg resource.Managed) error { return nil }),
				mg:    kubernetesObject(),
			},
			want: want{
				err: nil,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := &connector{
				logger:          logging.NewNopLogger(),
				kube:            tc.args.client,
				kcfgExtractorFn: tc.args.kcfgExtractorFn,
				gcpExtractorFn:  tc.args.gcpExtractorFn,
				gcpInjectorFn:   tc.args.gcpInjectorFn,
				newRESTConfigFn: tc.args.newRESTConfigFn,
				newKubeClientFn: tc.args.newKubeClientFn,
				usage:           tc.usage,
			}
			_, gotErr := c.Connect(context.Background(), tc.args.mg)
			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("Connect(...): -want error, +got error: %s", diff)
			}
		})
	}
}

func Test_helmExternal_Observe(t *testing.T) {
	type args struct {
		client resource.ClientApplicator
		mg     resource.Managed
	}
	type want struct {
		out managed.ExternalObservation
		err error
	}

	r := unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "myapi.version",
		"kind":       "Cluster",
		"metadata": map[string]interface{}{
			"namespace": "cross",
			"name":      "mycluster",
		},
		"spec": map[string]interface{}{
			"interestingfield": "88888",
		},
	}}

	cases := map[string]struct {
		args
		want
	}{
		"NotKubernetesObject": {
			args: args{
				mg: notKubernetesObject{},
			},
			want: want{
				err: errors.New(errNotKubernetesObject),
			},
		},
		"NoKubernetesObjectExists": {
			args: args{
				mg: kubernetesObject(),
				client: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
					},
				},
			},
			want: want{
				out: managed.ExternalObservation{ResourceExists: false},
				err: nil,
			},
		},
		"NotAValidManifest": {
			args: args{
				mg: kubernetesObject(func(obj *v1alpha1.Object) {
					obj.Spec.ForProvider.Manifest.Raw = []byte(`{"test": "not-a-valid-manifest"}`)
				}),
			},
			want: want{
				err: errors.Wrap(errors.Errorf(`Object 'Kind' is missing in '{"test": "not-a-valid-manifest"}'`), errUnmarshalTemplate),
			},
		},
		"FailedToGet": {
			args: args{
				mg: kubernetesObject(),
				client: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(errBoom),
					},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetObject),
			},
		},
		"NoLastAppliedAnnotation": {
			args: args{
				mg: kubernetesObject(),
				client: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							*obj.(*unstructured.Unstructured) = unstructured.Unstructured{
								Object: map[string]interface{}{
									"apiVersion": "v1",
									"kind":       "Namespace",
									"metadata": map[string]interface{}{
										"name": "crossplane-system",
									},
								},
							}
							return nil
						}),
					},
				},
			},
			want: want{
				out: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: false},
				err: nil,
			},
		},
		"NotUpToDate": {
			args: args{
				mg: kubernetesObject(),
				client: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							*obj.(*unstructured.Unstructured) = unstructured.Unstructured{
								Object: map[string]interface{}{
									"apiVersion": "v1",
									"kind":       "Namespace",
									"metadata": map[string]interface{}{
										"name": "crossplane-system",
										"annotations": map[string]interface{}{
											corev1.LastAppliedConfigAnnotation: `{"apiVersion":"v1","kind":"Namespace","metadata":{"name":"crossplane-system", "labels": {"old-label":"gone"}}}`,
										},
									},
								},
							}
							return nil
						}),
					},
				},
			},
			want: want{
				out: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: false},
				err: nil,
			},
		},
		"UpToDateNameDefaultsToObjectName": {
			args: args{
				mg: kubernetesObject(func(obj *v1alpha1.Object) {
					obj.Spec.ForProvider.Manifest.Raw = []byte(`{
				    "apiVersion": "v1",
				    "kind": "Namespace" }`)
				}),
				client: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							*obj.(*unstructured.Unstructured) = unstructured.Unstructured{
								Object: map[string]interface{}{
									"apiVersion": "v1",
									"kind":       "Namespace",
									"metadata": map[string]interface{}{
										"name": testObjectName,
										"annotations": map[string]interface{}{
											corev1.LastAppliedConfigAnnotation: `{"apiVersion":"v1","kind":"Namespace"}`,
										},
									},
								},
							}
							return nil
						}),
					},
				},
			},
			want: want{
				out: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true, ConnectionDetails: managed.ConnectionDetails{}},
				err: nil,
			},
		},
		"UpToDate": {
			args: args{
				mg: kubernetesObject(),
				client: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							*obj.(*unstructured.Unstructured) = unstructured.Unstructured{
								Object: map[string]interface{}{
									"apiVersion": "v1",
									"kind":       "Namespace",
									"metadata": map[string]interface{}{
										"name": "crossplane-system",
										"annotations": map[string]interface{}{
											corev1.LastAppliedConfigAnnotation: `{"apiVersion":"v1","kind":"Namespace","metadata":{"name":"crossplane-system"}}`,
										},
									},
								},
							}
							return nil
						}),
					},
				},
			},
			want: want{
				out: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true, ConnectionDetails: managed.ConnectionDetails{}},
				err: nil,
			},
		},
		"UpToDateConnDetailsConstantValues": {
			args: args{
				mg: kubernetesObject(func(obj *v1alpha1.Object) {
					obj.Spec.ConnectionDetails = []v1alpha1.ConnectionDetail{
						{
							ToConnectionSecretKey: "field.key.not.parsed.as.path",
							Value:                 "constant-value",
						},
						{
							ToConnectionSecretKey: "anotherKey",
							Value:                 "AnotherValue",
						},
					}
				}),
				client: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							*obj.(*unstructured.Unstructured) = unstructured.Unstructured{
								Object: map[string]interface{}{
									"apiVersion": "v1",
									"kind":       "Namespace",
									"metadata": map[string]interface{}{
										"name": "crossplane-system",
										"annotations": map[string]interface{}{
											corev1.LastAppliedConfigAnnotation: `{"apiVersion":"v1","kind":"Namespace","metadata":{"name":"crossplane-system"}}`,
										},
									},
								},
							}
							return nil
						}),
					},
				},
			},
			want: want{
				out: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true, ConnectionDetails: managed.ConnectionDetails{
					"field.key.not.parsed.as.path": []byte("constant-value"),
					"anotherKey":                   []byte("AnotherValue"),
				}},
				err: nil,
			},
		},
		"UpToDateConnDetailsFromResource": {
			args: args{
				mg: kubernetesObject(func(obj *v1alpha1.Object) {
					obj.Spec.ConnectionDetails = []v1alpha1.ConnectionDetail{
						{
							ObjectReference: corev1.ObjectReference{
								Kind:       "Cluster",
								Namespace:  "cross",
								Name:       "mycluster",
								APIVersion: "myapi.version",
								FieldPath:  "spec.interestingfield",
							},

							ToConnectionSecretKey: "fieldinconnectionsecret",
						},
					}
				}),
				client: resource.ClientApplicator{
					Client: &test.MockClient{

						MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
							if key.Name == "mycluster" && key.Namespace == "cross" {
								*obj.(*unstructured.Unstructured) = r
								return nil
							}

							*obj.(*unstructured.Unstructured) = unstructured.Unstructured{
								Object: map[string]interface{}{
									"apiVersion": "v1",
									"kind":       "Namespace",
									"metadata": map[string]interface{}{
										"name": "crossplane-system",
										"annotations": map[string]interface{}{
											corev1.LastAppliedConfigAnnotation: `{"apiVersion":"v1","kind":"Namespace","metadata":{"name":"crossplane-system"}}`,
										},
									},
								},
							}
							return nil
						},
					},
				},
			},
			want: want{
				out: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true, ConnectionDetails: managed.ConnectionDetails{
					"fieldinconnectionsecret": []byte("88888"),
				}},
				err: nil,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := &external{
				logger: logging.NewNopLogger(),
				client: tc.args.client,
			}
			got, gotErr := e.Observe(context.Background(), tc.args.mg)
			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("e.Observe(...): -want error, +got error: %s", diff)
			}

			if diff := cmp.Diff(tc.want.out, got); diff != "" {
				t.Fatalf("e.Observe(...): -want out, +got out: %s", diff)
			}
		})
	}
}

func Test_helmExternal_Create(t *testing.T) {
	type args struct {
		client resource.ClientApplicator
		mg     resource.Managed
	}
	type want struct {
		out managed.ExternalCreation
		err error
	}
	cases := map[string]struct {
		args
		want
	}{
		"NotKubernetesObject": {
			args: args{
				mg: notKubernetesObject{},
			},
			want: want{
				err: errors.New(errNotKubernetesObject),
			},
		},
		"NotAValidManifest": {
			args: args{
				mg: kubernetesObject(func(obj *v1alpha1.Object) {
					obj.Spec.ForProvider.Manifest.Raw = []byte(`{"test": "not-a-valid-manifest"}`)
				}),
			},
			want: want{
				err: errors.Wrap(errors.Errorf(`Object 'Kind' is missing in '{"test": "not-a-valid-manifest"}'`), errUnmarshalTemplate),
			},
		},
		"FailedToCreate": {
			args: args{
				mg: kubernetesObject(),
				client: resource.ClientApplicator{
					Client: &test.MockClient{
						MockCreate: test.NewMockCreateFn(errBoom),
					},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errCreateObject),
			},
		},
		"SuccessDefaultsToObjectName": {
			args: args{
				mg: kubernetesObject(func(obj *v1alpha1.Object) {
					obj.Spec.ForProvider.Manifest.Raw = []byte(`{
				    "apiVersion": "v1",
				    "kind": "Namespace" }`)
				}),
				client: resource.ClientApplicator{
					Client: &test.MockClient{
						MockCreate: test.NewMockCreateFn(nil, func(obj client.Object) error {
							_, ok := obj.GetAnnotations()[corev1.LastAppliedConfigAnnotation]
							if !ok {
								t.Errorf("Last applied annotation not set with create")
							}
							if obj.GetName() != testObjectName {
								t.Errorf("Name should default to object name when not provider in manifest")
							}
							return nil
						}),
					},
				},
			},
			want: want{
				err: nil,
			},
		},
		"Success": {
			args: args{
				mg: kubernetesObject(),
				client: resource.ClientApplicator{
					Client: &test.MockClient{
						MockCreate: test.NewMockCreateFn(nil, func(obj client.Object) error {
							_, ok := obj.GetAnnotations()[corev1.LastAppliedConfigAnnotation]
							if !ok {
								t.Errorf("Last applied annotation not set with create")
							}
							return nil
						}),
					},
				},
			},
			want: want{
				err: nil,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := &external{
				logger: logging.NewNopLogger(),
				client: tc.args.client,
			}
			got, gotErr := e.Create(context.Background(), tc.args.mg)
			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("e.Create(...): -want error, +got error: %s", diff)
			}

			if diff := cmp.Diff(tc.want.out, got); diff != "" {
				t.Fatalf("e.Create(...): -want out, +got out: %s", diff)
			}
		})
	}
}

func Test_helmExternal_Update(t *testing.T) {
	type args struct {
		client resource.ClientApplicator
		mg     resource.Managed
	}
	type want struct {
		out managed.ExternalUpdate
		err error
	}
	cases := map[string]struct {
		args
		want
	}{
		"NotKubernetesObject": {
			args: args{
				mg: notKubernetesObject{},
			},
			want: want{
				err: errors.New(errNotKubernetesObject),
			},
		},
		"NotAValidManifest": {
			args: args{
				mg: kubernetesObject(func(obj *v1alpha1.Object) {
					obj.Spec.ForProvider.Manifest.Raw = []byte(`{"test": "not-a-valid-manifest"}`)
				}),
			},
			want: want{
				err: errors.Wrap(errors.Errorf(`Object 'Kind' is missing in '{"test": "not-a-valid-manifest"}'`), errUnmarshalTemplate),
			},
		},
		"FailedToApply": {
			args: args{
				mg: kubernetesObject(),
				client: resource.ClientApplicator{
					Applicator: resource.ApplyFn(func(context.Context, client.Object, ...resource.ApplyOption) error {
						return errBoom
					}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errApplyObject),
			},
		},
		"SuccessDefaultsToObjectName": {
			args: args{
				mg: kubernetesObject(func(obj *v1alpha1.Object) {
					obj.Spec.ForProvider.Manifest.Raw = []byte(`{
				    "apiVersion": "v1",
				    "kind": "Namespace" }`)
				}),
				client: resource.ClientApplicator{
					Applicator: resource.ApplyFn(func(ctx context.Context, obj client.Object, op ...resource.ApplyOption) error {
						if obj.GetName() != testObjectName {
							t.Errorf("Name should default to object name when not provider in manifest")
						}
						return nil
					}),
				},
			},
			want: want{
				err: nil,
			},
		},
		"Success": {
			args: args{
				mg: kubernetesObject(),
				client: resource.ClientApplicator{
					Applicator: resource.ApplyFn(func(context.Context, client.Object, ...resource.ApplyOption) error {
						return nil
					}),
				},
			},
			want: want{
				err: nil,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := &external{
				logger: logging.NewNopLogger(),
				client: tc.args.client,
			}
			got, gotErr := e.Update(context.Background(), tc.args.mg)
			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("e.Update(...): -want error, +got error: %s", diff)
			}

			if diff := cmp.Diff(tc.want.out, got); diff != "" {
				t.Fatalf("e.Update(...): -want out, +got out: %s", diff)
			}
		})
	}
}

func Test_helmExternal_Delete(t *testing.T) {
	type args struct {
		client resource.ClientApplicator
		mg     resource.Managed
	}
	type want struct {
		err error
	}
	cases := map[string]struct {
		args
		want
	}{
		"NotKubernetesObject": {
			args: args{
				mg: notKubernetesObject{},
			},
			want: want{
				err: errors.New(errNotKubernetesObject),
			},
		},
		"NotAValidManifest": {
			args: args{
				mg: kubernetesObject(func(obj *v1alpha1.Object) {
					obj.Spec.ForProvider.Manifest.Raw = []byte(`{"test": "not-a-valid-manifest"}`)
				}),
			},
			want: want{
				err: errors.Wrap(errors.Errorf(`Object 'Kind' is missing in '{"test": "not-a-valid-manifest"}'`), errUnmarshalTemplate),
			},
		},
		"FailedToDelete": {
			args: args{
				mg: kubernetesObject(),
				client: resource.ClientApplicator{
					Client: &test.MockClient{
						MockDelete: test.NewMockDeleteFn(errBoom),
					},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errDeleteObject),
			},
		},
		"SuccessDefaultsToObjectName": {
			args: args{
				mg: kubernetesObject(func(obj *v1alpha1.Object) {
					obj.Spec.ForProvider.Manifest.Raw = []byte(`{
				    "apiVersion": "v1",
				    "kind": "Namespace" }`)
				}),
				client: resource.ClientApplicator{
					Client: &test.MockClient{
						MockDelete: test.NewMockDeleteFn(nil, func(obj client.Object) error {
							if obj.GetName() != testObjectName {
								t.Errorf("Name should default to object name when not provider in manifest")
							}
							return nil
						}),
					},
				},
			},
			want: want{
				err: nil,
			},
		},
		"Success": {
			args: args{
				mg: kubernetesObject(),
				client: resource.ClientApplicator{
					Client: &test.MockClient{
						MockDelete: test.NewMockDeleteFn(nil),
					},
				},
			},
			want: want{
				err: nil,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := &external{
				logger: logging.NewNopLogger(),
				client: tc.args.client,
			}
			gotErr := e.Delete(context.Background(), tc.args.mg)
			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("e.Delete(...): -want error, +got error: %s", diff)
			}
		})
	}
}
