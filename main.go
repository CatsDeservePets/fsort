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

var progName = strings.TrimSuffix(filepath.Base(os.Args[0]), ".exe")

type sortKey byte

const (
	nameKey sortKey = iota
	extKey
	mtimeKey
	sizeKey
)

type sortField struct {
	key  sortKey
	desc bool
}

type entry struct {
	info    os.FileInfo
	path    string
	cmpName string
	cmpExt  string
}

func newEntry(path string, fold bool) (entry, error) {
	info, err := os.Stat(path)
	if err != nil {
		return entry{}, err
	}
	name := info.Name()
	ext := filepath.Ext(name)
	if fold {
		name = strings.ToUpper(name)
		ext = strings.ToUpper(ext)
	}
	return entry{
		info:    info,
		path:    path,
		cmpName: name,
		cmpExt:  ext,
	}, nil
}

func usage() {
	fmt.Fprintf(os.Stderr, "usage: %s [-f] [-z] [-C dir] [-k key | -K key]... [file ...]\n", progName)
	flag.PrintDefaults()
	os.Exit(2)
}

func main() {
	log.SetFlags(0)
	log.SetPrefix(progName + ": ")

	var fold, zero bool
	var workDir string
	var order []sortField

	addOrder := func(s string, desc bool) error {
		var k sortKey
		switch s {
		case "name":
			k = nameKey
		case "ext", "extension":
			k = extKey
		case "size":
			k = sizeKey
		case "time", "mtime":
			k = mtimeKey
		default:
			return errors.New("must be name, extension, size, or time")
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
	flag.Func("k", "sort by `key` in ascending order. Key must be one of\n"+
		"name, extension, size, or time. The -k and -K options\n"+
		"may be specified multiple times; subsequent keys are\n"+
		"compared when earlier keys compare equal. By default,\n"+
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
		case extKey:
			n = strings.Compare(a.cmpExt, b.cmpExt)
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
