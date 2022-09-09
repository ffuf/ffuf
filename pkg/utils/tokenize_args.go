package utils

import (
	"fmt"
	"strings"
)

func TokenizeArgs(args string) ([]string, error) {

	tokenizer := newArgsTokenizer()

	for _, r := range args {
		tokenizer.consume(r)
	}
	if tokenizer.builder.Len() != 0 {
		tokenizer.tokens = append(tokenizer.tokens, tokenizer.builder.String())
		tokenizer.builder.Reset()
	}

	return tokenizer.getTokens()
}

///////////////
// AUXILIARY //
///////////////

type argsTokenizer struct {
	tokens  []string
	builder *strings.Builder
	consume consume_func
	state   tokenizer_state
}

func newArgsTokenizer() (tok *argsTokenizer) {

	tok = new(argsTokenizer)

	tok.tokens = make([]string, 0, 20)
	tok.builder = new(strings.Builder)
	tok.consume = tok.consumUntilWhiteSpace
	tok.state = white_space_state

	return tok
}

func (t *argsTokenizer) getTokens() ([]string, error) {

	switch t.state {
	case double_quote_state:
		return nil, fmt.Errorf("malformed configuration string: unbalanced double quotes. Either escape the quote or put it inside single quotes")
	case single_quote_state:
		return nil, fmt.Errorf("malformed configuration string: unbalanced double quotes. Either escape the quote or put it inside double quotes")
	case escape_state:
		return nil, fmt.Errorf("malformed configuration string: misplaced escape character")
	default:
		return t.tokens, nil
	}

}

const (
	white_space_state tokenizer_state = iota
	double_quote_state
	single_quote_state
	escape_state
)

type consume_func func(r rune)

type tokenizer_state int

func (t *argsTokenizer) consumUntilWhiteSpace(r rune) {

	// state transitions
	switch r {
	case 9, 10, 13, 32: // tab, linefeed, carriage return, space
		if t.builder.Len() != 0 {
			t.tokens = append(t.tokens, t.builder.String())
			t.builder.Reset()
		}
	case 34: // double quote
		t.state = double_quote_state
		t.consume = t.consumeUntilDoubleQuote
	case 39: // single quote
		t.state = single_quote_state
		t.consume = t.consumeUntilSingleQuote
	case 92: // backslash
		t.state = escape_state
		t.consume = t.consumeAny
	default:
		t.builder.WriteRune(r)
	}

}

func (t *argsTokenizer) consumeUntilDoubleQuote(r rune) {
	if r == 34 {
		t.tokens = append(t.tokens, t.builder.String())
		t.builder.Reset()
		t.state = white_space_state
		t.consume = t.consumUntilWhiteSpace
	} else {
		t.builder.WriteRune(r)
	}
}

func (t *argsTokenizer) consumeUntilSingleQuote(r rune) {
	if r == 39 {
		t.tokens = append(t.tokens, t.builder.String())
		t.builder.Reset()
		t.state = white_space_state
		t.consume = t.consumUntilWhiteSpace
	} else {
		t.builder.WriteRune(r)
	}

}

func (t *argsTokenizer) consumeAny(r rune) {
	t.builder.WriteRune(r)
	t.state = white_space_state
	t.consume = t.consumUntilWhiteSpace
}
