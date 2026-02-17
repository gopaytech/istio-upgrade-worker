package kubernetes

import (
	"testing"
)

func TestDeduplicateNamespaces(t *testing.T) {
	input := []Namespace{
		{Name: "app-a"},
		{Name: "app-b"},
		{Name: "app-a"}, // duplicate
		{Name: "app-c"},
		{Name: "app-b"}, // duplicate
	}

	result := deduplicateNamespaces(input)

	if len(result) != 3 {
		t.Errorf("expected 3 namespaces, got %d", len(result))
	}

	expected := map[string]bool{"app-a": true, "app-b": true, "app-c": true}
	for _, ns := range result {
		if !expected[ns.Name] {
			t.Errorf("unexpected namespace: %s", ns.Name)
		}
		delete(expected, ns.Name)
	}

	if len(expected) != 0 {
		t.Errorf("missing namespaces: %v", expected)
	}
}

func TestDeduplicateNamespaces_Empty(t *testing.T) {
	result := deduplicateNamespaces([]Namespace{})
	if len(result) != 0 {
		t.Errorf("expected 0 namespaces, got %d", len(result))
	}
}

func TestDeduplicateNamespaces_NoDuplicates(t *testing.T) {
	input := []Namespace{
		{Name: "app-a"},
		{Name: "app-b"},
		{Name: "app-c"},
	}

	result := deduplicateNamespaces(input)

	if len(result) != 3 {
		t.Errorf("expected 3 namespaces, got %d", len(result))
	}
}
