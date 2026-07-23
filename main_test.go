package main

import (
	"slices"
	"strings"
	"testing"
)

func TestParseSortKey(t *testing.T) {
	tests := []struct {
		input string
		want  sortKey
	}{
		// Canonical names.
		{"name", nameKey},
		{"path", pathKey},
		{"extension", extKey},
		{"type", typeKey},
		{"perm", permKey},
		{"size", sizeKey},
		{"time", mtimeKey},

		// Aliases.
		{"ext", extKey},
		{"permission", permKey},
		{"mtime", mtimeKey},

		// Case-insensitive matching.
		{"NaMe", nameKey},
	}

	for _, test := range tests {
		got, err := parseSortKey(test.input)
		if err != nil {
			t.Errorf("parseSortKey(%q) returned unexpected error: %v", test.input, err)
			continue
		}
		if got != test.want {
			t.Errorf("parseSortKey(%q) = %v, want %v", test.input, got, test.want)
		}
	}
}

func TestParseSortKeyError(t *testing.T) {
	for _, input := range []string{"", "unknown"} {
		if _, err := parseSortKey(input); err == nil {
			t.Errorf("parseSortKey(%q) error = nil, want non-nil", input)
		}
	}
}

func TestSortFieldsAdd(t *testing.T) {
	var got sortFields

	if err := got.add("path", false); err != nil {
		t.Fatalf(`sortFields.add("path", false) returned unexpected error: %v`, err)
	}
	if err := got.add("size", true); err != nil {
		t.Fatalf(`sortFields.add("size", true) returned unexpected error: %v`, err)
	}

	want := sortFields{
		{key: pathKey},
		{key: sizeKey, descending: true},
	}
	if !slices.Equal(got, want) {
		t.Errorf("fields after add calls = %v, want %v", got, want)
	}
}

func TestSortFieldsAddError(t *testing.T) {
	initial := sortFields{
		{key: pathKey},
		{key: sizeKey, descending: true},
	}

	for _, name := range []string{"size", "unknown"} {
		got := slices.Clone(initial)

		if err := got.add(name, false); err == nil {
			t.Errorf("sortFields.add(%q, false) error = nil, want non-nil",
				name)
		}
		if !slices.Equal(got, initial) {
			t.Errorf("sortFields.add(%q, false) changed fields to %v, want %v",
				name, got, initial)
		}
	}
}

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
				t.Fatalf("readPaths(%q, %q) returned unexpected error: %v", test.input, test.delim, err)
			}
			if !slices.Equal(got, test.want) {
				t.Errorf("readPaths(%q, %q) = %v, want %v", test.input, test.delim, got, test.want)
			}
		})
	}
}
