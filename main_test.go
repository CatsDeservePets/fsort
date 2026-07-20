package main

import (
	"slices"
	"strings"
	"testing"
)

func TestReadPaths(t *testing.T) {
	tests := []struct {
		name  string
		input string
		delim string
		want  []string
	}{
		{
			name:  "LF",
			input: "file1.txt\nfile2.txt\nfile3.txt\n",
			delim: "\n",
			want:  []string{"file1.txt", "file2.txt", "file3.txt"},
		},
		{
			name:  "CRLF",
			input: "file1.txt\r\nfile2.txt\r\nfile3.txt\r\n",
			delim: "\n",
			want:  []string{"file1.txt", "file2.txt", "file3.txt"},
		},
		{
			name:  "NUL",
			input: "file1.txt\x00file2.txt\x00file3.txt\x00",
			delim: "\x00",
			want:  []string{"file1.txt", "file2.txt", "file3.txt"},
		},
		{
			name:  "NoTrailingDelimiter",
			input: "file1.txt\nfile2.txt\nfile3.txt",
			delim: "\n",
			want:  []string{"file1.txt", "file2.txt", "file3.txt"},
		},
		{
			name:  "EmptyPaths",
			input: "\nfile1.txt\n\nfile2.txt\n\n",
			delim: "\n",
			want:  []string{"file1.txt", "file2.txt"},
		},
		{
			name:  "EmptyInput",
			input: "",
			delim: "\n",
			want:  nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := readPaths(strings.NewReader(test.input), test.delim)
			if err != nil {
				t.Errorf("readPaths(%q, %q) error = %v", test.input, test.delim, err)
				return
			}
			if !slices.Equal(got, test.want) {
				t.Errorf("readPaths(%q, %q) = %v, want %v", test.input, test.delim, got, test.want)
			}
		})
	}
}
