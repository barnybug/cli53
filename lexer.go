package cli53

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

type lexer struct {
	input string
	start int
	pos   int
	width int
}

const eof = -1

func lex(input string) *lexer {
	l := &lexer{input: input}
	return l
}

func (l *lexer) emit() string {
	ret := l.input[l.start:l.pos]
	l.start = l.pos
	return ret
}

func (l *lexer) accept(valid string) bool {
	if strings.IndexRune(valid, l.next()) >= 0 {
		l.emit()
		return true
	}
	l.backup()
	return false
}

func (l *lexer) acceptAny() string {
	l.next()
	return l.emit()
}

func (l *lexer) acceptRun(pred func(rune) bool) string {
	for pred(l.next()) {
	}
	l.backup()
	return l.emit()
}

func (l *lexer) next() (rune rune) {
	if l.pos >= len(l.input) {
		l.width = 0
		return eof
	}
	rune, l.width = utf8.DecodeRuneInString(l.input[l.pos:])
	l.pos += l.width
	return rune
}

func (l *lexer) backup() {
	l.pos -= l.width
}

func (l *lexer) eof() bool {
	return l.pos >= len(l.input)
}

func (l *lexer) Error(msg string) error {
	return fmt.Errorf("%s: %s[%s]%s", msg, l.input[0:l.start], l.input[l.start:l.pos], l.input[l.pos:])
}
