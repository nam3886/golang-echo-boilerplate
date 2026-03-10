package main

import (
	"bufio"
	"embed"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"
	"unicode"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

// ModuleData holds all naming variants passed to templates.
type ModuleData struct {
	Name            string // "product"
	NameTitle       string // "Product"
	NamePlural      string // "products"
	NamePluralTitle string // "Products"
	NameSnake       string // "product" (same for single word, "order_item" for multi)
	NamePackage     string // Go package identifier: "product" or "orderitem" (underscores stripped)
	NameID          string // "ProductID"
	Timestamp       string // "20260305153000" (for migration)
	GoModule        string // read from go.mod
}

// fileMapping pairs a template name with its output path.
type fileMapping struct {
	tmpl string
	out  string
}

// Go reserved words and builtins that would produce broken packages.
var reservedNames = map[string]bool{
	"break": true, "case": true, "chan": true, "const": true, "continue": true,
	"default": true, "defer": true, "else": true, "error": true, "fallthrough": true,
	"for": true, "func": true, "go": true, "goto": true, "if": true,
	"import": true, "interface": true, "map": true, "package": true, "range": true,
	"return": true, "select": true, "struct": true, "switch": true, "type": true,
	"var": true, "string": true, "int": true, "bool": true, "byte": true,
	"float32": true, "float64": true, "int32": true, "int64": true,
}

func main() {
	name := flag.String("name", "", "module name (singular lowercase, e.g. product)")
	plural := flag.String("plural", "", "plural form override (default: name + s)")
	flag.Parse()

	if *name == "" {
		fmt.Fprintln(os.Stderr, "error: -name is required")
		flag.Usage()
		os.Exit(1)
	}

	// Ensure go.mod exists (running from project root).
	if _, err := os.Stat("go.mod"); err != nil {
		fmt.Fprintln(os.Stderr, "error: go.mod not found — run from project root")
		os.Exit(1)
	}

	validateIdentifier(*name, "name")
	if *plural != "" {
		validateIdentifier(*plural, "plural")
	}

	if reservedNames[*name] {
		fmt.Fprintf(os.Stderr, "error: %q is a Go reserved word and cannot be used as a module name\n", *name)
		os.Exit(1)
	}

	// Derive naming variants.
	data := ModuleData{
		Name:        *name,
		NameTitle:   toTitle(*name),
		NameSnake:   *name,
		NamePackage: strings.ReplaceAll(*name, "_", ""),
		Timestamp:   time.Now().Format("20060102150405"),
		GoModule:    readGoModule(),
	}
	if *plural != "" {
		data.NamePlural = *plural
	} else {
		data.NamePlural = *name + "s"
	}
	data.NamePluralTitle = toTitle(data.NamePlural)
	data.NameID = data.NameTitle + "ID"

	// Define output file mappings in logical order.
	files := []fileMapping{
		{"proto.tmpl", filepath.Join("proto", data.Name, "v1", data.Name+".proto")},
		{"migration.tmpl", filepath.Join("db", "migrations", data.Timestamp+"_create_"+data.NamePlural+".sql")},
		{"queries.tmpl", filepath.Join("db", "queries", data.Name+".sql")},
		{"domain_entity.tmpl", filepath.Join("internal", "modules", data.Name, "domain", data.Name+".go")},
		{"domain_repository.tmpl", filepath.Join("internal", "modules", data.Name, "domain", "repository.go")},
		{"domain_errors.tmpl", filepath.Join("internal", "modules", data.Name, "domain", "errors.go")},
		{"events_contract.tmpl", filepath.Join("internal", "shared", "events", "contracts", data.Name+"_events.go")},
		{"domain_events.tmpl", filepath.Join("internal", "modules", data.Name, "domain", "events.go")},
		{"domain_test.tmpl", filepath.Join("internal", "modules", data.Name, "domain", data.Name+"_test.go")},
		{"app_create.tmpl", filepath.Join("internal", "modules", data.Name, "app", "create_"+data.Name+".go")},
		{"app_create_test.tmpl", filepath.Join("internal", "modules", data.Name, "app", "create_"+data.Name+"_test.go")},
		{"app_get.tmpl", filepath.Join("internal", "modules", data.Name, "app", "get_"+data.Name+".go")},
		{"app_get_test.tmpl", filepath.Join("internal", "modules", data.Name, "app", "get_"+data.Name+"_test.go")},
		{"app_list.tmpl", filepath.Join("internal", "modules", data.Name, "app", "list_"+data.NamePlural+".go")},
		{"app_list_test.tmpl", filepath.Join("internal", "modules", data.Name, "app", "list_"+data.NamePlural+"_test.go")},
		{"app_update.tmpl", filepath.Join("internal", "modules", data.Name, "app", "update_"+data.Name+".go")},
		{"app_update_test.tmpl", filepath.Join("internal", "modules", data.Name, "app", "update_"+data.Name+"_test.go")},
		{"app_delete.tmpl", filepath.Join("internal", "modules", data.Name, "app", "delete_"+data.Name+".go")},
		{"app_delete_test.tmpl", filepath.Join("internal", "modules", data.Name, "app", "delete_"+data.Name+"_test.go")},
		{"adapter_postgres.tmpl", filepath.Join("internal", "modules", data.Name, "adapters", "postgres", "repository.go")},
		{"adapter_postgres_mapper.tmpl", filepath.Join("internal", "modules", data.Name, "adapters", "postgres", "domain_mapper.go")},
		{"adapter_postgres_test.tmpl", filepath.Join("internal", "modules", data.Name, "adapters", "postgres", "repository_test.go")},
		{"adapter_grpc_handler.tmpl", filepath.Join("internal", "modules", data.Name, "adapters", "grpc", "handler.go")},
		{"adapter_grpc_mapper.tmpl", filepath.Join("internal", "modules", data.Name, "adapters", "grpc", "mapper.go")},
		{"adapter_grpc_routes.tmpl", filepath.Join("internal", "modules", data.Name, "adapters", "grpc", "routes.go")},
		{"module.tmpl", filepath.Join("internal", "modules", data.Name, "module.go")},
	}

	// Check for conflicts before writing anything.
	for _, f := range files {
		if _, err := os.Stat(f.out); err == nil {
			fmt.Fprintf(os.Stderr, "error: file already exists: %s\n", f.out)
			os.Exit(1)
		}
	}

	// Parse all templates.
	tmpl, err := template.ParseFS(templateFS, "templates/*.tmpl")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error parsing templates: %v\n", err)
		os.Exit(1)
	}

	// Track created files for rollback on partial failure.
	var created []string
	rollback := func() {
		for _, p := range created {
			_ = os.Remove(p)
		}
		// Remove empty directories left behind (best-effort, deepest first).
		dirs := map[string]struct{}{}
		for _, p := range created {
			dirs[filepath.Dir(p)] = struct{}{}
		}
		for d := range dirs {
			_ = os.Remove(d) // only removes if empty
		}
	}

	// Execute and write each template.
	for _, f := range files {
		if err := os.MkdirAll(filepath.Dir(f.out), 0755); err != nil {
			rollback()
			fmt.Fprintf(os.Stderr, "error creating directory for %s: %v\n", f.out, err)
			os.Exit(1)
		}

		out, err := os.OpenFile(f.out, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0644)
		if err != nil {
			rollback()
			fmt.Fprintf(os.Stderr, "error creating %s: %v\n", f.out, err)
			os.Exit(1)
		}

		if err := tmpl.ExecuteTemplate(out, f.tmpl, data); err != nil {
			_ = out.Close()
			rollback()
			fmt.Fprintf(os.Stderr, "error executing template %s: %v\n", f.tmpl, err)
			os.Exit(1)
		}
		_ = out.Close()
		created = append(created, f.out)
		fmt.Printf("  created: %s\n", f.out)
	}

	// Auto-register module in main.go
	autoRegistered := false
	if err := injectModuleRegistration(data); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not auto-register module: %v\n", err)
		fmt.Println("  Manual step: add module import and registration to cmd/server/main.go")
	} else {
		fmt.Printf("  registered: cmd/server/main.go\n")
		autoRegistered = true
	}

	// Auto-inject RBAC permissions and procedure mappings.
	rbacInjected := false
	if err := injectRBACPermissions(data); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not auto-inject RBAC: %v\n", err)
		fmt.Println("  Manual step: add permissions to rbac.go and procedure entries to rbac_interceptor.go")
	} else {
		fmt.Printf("  registered: internal/shared/middleware/rbac.go + rbac_interceptor.go\n")
		rbacInjected = true
	}

	// Print next steps.
	fmt.Printf("\n\033[32m✓ Module '%s' scaffolded successfully! (%d files)\033[0m\n\n", data.Name, len(files))
	fmt.Println("Next steps:")
	fmt.Printf("  1. Customize proto fields:      proto/%s/v1/%s.proto\n", data.Name, data.Name)
	fmt.Printf("  2. Customize DB columns:        db/migrations/%s_create_%s.sql\n", data.Timestamp, data.NamePlural)
	fmt.Printf("  3. Customize SQL queries:        db/queries/%s.sql\n", data.Name)
	fmt.Println("  4. Run code generation (required before tests compile): task generate")
	fmt.Printf("  5. Update generated code:        toDomain(), Create/UpdateParams, toProto()\n")
	fmt.Printf("  6. Extend event structs if needed: internal/modules/%s/domain/events.go\n", data.Name)
	step := 7
	if !autoRegistered {
		fmt.Printf("  %d. Register module in:           cmd/server/main.go\n", step)
		step++
	}
	if !rbacInjected {
		fmt.Printf("  %d. Add RBAC entries:             rbac.go + rbac_interceptor.go\n", step)
		step++
	}
	fmt.Printf("  %d. Run:                          task migrate:up && task check\n", step)
}

// injectModuleRegistration adds the module import and fx registration to cmd/server/main.go.
func injectModuleRegistration(data ModuleData) error {
	mainPath := filepath.Join("cmd", "server", "main.go")
	content, err := os.ReadFile(mainPath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", mainPath, err)
	}

	src := string(content)
	marker := "// ADD_MODULE_HERE"
	if !strings.Contains(src, marker) {
		return fmt.Errorf("marker %q not found in %s", marker, mainPath)
	}

	// Replace marker with module registration + marker (keep marker for future scaffolds).
	registration := fmt.Sprintf("\t\t%s.Module,\n\t\t%s", data.NamePackage, marker)
	src = strings.Replace(src, marker, registration, 1)

	// Inject import line after the last "internal/modules/" import in the import block.
	importLine := fmt.Sprintf("\t\"%s/internal/modules/%s\"", data.GoModule, data.Name)
	moduleImportPattern := "\"" + data.GoModule + "/internal/modules/"
	lastIdx := strings.LastIndex(src, moduleImportPattern)
	if lastIdx >= 0 {
		eol := strings.Index(src[lastIdx:], "\n")
		insertAt := lastIdx + eol + 1
		src = src[:insertAt] + importLine + "\n" + src[insertAt:]
	}

	if err := os.WriteFile(mainPath, []byte(src), 0600); err != nil {
		return fmt.Errorf("writing %s: %w", mainPath, err)
	}
	return nil
}

// injectRBACPermissions adds permission constants to rbac.go and procedure
// mappings to rbac_interceptor.go using the ADD_PERMISSION_HERE and
// ADD_PROCEDURE_PERMISSION_HERE markers.
func injectRBACPermissions(data ModuleData) error {
	// 1. Inject permission constants into rbac.go
	rbacPath := filepath.Join("internal", "shared", "middleware", "rbac.go")
	content, err := os.ReadFile(rbacPath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", rbacPath, err)
	}
	src := string(content)
	permMarker := "// ADD_PERMISSION_HERE"
	if !strings.Contains(src, permMarker) {
		return fmt.Errorf("marker %q not found in %s", permMarker, rbacPath)
	}
	permBlock := fmt.Sprintf(
		"Perm%sRead   Permission = \"%s:read\"\n\tPerm%sWrite  Permission = \"%s:write\"\n\tPerm%sDelete Permission = \"%s:delete\"\n\t%s",
		data.NameTitle, data.Name, data.NameTitle, data.Name, data.NameTitle, data.Name, permMarker,
	)
	src = strings.Replace(src, permMarker, permBlock, 1)
	if err := os.WriteFile(rbacPath, []byte(src), 0600); err != nil {
		return fmt.Errorf("writing %s: %w", rbacPath, err)
	}

	// 2. Inject procedure permission mappings into rbac_interceptor.go
	interceptorPath := filepath.Join("internal", "shared", "middleware", "rbac_interceptor.go")
	content, err = os.ReadFile(interceptorPath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", interceptorPath, err)
	}
	src = string(content)
	procMarker := "// ADD_PROCEDURE_PERMISSION_HERE"
	if !strings.Contains(src, procMarker) {
		return fmt.Errorf("marker %q not found in %s", procMarker, interceptorPath)
	}

	// Add import for the new module's connect package.
	connectPkg := fmt.Sprintf("%sv1connect", data.Name)
	connectImport := fmt.Sprintf("\t%s \"%s/gen/proto/%s/v1/%sv1connect\"", connectPkg, data.GoModule, data.Name, data.Name)
	// Insert after the last v1connect import line.
	lastConnect := strings.LastIndex(src, "v1connect\"")
	if lastConnect >= 0 {
		eol := strings.Index(src[lastConnect:], "\n")
		insertAt := lastConnect + eol + 1
		src = src[:insertAt] + connectImport + "\n" + src[insertAt:]
	}

	procBlock := fmt.Sprintf(
		"%s.%sServiceGet%sProcedure:    Perm%sRead,\n\t%s.%sServiceList%sProcedure:  Perm%sRead,\n\t%s.%sServiceCreate%sProcedure: Perm%sWrite,\n\t%s.%sServiceUpdate%sProcedure: Perm%sWrite,\n\t%s.%sServiceDelete%sProcedure: Perm%sDelete,\n\t%s",
		connectPkg, data.NameTitle, data.NameTitle, data.NameTitle,
		connectPkg, data.NameTitle, data.NamePluralTitle, data.NameTitle,
		connectPkg, data.NameTitle, data.NameTitle, data.NameTitle,
		connectPkg, data.NameTitle, data.NameTitle, data.NameTitle,
		connectPkg, data.NameTitle, data.NameTitle, data.NameTitle,
		procMarker,
	)
	src = strings.Replace(src, procMarker, procBlock, 1)
	if err := os.WriteFile(interceptorPath, []byte(src), 0600); err != nil {
		return fmt.Errorf("writing %s: %w", interceptorPath, err)
	}

	return nil
}

// validateIdentifier checks that s is a valid lowercase Go identifier.
func validateIdentifier(s, label string) {
	if s == "" {
		return
	}
	for i, r := range s {
		if i == 0 {
			if !unicode.IsLetter(r) && r != '_' {
				fmt.Fprintf(os.Stderr, "error: %s must start with a letter or underscore, got %q\n", label, s)
				os.Exit(1)
			}
		} else if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			fmt.Fprintf(os.Stderr, "error: %s must use snake_case (letters, digits, underscores), got %q\n", label, s)
			os.Exit(1)
		}
	}
	if s != strings.ToLower(s) {
		fmt.Fprintf(os.Stderr, "error: %s must use snake_case (e.g. order_item), got %q\n", label, s)
		os.Exit(1)
	}
	if strings.HasPrefix(s, "_") || strings.HasSuffix(s, "_") {
		fmt.Fprintf(os.Stderr, "error: %s must use snake_case (e.g. order_item), got %q\n", label, s)
		os.Exit(1)
	}
}

// readGoModule extracts the module path from go.mod.
func readGoModule() string {
	f, err := os.Open("go.mod")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading go.mod: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	if scanner.Scan() {
		if mod, ok := strings.CutPrefix(scanner.Text(), "module "); ok {
			return strings.TrimSpace(mod)
		}
	}
	fmt.Fprintln(os.Stderr, "error: could not parse module path from go.mod")
	os.Exit(1)
	return ""
}

// toTitle converts snake_case to PascalCase: "order_item" → "OrderItem".
func toTitle(s string) string {
	parts := strings.Split(s, "_")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, "")
}
