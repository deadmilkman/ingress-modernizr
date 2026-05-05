package main

import (
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func obj(kind, name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       kind,
		"metadata": map[string]interface{}{
			"name": name,
		},
	}}
}

func TestReadAllObjectsMultiDocAndEmptyDocs(t *testing.T) {
	input := strings.Join([]string{
		"apiVersion: v1",
		"kind: ConfigMap",
		"metadata:",
		"  name: cm-one",
		"---",
		"",
		"---",
		"apiVersion: networking.k8s.io/v1",
		"kind: Ingress",
		"metadata:",
		"  name: ing-one",
		"",
	}, "\n")

	objs, err := readAllObjects(strings.NewReader(input))
	if err != nil {
		t.Fatalf("readAllObjects() returned error: %v", err)
	}

	if got, want := len(objs), 2; got != want {
		t.Fatalf("readAllObjects() length = %d, want %d", got, want)
	}

	if got, want := objs[0].GetKind(), "ConfigMap"; got != want {
		t.Fatalf("first kind = %q, want %q", got, want)
	}

	if got, want := objs[1].GetKind(), "Ingress"; got != want {
		t.Fatalf("second kind = %q, want %q", got, want)
	}
}

func TestReadAllObjectsMalformedYAML(t *testing.T) {
	input := "apiVersion: v1\nkind: ConfigMap\nmetadata: [oops\n"

	_, err := readAllObjects(strings.NewReader(input))
	if err == nil {
		t.Fatalf("readAllObjects() error = nil, want non-nil")
	}
}

func TestBuildFinalObjectsDropsIngressKeepsOthersAndAppendsConverted(t *testing.T) {
	original := []*unstructured.Unstructured{
		obj("Service", "svc"),
		obj("Ingress", "ing"),
		obj("Deployment", "dep"),
	}
	converted := []*unstructured.Unstructured{
		obj("Gateway", "gw"),
		obj("HTTPRoute", "route"),
	}

	final := buildFinalObjects(original, converted)

	if got, want := len(final), 4; got != want {
		t.Fatalf("final length = %d, want %d", got, want)
	}

	kinds := []string{
		final[0].GetKind(),
		final[1].GetKind(),
		final[2].GetKind(),
		final[3].GetKind(),
	}
	wantKinds := []string{"Service", "Deployment", "Gateway", "HTTPRoute"}
	for i := range wantKinds {
		if kinds[i] != wantKinds[i] {
			t.Fatalf("kind[%d] = %q, want %q", i, kinds[i], wantKinds[i])
		}
	}
}
