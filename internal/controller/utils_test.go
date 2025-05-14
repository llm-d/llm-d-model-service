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

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"

	msv1alpha1 "github.com/neuralmagic/llm-d-model-service/api/v1alpha1"
)

const PVC_NAME = "my-pvc"
const MODEL_PATH = "path/to/model"

var _ = ginkgo.Describe("Model Artifacts", func() {
	ginkgo.Context("Given a model artifact with an invalid URI prefix", func() {
		modelArtifact := msv1alpha1.ModelArtifacts{
			URI: fmt.Sprintf("nothing://%s/%s", PVC_NAME, MODEL_PATH),
		}
		ginkgo.It("should parse correctly", func() {
			ginkgo.By("checking type of uri")
			gomega.Expect(isPVCURI(modelArtifact.URI)).To(gomega.BeFalse())
			gomega.Expect(isHFURI(modelArtifact.URI)).To(gomega.BeFalse())

			ginkgo.By("Parsing uri should fail")
			_, err := parsePVCURI(&modelArtifact)
			gomega.Expect(err).NotTo(gomega.BeNil())
		})
	})
	ginkgo.Context("Given a model artifact with a valid PVC URI", func() {
		modelArtifact := msv1alpha1.ModelArtifacts{
			URI: fmt.Sprintf("pvc://%s/%s", PVC_NAME, MODEL_PATH),
		}
		ginkgo.It("should parse correctly", func() {
			ginkgo.By("checking type of uri")
			gomega.Expect(isPVCURI(modelArtifact.URI)).To(gomega.BeTrue())
			gomega.Expect(isHFURI(modelArtifact.URI)).To(gomega.BeFalse())

			ginkgo.By("Parsing uri should be successful")
			parts, err := parsePVCURI(&modelArtifact)
			gomega.Expect(err).To(gomega.BeNil())
			gomega.Expect(len(parts) > 1).To(gomega.BeTrue())
			gomega.Expect(parts[0]).To(gomega.Equal(PVC_NAME))
			gomega.Expect(strings.Join(parts[1:], "/")).To(gomega.Equal(MODEL_PATH))
		})
		// ginkgo.It("should produce a valid volumeMount", func() {
		// 	volumeMount, err := getVolumeMountFromModelArtifacts(&modelArtifact)
		// 	gomega.Expect(err).To(gomega.BeNil())
		// 	gomega.Expect(volumeMount.Name).To(gomega.Equal(modelStorageVolumeName))
		// 	gomega.Expect(volumeMount.MountPath).To(gomega.Equal(modelStorageRoot))
		// 	gomega.Expect(volumeMount.ReadOnly).To(gomega.BeTrue())
		// })
		// ginkgo.It("should produce a valid volume", func() {
		// 	volume, err := getVolumeFromModelService(&modelArtifact)
		// 	gomega.Expect(err).To(gomega.BeNil())
		// 	gomega.Expect(volume.Name).To(gomega.Equal(modelStorageVolumeName))
		// 	gomega.Expect(volume.PersistentVolumeClaim.ClaimName).To(gomega.Equal(PVC_NAME))
		// 	gomega.Expect(volume.PersistentVolumeClaim.ReadOnly).To(gomega.BeTrue())
		// })
	})
	ginkgo.Context("Given a model artifact with a valid HF URI", func() {
		modelArtifact := msv1alpha1.ModelArtifacts{
			URI: fmt.Sprintf("hf://%s/%s", "repo", "model"),
		}
		ginkgo.It("should parse correctly", func() {
			ginkgo.By("checking type of uri")
			gomega.Expect(isPVCURI(modelArtifact.URI)).To(gomega.BeFalse())
			gomega.Expect(isHFURI(modelArtifact.URI)).To(gomega.BeTrue())
		})
	})
})
