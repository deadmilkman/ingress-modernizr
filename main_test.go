package main

import (
	"os"
	"path/filepath"
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

func TestRunIngress2GatewayUsesOverrideAndForwardsArgs(t *testing.T) {
	tmpDir := t.TempDir()
	argsFile := filepath.Join(tmpDir, "args.txt")
	inputFile := filepath.Join(tmpDir, "input.yaml")

	t.Setenv("INGRESS2GATEWAY_BIN", filepath.Join("scripts", "test", "fake-ingress2gateway.sh"))
	t.Setenv("ING2GW_ARGS_FILE", argsFile)
	t.Setenv("ING2GW_INPUT_FILE", inputFile)

	original := []*unstructured.Unstructured{
		obj("Service", "svc"),
		obj("Ingress", "ing"),
	}

	converted, err := runIngress2Gateway(original, []string{"print", "--providers=ingress-nginx", "--namespace=apps"})
	if err != nil {
		t.Fatalf("runIngress2Gateway() returned error: %v", err)
	}

	if got, want := len(converted), 1; got != want {
		t.Fatalf("converted length = %d, want %d", got, want)
	}
	if got, want := converted[0].GetKind(), "HTTPRoute"; got != want {
		t.Fatalf("converted kind = %q, want %q", got, want)
	}

	argsData, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("os.ReadFile(argsFile) error: %v", err)
	}
	argsLines := strings.Split(strings.TrimSpace(string(argsData)), "\n")

	if len(argsLines) < 5 {
		t.Fatalf("args lines length = %d, want at least 5 (%v)", len(argsLines), argsLines)
	}

	if got, want := argsLines[0], "print"; got != want {
		t.Fatalf("args[0] = %q, want %q", got, want)
	}
	if got, want := argsLines[1], "--input-file"; got != want {
		t.Fatalf("args[1] = %q, want %q", got, want)
	}

	printCount := 0
	for _, a := range argsLines {
		if a == "print" {
			printCount++
		}
	}
	if got, want := printCount, 1; got != want {
		t.Fatalf("print occurrences = %d, want %d", got, want)
	}

	if got, want := argsLines[3], "--providers=ingress-nginx"; got != want {
		t.Fatalf("providers arg = %q, want %q", got, want)
	}
	if got, want := argsLines[4], "--namespace=apps"; got != want {
		t.Fatalf("namespace arg = %q, want %q", got, want)
	}

	inputData, err := os.ReadFile(inputFile)
	if err != nil {
		t.Fatalf("os.ReadFile(inputFile) error: %v", err)
	}

	inputObjs, err := readAllObjects(strings.NewReader(string(inputData)))
	if err != nil {
		t.Fatalf("readAllObjects(inputFile) error: %v", err)
	}
	if got, want := len(inputObjs), 2; got != want {
		t.Fatalf("input object length = %d, want %d", got, want)
	}
	if got, want := inputObjs[0].GetKind(), "Service"; got != want {
		t.Fatalf("input first kind = %q, want %q", got, want)
	}
	if got, want := inputObjs[1].GetKind(), "Ingress"; got != want {
		t.Fatalf("input second kind = %q, want %q", got, want)
	}
}
