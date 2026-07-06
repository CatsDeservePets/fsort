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
	key  sortKey
	desc bool
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

func usage() {
	fmt.Fprintln(os.Stderr, "usage: fsort [-f] [-z] [-C dir] [-k key | -K key] ... [file ...]")
	flag.PrintDefaults()
	os.Exit(2)
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("fsort: ")

	var fold, zero bool
	var workDir string
	var order []sortField

	addOrder := func(s string, desc bool) error {
		var k sortKey
		switch s {
		case "name":
			k = nameKey
		case "path":
			k = pathKey
		case "ext", "extension":
			k = extKey
		case "type":
			k = typeKey
		case "perm", "permission":
			k = permKey
		case "size":
			k = sizeKey
		case "time", "mtime":
			k = mtimeKey
		default:
			return errors.New("must be name, path, extension, type, perm, size, or time")
		}

		for _, field := range order {
			if field.key == k {
				return errors.New("key already specified")
			}
		}
		order = append(order, sortField{k, desc})
		return nil
	}

	flag.Usage = usage
	flag.BoolVar(&fold, "f", false, "fold lowercase characters to uppercase before comparison")
	flag.BoolVar(&zero, "z", false, "line delimiter is NUL, not newline")
	flag.StringVar(&workDir, "C", "", "change to `dir` before resolving input names")
	flag.Func("k", "sort by `key` in ascending order. Key must be one of name,\n"+
		"path, extension, type, perm, size, or time. The -k and -K\n"+
		"options may be specified multiple times; subsequent keys\n"+
		"are compared when earlier keys compare equal. By default,\n"+
		"fsort sorts by name.", func(s string) error {
		return addOrder(s, false)
	})
	flag.Func("K", "same as -k, but sorts by `key` in descending order", func(s string) error {
		return addOrder(s, true)
	})
	flag.Parse()

	args := flag.Args()
	// Expand globs before -C, as a proper shell would.
	if runtime.GOOS == "windows" {
		args = expandGlobs(args)
	}

	delim := "\n"
	if zero {
		delim = "\x00"
	}

	paths, err := inputPaths(args, os.Stdin, delim)
	if err != nil {
		log.Fatal(err)
	}
	if len(paths) == 0 {
		return
	}

	if workDir != "" {
		if err := os.Chdir(workDir); err != nil {
			log.Fatal(err)
		}
	}

	ents, allOk := collectEntries(paths, fold)
	sortEntries(ents, order)

	for _, e := range ents {
		fmt.Print(e.path, delim)
	}

	if !allOk {
		os.Exit(1)
	}
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

func sortEntries(entries []entry, order []sortField) {
	if len(order) == 0 {
		order = []sortField{{key: nameKey}}
	}
	slices.SortStableFunc(entries, func(a, b entry) int {
		return compareEntries(a, b, order)
	})
}

func compareEntries(a, b entry, order []sortField) int {
	for _, field := range order {
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
		case field.desc:
			return -n
		default:
			return n
		}
	}

	return 0
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
