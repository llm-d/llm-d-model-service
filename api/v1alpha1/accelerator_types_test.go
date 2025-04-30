package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"

	corev1 "k8s.io/api/core/v1"
)

func TestToNodeAffinity(t *testing.T) {
	tests := []struct {
		name        string
		accelerator AcceleratorTypes
		expectError bool
	}{
		// valid label key and values
		{
			name: "valid accelerator",
			accelerator: AcceleratorTypes{
				LabelKey:    "nvidia.com/gpu.product",
				LabelValues: []string{"A100", "H100"},
			},
			expectError: false,
		},
		// missing LabelKey
		{
			name: "missing label key",
			accelerator: AcceleratorTypes{
				LabelKey:    "",
				LabelValues: []string{"A100"},
			},
			expectError: true,
		},
		// empty LabelValues slice
		{
			name: "empty label values",
			accelerator: AcceleratorTypes{
				LabelKey:    "nvidia.com/gpu.product",
				LabelValues: []string{},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodeAffinity, err := tt.accelerator.ToNodeAffinity()

			if tt.expectError {
				assert.Error(t, err, "expected error but got none")
				assert.Nil(t, nodeAffinity)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, nodeAffinity)
				assert.NotNil(t, nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution)

				// Validate NodeSelectorTerm with correct MatchExpression
				terms := nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms
				assert.Len(t, terms, 1)
				assert.Len(t, terms[0].MatchExpressions, 1)

				expr := terms[0].MatchExpressions[0]
				assert.Equal(t, corev1.NodeSelectorOpIn, expr.Operator)
				assert.Equal(t, tt.accelerator.LabelKey, expr.Key)
				assert.ElementsMatch(t, tt.accelerator.LabelValues, expr.Values)
			}
		})
	}
}
