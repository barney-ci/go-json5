package json5

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

type LexingError struct {
	Line   int
	Column int
	Err    error
}

func (e *LexingError) Error() string {
	return fmt.Sprintf("json5: at line %v column %v: %v", e.Line, e.Column, e.Err)
}

func (e *LexingError) Unwrap() error {
	return e.Err
}

// Reader translates on the fly JSON5 documents to equivalent JSON.
//
// Note that the result is not guaranteed to be valid JSON; the reader
// should be fed to an actual json decoder for validation.
type Reader struct {
	rd      io.RuneScanner
	state   stateFunc
	line    int
	col     int
	lastcol int
	quote   rune
	comma   bool
	noident bool
	remain  []byte
	tokens  chan token
}

func NewReader(rd io.Reader) *Reader {
	var scanner io.RuneScanner
	if in, ok := rd.(io.RuneScanner); ok {
		scanner = in
	} else {
		scanner = bufio.NewReader(rd)
	}
	return &Reader{
		rd:      scanner,
		state:   (*Reader).lex,
		line:    1,
		tokens:  make(chan token, 3),
	}
}

func (r *Reader) Read(buf []byte) (int, error) {
	i := copy(buf, r.remain)
	r.remain = nil

	for i < len(buf) {
		tok := r.next()
		switch tok.typ {
		case tokenError:
			return i, tok.err
		case tokenRune:
			var encoded [utf8.UTFMax]byte
			l := utf8.EncodeRune(encoded[:], tok.val)
			copied := copy(buf[i:], encoded[:l])
			if copied < l {
				r.remain = encoded[copied:l]
			}
			i += l
		case tokenNumber:
			copied := copy(buf[i:], tok.num)
			if copied < len(tok.num) {
				r.remain = []byte(tok.num[copied:])
			}
			i += copied
		}
	}
	return i, nil
}

func (r *Reader) next() token {
	for {
		select {
		case tok := <-r.tokens:
			return tok
		default:
			r.state = r.state(r)
		}
	}
}

func (r *Reader) emit(typ tokenType, val rune) {
	r.tokens <- token{typ: typ, val: val}
}

type stateFunc func(*Reader) stateFunc

func (r *Reader) err(err error) stateFunc {
	if err != io.EOF {
		err = &LexingError{Line: r.line, Column: r.col, Err: err}
	}

	var fn func(r *Reader) stateFunc
	fn = func(r *Reader) stateFunc {
		r.tokens <- token{typ: tokenError, err: err}
		return fn
	}
	return fn
}

func (r *Reader) peek() (rune, error) {
	next, err := r.pop()
	r.push()
	return next, err
}

func (r *Reader) pop() (rune, error) {
	next, _, err := r.rd.ReadRune()
	if err != nil {
		return 0, err
	}
	if next == '\n' {
		r.line++
		r.lastcol, r.col = r.col, 0
	} else {
		r.lastcol, r.col = r.col, r.col + 1
	}
	return next, nil
}

func (r *Reader) push() {
	r.rd.UnreadRune()
	r.col = r.lastcol
}

func (r *Reader) maybeEmitComma() {
	if r.comma {
		r.emit(tokenRune, ',')
	}
	r.comma = false
}

func (r *Reader) lex() stateFunc {
	b, err := r.pop()
	if err != nil {
		return r.err(err)
	}
	switch b {
	case '"', '\'':
		r.maybeEmitComma()
		r.quote = b
		r.emit(tokenRune, '"')
		return (*Reader).lexString
	case '/':
		next, err := r.pop()
		if err != nil {
			return r.err(err)
		}
		switch next {
		case '/':
			return (*Reader).lexLineComment
		}
		r.push()
	case ',':
		// omit all commas, we insert them ourselves
		r.comma = true
		r.noident = false
	case '{', '[':
		r.maybeEmitComma()
		r.noident = false
		r.emit(tokenRune, b)
	case '}', ']':
		r.comma = false
		r.noident = false
		r.emit(tokenRune, b)
	case '+':
		// omit leading +
	case '0': // either 0xabcd or 0.1234
		r.maybeEmitComma()
		next, err := r.pop()
		if err != nil {
			return r.err(err)
		}
		if next == 'x' || next == 'X' {
			return (*Reader).lexHex
		}
		r.emit(tokenRune, b)
		r.push()
	case '.':
		r.maybeEmitComma()
		r.emit(tokenRune, '0')
		r.emit(tokenRune, '.')
		return (*Reader).lexNumber
	case ':':
		r.noident = true
		r.emit(tokenRune, ':')
	default:
		if unicode.IsSpace(b) {
			for unicode.IsSpace(b) {
				b, err = r.pop()
				if err != nil {
					return r.err(err)
				}
			}
			r.push()
			return (*Reader).lex
		}
		r.maybeEmitComma()
		if !r.noident && unicode.IsLetter(b) || b == '$' || b == '_' || b == '\\' {
			r.emit(tokenRune, '"')
			r.emit(tokenRune, b)
			return (*Reader).lexIdentifier
		}
		if (b > '0' && b < '9') || b == '.' || b == '+' {
			r.push()
			return (*Reader).lexNumber
		}
		r.emit(tokenRune, b)
	}
	return (*Reader).lex
}

func (r *Reader) lexIdentifier() stateFunc {
	b, err := r.pop()
	if err != nil {
		return r.err(err)
	}
	// https://262.ecma-international.org/5.1/#sec-7.6
	if unicode.In(b, unicode.L, unicode.Nl, unicode.Nd, unicode.Mn, unicode.Mc, unicode.Pc) || b == '$' || b == '_' || b == '\\' || b == '\u200C' || b == '\u200D' {
		r.emit(tokenRune, b)
		return (*Reader).lexIdentifier
	}
	switch b {
	case ':':
		r.emit(tokenRune, '"')
		r.push()
		return (*Reader).lex
	default:
		return r.err(fmt.Errorf("unexpected character %q in identifier", b))
	}
}

func (r *Reader) lexNumber() stateFunc {
	b, err := r.pop()
	if err != nil {
		return r.err(err)
	}
	if b == '.' {
		next, err := r.pop()
		if err != nil {
			return r.err(err)
		}
		if strings.IndexRune("0123456789", next) == -1 {
			r.push()
			r.emit(tokenRune, '.')
			r.emit(tokenRune, '0')
		}
		return (*Reader).lexNumber
	}
	if strings.IndexRune("0123456789eE.+-", b) == -1 {
		r.push()
		return (*Reader).lex
	}
	if b == 'e' || b == 'E' {
		next, err := r.pop()
		if err != nil {
			return r.err(err)
		}
		// discard +, if any
		if next != '+' {
			r.push()
		}
	}
	r.emit(tokenRune, b)
	return (*Reader).lexNumber
}

func (r *Reader) lexHex() stateFunc {
	var out bytes.Buffer
	for {
		b, err := r.pop()
		if err != nil {
			return r.err(err)
		}
		if strings.IndexRune("0123456789abcdefABCDEF", b) == -1 {
			r.push()
			break
		}
		out.WriteRune(b)
	}
	val, err := strconv.ParseInt(out.String(), 16, 64)
	if err != nil {
		panic("programming error: we lexed a non-hexadecimal number")
	}
	r.tokens <- token{typ: tokenNumber, num: strconv.FormatInt(val, 10)}
	return (*Reader).lex
}

func (r *Reader) lexString() stateFunc {
	b, err := r.pop()
	if err != nil {
		return r.err(err)
	}
	switch b {
	case r.quote:
		r.emit(tokenRune, '"')
		return (*Reader).lex
	case '\n', '\r':
		return r.err(errors.New("unexpected newline"))
	case '\\':
		next, err := r.pop()
		if err != nil {
			return r.err(err)
		}
		r.emit(tokenRune, '\\')
		if next == '\n' {
			// support line-escaping for multiline strings
			r.emit(tokenRune, 'n')
		} else {
			r.emit(tokenRune, next)
		}
	case '"':
		// This is only reached in single-quote mode, and therefore
		// a double-quote in that context needs to be escaped.
		r.emit(tokenRune, '\\')
		fallthrough
	default:
		r.emit(tokenRune, b)
	}
	return (*Reader).lexString
}

func (r *Reader) lexLineComment() stateFunc {
	for {
		b, err := r.pop()
		if err != nil {
			return r.err(err)
		}
		if b == '\n' {
			return (*Reader).lex
		}
	}
}

type token struct {
	typ tokenType
	val rune
	num string
	err error
}

type tokenType int

const (
	tokenError tokenType = iota
	tokenRune
	tokenNumber
)
