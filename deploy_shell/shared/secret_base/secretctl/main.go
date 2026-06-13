package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"filippo.io/age"
	"gopkg.in/yaml.v3"
)

var placeholderPattern = regexp.MustCompile("^\\{\\{\\s*secret\\s+`([^`]+)`\\s*\\}\\}$")

type bundleFile struct {
	Version    int               `yaml:"version"`
	Repo       string            `yaml:"repo"`
	Subproject string            `yaml:"subproject"`
	Env        string            `yaml:"env"`
	Secrets    map[string]string `yaml:"secrets"`
}

type commandError struct {
	message string
}

func (e *commandError) Error() string {
	return e.message
}

func main() {
	if len(os.Args) < 2 {
		exitWithError(&commandError{message: usage()})
	}

	var err error
	switch os.Args[1] {
	case "encrypt":
		err = runEncrypt(os.Args[2:])
	case "decrypt":
		err = runDecrypt(os.Args[2:])
	case "validate":
		err = runValidate(os.Args[2:])
	case "render":
		err = runRender(os.Args[2:])
	case "help", "-h", "--help":
		fmt.Print(usage())
		return
	default:
		err = &commandError{message: fmt.Sprintf("unknown command: %s\n\n%s", os.Args[1], usage())}
	}

	if err != nil {
		exitWithError(err)
	}
}

func usage() string {
	return strings.TrimSpace(`
secretctl 用法:
  secretctl encrypt --in <bundle.yaml> --out <bundle.yaml.age> --recipient-file <active_key.pub>
  secretctl decrypt --bundle <bundle.yaml.age> --identity-file <active_key.txt> [--out <bundle.yaml>]
  secretctl validate --template <config.yaml> --bundle <bundle.yaml.age> --identity-file <active_key.txt> [--expect-repo <repo>] [--expect-subproject <subproject>] [--expect-env <env>]
  secretctl render --template <config.yaml> --bundle <bundle.yaml.age> --identity-file <active_key.txt> --out <config.yaml> [--expect-repo <repo>] [--expect-subproject <subproject>] [--expect-env <env>] [--audit-log <logfile>] [--operator <name>]
`)
}

func exitWithError(err error) {
	if err == nil {
		return
	}
	fmt.Fprintln(os.Stderr, err.Error())
	os.Exit(1)
}

func runEncrypt(args []string) error {
	fs := flag.NewFlagSet("encrypt", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var inPath string
	var outPath string
	var recipientFile string
	var recipient string

	fs.StringVar(&inPath, "in", "", "明文 bundle YAML 路径")
	fs.StringVar(&outPath, "out", "", "加密后的 .age 输出路径")
	fs.StringVar(&recipientFile, "recipient-file", "", "age recipient 文件路径")
	fs.StringVar(&recipient, "recipient", "", "单个 age recipient")
	if err := fs.Parse(args); err != nil {
		return &commandError{message: err.Error()}
	}

	if strings.TrimSpace(inPath) == "" || strings.TrimSpace(outPath) == "" {
		return &commandError{message: "encrypt 需要同时提供 --in 和 --out"}
	}
	if strings.TrimSpace(recipientFile) == "" && strings.TrimSpace(recipient) == "" {
		return &commandError{message: "encrypt 需要提供 --recipient-file 或 --recipient"}
	}

	recipients, err := loadRecipients(recipientFile, recipient)
	if err != nil {
		return err
	}

	content, err := os.ReadFile(inPath)
	if err != nil {
		return fmt.Errorf("read input bundle: %w", err)
	}

	if err := ensureParentDir(outPath, 0o750); err != nil {
		return err
	}

	tmpPath := outPath + ".tmp"
	outFile, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open output bundle: %w", err)
	}
	defer outFile.Close()

	writer, err := age.Encrypt(outFile, recipients...)
	if err != nil {
		return fmt.Errorf("encrypt bundle: %w", err)
	}
	if _, err := writer.Write(content); err != nil {
		return fmt.Errorf("write encrypted bundle: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("close encrypted bundle: %w", err)
	}
	if err := outFile.Close(); err != nil {
		return fmt.Errorf("close output bundle: %w", err)
	}
	if err := os.Rename(tmpPath, outPath); err != nil {
		return fmt.Errorf("rename encrypted bundle: %w", err)
	}
	return nil
}

func runDecrypt(args []string) error {
	fs := flag.NewFlagSet("decrypt", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var bundlePath string
	var identityFile string
	var outPath string

	fs.StringVar(&bundlePath, "bundle", "", "加密 bundle 路径")
	fs.StringVar(&identityFile, "identity-file", "", "age identity 文件路径")
	fs.StringVar(&outPath, "out", "", "解密后输出路径；不传则打印到 stdout")
	if err := fs.Parse(args); err != nil {
		return &commandError{message: err.Error()}
	}

	if strings.TrimSpace(bundlePath) == "" {
		return &commandError{message: "decrypt 需要提供 --bundle"}
	}
	if strings.TrimSpace(identityFile) == "" {
		return &commandError{message: "decrypt 需要提供 --identity-file"}
	}

	content, err := readBundleBytes(bundlePath, identityFile)
	if err != nil {
		return err
	}

	if strings.TrimSpace(outPath) == "" {
		_, err = os.Stdout.Write(content)
		return err
	}

	if err := ensureParentDir(outPath, 0o750); err != nil {
		return err
	}
	return os.WriteFile(outPath, content, 0o600)
}

func runValidate(args []string) error {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var templatePath string
	var bundlePath string
	var identityFile string
	var expectRepo string
	var expectSubproject string
	var expectEnv string

	fs.StringVar(&templatePath, "template", "", "模板 YAML 路径")
	fs.StringVar(&bundlePath, "bundle", "", "bundle 路径")
	fs.StringVar(&identityFile, "identity-file", "", "age identity 文件路径")
	fs.StringVar(&expectRepo, "expect-repo", "", "校验 bundle repo")
	fs.StringVar(&expectSubproject, "expect-subproject", "", "校验 bundle subproject")
	fs.StringVar(&expectEnv, "expect-env", "", "校验 bundle env")
	if err := fs.Parse(args); err != nil {
		return &commandError{message: err.Error()}
	}

	if strings.TrimSpace(templatePath) == "" || strings.TrimSpace(bundlePath) == "" {
		return &commandError{message: "validate 需要同时提供 --template 和 --bundle"}
	}

	doc, placeholders, err := loadTemplate(templatePath)
	if err != nil {
		return err
	}
	_ = doc

	bundle, err := loadBundle(bundlePath, identityFile)
	if err != nil {
		return err
	}
	if err := validateBundleMetadata(bundle, expectRepo, expectSubproject, expectEnv); err != nil {
		return err
	}

	missing := collectMissingPlaceholders(placeholders, bundle.Secrets)
	if len(missing) > 0 {
		return &commandError{message: fmt.Sprintf("bundle 缺少以下 secret key: %s", strings.Join(missing, ", "))}
	}

	keys := make([]string, 0, len(placeholders))
	for key := range placeholders {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	if len(keys) == 0 {
		fmt.Println("validate ok: template 未使用 secret 占位符")
		return nil
	}

	fmt.Printf("validate ok: %d secret key(s)\n", len(keys))
	for _, key := range keys {
		fmt.Printf("- %s\n", key)
	}
	return nil
}

func runRender(args []string) error {
	fs := flag.NewFlagSet("render", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var templatePath string
	var bundlePath string
	var identityFile string
	var outPath string
	var expectRepo string
	var expectSubproject string
	var expectEnv string
	var auditLog string
	var operator string

	fs.StringVar(&templatePath, "template", "", "模板 YAML 路径")
	fs.StringVar(&bundlePath, "bundle", "", "bundle 路径")
	fs.StringVar(&identityFile, "identity-file", "", "age identity 文件路径")
	fs.StringVar(&outPath, "out", "", "渲染后的 config.yaml 输出路径")
	fs.StringVar(&expectRepo, "expect-repo", "", "校验 bundle repo")
	fs.StringVar(&expectSubproject, "expect-subproject", "", "校验 bundle subproject")
	fs.StringVar(&expectEnv, "expect-env", "", "校验 bundle env")
	fs.StringVar(&auditLog, "audit-log", "", "审计日志路径")
	fs.StringVar(&operator, "operator", "", "操作者")
	if err := fs.Parse(args); err != nil {
		return &commandError{message: err.Error()}
	}

	if strings.TrimSpace(templatePath) == "" || strings.TrimSpace(bundlePath) == "" || strings.TrimSpace(outPath) == "" {
		return &commandError{message: "render 需要同时提供 --template、--bundle 和 --out"}
	}

	doc, placeholders, err := loadTemplate(templatePath)
	if err != nil {
		return err
	}

	bundle, err := loadBundle(bundlePath, identityFile)
	if err != nil {
		return err
	}
	if err := validateBundleMetadata(bundle, expectRepo, expectSubproject, expectEnv); err != nil {
		return err
	}

	missing := collectMissingPlaceholders(placeholders, bundle.Secrets)
	if len(missing) > 0 {
		return &commandError{message: fmt.Sprintf("bundle 缺少以下 secret key: %s", strings.Join(missing, ", "))}
	}

	if err := renderTemplate(doc, bundle.Secrets); err != nil {
		return err
	}

	rendered, err := marshalYAML(doc)
	if err != nil {
		return err
	}
	if err := writeFileAtomically(outPath, rendered, 0o640); err != nil {
		return err
	}
	if strings.TrimSpace(auditLog) != "" {
		if err := appendAuditLog(auditLog, bundlePath, outPath, operator, bundle); err != nil {
			return err
		}
	}
	return nil
}

func loadRecipients(recipientFile string, recipient string) ([]age.Recipient, error) {
	if strings.TrimSpace(recipientFile) != "" {
		content, err := os.ReadFile(recipientFile)
		if err != nil {
			return nil, fmt.Errorf("read recipient file: %w", err)
		}
		recipients, err := age.ParseRecipients(bytes.NewReader(content))
		if err != nil {
			return nil, fmt.Errorf("parse recipient file: %w", err)
		}
		return recipients, nil
	}

	parsed, err := age.ParseX25519Recipient(strings.TrimSpace(recipient))
	if err != nil {
		return nil, fmt.Errorf("parse recipient: %w", err)
	}
	return []age.Recipient{parsed}, nil
}

func loadIdentities(identityFile string) ([]age.Identity, error) {
	content, err := os.ReadFile(identityFile)
	if err != nil {
		return nil, fmt.Errorf("read identity file: %w", err)
	}
	identities, err := age.ParseIdentities(bytes.NewReader(content))
	if err != nil {
		return nil, fmt.Errorf("parse identity file: %w", err)
	}
	return identities, nil
}

func readBundleBytes(bundlePath string, identityFile string) ([]byte, error) {
	if strings.HasSuffix(bundlePath, ".age") {
		if strings.TrimSpace(identityFile) == "" {
			return nil, &commandError{message: "bundle 为 .age 文件时必须提供 --identity-file"}
		}

		identities, err := loadIdentities(identityFile)
		if err != nil {
			return nil, err
		}
		in, err := os.Open(bundlePath)
		if err != nil {
			return nil, fmt.Errorf("open encrypted bundle: %w", err)
		}
		defer in.Close()

		reader, err := age.Decrypt(in, identities...)
		if err != nil {
			return nil, fmt.Errorf("decrypt bundle: %w", err)
		}
		content, err := io.ReadAll(reader)
		if err != nil {
			return nil, fmt.Errorf("read decrypted bundle: %w", err)
		}
		return content, nil
	}

	content, err := os.ReadFile(bundlePath)
	if err != nil {
		return nil, fmt.Errorf("read bundle: %w", err)
	}
	return content, nil
}

func loadBundle(bundlePath string, identityFile string) (*bundleFile, error) {
	content, err := readBundleBytes(bundlePath, identityFile)
	if err != nil {
		return nil, err
	}
	var bundle bundleFile
	if err := yaml.Unmarshal(content, &bundle); err != nil {
		return nil, fmt.Errorf("parse bundle yaml: %w", err)
	}
	if bundle.Secrets == nil {
		bundle.Secrets = map[string]string{}
	}
	return &bundle, nil
}

func loadTemplate(templatePath string) (*yaml.Node, map[string]struct{}, error) {
	content, err := os.ReadFile(templatePath)
	if err != nil {
		return nil, nil, fmt.Errorf("read template yaml: %w", err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(content, &doc); err != nil {
		return nil, nil, fmt.Errorf("parse template yaml: %w", err)
	}
	placeholders := map[string]struct{}{}
	collectPlaceholders(&doc, placeholders)
	return &doc, placeholders, nil
}

func collectPlaceholders(node *yaml.Node, placeholders map[string]struct{}) {
	if node == nil {
		return
	}
	if key, ok := extractPlaceholder(node.Value); ok {
		placeholders[key] = struct{}{}
	}
	for _, child := range node.Content {
		collectPlaceholders(child, placeholders)
	}
}

func collectMissingPlaceholders(placeholders map[string]struct{}, secrets map[string]string) []string {
	missing := make([]string, 0)
	for key := range placeholders {
		if _, ok := secrets[key]; !ok {
			missing = append(missing, key)
		}
	}
	sort.Strings(missing)
	return missing
}

func extractPlaceholder(value string) (string, bool) {
	matches := placeholderPattern.FindStringSubmatch(strings.TrimSpace(value))
	if len(matches) != 2 {
		return "", false
	}
	return matches[1], true
}

func renderTemplate(node *yaml.Node, secrets map[string]string) error {
	if node == nil {
		return nil
	}
	if key, ok := extractPlaceholder(node.Value); ok {
		value, exists := secrets[key]
		if !exists {
			return &commandError{message: fmt.Sprintf("bundle 中缺少 secret key: %s", key)}
		}
		node.Tag = "!!str"
		node.Style = 0
		node.Value = value
	}
	for _, child := range node.Content {
		if err := renderTemplate(child, secrets); err != nil {
			return err
		}
	}
	return nil
}

func marshalYAML(node *yaml.Node) ([]byte, error) {
	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(node); err != nil {
		return nil, fmt.Errorf("encode rendered yaml: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return nil, fmt.Errorf("close yaml encoder: %w", err)
	}
	return buf.Bytes(), nil
}

func validateBundleMetadata(bundle *bundleFile, expectRepo string, expectSubproject string, expectEnv string) error {
	checks := []struct {
		expect string
		actual string
		label  string
	}{
		{expect: strings.TrimSpace(expectRepo), actual: strings.TrimSpace(bundle.Repo), label: "repo"},
		{expect: strings.TrimSpace(expectSubproject), actual: strings.TrimSpace(bundle.Subproject), label: "subproject"},
		{expect: strings.TrimSpace(expectEnv), actual: strings.TrimSpace(bundle.Env), label: "env"},
	}

	for _, check := range checks {
		if check.expect == "" {
			continue
		}
		if check.actual == "" {
			return &commandError{message: fmt.Sprintf("bundle 缺少元数据 %s", check.label)}
		}
		if check.actual != check.expect {
			return &commandError{message: fmt.Sprintf("bundle %s 不匹配: expect=%s actual=%s", check.label, check.expect, check.actual)}
		}
	}
	return nil
}

func appendAuditLog(logPath string, bundlePath string, outPath string, operator string, bundle *bundleFile) error {
	if err := ensureParentDir(logPath, 0o750); err != nil {
		return err
	}
	if strings.TrimSpace(operator) == "" {
		operator = "unknown"
	}
	line := fmt.Sprintf(
		"%s operator=%s repo=%s subproject=%s env=%s bundle=%s out=%s result=success\n",
		time.Now().Format(time.RFC3339),
		operator,
		strings.TrimSpace(bundle.Repo),
		strings.TrimSpace(bundle.Subproject),
		strings.TrimSpace(bundle.Env),
		bundlePath,
		outPath,
	)

	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o640)
	if err != nil {
		return fmt.Errorf("open audit log: %w", err)
	}
	defer file.Close()
	if _, err := file.WriteString(line); err != nil {
		return fmt.Errorf("write audit log: %w", err)
	}
	return nil
}

func ensureParentDir(path string, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, mode); err != nil {
		return fmt.Errorf("create parent dir: %w", err)
	}
	return nil
}

func writeFileAtomically(path string, content []byte, perm os.FileMode) error {
	if err := ensureParentDir(path, 0o750); err != nil {
		return err
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, content, perm); err != nil {
		return fmt.Errorf("write output file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename output file: %w", err)
	}
	return nil
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func init() {
	must(validatePlaceholderPattern())
}

func validatePlaceholderPattern() error {
	samples := []struct {
		value string
		ok    bool
	}{
		{value: "{{ secret `mysql.password` }}", ok: true},
		{value: "{{secret `mysql.password`}}", ok: true},
		{value: "mysql.password", ok: false},
	}

	for _, sample := range samples {
		_, ok := extractPlaceholder(sample.value)
		if ok != sample.ok {
			return errors.New("placeholder pattern init failed")
		}
	}
	return nil
}
