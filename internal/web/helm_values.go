// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/transform"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

// helmValuesFromReferences resolves Helm release values from ConfigMap/Secret
// references, merging them in the order given. If provided, the values map is
// merged last overwriting values from references, unless a reference has a
// targetPath specified, in which case it will overwrite all.
func helmValuesFromReferences(ctx context.Context, c client.Reader, namespace string,
	values map[string]any, refs ...meta.ValuesReference) (map[string]any, error) {

	result := make(map[string]any)
	resources := make(map[string]client.Object)

	for _, ref := range refs {
		namespacedName := types.NamespacedName{Namespace: namespace, Name: ref.Name}
		var valuesData []byte

		switch ref.Kind {
		case "ConfigMap", "Secret":
			index := ref.Kind + namespacedName.String()

			resource, ok := resources[index]
			if !ok {
				// The resource may not exist, but we want to act on a single version
				// of the resource in case the values reference is marked as optional.
				resources[index] = nil

				switch ref.Kind {
				case "Secret":
					resource = &corev1.Secret{}
				case "ConfigMap":
					resource = &corev1.ConfigMap{}
				}

				if resource != nil {
					if err := c.Get(ctx, namespacedName, resource); err != nil {
						if errors.IsNotFound(err) {
							if ref.Optional {
								continue
							}
							return nil, fmt.Errorf("values reference %s '%s' not found", ref.Kind, namespacedName)
						}
						return nil, err
					}
					resources[index] = resource
				}
			}

			if resource == nil {
				if ref.Optional {
					continue
				}
				return nil, fmt.Errorf("values reference %s '%s' not found", ref.Kind, namespacedName)
			}

			key := ref.GetValuesKey()
			switch typedRes := resource.(type) {
			case *corev1.Secret:
				data, ok := typedRes.Data[key]
				if !ok {
					if ref.Optional {
						continue
					}
					return nil, fmt.Errorf("key '%s' not found in %s '%s'", key, ref.Kind, namespacedName)
				}
				valuesData = data
			case *corev1.ConfigMap:
				data, ok := typedRes.Data[key]
				if !ok {
					if ref.Optional {
						continue
					}
					return nil, fmt.Errorf("key '%s' not found in %s '%s'", key, ref.Kind, namespacedName)
				}
				valuesData = []byte(data)
			}
		default:
			return nil, fmt.Errorf("unsupported values reference kind '%s'", ref.Kind)
		}

		if ref.TargetPath != "" {
			result = transform.MergeMaps(result, values)
			if err := replacePathValue(result, ref.TargetPath, string(valuesData)); err != nil {
				return nil, fmt.Errorf("failed to set target path '%s' for %s '%s': %w",
					ref.TargetPath, ref.Kind, namespacedName, err)
			}
			continue
		}

		var parsed map[string]any
		if err := yaml.Unmarshal(valuesData, &parsed); err != nil {
			return nil, fmt.Errorf("failed to parse values from %s '%s': %w", ref.Kind, namespacedName, err)
		}
		result = transform.MergeMaps(result, parsed)
	}

	return transform.MergeMaps(result, values), nil
}

// replacePathValue sets a value at the given dot-notation path in the values map.
// Quoted values (single or double) are always set as strings.
// Unquoted values undergo type coercion matching Helm's strvals behavior.
// Supports array index syntax (e.g. "a[0].b") and escaped dots.
func replacePathValue(values map[string]any, path string, value string) error {
	const (
		singleQuote = "'"
		doubleQuote = `"`
	)
	isSingleQuoted := strings.HasPrefix(value, singleQuote) && strings.HasSuffix(value, singleQuote)
	isDoubleQuoted := strings.HasPrefix(value, doubleQuote) && strings.HasSuffix(value, doubleQuote)
	forceString := isSingleQuoted || isDoubleQuoted
	if forceString {
		value = strings.Trim(value, singleQuote+doubleQuote)
	}

	segments := parsePathSegments(path)
	if len(segments) == 0 {
		return fmt.Errorf("empty path")
	}

	resolved := resolveValue(value, forceString)
	// setInParent is used to propagate slice changes back up to the parent
	// container, since setSliceIndex may allocate a new backing array.
	var setInParent func(any)
	var current any = values
	for i, seg := range segments {
		isLast := i == len(segments)-1

		switch c := current.(type) {
		case map[string]any:
			if seg.index >= 0 {
				// key[index] on a map: get or create the slice at key
				arr, _ := c[seg.key].([]any)
				if isLast {
					c[seg.key] = setSliceIndex(arr, seg.index, resolved)
					return nil
				}
				// Check if the next segment is also an array index (nested arrays)
				if i+1 < len(segments) && segments[i+1].key == "" {
					var inner []any
					if seg.index < len(arr) {
						inner, _ = arr[seg.index].([]any)
					}
					newArr := setSliceIndex(arr, seg.index, inner)
					c[seg.key] = newArr
					current = inner
					parentArr := newArr
					parentIdx := seg.index
					parentKey := seg.key
					parentMap := c
					setInParent = func(v any) {
						parentArr[parentIdx] = v
						parentMap[parentKey] = parentArr
					}
				} else {
					// Ensure a map exists at the array index for further nesting
					next := make(map[string]any)
					if seg.index < len(arr) {
						if m, ok := arr[seg.index].(map[string]any); ok {
							next = m
						}
					}
					c[seg.key] = setSliceIndex(arr, seg.index, next)
					current = next
					setInParent = nil
				}
			} else {
				if isLast {
					c[seg.key] = resolved
					return nil
				}
				// Navigate or create nested map
				if existing, ok := c[seg.key]; ok {
					if m, ok := existing.(map[string]any); ok {
						current = m
						setInParent = nil
						continue
					}
				}
				next := make(map[string]any)
				c[seg.key] = next
				current = next
				setInParent = nil
			}
		case []any:
			// Nested array index: seg.key is "" and seg.index >= 0
			if seg.index >= 0 {
				if isLast {
					newArr := setSliceIndex(c, seg.index, resolved)
					if setInParent != nil {
						setInParent(newArr)
					}
					return nil
				}
				// Check if next segment is also an array index
				if i+1 < len(segments) && segments[i+1].key == "" {
					var inner []any
					if seg.index < len(c) {
						inner, _ = c[seg.index].([]any)
					}
					newArr := setSliceIndex(c, seg.index, inner)
					if setInParent != nil {
						setInParent(newArr)
					}
					current = inner
					outerArr := newArr
					outerIdx := seg.index
					outerSetInParent := setInParent
					setInParent = func(v any) {
						outerArr[outerIdx] = v
						if outerSetInParent != nil {
							outerSetInParent(outerArr)
						}
					}
				} else {
					// Next segment is an object key: ensure a map at this index
					next := make(map[string]any)
					if seg.index < len(c) {
						if m, ok := c[seg.index].(map[string]any); ok {
							next = m
						}
					}
					newArr := setSliceIndex(c, seg.index, next)
					if setInParent != nil {
						setInParent(newArr)
					}
					current = next
					setInParent = nil
				}
			} else {
				return fmt.Errorf("cannot use key %q on array", seg.key)
			}
		default:
			return fmt.Errorf("cannot navigate into %T at segment %q", current, seg.key)
		}
	}
	return nil
}

// pathSegment represents a parsed segment of a dot-notation path.
type pathSegment struct {
	key   string
	index int // -1 if not an array index
}

// parsePathSegments parses a dot-notation path into segments,
// handling escaped dots, array index syntax (e.g. "a.b[0].c"),
// and nested array indices (e.g. "a[0][1]").
func parsePathSegments(path string) []pathSegment {
	var segments []pathSegment
	var current strings.Builder
	escaped := false

	for i := 0; i < len(path); i++ {
		c := path[i]
		if escaped {
			current.WriteByte(c)
			escaped = false
			continue
		}
		if c == '\\' {
			escaped = true
			continue
		}
		if c == '.' {
			if current.Len() > 0 {
				segments = append(segments, parseSegmentKeys(current.String())...)
				current.Reset()
			}
			continue
		}
		current.WriteByte(c)
	}
	if current.Len() > 0 {
		segments = append(segments, parseSegmentKeys(current.String())...)
	}
	return segments
}

// parseSegmentKeys parses a single dot-separated token into one or more
// path segments. Handles array indices including nested arrays:
// "key" → [{key:"key", index:-1}]
// "key[0]" → [{key:"key", index:0}]
// "key[0][1]" → [{key:"key", index:0}, {key:"", index:1}]
func parseSegmentKeys(s string) []pathSegment {
	idx := strings.IndexByte(s, '[')
	if idx < 0 {
		return []pathSegment{{key: s, index: -1}}
	}

	var segments []pathSegment
	key := s[:idx]
	rest := s[idx:]

	for len(rest) > 0 && rest[0] == '[' {
		end := strings.IndexByte(rest, ']')
		if end < 2 {
			break
		}
		n, err := strconv.Atoi(rest[1:end])
		if err != nil {
			break
		}
		segments = append(segments, pathSegment{key: key, index: n})
		key = "" // subsequent indices have no key
		rest = rest[end+1:]
	}

	if len(segments) == 0 {
		return []pathSegment{{key: s, index: -1}}
	}
	return segments
}

// setSliceIndex sets the value at the given index in the slice,
// extending the slice with nil values if necessary.
func setSliceIndex(arr []any, index int, value any) []any {
	for len(arr) <= index {
		arr = append(arr, nil)
	}
	arr[index] = value
	return arr
}

// resolveValue parses a raw value string, handling inline list syntax ({a,b,c})
// and backslash escape sequences. Returns the resolved value with type coercion applied.
func resolveValue(value string, forceString bool) any {
	if strings.HasPrefix(value, "{") && strings.HasSuffix(value, "}") {
		inner := value[1 : len(value)-1]
		items := splitEscapedComma(inner)
		list := make([]any, 0, len(items))
		for _, item := range items {
			list = append(list, typedVal(item, forceString))
		}
		return list
	}
	return typedVal(unescapeValue(value), forceString)
}

// typedVal converts a string value to a typed value matching Helm's strvals behavior.
// When forceString is true, the value is always returned as a string.
func typedVal(value string, forceString bool) any {
	if forceString {
		return value
	}
	if strings.EqualFold(value, "true") {
		return true
	}
	if strings.EqualFold(value, "false") {
		return false
	}
	if strings.EqualFold(value, "null") {
		return nil
	}
	if value == "0" {
		return int64(0)
	}
	// Parse as int64, but not if it starts with 0 (to avoid octal interpretation).
	if len(value) > 0 && value[0] != '0' {
		if iv, err := strconv.ParseInt(value, 10, 64); err == nil {
			return iv
		}
	}
	return value
}

// unescapeValue processes backslash escape sequences in a value string.
func unescapeValue(s string) string {
	if !strings.ContainsRune(s, '\\') {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	escaped := false
	for _, r := range s {
		if escaped {
			b.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// splitEscapedComma splits a string by commas, handling backslash escapes.
func splitEscapedComma(s string) []string {
	var result []string
	var current strings.Builder
	escaped := false
	for _, r := range s {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if r == ',' {
			result = append(result, current.String())
			current.Reset()
			continue
		}
		current.WriteRune(r)
	}
	if current.Len() > 0 || len(result) > 0 {
		result = append(result, current.String())
	}
	return result
}
