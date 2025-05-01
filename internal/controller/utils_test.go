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
	Context("Given a model artifact with a valid PVC URI", func() {
		modelArtifact := msv1alpha1.ModelArtifacts{
			URI: fmt.Sprintf("pvc://%s/%s", PVC_NAME, MODEL_PATH),
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
		It("should produce a valid volumeMount", func() {
			volumeMount, err := getVolumeMountFromModelArtifacts(&modelArtifact)
			Expect(err).To(BeNil())
			Expect(volumeMount.Name).To(Equal(modelStorageVolumeName))
			Expect(volumeMount.MountPath).To(Equal(modelStorageRoot))
			Expect(volumeMount.ReadOnly).To(BeTrue())
		})
		It("should produce a valid volume", func() {
			volume, err := getVolumeFromModelArtifacts(&modelArtifact)
			Expect(err).To(BeNil())
			Expect(volume.Name).To(Equal(modelStorageVolumeName))
			Expect(volume.PersistentVolumeClaim.ClaimName).To(Equal(PVC_NAME))
			Expect(volume.PersistentVolumeClaim.ReadOnly).To(BeTrue())
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
