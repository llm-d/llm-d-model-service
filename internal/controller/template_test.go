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
	"context"
	"fmt"
	"testing"

	msv1alpha1 "github.com/llm-d/llm-d-model-service/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const msvcName = "msvc-test"
const msvcNamespace = "default"
const modelName = "modelName"
const sanitizedModelName = "modelname"
const pvcName = "pvc-name"
const modelPath = "path/to/" + modelName
const mountedModelPathInVolume = modelStorageRoot + pathSep + modelPath
const pvcURI = "pvc://" + pvcName + "/" + modelPath
const hfModelName = pvcName + "/" + modelName
const hfURI = "hf://" + hfModelName
const authSecretName = "hf-secret"

var authSecretNameCopy = authSecretName
var authSecretNamePtr = &authSecretNameCopy // ugly workaround ModelArtifacts.AuthSecretName is *strings

// returns a minimal valid msvc
func minimalMSVC() *msv1alpha1.ModelService {
	return &msv1alpha1.ModelService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      msvcName,
			Namespace: msvcNamespace,
		},
		Spec: msv1alpha1.ModelServiceSpec{
			Routing: msv1alpha1.Routing{
				ModelName: modelName,
			},
			ModelArtifacts: msv1alpha1.ModelArtifacts{
				URI:            pvcURI,
				AuthSecretName: authSecretNamePtr,
			},
		},
	}
}

// createMSVCWithPDSpec creates a minimal msvc with the appropriate decode
func createMSVCWithDecode(decodeSpec *msv1alpha1.PDSpec) *msv1alpha1.ModelService {

	minimalMSVC := minimalMSVC()
	minimalMSVC.Spec.Decode = decodeSpec
	return minimalMSVC
}

func TestTemplateVars(t *testing.T) {
	// Test that each template var can be interpolated in MSVC

	tests := map[string]struct {
		expectedValue string
		uri           string
	}{
		"ModelServiceName": {
			expectedValue: msvcName,
		},
		"ModelServiceNamespace": {
			expectedValue: msvcNamespace,
		},
		"ModelName": {
			expectedValue: modelName,
		},
		"HFModelName": {
			expectedValue: hfModelName,
			uri:           hfURI,
		},
		"SanitizedModelName": {
			expectedValue: sanitizedModelName,
		},
		"ModelPath": {
			expectedValue: modelPath,
		},
		"MountedModelPath": {
			expectedValue: mountedModelPathInVolume,
		},
		"AuthSecretName": {
			expectedValue: authSecretName,
		},
		"EPPServiceName": {
			expectedValue: msvcName + "-epp-service",
		},
		"EPPDeploymentName": {
			expectedValue: msvcName + "-epp",
		},
		"PrefillDeploymentName": {
			expectedValue: msvcName + "-prefill",
		},
		"DecodeDeploymentName": {
			expectedValue: msvcName + "-decode",
		},
		"PrefillServiceName": {
			expectedValue: msvcName + "-service-prefill",
		},
		"DecodeServiceName": {
			expectedValue: msvcName + "-service-decode",
		},
		"InferencePoolName": {
			expectedValue: msvcName + "-inference-pool",
		},
		"InferenceModelName": {
			expectedValue: msvcName,
		},
	}

	for templateVar, testCase := range tests {
		ctx := context.Background()

		minimalMSVC := createMSVCWithDecode(&msv1alpha1.PDSpec{
			ModelServicePodSpec: msv1alpha1.ModelServicePodSpec{
				Containers: []msv1alpha1.ContainerSpec{
					{
						Args: []string{
							// This becomes, for example, {{ .ModelService }}
							fmt.Sprintf("{{ .%s }}", templateVar),
						},
					},
				},
			},
		})

		if testCase.uri != "" {
			minimalMSVC.Spec.ModelArtifacts.URI = testCase.uri
		}

		interpolatedMSVC, err := InterpolateModelService(ctx, minimalMSVC)
		assert.NoError(t, err, "got error but expected none")

		// Assert that the template var is interpolated and the expected values match
		// Check that Args[0] matches
		actualValue := interpolatedMSVC.Spec.Decode.Containers[0].Args[0]
		assert.Equal(t, testCase.expectedValue, actualValue, fmt.Sprintf("%s should be interpolated", templateVar))
	}
}

func TestMSVCInterpolation(t *testing.T) {

	tests := []struct {
		name         string
		originalMSVC *msv1alpha1.ModelService
		expectedMSVC *msv1alpha1.ModelService
		expectError  bool
	}{
		{
			name:         "no interpolation required should pass",
			originalMSVC: minimalMSVC(),
			expectedMSVC: minimalMSVC(),
			expectError:  false,
		},
		{
			name: "one interpolation required in args should pass",
			originalMSVC: createMSVCWithDecode(&msv1alpha1.PDSpec{
				ModelServicePodSpec: msv1alpha1.ModelServicePodSpec{
					Containers: []msv1alpha1.ContainerSpec{
						{
							Args: []string{
								"{{ .ModelPath }}",
							},
						},
					},
				},
			}),
			expectedMSVC: createMSVCWithDecode(&msv1alpha1.PDSpec{
				ModelServicePodSpec: msv1alpha1.ModelServicePodSpec{
					Containers: []msv1alpha1.ContainerSpec{
						{
							Args: []string{
								modelPath,
							},
						},
					},
				},
			}),
			expectError: false,
		},
		{
			name: "1+ interpolation required in args should pass",
			originalMSVC: createMSVCWithDecode(&msv1alpha1.PDSpec{
				ModelServicePodSpec: msv1alpha1.ModelServicePodSpec{
					Containers: []msv1alpha1.ContainerSpec{
						{
							Args: []string{
								"{{ .ModelPath }}",
								"--arg2",
								"{{ .DecodeDeploymentName }}",
							},
						},
					},
				},
			}),
			expectedMSVC: createMSVCWithDecode(&msv1alpha1.PDSpec{
				ModelServicePodSpec: msv1alpha1.ModelServicePodSpec{
					Containers: []msv1alpha1.ContainerSpec{
						{
							Args: []string{
								modelPath,
								"--arg2",
								msvcName + "-decode",
							},
						},
					},
				},
			}),
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			interpolatedMSVC, err := InterpolateModelService(ctx, tt.originalMSVC)

			if tt.expectError {
				assert.Error(t, err, "expected error but got none")
			} else {
				assert.NoError(t, err)

				// Assert that expected args matches interpolated args
				if tt.expectedMSVC.Spec.Decode != nil && interpolatedMSVC.Spec.Decode != nil {
					expectedContainers := tt.expectedMSVC.Spec.Decode.Containers
					interpolatedContainers := interpolatedMSVC.Spec.Decode.Containers

					assert.Equal(t, len(expectedContainers), len(interpolatedContainers), "container lengths don't match")

					for i := range len(expectedContainers) {
						expectedContainer := expectedContainers[i]
						interpolatedContainer := interpolatedContainers[i]

						// assert args match
						assertEqualSlices(t, expectedContainer.Args, interpolatedContainer.Args)
					}
				} else if tt.expectedMSVC.Spec.Decode == nil && interpolatedMSVC.Spec.Decode == nil {
					// both decode specs are nil, pass
				} else {
					assert.Fail(t, fmt.Sprintf("decode specs don't match\ngot: %v\nwant:%v", interpolatedMSVC.Spec.Decode, tt.expectedMSVC.Spec.Decode))
				}

			}
		})
	}
}
