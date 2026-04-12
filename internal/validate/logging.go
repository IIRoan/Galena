package validate

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const charmLoggerImport = "github.com/charmbracelet/log"

var disallowedLoggerImports = map[string]string{
	"log":                        "stdlib logger",
	"log/slog":                   "stdlib structured logger",
	"github.com/rs/zerolog":      "zerolog logger",
	"github.com/sirupsen/logrus": "logrus logger",
	"go.uber.org/zap":            "zap logger",
	"k8s.io/klog/v2":             "klog logger",
}

var disallowedFmtFunctions = map[string]bool{
	"Print":   true,
	"Printf":  true,
	"Println": true,
}

func pipelineLoggingPolicyApplies(relPath string) bool {
	normalized := filepath.ToSlash(relPath)
	if normalized == "cmd/galena/cmd/ci.go" {
		return true
	}
	if strings.HasPrefix(normalized, "internal/build/") {
		return true
	}
	if strings.HasPrefix(normalized, "internal/exec/") {
		return true
	}
	if strings.HasPrefix(normalized, "internal/validate/") {
		return true
	}
	return false
}

func pipelineLoggingPolicyExempt(relPath string) bool {
	normalized := filepath.ToSlash(relPath)
	return normalized == "internal/ci/github.go"
}

// Logging validates the repository logging policy.
func Logging(_ context.Context, rootDir string) Result {
	result := Result{}

	var scanned int
	var violations []string

	err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".cache", "vendor", "node_modules":
				return filepath.SkipDir
			default:
				return nil
			}
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}

		file, parseErr := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if parseErr != nil {
			relPath, _ := filepath.Rel(rootDir, path)
			result.AddWarning(fmt.Sprintf("could not parse %s: %v", relPath, parseErr))
			return nil
		}
		scanned++

		for _, spec := range file.Imports {
			importPath, unquoteErr := strconv.Unquote(spec.Path.Value)
			if unquoteErr != nil {
				continue
			}
			if importPath == charmLoggerImport {
				continue
			}
			if reason, disallowed := disallowedLoggerImports[importPath]; disallowed {
				relPath, _ := filepath.Rel(rootDir, path)
				violations = append(violations, fmt.Sprintf("%s imports %s (%s)", relPath, importPath, reason))
			}
		}

		relPath, _ := filepath.Rel(rootDir, path)
		if pipelineLoggingPolicyApplies(relPath) && !pipelineLoggingPolicyExempt(relPath) {
			if hasDisallowedFmtPrint(path) {
				violations = append(violations, fmt.Sprintf("%s uses fmt.Print* in pipeline code; use github.com/charmbracelet/log", relPath))
			}
		}
		return nil
	})
	if err != nil {
		result.AddError(fmt.Sprintf("logging validation failed: %v", err))
		result.AddItem(StatusError, "Logging Policy", "scan failed")
		return result
	}

	if len(violations) > 0 {
		sort.Strings(violations)
		result.AddError("logging policy violations found")
		result.AddItem(StatusError, "Logging Policy", fmt.Sprintf("%d disallowed import(s)", len(violations)))
		for _, violation := range violations {
			result.AddWarning(violation)
		}
		return result
	}

	result.AddItem(StatusSuccess, "Logging Policy", fmt.Sprintf("enforced in %d Go file(s)", scanned))
	return result
}

func hasDisallowedFmtPrint(path string) bool {
	src, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	file, err := parser.ParseFile(token.NewFileSet(), path, src, 0)
	if err != nil {
		return false
	}

	hasFmtImport := false
	for _, spec := range file.Imports {
		importPath, unquoteErr := strconv.Unquote(spec.Path.Value)
		if unquoteErr != nil {
			continue
		}
		if importPath == "fmt" {
			hasFmtImport = true
			break
		}
	}
	if !hasFmtImport {
		return false
	}

	found := false
	ast.Inspect(file, func(node ast.Node) bool {
		if found {
			return false
		}
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		x, ok := sel.X.(*ast.Ident)
		if !ok || x.Name != "fmt" {
			return true
		}
		if disallowedFmtFunctions[sel.Sel.Name] {
			found = true
			return false
		}
		return true
	})

	return found
}
