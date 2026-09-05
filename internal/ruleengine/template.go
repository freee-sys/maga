package ruleengine

import "strings"

// renderTemplate replaces every {token} occurrence in tmpl with
// values[token]. This is deliberately not text/template: the only
// supported syntax is flat token substitution, nothing more. A token with
// no entry in values renders as an empty string rather than erroring —
// rules are validated against their own capture groups at save time, so
// this path only matters as a defensive fallback.
func renderTemplate(tmpl string, values map[string]string) string {
	var b strings.Builder
	walkTemplateTokens(tmpl, func(literal, token string, isToken bool) {
		if isToken {
			b.WriteString(values[token])
		} else {
			b.WriteString(literal)
		}
	})
	return b.String()
}

// extractTemplateTokens returns every {token} name referenced in tmpl, in
// order of appearance, without substituting anything.
func extractTemplateTokens(tmpl string) []string {
	var tokens []string
	walkTemplateTokens(tmpl, func(_, token string, isToken bool) {
		if isToken {
			tokens = append(tokens, token)
		}
	})
	return tokens
}

// walkTemplateTokens scans tmpl once, calling visit for each literal
// segment (isToken=false, literal set) and each {token} found
// (isToken=true, token set). An unclosed '{' is treated as a trailing
// literal.
func walkTemplateTokens(tmpl string, visit func(literal, token string, isToken bool)) {
	i := 0
	for i < len(tmpl) {
		open := strings.IndexByte(tmpl[i:], '{')
		if open == -1 {
			visit(tmpl[i:], "", false)
			return
		}
		open += i
		if open > i {
			visit(tmpl[i:open], "", false)
		}

		close := strings.IndexByte(tmpl[open:], '}')
		if close == -1 {
			visit(tmpl[open:], "", false)
			return
		}
		close += open

		visit("", tmpl[open+1:close], true)
		i = close + 1
	}
}
