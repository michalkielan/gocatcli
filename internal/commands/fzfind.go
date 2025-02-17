/*
author: deadc0de6 (https://github.com/deadc0de6)
Copyright (c) 2024, deadc0de6
*/

package commands

import (
	"fmt"
	"gocatcli/internal/log"
	"gocatcli/internal/node"
	"gocatcli/internal/stringer"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ktr0731/go-fuzzyfinder"
	"github.com/spf13/cobra"
)

const (
	initialChunkSize = 1000
	fzFinderRefreshInterval = 150 * time.Millisecond
)

var (
	fzfindCmd = &cobra.Command{
		Use:    "fzfind [<path>]",
		Short:  "Fuzzy find files in the catalog",
		PreRun: preRun(true),
		RunE:   fzFind,
	}

	fzFindOptFormat  string
	fzFindOptDepth   int
	fzFindOptShowAll bool
)

type fzfEntry struct {
	Path    string
	item    node.Node
	storage *node.StorageNode
}

func init() {
	rootCmd.AddCommand(fzfindCmd)

	hlp := fmt.Sprintf("output format (%s)", strings.Join(stringer.GetSupportedFormats(true, false), ","))
	fzfindCmd.PersistentFlags().StringVarP(&fzFindOptFormat, "format", "f", "native", hlp)
	fzfindCmd.PersistentFlags().IntVarP(&fzFindOptDepth, "depth", "D", -1, "max hierarchy depth when printing selected entry")
	fzfindCmd.PersistentFlags().BoolVarP(&fzFindOptShowAll, "all", "a", false, "do not ignore entries starting with a dot")
}

func fzFind(_ *cobra.Command, args []string) error {
	if !formatOk(fzFindOptFormat, true, false) {
		return fmt.Errorf("unsupported format %s", fzFindOptFormat)
	}

	var startPath string
	if len(args) > 0 {
		startPath = args[0]
	}

	// get the stringer
	m := &stringer.PrintMode{
		FullPath:    false,
		Long:        false,
		InlineColor: false,
		RawSize:     false,
		Separator:   separator,
	}
	stringGetter, err := stringer.GetStringer(loadedTree, fzFindOptFormat, m)
	if err != nil {
		return err
	}

	// get the base paths for start
	var startNodes []node.Node
	if len(startPath) > 0 {
		startNodes = getStartPaths(startPath)
		if startNodes == nil {
			return fmt.Errorf("no such start path: \"%s\"", startPath)
		}
	} else {
		for _, top := range loadedTree.GetStorages() {
			startNodes = append(startNodes, top)
		}
	}
	bufferedEntryCh := make(chan *fzfEntry, initialChunkSize)
	go func() {
		for _, foundNode := range startNodes {
			fzFindFillList(foundNode, bufferedEntryCh)
		}
	}()

	// list of entries
	var entries []*fzfEntry
	var entriesMutex sync.RWMutex
	log.Debugf("start nodes: %v", startNodes)
	for i := 0; i < initialChunkSize; i++ {
		entry, ok := <-bufferedEntryCh
		if !ok {
			break
		}
		entries = append(entries, entry)
	}

	loadMoreEntries := func() {
		for {
			select {
			case entry, ok := <-bufferedEntryCh:
				if !ok {
					log.Debugf("Channel closed, no more entries")
					return
				}
				entries = append(entries, entry)
			default:
				return
			}
		}
	}

	getItemFunc := func(i int) string {
		if i >= len(entries) {
			loadMoreEntries()
		}
		if i < len(entries) {
			return entries[i].Path
		}
		return ""
	}

	previewFunc := func(i, _, _ int) string {
		if i == -1 || i >= len(entries) {
			return ""
		}
		entriesMutex.RLock()
		defer entriesMutex.RUnlock()
		entry := entries[i]
		var outs []string
		outs = append(outs, fmt.Sprintf("storage: %s", entry.storage.Name))
		outs = append(outs, fmt.Sprintf("path: %s", entry.item.GetPath()))

		entryAttrs := entry.item.GetAttr(m.RawSize, m.Long)
		attrs := stringer.AttrsToString(entryAttrs, m, "\n")

		return strings.Join(outs, "\n") + "\n" + attrs
	}

	// display fzf finder interface
	entriesLoadRefreshTicker := time.NewTicker(fzFinderRefreshInterval)
	defer entriesLoadRefreshTicker.Stop()

	go func() {
		for range entriesLoadRefreshTicker.C {
			loadMoreEntries()
		}
	}()

	idx, err := fuzzyfinder.Find(
		&entries,
		getItemFunc,
		fuzzyfinder.WithPreviewWindow(previewFunc),
		fuzzyfinder.WithHotReloadLock(&entriesMutex),
	)
	if err != nil {
		return err
	}

	// print result
	if idx >= 0 && idx < len(entries) {
		// list parent directory
		entry := entries[idx]
		log.Debugf("selected entry: %s", entry.Path)

		// get the parent
		hasChildren := false
		typ := entry.item.GetType()
		if typ == node.FileTypeDir || typ == node.FileTypeArchive || typ == node.FileTypeStorage {
			hasChildren = true
		}

		// print the rest
		callback := func(n node.Node, depth int, _ node.Node) bool {
			stringGetter.Print(n, depth+1)
			return true
		}

		stringGetter.PrintPrefix()
		stringGetter.Print(entry.item, 0)
		if hasChildren {
			loadedTree.ProcessChildren(entry.item, fzFindOptShowAll, callback, 1)
		}
		stringGetter.PrintSuffix()
	}

	return nil
}

func fzFindFillList(n node.Node, entryCh chan<- *fzfEntry) {
	top := loadedTree.GetStorageNode(n)
	callback := func(n node.Node, _ int, _ node.Node) bool {
		item := &fzfEntry{
			Path:    filepath.Join(top.GetName(), n.GetPath()),
			item:    n,
			storage: top,
		}
		entryCh <- item
		return true
	}
	loadedTree.ProcessChildren(n, fzFindOptShowAll, callback, -1)
	close(entryCh)
}
