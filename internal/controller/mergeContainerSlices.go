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
	"reflect"
	"slices"

	"dario.cat/mergo"
	corev1 "k8s.io/api/core/v1"
)

type envVarSliceTransformer struct{}

// Transformer for the []corev1.Env
func (e envVarSliceTransformer) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {

	if typ == reflect.TypeOf([]corev1.EnvVar{}) {
		return func(dst, src reflect.Value) error {
			// Reject transforming anything other than slices
			if dst.Kind() != reflect.Slice || src.Kind() != reflect.Slice {
				return nil
			}

			srcEnvVarSlice := src.Interface().([]corev1.EnvVar)
			dstEnvVarSlice := dst.Interface().([]corev1.EnvVar)

			// keep track of the common envVar among src and dst
			srcEnvVarNames := make(map[string]corev1.EnvVar, len(srcEnvVarSlice))
			commonEnvVarNames := []string{}

			for _, envVar := range srcEnvVarSlice {
				srcEnvVarNames[envVar.Name] = envVar
			}

			for _, dstEnvVar := range dstEnvVarSlice {
				if _, found := srcEnvVarNames[dstEnvVar.Name]; found {
					commonEnvVarNames = append(commonEnvVarNames, dstEnvVar.Name)
				}
			}

			// now loop over dstEnvVarSlice and see if there is a EnvVar with same name in src
			for i, dstEnvVar := range dstEnvVarSlice {
				dstEnvVarName := dstEnvVar.Name

				// Found a matching src EnvVar with same name
				if srcEnvVar, found := srcEnvVarNames[dstEnvVarName]; found {
					// Set new value and valueFrom from src
					err := mergo.Merge(&dstEnvVar, srcEnvVar, mergo.WithOverride)
					if err != nil {
						return err
					}

					dstEnvVarSlice[i] = dstEnvVar
				}
			}

			// Construct the mergedEnvVarSlice combining both src and dst
			mergedEnvVarSlice := []corev1.EnvVar{}

			// mergedEnvVarSlice contains everything already present in dst to begin with,
			// with the common EnvVar already merged from src
			mergedEnvVarSlice = append(mergedEnvVarSlice, dstEnvVarSlice...)

			// for other src EnvVar, skip the ones that are common
			for _, srcEnvVar := range srcEnvVarSlice {
				if !slices.Contains(commonEnvVarNames, srcEnvVar.Name) {
					mergedEnvVarSlice = append(mergedEnvVarSlice, srcEnvVar)
				}
			}

			// Now rewrite dst with mergedEnvVarSlice
			dst.Set(reflect.ValueOf(mergedEnvVarSlice))
			return nil
		}
	}
	return nil
}

type containerSliceTransformer struct{}

// Transformer merges two []corev1.Container based on their Name, and applies
// transformers for each Container.Spec fields
func (c containerSliceTransformer) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {

	if typ == reflect.TypeOf([]corev1.Container{}) {
		return func(dst, src reflect.Value) error {
			// Reject transforming anything other than slices
			if dst.Kind() != reflect.Slice || src.Kind() != reflect.Slice {
				return nil
			}

			srcContainerSlice := src.Interface().([]corev1.Container)
			dstContainerSlice := dst.Interface().([]corev1.Container)

			// keep track of the common containers among src and dst
			srcContainerNames := make(map[string]corev1.Container, len(srcContainerSlice))
			commonContainerNames := []string{}

			for _, container := range srcContainerSlice {
				srcContainerNames[container.Name] = container
			}

			for _, dstContainer := range dstContainerSlice {
				if _, found := srcContainerNames[dstContainer.Name]; found {
					commonContainerNames = append(commonContainerNames, dstContainer.Name)
				}
			}

			// now loop over dstContainerSlice and see if there is a container with same name in src
			for i, dstContainer := range dstContainerSlice {
				dstContainerName := dstContainer.Name

				// Found a matching src container with same name
				if srcContainer, found := srcContainerNames[dstContainerName]; found {
					// TODO: update this
					err := mergo.Merge(
						&dstContainer,
						srcContainer,
						mergo.WithAppendSlice,
						mergo.WithOverride,
						mergo.WithTransformers(envVarSliceTransformer{}),
					)

					if err != nil {
						return err
					}

					dstContainerSlice[i] = dstContainer
				}
			}

			// Construct the mergedContainerSlice combining both src and dst
			mergedContainerSlice := []corev1.Container{}

			// mergedContainerSlice contains everything already present in dst to begin with,
			// with the common containers already merged from src
			mergedContainerSlice = append(mergedContainerSlice, dstContainerSlice...)

			// for other src containers, skip the ones that are common
			for _, srcContainer := range srcContainerSlice {
				if !slices.Contains(commonContainerNames, srcContainer.Name) {
					mergedContainerSlice = append(mergedContainerSlice, srcContainer)
				}
			}

			// Now rewrite dst with mergedContainerSlice
			dst.Set(reflect.ValueOf(mergedContainerSlice))
			return nil
		}
	}
	return nil
}

// MergeContainerSlices merges src slice into dest in place
func MergeContainerSlices(dest, src []corev1.Container) ([]corev1.Container, error) {
	err := mergo.Merge(&dest, src, mergo.WithTransformers(containerSliceTransformer{}))

	if err != nil {
		return []corev1.Container{}, err
	}

	return dest, err
}
