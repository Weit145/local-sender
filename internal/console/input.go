package console

import (
	"bufio"
	"context"
	"io"
)

type line struct {
	text string
	err  error
}

// Input owns the application's single console scanner.
// A blocked terminal read lives until input arrives or the process exits.
// Consumers can stop waiting independently through their context.
type Input struct {
	lines <-chan line
}

func NewInput(ctx context.Context, source io.Reader) *Input {
	lines := make(chan line)
	go func() {
		defer close(lines)
		scanner := bufio.NewScanner(source)
		for scanner.Scan() {
			select {
			case lines <- line{text: scanner.Text()}:
			case <-ctx.Done():
				return
			}
		}
		if err := scanner.Err(); err != nil {
			select {
			case lines <- line{err: err}:
			case <-ctx.Done():
			}
		}
	}()
	return &Input{lines: lines}
}

func (i *Input) Read(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case value, ok := <-i.lines:
		if !ok {
			return "", io.EOF
		}
		return value.text, value.err
	}
}
