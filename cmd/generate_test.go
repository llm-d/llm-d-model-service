package cmd

import (
	"context"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("root command", func() {

	Context("controller starts", func() {
		var started bool
		rootCmd.SetArgs([]string{
			"--epp-cluster-role=dummy",
		})
		testStartedHook = func() {
			started = true
		}
		err := rootCmd.Execute()
		Expect(err).ToNot(HaveOccurred())
		Expect(started).To(BeTrue())

	})
})

var _ = Describe("generate command", func() {

	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	Context("simulate call", func() {
		modelServiceYaml := filepath.Join("..", "samples", "test", "msvc.yaml")
		baseConfigYaml := filepath.Join("..", "samples", "test", "baseconfig.yaml")
		rootCmd.SetArgs([]string{
			"--epp-cluster-role=dummy",
			"generate",
			"-m", modelServiceYaml,
			"-b", baseConfigYaml,
		})
		err := rootCmd.Execute()
		Expect(err).ToNot(HaveOccurred())
	})

	Context("call with valid inputs", func() {
		modelServiceYaml := filepath.Join("..", "samples", "test", "msvc.yaml")
		baseConfigYaml := filepath.Join("..", "samples", "test", "baseconfig.yaml")
		It("should generate manifests", func() {
			msvc, err := generateManifests(ctx, modelServiceYaml, baseConfigYaml)
			Expect(err).To(BeNil())
			Expect(msvc).ToNot(BeNil())
		})
	})

	Context("call with invalid modelService filename", func() {
		modelServiceYaml := filepath.Join(".", "invalid")
		baseConfigYaml := filepath.Join("..", "samples", "test", "baseconfig.yaml")
		It("should report an error", func() {
			_, err := generateManifests(ctx, modelServiceYaml, baseConfigYaml)
			Expect(err).ToNot(BeNil())
		})
	})

	Context("call with invalid modelService content", func() {
		modelServiceYaml := filepath.Join("..", "test", "modelservices", "invalidyaml.yaml")
		baseConfigYaml := filepath.Join("..", "samples", "test", "baseconfig.yaml")
		It("should report an error", func() {
			_, err := generateManifests(ctx, modelServiceYaml, baseConfigYaml)
			Expect(err).ToNot(BeNil())
		})
	})

	Context("call with empty baseConfiguration filename", func() {
		modelServiceYaml := filepath.Join("..", "samples", "test", "msvc.yaml")
		baseConfigYaml := ""
		It("should generate manifests", func() {
			msvc, err := generateManifests(ctx, modelServiceYaml, baseConfigYaml)
			Expect(err).To(BeNil())
			Expect(msvc).ToNot(BeNil())
		})
	})

	Context("call with invalid baseConfiguration filename", func() {
		modelServiceYaml := filepath.Join("..", "samples", "test", "msvc.yaml")
		baseConfigYaml := filepath.Join(".", "invalid")
		It("should report an error", func() {
			_, err := generateManifests(ctx, modelServiceYaml, baseConfigYaml)
			Expect(err).ToNot(BeNil())
		})
	})

	Context("call with invalid baseConfiguration content", func() {
		modelServiceYaml := filepath.Join("..", "samples", "test", "msvc.yaml")
		baseConfigYaml := filepath.Join("..", "test", "modelservices", "invalidyaml.yaml")
		It("should report an error", func() {
			_, err := generateManifests(ctx, modelServiceYaml, baseConfigYaml)
			Expect(err).ToNot(BeNil())
		})
	})
})
