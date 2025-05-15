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
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	msv1alpha1 "github.com/neuralmagic/llm-d-model-service/api/v1alpha1"
)

const PVC_NAME = "my-pvc"
const MODEL_PATH = "path/to/model"

var _ = Describe("Model Artifacts", func() {
	Context("Given a model artifact with an invalid URI prefix", func() {
		modelArtifact := msv1alpha1.ModelArtifacts{
			URI: fmt.Sprintf("nothing://%s/%s", PVC_NAME, MODEL_PATH),
		}

		It("should parse correctly", func() {
			By("checking type of uri")
			Expect(isPVCURI(modelArtifact.URI)).To(BeFalse())
			Expect(isHFURI(modelArtifact.URI)).To(BeFalse())

			By("Parsing uri should fail")
			_, err := parsePVCURI(&modelArtifact)
			Expect(err).NotTo(BeNil())
		})
	})

	Context("Given an URI string", func() {
		It("should determine the type of the URI correctly", func() {
			tests := map[string]URIType{
				"pvc://pvc-name/path/to/model":       PVC,
				"oci://repo-with-tag::path/to/model": OCI,
				"hf://repo-id/model-id":              HF,
				"pvc://pvc-name":                     PVC,
				"oci://":                             OCI,
				"hf://wrong":                         HF,
				"random://":                          UnknownURI,
				"":                                   UnknownURI,
				"PVC://":                             UnknownURI,
				"HF://":                              UnknownURI,
				"OCI://":                             UnknownURI,
			}

			for uri, expectedURIType := range tests {
				actualURIType := UriType(uri)
				Expect(actualURIType).To(Equal(expectedURIType))
			}

		})
	})

	Context("Given a model artifact with a valid PVC URI", func() {
		ctx := context.Background()
		modelArtifact := msv1alpha1.ModelArtifacts{
			URI: fmt.Sprintf("pvc://%s/%s", PVC_NAME, MODEL_PATH),
		}

		modelService := msv1alpha1.ModelService{
			Spec: msv1alpha1.ModelServiceSpec{
				ModelArtifacts: modelArtifact,
			},
		}

		It("should parse correctly", func() {
			By("checking type of uri")
			Expect(isPVCURI(modelArtifact.URI)).To(BeTrue())
			Expect(isHFURI(modelArtifact.URI)).To(BeFalse())

			By("Parsing uri should be successful")
			parts, err := parsePVCURI(&modelArtifact)
			Expect(err).To(BeNil())
			Expect(len(parts) > 1).To(BeTrue())
			Expect(parts[0]).To(Equal(PVC_NAME))
			Expect(strings.Join(parts[1:], "/")).To(Equal(MODEL_PATH))
		})
		It("should produce a valid volumeMounts list", func() {
			volumeMounts := getVolumeMountsForContainer(ctx, &modelService)
			Expect(len(volumeMounts)).To(Equal(1))
			firstVolumeMount := volumeMounts[0]

			Expect(firstVolumeMount.Name).To(Equal(modelStorageVolumeName))
			Expect(firstVolumeMount.MountPath).To(Equal(modelStorageRoot))
			Expect(firstVolumeMount.ReadOnly).To(BeTrue())
		})
		It("should produce a valid volumes list", func() {
			volumes := getVolumeForPDDeployment(ctx, &modelService)
			Expect(len(volumes)).To(Equal(1))
			firstVolume := volumes[0]
			Expect(firstVolume.Name).To(Equal(modelStorageVolumeName))
			Expect(firstVolume.PersistentVolumeClaim.ClaimName).To(Equal(PVC_NAME))
			Expect(firstVolume.PersistentVolumeClaim.ReadOnly).To(BeTrue())
		})
	})
	Context("Given a model artifact with a valid HF URI", func() {
		modelArtifact := msv1alpha1.ModelArtifacts{
			URI: fmt.Sprintf("hf://%s/%s", "repo", "model"),
		}
		It("should parse correctly", func() {
			By("checking type of uri")
			Expect(isPVCURI(modelArtifact.URI)).To(BeFalse())
			Expect(isHFURI(modelArtifact.URI)).To(BeTrue())
		})
	})
})
