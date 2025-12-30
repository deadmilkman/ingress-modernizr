package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
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
	var (
		inputFile string
		version   bool
	)

	rootCmd := &cobra.Command{
		Use:   "ingress-modernizr [flags] [ingress2gateway-args...]",
		Short: "Convert Kubernetes Ingress to Gateway API resources",
		Long: `This tool reads Kubernetes manifests (rendered by Helm or any other tool), converts
Ingress resources to Gateway API resources using ingress2gateway, and outputs the
transformed manifests.`,
		Example: `  # As Helm post-renderer (reads from stdin)
  helm template myapp ./chart | ingress-modernizr --providers=ingress-nginx

  # From a file
  ingress-modernizr --input-file=manifests.yaml --providers=ingress-nginx

  # With kubectl apply
  kubectl apply -k . --dry-run=client -o yaml | ingress-modernizr --providers=ingress-nginx | kubectl apply -f -`,
		// Allow unknown flags to be passed to ingress2gateway
		FParseErrWhitelist: cobra.FParseErrWhitelist{
			UnknownFlags: true,
		},
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if version {
				fmt.Println("ingress-modernizr v0.0.1")
				return nil
			}

			// Extract ingress2gateway args. Since we set UnknownFlags: true,
			// cobra will ignore flags it doesn't know, but they won't be in 'args'.
			var ingress2gatewayArgs []string

			// Simple approach: any argument in os.Args that isn't --input-file or its value
			// and isn't --help/--version is a candidate for ingress2gateway.
			for i := 1; i < len(os.Args); i++ {
				arg := os.Args[i]
				// Skip our known flags and their values
				if arg == "--input-file" {
					i++
					continue
				}
				if strings.HasPrefix(arg, "--input-file=") {
					continue
				}
				if arg == "--help" || arg == "-h" || arg == "--version" {
					continue
				}
				ingress2gatewayArgs = append(ingress2gatewayArgs, arg)
			}

			// Basic validation for ingress2gateway args
			hasProviders := false
			for _, arg := range ingress2gatewayArgs {
				if strings.HasPrefix(arg, "--providers=") || arg == "--providers" || strings.HasPrefix(arg, "-providers=") || arg == "-providers" {
					hasProviders = true
					break
				}
			}
			if !hasProviders {
				return fmt.Errorf("--providers flag is required for ingress2gateway (e.g., --providers=ingress-nginx)")
			}

			// Determine input source
			var input io.Reader
			var inputSource string
			if inputFile != "" {
				file, err := os.Open(inputFile)
				if err != nil {
					return fmt.Errorf("failed to open input file %s: %w", inputFile, err)
				}
				defer file.Close()
				input = file
				inputSource = inputFile
			} else {
				input = os.Stdin
				inputSource = "stdin"
			}

			// 1. Read manifests from input source
			originalObjects, err := readAllObjects(input)
			if err != nil {
				return fmt.Errorf("failed to read input manifests from %s: %w", inputSource, err)
			}

			// Nothing in, nothing out.
			if len(originalObjects) == 0 {
				if inputFile != "" {
					fmt.Fprintf(os.Stderr, "ingress-modernizr: warning: no objects found in %s\n", inputFile)
				}
				return nil
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
				fmt.Fprintf(os.Stderr, "ingress-modernizr: warning: no Ingress resources found in input\n")
				return writeObjectsYAML(originalObjects, os.Stdout)
			}

			// 2. Run ingress2gateway on the whole set
			convertedObjects, err := runIngress2Gateway(originalObjects, ingress2gatewayArgs)
			if err != nil {
				return err
			}

			// 3. Build final manifest set
			var final []*unstructured.Unstructured
			for _, obj := range originalObjects {
				if obj.GetKind() == "Ingress" {
					continue
				}
				final = append(final, obj)
			}
			final = append(final, convertedObjects...)

			// 4. Emit final manifests to stdout
			return writeObjectsYAML(final, os.Stdout)
		},
	}

	rootCmd.Flags().StringVar(&inputFile, "input-file", "", "Path to input manifest file (default: read from stdin)")
	rootCmd.Flags().BoolVar(&version, "version", false, "Show version")

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
