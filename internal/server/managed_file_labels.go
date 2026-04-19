package server

import (
	"path/filepath"
	"strings"

	"github.com/bioflowy/flowy/wdl/ast"
)

type labelTemplateOp int

const (
	labelTemplateReplace labelTemplateOp = iota
	labelTemplateTrimSuffixOnce
	labelTemplateTrimSuffixAll
	labelTemplateTrimPrefixOnce
	labelTemplateTrimPrefixAll
)

type labelTemplateExpansion struct {
	name    string
	op      labelTemplateOp
	operand string
}

func managedFileLabelForPath(path string, template *string, wf *ast.Workflow, inputs map[string]interface{}) string {
	fallback := filepath.Base(path)
	if template == nil || strings.TrimSpace(*template) == "" {
		return fallback
	}

	evaluated, ok := evaluateManagedFileLabelTemplate(*template, workflowInputStrings(wf, inputs))
	if !ok || strings.TrimSpace(evaluated) == "" {
		return fallback
	}
	return evaluated
}

func workflowInputStrings(wf *ast.Workflow, inputs map[string]interface{}) map[string]string {
	if wf == nil || len(inputs) == 0 {
		return nil
	}

	allowed := make(map[string]struct{}, len(wf.Inputs))
	for _, decl := range wf.Inputs {
		allowed[decl.Name] = struct{}{}
	}

	values := make(map[string]string, len(inputs))
	for name, raw := range inputs {
		if _, ok := allowed[name]; !ok {
			continue
		}
		value, ok := raw.(string)
		if !ok {
			continue
		}
		values[name] = value
	}
	return values
}

func evaluateManagedFileLabelTemplate(template string, vars map[string]string) (string, bool) {
	var out strings.Builder
	rest := template
	for len(rest) > 0 {
		start := strings.Index(rest, "${")
		if start == -1 {
			out.WriteString(rest)
			break
		}

		out.WriteString(rest[:start])
		rest = rest[start+2:]

		end := strings.IndexByte(rest, '}')
		if end == -1 {
			return "", false
		}

		expansion, ok := parseLabelTemplateExpansion(rest[:end])
		if !ok {
			return "", false
		}

		value, ok := vars[expansion.name]
		if !ok {
			return "", false
		}
		out.WriteString(applyLabelTemplateExpansion(value, expansion))
		rest = rest[end+1:]
	}

	return out.String(), true
}

func parseLabelTemplateExpansion(expr string) (labelTemplateExpansion, bool) {
	if expr == "" {
		return labelTemplateExpansion{}, false
	}

	nameEnd := len(expr)
	for i, ch := range expr {
		if ch == '%' || ch == '#' {
			nameEnd = i
			break
		}
		if !(ch == '_' || ch >= '0' && ch <= '9' || ch >= 'A' && ch <= 'Z' || ch >= 'a' && ch <= 'z') {
			return labelTemplateExpansion{}, false
		}
	}
	if nameEnd == 0 {
		return labelTemplateExpansion{}, false
	}
	name := expr[:nameEnd]
	if !isLabelTemplateIdentifier(name) {
		return labelTemplateExpansion{}, false
	}
	if nameEnd == len(expr) {
		return labelTemplateExpansion{name: name, op: labelTemplateReplace}, true
	}

	rem := expr[nameEnd:]
	switch {
	case strings.HasPrefix(rem, "%%"):
		if len(rem) == 2 {
			return labelTemplateExpansion{}, false
		}
		return labelTemplateExpansion{name: name, op: labelTemplateTrimSuffixAll, operand: rem[2:]}, true
	case strings.HasPrefix(rem, "%"):
		if len(rem) == 1 {
			return labelTemplateExpansion{}, false
		}
		return labelTemplateExpansion{name: name, op: labelTemplateTrimSuffixOnce, operand: rem[1:]}, true
	case strings.HasPrefix(rem, "##"):
		if len(rem) == 2 {
			return labelTemplateExpansion{}, false
		}
		return labelTemplateExpansion{name: name, op: labelTemplateTrimPrefixAll, operand: rem[2:]}, true
	case strings.HasPrefix(rem, "#"):
		if len(rem) == 1 {
			return labelTemplateExpansion{}, false
		}
		return labelTemplateExpansion{name: name, op: labelTemplateTrimPrefixOnce, operand: rem[1:]}, true
	default:
		return labelTemplateExpansion{}, false
	}
}

func isLabelTemplateIdentifier(name string) bool {
	if name == "" {
		return false
	}
	first := name[0]
	if !(first == '_' || first >= 'A' && first <= 'Z' || first >= 'a' && first <= 'z') {
		return false
	}
	for i := 1; i < len(name); i++ {
		ch := name[i]
		if !(ch == '_' || ch >= '0' && ch <= '9' || ch >= 'A' && ch <= 'Z' || ch >= 'a' && ch <= 'z') {
			return false
		}
	}
	return true
}

func applyLabelTemplateExpansion(value string, expansion labelTemplateExpansion) string {
	switch expansion.op {
	case labelTemplateReplace:
		return value
	case labelTemplateTrimSuffixOnce:
		return trimSuffixOnce(value, expansion.operand)
	case labelTemplateTrimSuffixAll:
		return trimSuffixAll(value, expansion.operand)
	case labelTemplateTrimPrefixOnce:
		return trimPrefixOnce(value, expansion.operand)
	case labelTemplateTrimPrefixAll:
		return trimPrefixAll(value, expansion.operand)
	default:
		return value
	}
}

func trimSuffixOnce(value string, suffix string) string {
	if suffix == "" {
		return value
	}
	return strings.TrimSuffix(value, suffix)
}

func trimSuffixAll(value string, suffix string) string {
	if suffix == "" {
		return value
	}
	for strings.HasSuffix(value, suffix) {
		value = strings.TrimSuffix(value, suffix)
	}
	return value
}

func trimPrefixOnce(value string, prefix string) string {
	if prefix == "" {
		return value
	}
	return strings.TrimPrefix(value, prefix)
}

func trimPrefixAll(value string, prefix string) string {
	if prefix == "" {
		return value
	}
	for strings.HasPrefix(value, prefix) {
		value = strings.TrimPrefix(value, prefix)
	}
	return value
}
