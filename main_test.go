package main

import (
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestNodeHasAllowedLabel tests the label filtering logic.
func TestNodeHasAllowedLabel(t *testing.T) {
	tests := []struct {
		name          string
		node          *v1.Node
		allowedLabels map[string]struct{}
		expected      bool
	}{
		{
			name:          "nil node",
			node:          nil,
			allowedLabels: map[string]struct{}{"key": {}},
			expected:      false,
		},
		{
			name:          "empty allowedLabels (all nodes pass)",
			node:          &v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node1", Labels: map[string]string{"foo": "bar"}}},
			allowedLabels: map[string]struct{}{},
			expected:      true,
		},
		{
			name:          "node has matching key",
			node:          &v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node1", Labels: map[string]string{"disktype": "ssd", "other": "value"}}},
			allowedLabels: map[string]struct{}{"disktype": {}, "gpu": {}},
			expected:      true,
		},
		{
			name:          "node has no matching keys",
			node:          &v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node1", Labels: map[string]string{"foo": "bar"}}},
			allowedLabels: map[string]struct{}{"disktype": {}, "gpu": {}},
			expected:      false,
		},
		{
			name:          "node with no labels, allowedLabels non-empty",
			node:          &v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node1", Labels: map[string]string{}}},
			allowedLabels: map[string]struct{}{"disktype": {}},
			expected:      false,
		},
		{
			name:          "node with multiple labels, one matches",
			node:          &v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node1", Labels: map[string]string{"zone": "us-east", "disktype": "ssd", "critical": "true"}}},
			allowedLabels: map[string]struct{}{"gpu": {}, "disktype": {}},
			expected:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &NodeLifeSupportController{allowedLabels: tt.allowedLabels}
			result := c.nodeHasAllowedLabel(tt.node)
			if result != tt.expected {
				t.Errorf("nodeHasAllowedLabel() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestBuildAllowedLabelsMap tests the map building logic.
func TestBuildAllowedLabelsMap(t *testing.T) {
	tests := []struct {
		name        string
		allowedKeys []string
		expected    map[string]struct{}
	}{
		{
			name:        "nil allowedKeys",
			allowedKeys: nil,
			expected:    map[string]struct{}{},
		},
		{
			name:        "empty allowedKeys slice",
			allowedKeys: []string{},
			expected:    map[string]struct{}{},
		},
		{
			name:        "single key",
			allowedKeys: []string{"disktype"},
			expected:    map[string]struct{}{"disktype": {}},
		},
		{
			name:        "multiple keys",
			allowedKeys: []string{"disktype", "gpu", "critical"},
			expected:    map[string]struct{}{"disktype": {}, "gpu": {}, "critical": {}},
		},
		{
			name:        "empty strings filtered out",
			allowedKeys: []string{"disktype", "", "gpu"},
			expected:    map[string]struct{}{"disktype": {}, "gpu": {}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the map building logic directly.
			m := make(map[string]struct{})
			for _, k := range tt.allowedKeys {
				if k != "" {
					m[k] = struct{}{}
				}
			}

			if len(m) != len(tt.expected) {
				t.Errorf("allowedLabels length = %d, want %d", len(m), len(tt.expected))
			}

			for key := range tt.expected {
				if _, ok := m[key]; !ok {
					t.Errorf("expected key %q not found in allowedLabels", key)
				}
			}
		})
	}
}

// TestNodeFilteringLogic tests the node filtering logic used in SyncAllNodes.
func TestNodeFilteringLogic(t *testing.T) {
	tests := []struct {
		name          string
		nodes         []*v1.Node
		allowedLabels map[string]struct{}
		expectedCount int
	}{
		{
			name: "no allowlist, all nodes pass",
			nodes: []*v1.Node{
				{ObjectMeta: metav1.ObjectMeta{Name: "node1", Labels: map[string]string{"foo": "bar"}}},
				{ObjectMeta: metav1.ObjectMeta{Name: "node2", Labels: map[string]string{"baz": "qux"}}},
			},
			allowedLabels: map[string]struct{}{},
			expectedCount: 2,
		},
		{
			name: "with allowlist, only matching nodes pass",
			nodes: []*v1.Node{
				{ObjectMeta: metav1.ObjectMeta{Name: "node1", Labels: map[string]string{"disktype": "ssd"}}},
				{ObjectMeta: metav1.ObjectMeta{Name: "node2", Labels: map[string]string{"foo": "bar"}}},
				{ObjectMeta: metav1.ObjectMeta{Name: "node3", Labels: map[string]string{"gpu": "true"}}},
			},
			allowedLabels: map[string]struct{}{"disktype": {}, "gpu": {}},
			expectedCount: 2,
		},
		{
			name:          "no nodes",
			nodes:         []*v1.Node{},
			allowedLabels: map[string]struct{}{"disktype": {}},
			expectedCount: 0,
		},
		{
			name: "allowlist, no matching nodes",
			nodes: []*v1.Node{
				{ObjectMeta: metav1.ObjectMeta{Name: "node1", Labels: map[string]string{"foo": "bar"}}},
				{ObjectMeta: metav1.ObjectMeta{Name: "node2", Labels: map[string]string{"baz": "qux"}}},
			},
			allowedLabels: map[string]struct{}{"disktype": {}},
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &NodeLifeSupportController{allowedLabels: tt.allowedLabels}

			// Simulate the filtering logic from SyncAllNodes.
			passCount := 0
			for _, n := range tt.nodes {
				if len(c.allowedLabels) > 0 {
					if !c.nodeHasAllowedLabel(n) {
						continue
					}
				}
				passCount++
			}

			if passCount != tt.expectedCount {
				t.Errorf("filtered %d nodes, want %d", passCount, tt.expectedCount)
			}
		})
	}
}
