package main

import (
	"bufio"
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
	scanner := utilyaml.NewYAMLReader(bufio.NewReader(r))
	var objs []*unstructured.Unstructured
	for {
		doc, err := scanner.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("failed to read document: %w", err)
		}

		trimmed := bytes.TrimSpace(doc)
		if len(trimmed) == 0 {
			continue
		}

		var raw map[string]interface{}
		if err := syaml.Unmarshal(doc, &raw); err != nil {
			// If it's not a map (e.g. a scalar or comment doc), skip it for object parsing
			continue
		}

		if len(raw) == 0 {
			continue
		}

		objs = append(objs, &unstructured.Unstructured{Object: raw})
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

// runIngress2Gateway takes only the Ingress resources, writes them to a temp file,
// and calls `ingress2gateway print --input-file=<temp>`, passing through any extraArgs.
// It returns the converted Gateway API resources as Unstructured objects.
func runIngress2Gateway(ingresses []*unstructured.Unstructured, extraArgs []string) ([]*unstructured.Unstructured, error) {
	if len(ingresses) == 0 {
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

	if err := writeObjectsYAML(ingresses, tmpFile); err != nil {
		_ = tmpFile.Close()
		return nil, fmt.Errorf("failed to write manifests to temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return nil, fmt.Errorf("failed to close temp file: %w", err)
	}

	bin := os.Getenv("INGRESS2GATEWAY_BIN")
	if bin == "" {
		bin = "ingress2gateway"
	}

	if len(extraArgs) > 0 && extraArgs[0] == "print" {
		extraArgs = extraArgs[1:]
	}

	args := []string{
		"print",
		"--input-file", tmpPath,
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

			var ingress2gatewayArgs []string
			for i := 1; i < len(os.Args); i++ {
				arg := os.Args[i]
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

			// Read all input at once for smallish files to avoid streaming issues
			allInput, err := io.ReadAll(input)
			if err != nil {
				return fmt.Errorf("failed to read from %s: %w", inputSource, err)
			}

			fmt.Fprintf(os.Stderr, "ingress-modernizr: info: processing %d bytes of manifests from %s\n", len(allInput), inputSource)

			// Split by --- manually to be more resilient
			var ingresses []*unstructured.Unstructured
			var preservedRaw [][]byte

			// Use YAMLReader on the buffer which is more reliable than direct streaming
			scanner := utilyaml.NewYAMLReader(bufio.NewReader(bytes.NewReader(allInput)))
			for {
				doc, err := scanner.Read()
				if err != nil {
					if err == io.EOF {
						break
					}
					return fmt.Errorf("failed to parse manifests from %s: %w", inputSource, err)
				}

				trimmed := bytes.TrimSpace(doc)
				if len(trimmed) == 0 {
					continue
				}

				var meta struct {
					Kind string `json:"kind"`
				}
				if err := syaml.Unmarshal(doc, &meta); err != nil {
					preservedRaw = append(preservedRaw, doc)
					continue
				}

				if meta.Kind == "Ingress" {
					u := &unstructured.Unstructured{}
					if err := syaml.Unmarshal(doc, &u.Object); err != nil {
						return fmt.Errorf("failed to unmarshal Ingress: %w", err)
					}
					ingresses = append(ingresses, u)
				} else {
					preservedRaw = append(preservedRaw, doc)
				}
			}

			if len(ingresses) == 0 {
				if inputFile != "" {
					fmt.Fprintf(os.Stderr, "ingress-modernizr: warning: no Ingress resources found in %s\n", inputFile)
				}
				for i, raw := range preservedRaw {
					if i > 0 {
						fmt.Fprintln(os.Stdout, "---")
					}
					os.Stdout.Write(raw)
				}
				return nil
			}

			// 2. Run ingress2gateway only on the Ingress resources
			convertedObjects, err := runIngress2Gateway(ingresses, ingress2gatewayArgs)
			if err != nil {
				return err
			}

			// 3. Emit final manifests
			first := true
			for _, raw := range preservedRaw {
				if !first {
					fmt.Fprintln(os.Stdout, "---")
				}
				os.Stdout.Write(raw)
				first = false
			}

			for _, obj := range convertedObjects {
				if !first {
					fmt.Fprintln(os.Stdout, "---")
				}
				data, err := syaml.Marshal(obj.Object)
				if err != nil {
					return fmt.Errorf("failed to marshal converted object: %w", err)
				}
				os.Stdout.Write(data)
				first = false
			}

			return nil
		},
	}

	rootCmd.Flags().StringVar(&inputFile, "input-file", "", "Path to input manifest file (default: read from stdin)")
	rootCmd.Flags().BoolVar(&version, "version", false, "Show version")

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
