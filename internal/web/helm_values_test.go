// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"context"
	"testing"

	"github.com/fluxcd/pkg/apis/meta"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"
)

func Test_helmValuesFromReferences(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	tests := []struct {
		name       string
		objects    []client.Object
		namespace  string
		references []meta.ValuesReference
		values     string
		want       map[string]any
		wantErr    bool
	}{
		{
			name: "merges",
			objects: []client.Object{
				mockConfigMap("values", map[string]string{
					"values.yaml": `flat: value
nested:
  configuration: value
`,
				}),
				mockSecret("values", map[string][]byte{
					"values.yaml": []byte(`flat:
  nested: value
nested: value
`),
				}),
			},
			references: []meta.ValuesReference{
				{
					Kind: "ConfigMap",
					Name: "values",
				},
				{
					Kind: "Secret",
					Name: "values",
				},
			},
			values: `
other: values
`,
			want: map[string]any{
				"flat": map[string]any{
					"nested": "value",
				},
				"nested": "value",
				"other":  "values",
			},
		},
		{
			name: "with target path",
			objects: []client.Object{
				mockSecret("values", map[string][]byte{"single": []byte("value")}),
			},
			references: []meta.ValuesReference{
				{
					Kind:       "Secret",
					Name:       "values",
					ValuesKey:  "single",
					TargetPath: "merge.at.specific.path",
				},
			},
			want: map[string]any{
				"merge": map[string]any{
					"at": map[string]any{
						"specific": map[string]any{
							"path": "value",
						},
					},
				},
			},
		},
		{
			name: "target path precedence over all",
			objects: []client.Object{
				mockConfigMap("values", map[string]string{
					"values.yaml": `flat: value
nested:
  configuration:
  - one
  - two
  - three
`,
				}),
				mockSecret("values", map[string][]byte{"key": []byte("value")}),
			},
			references: []meta.ValuesReference{
				{
					Kind:       "Secret",
					Name:       "values",
					ValuesKey:  "key",
					TargetPath: "nested.configuration[0]",
				},
				{
					Kind: "ConfigMap",
					Name: "values",
				},
			},

			values: `
nested:
  configuration:
  - list
  - item
  - option
`,
			want: map[string]any{
				"flat": "value",
				"nested": map[string]any{
					"configuration": []any{"value", "item", "option"},
				},
			},
		},
		{
			name: "target path for string type array item",
			objects: []client.Object{
				mockConfigMap("values", map[string]string{
					"values.yaml": `flat: value
nested:
  configuration:
  - list
  - item
  - option
`,
				}),
				mockSecret("values", map[string][]byte{
					"values.yaml": []byte(`foo`),
				}),
			},
			references: []meta.ValuesReference{
				{
					Kind: "ConfigMap",
					Name: "values",
				},
				{
					Kind:       "Secret",
					Name:       "values",
					TargetPath: "nested.configuration[1]",
				},
			},
			values: `
other: values
`,
			want: map[string]any{
				"flat": "value",
				"nested": map[string]any{
					"configuration": []any{"list", "foo", "option"},
				},
				"other": "values",
			},
		},
		{
			name: "values reference to non existing secret",
			references: []meta.ValuesReference{
				{
					Kind: "Secret",
					Name: "missing",
				},
			},
			wantErr: true,
		},
		{
			name: "optional values reference to non existing secret",
			references: []meta.ValuesReference{
				{
					Kind:     "Secret",
					Name:     "missing",
					Optional: true,
				},
			},
			want:    map[string]any{},
			wantErr: false,
		},
		{
			name: "values reference to non existing config map",
			references: []meta.ValuesReference{
				{
					Kind: "ConfigMap",
					Name: "missing",
				},
			},
			wantErr: true,
		},
		{
			name: "optional values reference to non existing config map",
			references: []meta.ValuesReference{
				{
					Kind:     "ConfigMap",
					Name:     "missing",
					Optional: true,
				},
			},
			want:    map[string]any{},
			wantErr: false,
		},
		{
			name: "missing secret key",
			objects: []client.Object{
				mockSecret("no-key", nil),
			},
			references: []meta.ValuesReference{
				{
					Kind:      "Secret",
					Name:      "no-key",
					ValuesKey: "nonexisting",
				},
			},
			wantErr: true,
		},
		{
			name: "missing config map key",
			objects: []client.Object{
				mockConfigMap("no-key", nil),
			},
			references: []meta.ValuesReference{
				{
					Kind:      "ConfigMap",
					Name:      "no-key",
					ValuesKey: "nonexisting",
				},
			},
			wantErr: true,
		},
		{
			name: "unsupported values reference kind",
			references: []meta.ValuesReference{
				{
					Kind: "Unsupported",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid values",
			objects: []client.Object{
				mockConfigMap("values", map[string]string{
					"values.yaml": `
invalid`,
				}),
			},
			references: []meta.ValuesReference{
				{
					Kind: "ConfigMap",
					Name: "values",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tt.objects...).Build()
			var values map[string]any
			if tt.values != "" {
				err := yaml.Unmarshal([]byte(tt.values), &values)
				g.Expect(err).ToNot(HaveOccurred())
			}
			got, err := helmValuesFromReferences(context.TODO(), c, tt.namespace, values, tt.references...)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
				g.Expect(got).To(BeNil())
				return
			}
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(got).To(Equal(tt.want))
		})
	}
}

func Test_replacePathValue(t *testing.T) {
	tests := []struct {
		name    string
		value   []byte
		path    string
		want    map[string]any
		wantErr bool
	}{
		{
			name:  "outer inner",
			value: []byte("value"),
			path:  "outer.inner",
			want: map[string]any{
				"outer": map[string]any{
					"inner": "value",
				},
			},
		},
		{
			name:  "inline list",
			value: []byte("{a,b,c}"),
			path:  "name",
			want: map[string]any{
				"name": []any{"a", "b", "c"},
			},
		},
		{
			name:  "with escape",
			value: []byte(`value1\,value2`),
			path:  "name",
			want: map[string]any{
				"name": "value1,value2",
			},
		},
		{
			name:  "target path with boolean value",
			value: []byte("true"),
			path:  "merge.at.specific.path",
			want: map[string]any{
				"merge": map[string]any{
					"at": map[string]any{
						"specific": map[string]any{
							"path": true,
						},
					},
				},
			},
		},
		{
			name:  "target path with set-string behavior",
			value: []byte(`"true"`),
			path:  "merge.at.specific.path",
			want: map[string]any{
				"merge": map[string]any{
					"at": map[string]any{
						"specific": map[string]any{
							"path": "true",
						},
					},
				},
			},
		},
		{
			name:  "target path with array item",
			value: []byte("value"),
			path:  "merge.at[2]",
			want: map[string]any{
				"merge": map[string]any{
					"at": []any{nil, nil, "value"},
				},
			},
		},
		{
			name:  "dot sequence escaping path",
			value: []byte("master"),
			path:  `nodeSelector.kubernetes\.io/role`,
			want: map[string]any{
				"nodeSelector": map[string]any{
					"kubernetes.io/role": "master",
				},
			},
		},
		{
			name:  "integer value",
			value: []byte("42"),
			path:  "replicas",
			want: map[string]any{
				"replicas": int64(42),
			},
		},
		{
			name:  "zero value",
			value: []byte("0"),
			path:  "count",
			want: map[string]any{
				"count": int64(0),
			},
		},
		{
			name:  "leading zeros stay string",
			value: []byte("00009"),
			path:  "code",
			want: map[string]any{
				"code": "00009",
			},
		},
		{
			name:  "null value",
			value: []byte("null"),
			path:  "key",
			want: map[string]any{
				"key": nil,
			},
		},
		{
			name:  "empty value",
			value: []byte(""),
			path:  "key",
			want: map[string]any{
				"key": "",
			},
		},
		{
			name:  "single-quoted string preserves type",
			value: []byte("'42'"),
			path:  "version",
			want: map[string]any{
				"version": "42",
			},
		},
		{
			name:  "array index with nested object",
			value: []byte("bar"),
			path:  "list[0].foo",
			want: map[string]any{
				"list": []any{
					map[string]any{"foo": "bar"},
				},
			},
		},
		{
			name:  "integer inline list",
			value: []byte("{8080,9090}"),
			path:  "ports",
			want: map[string]any{
				"ports": []any{int64(8080), int64(9090)},
			},
		},
		{
			name:  "escaped equals in value",
			value: []byte(`one\=two`),
			path:  "name",
			want: map[string]any{
				"name": "one=two",
			},
		},
		{
			name:  "nested array indices",
			value: []byte("1"),
			path:  "nested[0][0]",
			want: map[string]any{
				"nested": []any{[]any{int64(1)}},
			},
		},
		{
			name:  "nested array indices with gaps",
			value: []byte("1"),
			path:  "nested[1][1]",
			want: map[string]any{
				"nested": []any{nil, []any{nil, int64(1)}},
			},
		},
		{
			name:  "nested array with object key",
			value: []byte("bar"),
			path:  "list[0][0].foo",
			want: map[string]any{
				"list": []any{[]any{map[string]any{"foo": "bar"}}},
			},
		},
		{
			name:  "triple nested array indices",
			value: []byte("deep"),
			path:  "a[0][0][0]",
			want: map[string]any{
				"a": []any{[]any{[]any{"deep"}}},
			},
		},
		{
			name:  "triple nested array with gaps",
			value: []byte("1"),
			path:  "a[1][0][2]",
			want: map[string]any{
				"a": []any{nil, []any{[]any{nil, nil, int64(1)}}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			values := map[string]any{}
			err := replacePathValue(values, tt.path, string(tt.value))
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
				return
			}
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(values).To(Equal(tt.want))
		})
	}
}

func mockSecret(name string, data map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Data:       data,
	}
}

func mockConfigMap(name string, data map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Data:       data,
	}
}
