/*
Copyright 2025.

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

package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

func TestMergeContainerSlices(t *testing.T) {

	tests := []struct {
		name                string
		destSlice           []corev1.Container
		srcSlice            []corev1.Container
		expectedMergedSlice []corev1.Container
		expectError         bool
	}{
		{
			name: "simple append",
			destSlice: []corev1.Container{
				{
					Name: "c1",
				},
			},
			srcSlice: []corev1.Container{
				{
					Name: "c2",
				},
			},
			expectedMergedSlice: []corev1.Container{
				{
					Name: "c1",
				},
				{
					Name: "c2",
				},
			},
			expectError: false,
		},
		{
			name: "with simple override",
			destSlice: []corev1.Container{
				{
					Name:  "c1",
					Image: "dest-image",
				},
			},
			srcSlice: []corev1.Container{
				{
					Name:  "c1", // note name is same as dest
					Image: "src-image",
				},
			},
			expectedMergedSlice: []corev1.Container{
				{
					Name:  "c1",
					Image: "src-image",
				},
			},
			expectError: false,
		},
		{
			name: "with env var overrides",
			destSlice: []corev1.Container{
				{
					Name:  "c1",
					Image: "dest-image",
					Env: []corev1.EnvVar{
						{
							Name:  "e1",
							Value: "e1-val-dest",
						},
					},
				},
			},
			srcSlice: []corev1.Container{
				{
					Name:  "c1", // note name is same as dest
					Image: "src-image",
					Env: []corev1.EnvVar{
						{
							Name:  "e1",
							Value: "e1-val-src",
						},
					},
				},
			},
			expectedMergedSlice: []corev1.Container{
				{
					Name:  "c1",
					Image: "src-image",
					Env: []corev1.EnvVar{
						{
							Name:  "e1",
							Value: "e1-val-src",
						},
					},
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			actualMergedSlice, err := MergeContainerSlices(tt.destSlice, tt.srcSlice)

			if tt.expectError {
				assert.Error(t, err, "expected error but got none")
			} else {
				assert.NoError(t, err)

				// Assert that destSlice matches mergedSlice
				assert.Equal(t, len(tt.expectedMergedSlice), len(actualMergedSlice))

				for i := range len(tt.expectedMergedSlice) {
					expectedContainer := tt.expectedMergedSlice[i]
					actualContainer := actualMergedSlice[i]

					// Assert name
					assert.Equal(t, expectedContainer.Name, actualContainer.Name)

					// Assert image
					assert.Equal(t, expectedContainer.Image, actualContainer.Image)

					// Assert env var list
					assert.Equal(t, len(expectedContainer.Env), len(actualContainer.Env))

					for j, expectedEnvVar := range expectedContainer.Env {
						actualEnvVar := actualContainer.Env[j]
						assert.Equal(t, expectedEnvVar.Name, actualEnvVar.Name)
						assert.Equal(t, expectedEnvVar.Value, actualEnvVar.Value)
						assert.Equal(t, expectedEnvVar.ValueFrom, actualEnvVar.ValueFrom)
					}

					// add more assertions
					// ...
				}

			}
		})
	}

}
