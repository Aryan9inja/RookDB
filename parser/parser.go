package parser

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
)

type Parser struct {
	reader *bufio.Reader
}

func NewParser(r io.Reader) *Parser {
	return &Parser{
		reader: bufio.NewReader(r),
	}
}

func (p *Parser) ReadCommand() (string, string, string, error) {
	line, err := p.reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", "", "", fmt.Errorf("parser: read command: read string: %w", err)
	}
	if len(strings.TrimSpace(line)) == 0 && errors.Is(err, io.EOF) {
		return "", "", "", io.EOF
	}

	parts := strings.Fields(line)
	if len(parts) > 3 {
		return "", "", "", fmt.Errorf("parser: too many arguments")
	}
	if len(parts) < 2 {
		return "", "", "", fmt.Errorf("parser: too few arguments")
	}

	var command, key, value string

	command = parts[0]
	key = parts[1]
	if len(parts) > 2 {
		value = parts[2]
	}

	return command, key, value, nil
}
