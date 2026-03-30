package architecture_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestLayerBoundaries(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	internalRoot := filepath.Join(root, "internal")

	var violations []string
	err := filepath.WalkDir(internalRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)

		role, forbidden := boundaryRule(rel)
		if role == "" {
			return nil
		}

		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range file.Imports {
			importPath := strings.Trim(imp.Path.Value, "\"")
			if forbidden(importPath) {
				violations = append(violations, rel+" ("+role+") imports "+importPath)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk boundaries: %v", err)
	}

	if len(violations) > 0 {
		slices.Sort(violations)
		t.Fatalf("layer boundary violations:\n%s", strings.Join(violations, "\n"))
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve caller")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../.."))
}

func boundaryRule(rel string) (string, func(string) bool) {
	if strings.HasPrefix(rel, "internal/infrastructure/") {
		return "", nil
	}

	switch {
	case strings.Contains(rel, "/domain/"):
		return "domain", isForbiddenDomainImport
	case strings.Contains(rel, "/services/"):
		return "services", isForbiddenAppLayerImport
	case strings.Contains(rel, "/handlers/"):
		return "handlers", isForbiddenAppLayerImport
	case strings.Contains(rel, "/resolvers/"):
		return "resolvers", isForbiddenResolverImport
	default:
		return "", nil
	}
}

func isForbiddenDomainImport(importPath string) bool {
	if importPath == "database/sql" || importPath == "github.com/jmoiron/sqlx" {
		return true
	}
	return strings.HasPrefix(importPath, "github.com/movebigrocks/platform/internal/infrastructure/")
}

func isForbiddenAppLayerImport(importPath string) bool {
	if importPath == "database/sql" || importPath == "github.com/jmoiron/sqlx" {
		return true
	}
	return importPath == "github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores/sql" ||
		strings.HasPrefix(importPath, "github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores/sql/") ||
		strings.Contains(importPath, "/resolvers/")
}

func isForbiddenResolverImport(importPath string) bool {
	if isForbiddenAppLayerImport(importPath) {
		return true
	}
	return importPath == "github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores/shared" ||
		strings.HasPrefix(importPath, "github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores/shared/")
}

func TestDomainExportedTypesHaveNoTransportTags(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	internalRoot := filepath.Join(root, "internal")

	var violations []string
	err := filepath.WalkDir(internalRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if !strings.Contains(rel, "/domain/") {
			return nil
		}

		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ParseComments)
		if err != nil {
			return err
		}

		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				structType, ok := typeSpec.Type.(*ast.StructType)
				if !ok {
					continue
				}
				if !ast.IsExported(typeSpec.Name.Name) && strings.HasSuffix(typeSpec.Name.Name, "JSON") {
					continue
				}
				for _, field := range structType.Fields.List {
					if field.Tag == nil {
						continue
					}
					violations = append(violations, rel+" type "+typeSpec.Name.Name+" has transport tag "+field.Tag.Value)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk domain tags: %v", err)
	}

	if len(violations) > 0 {
		slices.Sort(violations)
		t.Fatalf("domain transport-tag violations:\n%s", strings.Join(violations, "\n"))
	}
}
