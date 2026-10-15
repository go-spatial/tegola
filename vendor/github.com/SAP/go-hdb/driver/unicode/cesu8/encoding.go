package cesu8

import (
	"context"
	"fmt"
	"runtime/pprof"
	"unicode"
	"unicode/utf8"

	"github.com/SAP/go-hdb/driver/internal/profile"
	"golang.org/x/text/transform"
)

// Encoding constants.
const (
	UTF8  = "UTF-8"
	CESU8 = "CESU-8"
)

// NumChar functions:
// CESU-8 surrogate pairs count as 2 characters in the HANA server's rune count.

// NumCharTransformer implementors support providing HANA protocol numchar count.
type NumCharTransformer interface {
	TransformNumChar(dst, src []byte, atEOF bool) (nDst, nSrc, numChar int, err error)
}

// NumChar returns the HANA protocol numchar count for transformers not implementing NumCharTransformer.
func NumChar(b []byte) int {
	n := 0
	for len(b) > 0 {
		_, size := utf8.DecodeRune(b)
		b = b[size:]
		if size == 4 {
			n += 2
		} else {
			n++
		}
	}
	return n
}

// DecodeError is raised when a transformer detects invalid encoded data.
type DecodeError struct {
	enc string // encoding
	p   int    // position of error in value
	v   []byte // value
}

func newDecodeError(enc string, p int, v []byte) *DecodeError {
	// copy value
	cv := make([]byte, len(v))
	copy(cv, v)
	return &DecodeError{enc: enc, p: p, v: cv}
}

func (e *DecodeError) Error() string {
	return fmt.Sprintf("invalid %s: %x at position %d", e.enc, e.v, e.p)
}

// Enc returns the expected encoding of the erroneous data.
func (e *DecodeError) Enc() string { return e.enc }

// Pos returns the position of the invalid rune.
func (e *DecodeError) Pos() int { return e.p }

// Value returns the value which should be decoded.
func (e *DecodeError) Value() []byte { return e.v }

// Encoder supports encoding of UTF-8 encoded data into CESU-8.
type Encoder struct {
	transform.NopResetter // stateless
	errorHandler          func(err *DecodeError) (rune, error)
}

type profileEncoder struct {
	*Encoder
}

func newProfileEncoder(encoder *Encoder) transform.Transformer {
	return &profileEncoder{Encoder: encoder}
}

func (d *profileEncoder) Transform(dst, src []byte, atEOF bool) (nDst, nSrc int, err error) {
	pprof.Do(context.Background(), pprof.Labels("cesu8", "encode"), func(ctx context.Context) {
		nDst, nSrc, err = d.Encoder.Transform(dst, src, atEOF)
	})
	return
}

// NewEncoder creates a new encoder instance. The errorHandler parameter allows a custom error-handling function to be used if
// the encoder detects invalid UTF-8 encoded characters.
func NewEncoder(errorHandler func(err *DecodeError) (rune, error)) *Encoder {
	return &Encoder{errorHandler: errorHandler}
}

// Transform implements the transform.Transformer interface.
func (e *Encoder) Transform(dst, src []byte, atEOF bool) (nDst, nSrc int, err error) {
	i, j := 0, 0
	for i < len(src) {
		if src[i] < utf8.RuneSelf {
			if j >= len(dst) {
				return j, i, transform.ErrShortDst
			}
			dst[j] = src[i]
			i++
			j++
			continue
		}
		// check if additional bytes needed (ErrShortSrc) only
		// - if further bytes are potentially available (!atEOF) and
		// - remaining buffer smaller than max size for an encoded UTF-8 rune
		if !atEOF && len(src[i:]) < utf8.UTFMax {
			if !utf8.FullRune(src[i:]) {
				return j, i, transform.ErrShortSrc
			}
		}
		r, n := utf8.DecodeRune(src[i:])
		// invalid UTF-8 cases:
		// - if p is empty it returns (RuneError, 0)
		// - otherwise, if the encoding is invalid, it returns (RuneError, 1)
		if (n == 0 || n == 1) && r == utf8.RuneError {
			decodeErr := newDecodeError(UTF8, i, src)
			if e.errorHandler == nil {
				return j, i, decodeErr
			}
			r, err = e.errorHandler(decodeErr)
			if err != nil {
				return j, i, err
			}
		}
		m := RuneLen(r)
		switch {
		case m == -1:
			panic("internal UTF-8 to CESU-8 transformation error")
		case j+m > len(dst):
			return j, i, transform.ErrShortDst
		}
		EncodeRune(dst[j:], r)
		i += n
		j += m
	}
	return j, i, nil
}

// Decoder supports decoding of CESU-8 encoded data into UTF-8.
type Decoder struct {
	transform.NopResetter // stateless
	errorHandler          func(err *DecodeError) (rune, error)
}

type profileDecoder struct {
	*Decoder
}

func newProfileDecoder(decoder *Decoder) transform.Transformer {
	return &profileDecoder{Decoder: decoder}
}

func (d *profileDecoder) TransformNumChar(dst, src []byte, atEOF bool) (nDst, nSrc, numChar int, err error) {
	pprof.Do(context.Background(), pprof.Labels("cesu8", "decode"), func(ctx context.Context) {
		nDst, nSrc, numChar, err = d.Decoder.TransformNumChar(dst, src, atEOF)
	})
	return
}

func (d *profileDecoder) Transform(dst, src []byte, atEOF bool) (nDst, nSrc int, err error) {
	pprof.Do(context.Background(), pprof.Labels("cesu8", "decode"), func(ctx context.Context) {
		nDst, nSrc, err = d.Decoder.Transform(dst, src, atEOF)
	})
	return
}

// NewDecoder creates a new decoder instance. The errorHandler parameter allows a custom error-handling function to be used if
// the decoder detects invalid CESU-8 encoded characters.
func NewDecoder(errorHandler func(err *DecodeError) (rune, error)) *Decoder {
	return &Decoder{errorHandler: errorHandler}
}

func (d *Decoder) handleDecodeError(r rune, i int, src []byte) (rune, error) {
	decodeErr := newDecodeError(CESU8, i, src)
	if d.errorHandler == nil {
		return r, decodeErr
	}
	return d.errorHandler(decodeErr)
}

// TransformNumChar implements the extended NumCharTransformer interface.
func (d *Decoder) TransformNumChar(dst, src []byte, atEOF bool) (nDst, nSrc, numChar int, err error) {
	i, j := 0, 0
	for i < len(src) {
		if src[i] < utf8.RuneSelf {
			if j >= len(dst) {
				return j, i, numChar, transform.ErrShortDst
			}
			dst[j] = src[i]
			i++
			j++
			numChar++
			continue
		}
		p := src[i:]
		// check if additional bytes needed (ErrShortSrc) only
		// - if further bytes are potentially available (!atEOF) and
		// - remaining buffer smaller than max size for an encoded CESU-8 rune
		if !atEOF && len(p) < CESUMax {
			if !FullRune(p) {
				return j, i, numChar, transform.ErrShortSrc
			}
		}
		/*
			cannot use DecodeRune as we cannot distinguish between
			.unicode replacement character and
			.invalid surrogate
			r, n := DecodeRune(src[i:])
		*/
		var r rune
		var n int
		if !isSurrogate(p) {
			if r, n = utf8.DecodeRune(p); r == utf8.RuneError && (n == 0 || n == 1) {
				if r, err = d.handleDecodeError(r, i, src); err != nil {
					return j, i, numChar, err
				}
			}
		} else {
			if r, n = decodeSurrogates(p); r == utf8.RuneError {
				if r, err = d.handleDecodeError(r, i, src); err != nil {
					return j, i, numChar, err
				}
			}
		}
		m := utf8.RuneLen(r)
		switch {
		case m == -1:
			panic("internal CESU-8 to UTF-8 transformation error")
		case j+m > len(dst):
			return j, i, numChar, transform.ErrShortDst
		}
		utf8.EncodeRune(dst[j:], r)
		i += n
		j += m
		// numChar count
		if m == 4 {
			numChar += 2
		} else {
			numChar++
		}
	}
	return j, i, numChar, nil
}

// Transform implements the transform.Transformer interface.
func (d *Decoder) Transform(dst, src []byte, atEOF bool) (nDst, nSrc int, err error) {
	nDst, nSrc, _, err = d.TransformNumChar(dst, src, atEOF)
	return nDst, nSrc, err
}

var (
	defaultDecoder        = NewDecoder(nil)
	defaultEncoder        = NewEncoder(nil)
	defaultProfileDecoder = newProfileDecoder(defaultDecoder)
	defaultProfileEncoder = newProfileEncoder(defaultEncoder)
)

// DefaultDecoder returns the default CESU-8 to UTF-8 decoder.
// Profile decision must be made here as profile.Active is set after package init.
func DefaultDecoder() transform.Transformer {
	if profile.Active {
		return defaultProfileDecoder
	}
	return defaultDecoder
}

// DefaultEncoder returns the default UTF-8 to CESU-8 encoder.
// Profile decision must be made here as profile.Active is set after package init.
func DefaultEncoder() transform.Transformer {
	if profile.Active {
		return defaultProfileEncoder
	}
	return defaultEncoder
}

// ReplaceErrorHandler is a decoding error handling function replacing invalid CESU-8 data with the
// unicode replacement character '\uFFFD'.
func ReplaceErrorHandler(err *DecodeError) (rune, error) { return unicode.ReplacementChar, nil }
