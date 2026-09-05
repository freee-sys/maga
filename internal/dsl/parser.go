package dsl

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// maxPipelineLength bounds how many steps a single pipeline may contain.
// Hostnames are short (<=253 chars) and transforms are cosmetic, so there
// is no legitimate use case for a longer chain; the cap exists purely to
// keep pathological input cheap to reject.
const maxPipelineLength = 20

// Parse reads the pipe-delimited text syntax (e.g.
// `trim() | replace("-old-", "-new-")`) into a Pipeline. An empty or
// whitespace-only input parses to an empty Pipeline, not an error.
func Parse(src string) (Pipeline, error) {
	p := &parser{input: []rune(src)}
	pipeline, err := p.parsePipeline()
	if err != nil {
		return nil, err
	}
	if len(pipeline) > maxPipelineLength {
		return nil, fmt.Errorf("pipeline has %d steps, exceeds maximum of %d", len(pipeline), maxPipelineLength)
	}
	return pipeline, nil
}

type parser struct {
	input []rune
	pos   int
}

func (p *parser) parsePipeline() (Pipeline, error) {
	p.skipSpace()
	if p.pos >= len(p.input) {
		return Pipeline{}, nil
	}

	var pipeline Pipeline
	for {
		op, err := p.parseStep()
		if err != nil {
			return nil, err
		}
		pipeline = append(pipeline, op)

		p.skipSpace()
		if p.pos >= len(p.input) {
			break
		}
		if p.input[p.pos] != '|' {
			return nil, fmt.Errorf("unexpected character %q at position %d, expected '|'", p.input[p.pos], p.pos)
		}
		p.pos++
		p.skipSpace()
	}
	return pipeline, nil
}

func (p *parser) parseStep() (Op, error) {
	name, err := p.parseIdent()
	if err != nil {
		return Op{}, err
	}
	p.skipSpace()
	if p.pos >= len(p.input) || p.input[p.pos] != '(' {
		return Op{}, fmt.Errorf("expected '(' after op name %q", name)
	}
	p.pos++
	p.skipSpace()

	var args []Arg
	if p.pos < len(p.input) && p.input[p.pos] != ')' {
		for {
			arg, err := p.parseArg()
			if err != nil {
				return Op{}, err
			}
			args = append(args, arg)

			p.skipSpace()
			if p.pos >= len(p.input) {
				return Op{}, fmt.Errorf("unterminated argument list for op %q", name)
			}
			if p.input[p.pos] == ',' {
				p.pos++
				p.skipSpace()
				continue
			}
			break
		}
	}

	if p.pos >= len(p.input) || p.input[p.pos] != ')' {
		return Op{}, fmt.Errorf("expected ')' to close op %q", name)
	}
	p.pos++

	return Op{Name: name, Args: args}, nil
}

func (p *parser) parseIdent() (string, error) {
	start := p.pos
	if p.pos >= len(p.input) || !isIdentStart(p.input[p.pos]) {
		return "", fmt.Errorf("expected identifier at position %d", p.pos)
	}
	p.pos++
	for p.pos < len(p.input) && isIdentPart(p.input[p.pos]) {
		p.pos++
	}
	return string(p.input[start:p.pos]), nil
}

func (p *parser) parseArg() (Arg, error) {
	if p.pos >= len(p.input) {
		return Arg{}, fmt.Errorf("unexpected end of input, expected argument")
	}
	switch {
	case p.input[p.pos] == '"' || p.input[p.pos] == '\'':
		s, err := p.parseString()
		if err != nil {
			return Arg{}, err
		}
		return Arg{S: &s}, nil
	case p.input[p.pos] == '-' || unicode.IsDigit(p.input[p.pos]):
		n, err := p.parseInt()
		if err != nil {
			return Arg{}, err
		}
		return Arg{I: &n}, nil
	default:
		return Arg{}, fmt.Errorf("unexpected character %q at position %d, expected string or integer argument", p.input[p.pos], p.pos)
	}
}

func (p *parser) parseString() (string, error) {
	quote := p.input[p.pos]
	p.pos++
	var b strings.Builder
	for {
		if p.pos >= len(p.input) {
			return "", fmt.Errorf("unterminated string literal")
		}
		c := p.input[p.pos]
		if c == '\\' {
			p.pos++
			if p.pos >= len(p.input) {
				return "", fmt.Errorf("unterminated escape sequence in string literal")
			}
			b.WriteRune(p.input[p.pos])
			p.pos++
			continue
		}
		if c == quote {
			p.pos++
			return b.String(), nil
		}
		b.WriteRune(c)
		p.pos++
	}
}

func (p *parser) parseInt() (int, error) {
	start := p.pos
	if p.input[p.pos] == '-' {
		p.pos++
	}
	digitsStart := p.pos
	for p.pos < len(p.input) && unicode.IsDigit(p.input[p.pos]) {
		p.pos++
	}
	if p.pos == digitsStart {
		return 0, fmt.Errorf("expected digits in integer literal at position %d", digitsStart)
	}
	n, err := strconv.Atoi(string(p.input[start:p.pos]))
	if err != nil {
		return 0, fmt.Errorf("invalid integer literal: %w", err)
	}
	return n, nil
}

func (p *parser) skipSpace() {
	for p.pos < len(p.input) && unicode.IsSpace(p.input[p.pos]) {
		p.pos++
	}
}

func isIdentStart(r rune) bool { return unicode.IsLetter(r) || r == '_' }
func isIdentPart(r rune) bool  { return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' }
