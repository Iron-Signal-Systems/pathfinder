package cisakev

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

func FuzzBoundedReadCloser(f *testing.F) {
	f.Add([]byte("12345"), uint16(5))
	f.Add([]byte("123456"), uint16(5))
	f.Add([]byte{}, uint16(0))
	f.Add([]byte("x"), uint16(0))

	f.Fuzz(func(t *testing.T, input []byte, rawLimit uint16) {
		const maxInput = 8192
		if len(input) > maxInput {
			input = input[:maxInput]
		}

		limit := int64(rawLimit % (maxInput + 1))

		reader := &boundedReadCloser{
			body:      io.NopCloser(bytes.NewReader(input)),
			remaining: limit,
		}
		defer reader.Close()

		output, err := io.ReadAll(reader)

		if int64(len(input)) <= limit {
			if err != nil {
				t.Fatalf(
					"len=%d limit=%d unexpected error: %v",
					len(input),
					limit,
					err,
				)
			}
			if !bytes.Equal(output, input) {
				t.Fatalf(
					"bounded reader changed bytes: got=%d want=%d",
					len(output),
					len(input),
				)
			}
			return
		}

		if !errors.Is(err, ErrResponseTooLarge) {
			t.Fatalf(
				"len=%d limit=%d error=%v, want ErrResponseTooLarge",
				len(input),
				limit,
				err,
			)
		}
		if int64(len(output)) != limit {
			t.Fatalf(
				"oversized output bytes=%d, want limit=%d",
				len(output),
				limit,
			)
		}
		if !bytes.Equal(output, input[:limit]) {
			t.Fatal("oversized response prefix was altered")
		}
	})
}
