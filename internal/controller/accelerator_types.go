package controller

import (
	"fmt"

	"github.com/neuralmagic/llm-d-model-service/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// ToNodeAffinity generates a NodeAffinity rule that requires nodes to match
// the specified accelerator label key and one of the allowed values.
//
// Returns an error if LabelKey is empty or LabelValues is empty.
func AcceleratorTypesToNodeAffinity(a *v1alpha1.AcceleratorTypes) (*corev1.NodeAffinity, error) {
	if a == nil {
		return nil, nil
	}
	if a.LabelKey == "" {
		return nil, fmt.Errorf("LabelKey must not be empty")
	}
	if len(a.LabelValues) == 0 {
		return nil, fmt.Errorf("LabelValues must contain at least one value")
	}

	// Construct the node affinity rule
	nodeAffinity := &corev1.NodeAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
			NodeSelectorTerms: []corev1.NodeSelectorTerm{
				{
					MatchExpressions: []corev1.NodeSelectorRequirement{
						{
							Key:      a.LabelKey,
							Operator: corev1.NodeSelectorOpIn,
							Values:   a.LabelValues,
						},
					},
				},
			},
		},
	}

	return nodeAffinity, nil
}
