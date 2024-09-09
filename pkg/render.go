package pkg

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"
)

const VERSION = "0.0.1"

// REGEXP_ENV_SUBST is a regular expression to match custom placeholders in the kustomize manifests
// Example ${{{ NFS_SERVER_IP }}}
const REGEXP_ENV_SUBST = `\$\{\{\{\s*([A-Za-z0-9_:/-]+)\s*\}\}\}`

// RenderCmd renders the kustomize manifests to rendered.yaml
func RenderCmd(envFile string) {
	fmt.Printf(" 🚀 kubectl env render version %s loading env file %s\n\n", VERSION, envFile)

	// Load environment variables from the specified file
	if err := loadEnvFile(envFile, true); err != nil {
		fmt.Printf(" ⚠️ unable to load env file %s. %v \n", envFile, err)
	}

	envPrefix := os.Getenv("ENV_KUBECTL_PREFIX")
	if envPrefix == "" {
		fmt.Printf(" ⚠️ ENV_KUBECTL_PREFIX environment variable is not set, all variables will be loaded, to avoid conflict please avoid loading all envs present on your enviornment.\n")
	}

	fmt.Printf(" ℹ️ Environment variables with prefix '%s'\n", envPrefix)
	envVars := getEnvVarsWithPrefix(envPrefix)

	for key, value := range envVars {
		if strings.Contains(key, "SECRET") || strings.Contains(key, "PASS") || strings.Contains(key, "KEY") {
			masked := strings.Repeat("*", 4) + value[len(value)-3:]
			fmt.Printf("   - %s=%s\n", key, masked)
		} else {
			fmt.Printf("   - %s=%s\n", key, value)
		}
	}

	fmt.Println()

	// Build the kustomize manifests
	diskFsSys := filesys.MakeFsOnDisk()
	fSys := NewEnvAwareFileSystem(diskFsSys, expandAllEnvs)
	opts := krusty.MakeDefaultOptions()

	// Honor kustomize flag to enable exec and helm plugins
	opts.PluginConfig = types.EnabledPluginConfig(types.BploUseStaticallyLinked)
	opts.PluginConfig.FnpLoadingOptions.EnableExec = true
	opts.PluginConfig.HelmConfig.Enabled = true
	opts.PluginConfig.HelmConfig.Command = "helm"

	k := krusty.MakeKustomizer(opts)

	var buf bytes.Buffer
	// If kustomization.yaml exists in the current directory, run kustomize in the current directory
	// Otherwise, iterate through each subdirectory and run kustomize
	if fSys.Exists("kustomization.yaml") {
		// Run kustomize in the current directory
		appendKustomizeOutput(fSys, k, ".", &buf)
	} else {
		// Iterate through each subdirectory and run kustomize
		err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() && path != "." {
				if fSys.Exists(filepath.Join(path, "kustomization.yaml")) {
					appendKustomizeOutput(fSys, k, path, &buf)
				}
			}
			return nil
		})

		if err != nil {
			fmt.Fprintf(os.Stderr, "Error iterating through directories: %v\n", err)
			os.Exit(1)
		}
	}

	// render the final manifests (already expanded and processed by kustomize)
	if err := ioutil.WriteFile("rendered.yaml", buf.Bytes(), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing rendered.yaml: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf(" 📄 Rendered manifests written to rendered.yaml\n\n")
}

// Replace the custom holders for the expanded values
func expandAllEnvs(data []byte) ([]byte, error) {
	// expand all env-expand:// placeholders
	renderedBytes := expandContainerEnvs(data)

	// Replace custom placeholders with environment variables
	re := regexp.MustCompile(REGEXP_ENV_SUBST)
	replaced := re.ReplaceAllFunc(renderedBytes, func(b []byte) []byte {
		key := string(b[4 : len(b)-3])
		key = strings.TrimSpace(key)
		value, ok := os.LookupEnv(key)

		if !ok {
			fmt.Fprintf(os.Stderr, " - Error: Environment variable ${{{ %s }}} is not set\n", key)
			os.Exit(1)
		}

		return []byte(value)
	})

	// check if there are any placeholders left
	if re.Match(replaced) {
		fmt.Fprintf(os.Stderr, "Error: Some placeholders are not replaced\n")
		os.Exit(1)
	}

	return replaced, nil
}

// Load environment variables from the specified file using source command
// Recursively load environment variables from the specified file and any files it sources.
func loadEnvFile(filename string, processSecrets bool) error {
	fmt.Printf(" 📄 Sourcing environment variables from %s\n", filename)
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		// trim after # for comments
		if strings.Contains(line, "#") {
			line = line[:strings.Index(line, "#")]
		}
		line = strings.TrimSpace(line)

		// if empty line, skip
		if len(line) == 0 {
			continue
		}

		if strings.HasPrefix(line, "source ") {
			// Extract the path of the sourced file
			sourcedFile := strings.TrimSpace(line[len("source "):])
			// Resolve the path of the sourced file relative to the current file
			sourcedFilePath := filepath.Join(filepath.Dir(filename), sourcedFile)

			// Recursively load the sourced file
			err := loadEnvFile(sourcedFilePath, processSecrets)
			if err != nil {
				return err
			}
		} else {
			// load the environment variable (format export key=value)
			// drop export
			var keyValue string
			if strings.HasPrefix(line, "export ") {
				keyValue = strings.TrimSpace(line[len("export "):]) // drop export if pressent
			} else {
				keyValue = line
			}

			parts := strings.SplitN(keyValue, "=", 2)
			if len(parts) != 2 {
				return fmt.Errorf("invalid line: %s", line)
			}
			key := parts[0]
			value := parts[1]
			value = strings.Trim(value, "'\"")
			value = os.ExpandEnv(value)
			if processSecrets {
				value, err = expandGcpsecret(key, value)
				if err != nil {
					return err
				}
				value, err = expandGcpsecretBase64(key, value)
				if err != nil {
					return err
				}

			}
			if err != nil {
				return err
			}

			os.Setenv(key, value)
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return nil
}

// fetchSecret retrieves the secret from Google Cloud Secret Manager
func fetchSecret(prefix, value string) ([]byte, string, error) {
	if !strings.HasPrefix(value, prefix) {
		return nil, value, nil
	}

	ctx := context.Background()

	secretPath := strings.TrimPrefix(value, prefix)

	// Create the client.
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create secret manager client: %v", err)
	}
	defer client.Close()

	// Build the request.
	name := fmt.Sprintf("%s/versions/latest", secretPath)

	req := &secretmanagerpb.AccessSecretVersionRequest{
		Name: name,
	}

	// Call the API.
	result, err := client.AccessSecretVersion(ctx, req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch secret from %s: %v", name, err)
	}

	fmt.Printf(" - 🔐 Secret fetched from %s\n", name)

	return result.Payload.Data, name, nil
}

// expandGcpsecret fetches and returns the secret value from Google Cloud Secret Manager
func expandGcpsecret(key, value string) (string, error) {
	secretData, finalValue, err := fetchSecret("gcp-secret://", value)
	if err != nil {
		return "", err
	}
	if secretData == nil {
		return finalValue, nil
	}

	return string(secretData), nil
}

// expandGcpsecretBase64 fetches the base64 encoded secret value from Google Cloud Secret Manager
func expandGcpsecretBase64(key, value string) (string, error) {
	secretData, finalValue, err := fetchSecret("gcp-secret-base64://", value)
	if err != nil {
		return "", err
	}
	if secretData == nil {
		return finalValue, nil
	}

	return base64.StdEncoding.EncodeToString(secretData), nil
}

func expandContainerEnvs(buf []byte) []byte {
	lines := bytes.Split(buf, []byte("\n"))
	var result []byte

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := bytes.TrimSpace(line)
		if bytes.HasPrefix(trimmed, []byte("- name: ${{{env-expand://")) {
			endIndex := bytes.Index(trimmed, []byte("}}}"))
			if endIndex != -1 && i+1 < len(lines) && bytes.Contains(bytes.TrimSpace(lines[i+1]), []byte("value: ${{{env-expand://")) {
				prefix := string(trimmed[len("- name: ${{{env-expand://"):endIndex])
				envVars := getEnvVarsWithPrefix(prefix)
				padding := bytes.Repeat([]byte(" "), bytes.Index(line, []byte("-")))

				// Extract keys and sort them to ensure deterministic output
				keys := make([]string, 0, len(envVars))
				for k := range envVars {
					keys = append(keys, k)
				}
				sort.Strings(keys)

				for _, key := range keys {
					value := envVars[key]
					result = append(result, padding...)
					result = append(result, []byte(fmt.Sprintf("- name: %s\n", key))...)
					result = append(result, padding...)
					result = append(result, []byte(fmt.Sprintf("  value: \"%s\"\n", value))...)
				}
				i++ // Skip the next line as it is already processed
			}
		} else {
			result = append(result, line...)
			result = append(result, '\n')
		}
	}

	return result
}

func getEnvVarsWithPrefix(prefix string) map[string]string {
	envVars := make(map[string]string)
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 && strings.HasPrefix(parts[0], prefix) {
			value := parts[1]
			envVars[parts[0]] = value
		}
	}

	return envVars
}

// Run kustomize in the specified directory and append the output to the buffer
func appendKustomizeOutput(fSys filesys.FileSystem, k *krusty.Kustomizer, path string, buf *bytes.Buffer) {
	fmt.Printf(" 🖨️ Building kustomize manifests in %s\n", path)
	resMap, err := k.Run(fSys, path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error building kustomize manifests in %s: %v\n", path, err)
		os.Exit(1)
	}

	yamlData, err := resMap.AsYaml()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing manifests to buffer in %s: %v\n", path, err)
		os.Exit(1)
	}

	buf.Write(yamlData)
	buf.WriteString("\n---\n")
}
