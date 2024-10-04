// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package csi

import (
	"testing"

	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestGetVolumeTypeMapping(t *testing.T) {
	// Test a bad key; the remaining tests all use a good key.
	_, err := getVolumeTypeMapping(map[string]string{"foo": "node,type=bar"})
	assert.ErrorContains(t, err, "not found")

	for _, testCase := range []struct {
		name          string
		input         string
		expected      map[string]volumeTypeInfo
		expectedError bool
	}{
		{
			name:     "empty",
			input:    "",
			expected: map[string]volumeTypeInfo{},
		},
		{
			name:     "empty space",
			input:    " ",
			expected: map[string]volumeTypeInfo{},
		},
		{
			name:     "empty lines",
			input:    " \n  \n \n\n  \n",
			expected: map[string]volumeTypeInfo{},
		},
		{
			name:  "one item",
			input: "node,type=foo",
			expected: map[string]volumeTypeInfo{
				"node": {
					VolumeType: "foo",
				},
			},
		},
		{
			name:  "one item, spaces",
			input: "node, type = foo",
			expected: map[string]volumeTypeInfo{
				"node": {
					VolumeType: "foo",
				},
			},
		},
		{
			name:          "one item, extra comma",
			input:         "node, type = foo,",
			expectedError: true,
		},
		{
			name:  "one item, size",
			input: "node, type=foo, size=10Mi",
			expected: map[string]volumeTypeInfo{
				"node": {
					VolumeType: "foo",
					Size:       resource.MustParse("10Mi"),
				},
			},
		},
		{
			name:          "one item, bad param",
			input:         "node, type=foo, unknown=yes",
			expectedError: true,
		},
		{
			name:  "two items",
			input: "node-a, type=foo, size=10Mi\nnode-b, type=bar",
			expected: map[string]volumeTypeInfo{
				"node-a": {
					VolumeType: "foo",
					Size:       resource.MustParse("10Mi"),
				},
				"node-b": {
					VolumeType: "bar",
				},
			},
		},
		{
			name:          "two items, one bad",
			input:         "node-b, unknown=true,node, type=foo, size=10Mi",
			expectedError: true,
		},
		{
			name:          "repeated item, one bad",
			input:         "node-a,type=A\nnode-b,type=B,node-a,type=C",
			expectedError: true,
		},
		{
			name:  "two items, blank lines",
			input: "\nnode-a, type=foo, size=10Mi\n\nnode-b, type=bar\n\n",
			expected: map[string]volumeTypeInfo{
				"node-a": {
					VolumeType: "foo",
					Size:       resource.MustParse("10Mi"),
				},
				"node-b": {
					VolumeType: "bar",
				},
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			output, err := getVolumeTypeMapping(map[string]string{volumeTypeInfoKey: testCase.input})
			if testCase.expectedError {
				assert.ErrorContains(t, err, "")
			} else {
				assert.NilError(t, err)
				assert.DeepEqual(t, output, testCase.expected)
			}
		})
	}
}

func TestWriteVolumeTypeMapping(t *testing.T) {
	output := map[string]string{}
	err := writeVolumeTypeMapping(output, map[string]volumeTypeInfo{
		"a": {VolumeType: "foo"},
		"b": {VolumeType: "bar", Size: resource.MustParse("10Mi")},
		"c": {VolumeType: "pd", Size: resource.MustParse("10Gi"), Disk: "foobar"},
	})
	assert.NilError(t, err)
	assert.Equal(t, output[volumeTypeInfoKey], "a,type=foo\nb,type=bar,size=10Mi\nc,type=pd,size=10Gi,disk=foobar")
}

func TestGetVolumeTypeFromNode(t *testing.T) {
	for _, testCase := range []struct {
		name          string
		labels        map[string]string
		expected      volumeTypeInfo
		expectedError string
	}{
		{
			name:          "no labels",
			expectedError: "not found",
		},
		{
			name:          "bad labels",
			labels:        map[string]string{"some-label": "some value"},
			expectedError: "not found",
		},
		{
			name:     "type",
			labels:   map[string]string{"node-cache.gke.io": "foo"},
			expected: volumeTypeInfo{VolumeType: "foo"},
		},
		{
			name: "size",
			labels: map[string]string{
				"node-cache.gke.io":          "foo",
				"node-cache-size-mib.gke.io": "10",
			},
			expected: volumeTypeInfo{VolumeType: "foo", Size: resource.MustParse("10Mi")},
		},
		{
			name: "bad size",
			labels: map[string]string{
				"node-cache.gke.io":          "foo",
				"node-cache-size-mib.gke.io": "ten",
			},
			expectedError: "bad MiB size",
		},
		{
			name: "only size",
			labels: map[string]string{
				"node-cache-size-mib.gke.io": "10",
			},
			expectedError: "not found",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			var node corev1.Node
			node.SetLabels(testCase.labels)
			info, err := getVolumeTypeFromNode(&node)
			if testCase.expectedError != "" {
				assert.ErrorContains(t, err, testCase.expectedError)
			} else {
				assert.NilError(t, err)
				assert.DeepEqual(t, info, testCase.expected)
			}
		})
	}
}
