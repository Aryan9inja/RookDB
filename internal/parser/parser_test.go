package parser

import (
	"errors"
	"io"
	"strings"
	"testing"
)

func TestParser_ReadCommand(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedCmd   string
		expectedKey   string
		expectedVal   string
		expectedError string
	}{
		{
			name:        "valid command with 3 parts",
			input:       "Set name Aryan\n",
			expectedCmd: "SET",
			expectedKey: "name",
			expectedVal: "Aryan",
		},
		{
			name:        "valid command with EOF and no trailing newline",
			input:       "Set name Aryan", // No \n at the end
			expectedCmd: "SET",
			expectedKey: "name",
			expectedVal: "Aryan",
		},
		{
			name:          "too few arguments",
			input:         "Set\n",
			expectedError: "parser: too few arguments",
		},
		{
			name:          "too many arguments",
			input:         "Set name Aryan extra\n",
			expectedError: "parser: too many arguments",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser(strings.NewReader(tt.input))
			cmd, key, val, err := p.ReadCommand()

			if tt.expectedError != "" {
				if err == nil || err.Error() != tt.expectedError {
					t.Fatalf("expected error %q, got %v", tt.expectedError, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if cmd != tt.expectedCmd || key != tt.expectedKey || val != tt.expectedVal {
				t.Fatalf("got (%q, %q, %q), expected (%q, %q, %q)",
					cmd, key, val, tt.expectedCmd, tt.expectedKey, tt.expectedVal)
			}
		})
	}

	t.Run("multiple valid commands", func(t *testing.T) {
		input := "Set name Aryan\nGet name\n"
		p := NewParser(strings.NewReader(input))

		// 1st Command
		cmd1, key1, val1, err1 := p.ReadCommand()
		if err1 != nil {
			t.Fatalf("unexpected error on command 1: %v", err1)
		}
		if cmd1 != "SET" || key1 != "name" || val1 != "Aryan" {
			t.Errorf("command 1 failed: got (%q, %q, %q)", cmd1, key1, val1)
		}

		// 2nd Command
		cmd2, key2, val2, err2 := p.ReadCommand()
		if err2 != nil {
			t.Fatalf("unexpected error on command 2: %v", err2)
		}
		if cmd2 != "GET" || key2 != "name" || val2 != "" {
			t.Errorf("command 2 failed: got (%q, %q, %q)", cmd2, key2, val2)
		}

		// 3rd Read should trigger EOF
		_, _, _, err3 := p.ReadCommand()
		if !errors.Is(err3, io.EOF) {
			t.Fatalf("expected io.EOF on finish, got %v", err3)
		}
	})
}
