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

type sortField struct {
	key        sortKey
	descending bool
}

type sortFields []sortField

func (f *sortFields) add(name string, desc bool) error {
	var key sortKey
	switch name {
	case "name":
		key = nameKey
	case "path":
		key = pathKey
	case "ext", "extension":
		key = extKey
	case "type":
		key = typeKey
	case "perm", "permission":
		key = permKey
	case "size":
		key = sizeKey
	case "time", "mtime":
		key = mtimeKey
	default:
		return errors.New("must be name, path, extension, type, perm, size, or time")
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

	args := flag.Args()
	// Expand globs before -C, as a proper shell would.
	if runtime.GOOS == "windows" {
		args = expandGlobs(args)
	}

	delim := "\n"
	if *zero {
		delim = "\x00"
	}

	paths, err := inputPaths(args, os.Stdin, delim)
	if err != nil {
		log.Fatal(err)
	}
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

func inputPaths(args []string, r io.Reader, delim string) ([]string, error) {
	if len(args) > 0 {
		return args, nil
	}

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

type entry struct {
	info    os.FileInfo
	path    string
	cmpName string
	cmpPath string
	cmpExt  string
	cmpType uint8
}

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

func sortEntries(entries []entry, fields sortFields) {
	if len(fields) == 0 {
		fields = sortFields{{key: nameKey}}
	}
	slices.SortStableFunc(entries, func(a, b entry) int {
		return compareEntries(a, b, fields)
	})
}

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
