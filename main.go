package main

import (
	"bufio"
	"cmp"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
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
	info      os.FileInfo
	path      string
	lowerName string
	lowerExt  string
}

func newEntry(path string) (entry, error) {
	info, err := os.Stat(path)
	if err != nil {
		return entry{}, err
	}
	name := info.Name()
	e := entry{
		info:      info,
		path:      path,
		lowerName: strings.ToLower(name),
		lowerExt:  strings.ToLower(filepath.Ext(name)),
	}
	return e, nil
}

func usage() {
	fmt.Fprintf(os.Stderr, "usage: %s [-a key | -d key]... [file ...]\n", progName)
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, `
The key argument may be name, extension, size, or time.
If no key is specified, name is used in ascending order.
Multiple keys are applied in the order specified.
`)
	os.Exit(2)
}

func main() {
	log.SetFlags(0)
	log.SetPrefix(progName + ": ")

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
	flag.Func("a", "ascending sort `key`", func(s string) error {
		return addOrder(s, false)
	})
	flag.Func("d", "descending sort `key`", func(s string) error {
		return addOrder(s, true)
	})
	flag.Parse()

	paths, err := inputPaths(flag.Args(), os.Stdin)
	if err != nil {
		log.Fatal(err)
	}
	if len(paths) == 0 {
		flag.Usage()
	}

	ents := collectEntries(paths)
	sortEntries(ents, order)

	for _, e := range ents {
		fmt.Println(e.path)
	}
}

func inputPaths(args []string, r io.Reader) ([]string, error) {
	if len(args) > 0 {
		return args, nil
	}

	var paths []string
	s := bufio.NewScanner(r)
	for s.Scan() {
		if p := s.Text(); p != "" {
			paths = append(paths, p)
		}
	}

	return paths, s.Err()
}

func collectEntries(paths []string) []entry {
	ents := make([]entry, 0, len(paths))
	for _, p := range paths {
		e, err := newEntry(p)
		if err != nil {
			log.Println(err)
			continue
		}
		ents = append(ents, e)
	}
	return ents
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
			n = strings.Compare(a.lowerName, b.lowerName)
		case extKey:
			n = strings.Compare(a.lowerExt, b.lowerExt)
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
