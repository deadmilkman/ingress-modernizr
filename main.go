package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	syaml "sigs.k8s.io/yaml"
)

// readAllObjects reads a multi-doc YAML/JSON stream into a slice of Unstructured.
func readAllObjects(r io.Reader) ([]*unstructured.Unstructured, error) {
	decoder := utilyaml.NewYAMLOrJSONDecoder(r, 4096)

	var objs []*unstructured.Unstructured
	for {
		raw := make(map[string]interface{})
		if err := decoder.Decode(&raw); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("failed to decode manifest: %w", err)
		}

		// Skip empty docs (e.g. trailing ---)
		if len(raw) == 0 {
			continue
		}

		u := &unstructured.Unstructured{Object: raw}
		objs = append(objs, u)
	}

	return objs, nil
}

// writeObjectsYAML writes objects as multi-document YAML to w.
func writeObjectsYAML(objs []*unstructured.Unstructured, w io.Writer) error {
	for i, u := range objs {
		if i > 0 {
			if _, err := fmt.Fprintln(w, "---"); err != nil {
				return fmt.Errorf("failed to write document separator: %w", err)
			}
		}

		data, err := syaml.Marshal(u.Object)
		if err != nil {
			return fmt.Errorf("failed to marshal object to YAML: %w", err)
		}

		if _, err := w.Write(data); err != nil {
			return fmt.Errorf("failed to write YAML object: %w", err)
		}
	}

	return nil
}

// runIngress2Gateway takes the full original manifest set, writes it to a temp file,
// and calls `ingress2gateway print --input-file=<temp>`, passing through any extraArgs.
// It returns the converted Gateway API resources as Unstructured objects.
func runIngress2Gateway(allOriginal []*unstructured.Unstructured, extraArgs []string) ([]*unstructured.Unstructured, error) {
	// If there's nothing, nothing to convert.
	if len(allOriginal) == 0 {
		return nil, nil
	}

	tmpFile, err := os.CreateTemp("", "ingress-modernizr-*.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	if err := writeObjectsYAML(allOriginal, tmpFile); err != nil {
		_ = tmpFile.Close()
		return nil, fmt.Errorf("failed to write manifests to temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return nil, fmt.Errorf("failed to close temp file: %w", err)
	}

	// Decide which ingress2gateway binary to use.
	bin := os.Getenv("INGRESS2GATEWAY_BIN")
	if bin == "" {
		bin = "ingress2gateway"
	}

	// If the user passed "print" explicitly, drop it — we always call print.
	if len(extraArgs) > 0 && extraArgs[0] == "print" {
		extraArgs = extraArgs[1:]
	}

	args := []string{
		"print",
		"--input-file", tmpPath,
		// don't force --output; default is yaml and user can override if desired
	}
	args = append(args, extraArgs...)

	cmd := exec.Command(bin, args...)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ingress2gateway failed: %w\nstderr:\n%s", err, stderr.String())
	}

	converted, err := readAllObjects(&stdout)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ingress2gateway output: %w", err)
	}

	return converted, nil
}

func main() {
	log.SetFlags(0)

	var (
		inputFile = flag.String("input-file", "", "Path to input manifest file (default: read from stdin)")
		help      = flag.Bool("help", false, "Show help message")
		version   = flag.Bool("version", false, "Show version")
	)
	
	// Custom usage function
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `ingress-modernizr - Convert Kubernetes Ingress to Gateway API resources

Usage: %s [flags] [ingress2gateway-args...]

This tool reads Kubernetes manifests (rendered by Helm or any other tool), converts
Ingress resources to Gateway API resources using ingress2gateway, and outputs the
transformed manifests.

Flags:
`, os.Args[0])
		flag.PrintDefaults()
		fmt.Fprint(os.Stderr, `
Examples:
  # As Helm post-renderer (reads from stdin)
  helm template myapp ./chart | ingress-modernizr --providers=ingress-nginx

  # From a file
  ingress-modernizr --input-file=manifests.yaml --providers=ingress-nginx

  # With kubectl apply
  kubectl apply -k . --dry-run=client -o yaml | ingress-modernizr --providers=ingress-nginx | kubectl apply -f -

All arguments after flags are passed directly to ingress2gateway.
Provider is mandatory (e.g., --providers=ingress-nginx).

`)
	}

	flag.Parse()

	if *help {
		flag.Usage()
		return
	}

	if *version {
		fmt.Println("ingress-modernizr v1.0.0")
		return
	}

	// Get remaining args to pass to ingress2gateway
	ingress2gatewayArgs := flag.Args()

	// Basic validation for ingress2gateway args
	hasProviders := false
	for _, arg := range ingress2gatewayArgs {
		if strings.HasPrefix(arg, "--providers=") || arg == "--providers" {
			hasProviders = true
			break
		}
	}
	if !hasProviders {
		log.Fatalf("ingress-modernizr: error: --providers flag is required for ingress2gateway (e.g., --providers=ingress-nginx)")
	}

	// Determine input source
	var input io.Reader
	var inputSource string
	if *inputFile != "" {
		file, err := os.Open(*inputFile)
		if err != nil {
			log.Fatalf("ingress-modernizr: failed to open input file %s: %v", *inputFile, err)
		}
		defer file.Close()
		input = file
		inputSource = *inputFile
	} else {
		input = os.Stdin
		inputSource = "stdin"
	}

	// 1. Read manifests from input source
	originalObjects, err := readAllObjects(input)
	if err != nil {
		log.Fatalf("ingress-modernizr: failed to read input manifests from %s: %v", inputSource, err)
	}

	// Nothing in, nothing out.
	if len(originalObjects) == 0 {
		if *inputFile != "" {
			log.Printf("ingress-modernizr: warning: no objects found in %s", *inputFile)
		}
		return
	}

	// Check if there are any Ingress resources to convert
	hasIngress := false
	for _, obj := range originalObjects {
		if obj.GetKind() == "Ingress" {
			hasIngress = true
			break
		}
	}

	if !hasIngress {
		log.Printf("ingress-modernizr: warning: no Ingress resources found in input")
		// Still output the original manifests
		if err := writeObjectsYAML(originalObjects, os.Stdout); err != nil {
			log.Fatalf("ingress-modernizr: failed to write output manifests: %v", err)
		}
		return
	}

	// 2. Run ingress2gateway on the whole set, letting it pick and process
	//    Ingress + provider-specific CRDs. Other resources are ignored by it.
	convertedObjects, err := runIngress2Gateway(originalObjects, ingress2gatewayArgs)
	if err != nil {
		log.Fatalf("ingress-modernizr: %v", err)
	}

	// 3. Build final manifest set:
	//    - Drop original Ingress resources
	//    - Keep all other original resources
	//    - Append converted Gateway API resources
	var final []*unstructured.Unstructured

	for _, obj := range originalObjects {
		if obj.GetKind() == "Ingress" {
			// Strip all Ingress objects; they are replaced by converted ones.
			continue
		}
		final = append(final, obj)
	}

	final = append(final, convertedObjects...)

	// 4. Emit final manifests to stdout
	if err := writeObjectsYAML(final, os.Stdout); err != nil {
		log.Fatalf("ingress-modernizr: failed to write output manifests: %v", err)
	}
}
