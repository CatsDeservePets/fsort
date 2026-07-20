package main

import (
	"cmp"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

// A sortKey specifies an attribute used to order entries.
type sortKey byte

const (
	nameKey sortKey = iota
	pathKey
	extKey
	typeKey
	permKey
	sizeKey
	mtimeKey
)

func parseSortKey(s string) (sortKey, error) {
	switch strings.ToLower(s) {
	case "name":
		return nameKey, nil
	case "path":
		return pathKey, nil
	case "ext", "extension":
		return extKey, nil
	case "type":
		return typeKey, nil
	case "perm", "permission":
		return permKey, nil
	case "size":
		return sizeKey, nil
	case "time", "mtime":
		return mtimeKey, nil
	default:
		return 0, errors.New("must be name, path, extension, type, perm, size, or time")
	}
}

// A sortField holds a [sortKey] and its direction.
type sortField struct {
	key        sortKey // [entry] attribute to compare
	descending bool    // whether greater values sort first
}

// sortFields represent the requested sort order.
type sortFields []sortField

// add parses name as a sort key and appends the resulting sort field to f.
// If desc is true, the field sorts in descending order.
// add returns an error if name is invalid or specifies a key already used.
func (f *sortFields) add(name string, desc bool) error {
	key, err := parseSortKey(name)
	if err != nil {
		return err
	}
	for _, field := range *f {
		if field.key == key {
			return errors.New("key already specified")
		}
	}
	*f = append(*f, sortField{key, desc})
	return nil
}

const keyHelp = `sort by %s in ascending order. Key must be one of name,
path, extension, type, perm, size, or time. The -k and -K
options may be specified multiple times; subsequent keys
are compared when earlier keys compare equal. By default,
fsort sorts by name.`

var (
	fold    = flag.Bool("f", false, "fold lowercase characters to uppercase before comparison")
	zero    = flag.Bool("z", false, "line delimiter is NUL, not newline")
	workDir = flag.String("C", "", "change to `dir` before resolving input names")
	sortBy  sortFields
)

func init() {
	flag.Func("k", fmt.Sprintf(keyHelp, "`key`"), func(s string) error {
		return sortBy.add(s, false)
	})
	flag.Func("K", "same as -k, but sorts by `key` in descending order", func(s string) error {
		return sortBy.add(s, true)
	})
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: fsort [-f] [-z] [-C dir] [-k key | -K key] ... [file ...]")
	flag.PrintDefaults()
	os.Exit(2)
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("fsort: ")
	flag.Usage = usage
	flag.Parse()

	delim := "\n"
	if *zero {
		delim = "\x00"
	}

	paths := flag.Args()
	// Expand globs before -C, as a proper shell would.
	if runtime.GOOS == "windows" {
		paths = expandGlobs(paths)
	}

	if len(paths) == 0 {
		var err error
		paths, err = readPaths(os.Stdin, delim)
		if err != nil {
			log.Fatal(err)
		}
	}
	// An empty input is already sorted.
	if len(paths) == 0 {
		return
	}

	if *workDir != "" {
		if err := os.Chdir(*workDir); err != nil {
			log.Fatal(err)
		}
	}

	ents, allOk := collectEntries(paths, *fold)
	sortEntries(ents, sortBy)

	for _, e := range ents {
		fmt.Print(e.path, delim)
	}

	if !allOk {
		os.Exit(1)
	}
}

// expandGlobs expands wildcards in args using [filepath.Glob].
// If an argument returns no matches, it is left unchanged.
func expandGlobs(args []string) []string {
	out := make([]string, 0, len(args))
	for _, pattern := range args {
		if matches, _ := filepath.Glob(pattern); len(matches) > 0 {
			out = append(out, matches...)
		} else {
			out = append(out, pattern)
		}
	}
	return out
}

// readPaths reads paths from r, using delim as the separator.
func readPaths(r io.Reader, delim string) ([]string, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	var paths []string
	for p := range strings.SplitSeq(string(b), delim) {
		if delim == "\n" {
			p = strings.TrimSuffix(p, "\r")
		}
		if p != "" {
			paths = append(paths, p)
		}
	}
	return paths, nil
}

// An entry is a path being sorted.
type entry struct {
	info    os.FileInfo
	path    string
	cmpName string
	cmpPath string
	cmpExt  string
	cmpType uint8
}

// newEntry returns an entry for path without following symlinks.
// If fold is true, it stores uppercase strings for comparisons.
func newEntry(path string, fold bool) (entry, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return entry{}, err
	}
	cmpName := info.Name()
	cmpPath := path
	cmpExt := filepath.Ext(cmpName)
	if fold {
		cmpName = strings.ToUpper(cmpName)
		cmpPath = strings.ToUpper(cmpPath)
		cmpExt = strings.ToUpper(cmpExt)
	}
	return entry{
		info:    info,
		path:    path,
		cmpName: cmpName,
		cmpPath: cmpPath,
		cmpExt:  cmpExt,
		cmpType: typeRank(info.Mode()),
	}, nil
}

func typeRank(m os.FileMode) uint8 {
	switch m.Type() {
	case os.ModeDir:
		return 0
	case 0:
		return 1
	case os.ModeSymlink:
		return 2
	case os.ModeNamedPipe:
		return 3
	case os.ModeSocket:
		return 4
	case os.ModeDevice | os.ModeCharDevice:
		return 5
	case os.ModeDevice:
		return 6
	case os.ModeIrregular:
		return 7
	}
	return 8
}

// collectEntries creates entries from paths.
// It logs errors and returns false if any occurred.
func collectEntries(paths []string, fold bool) ([]entry, bool) {
	ents := make([]entry, 0, len(paths))
	ok := true

	for _, p := range paths {
		e, err := newEntry(p, fold)
		if err != nil {
			log.Println(err)
			ok = false
			continue
		}
		ents = append(ents, e)
	}
	return ents, ok
}

// sortEntries sorts entries according to fields.
// If fields is empty, it sorts by name in ascending order.
func sortEntries(entries []entry, fields sortFields) {
	if len(fields) == 0 {
		fields = sortFields{{key: nameKey}}
	}
	slices.SortStableFunc(entries, func(a, b entry) int {
		return compareEntries(a, b, fields)
	})
}

// compareEntries compares a and b by fields in order.
// It stops at the first difference.
func compareEntries(a, b entry, fields sortFields) int {
	for _, field := range fields {
		var n int
		switch field.key {
		case nameKey:
			n = strings.Compare(a.cmpName, b.cmpName)
		case pathKey:
			n = strings.Compare(a.cmpPath, b.cmpPath)
		case extKey:
			n = strings.Compare(a.cmpExt, b.cmpExt)
		case typeKey:
			n = cmp.Compare(a.cmpType, b.cmpType)
		case permKey:
			n = cmp.Compare(a.info.Mode().Perm(), b.info.Mode().Perm())
		case sizeKey:
			n = cmp.Compare(a.info.Size(), b.info.Size())
		case mtimeKey:
			n = a.info.ModTime().Compare(b.info.ModTime())
		}
		switch {
		case n == 0:
			continue
		case field.descending:
			return -n
		default:
			return n
		}
	}

	return 0
}
