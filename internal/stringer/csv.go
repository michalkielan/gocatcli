/*
author: deadc0de6 (https://github.com/deadc0de6)
Copyright (c) 2024, deadc0de6
*/

package stringer

import (
	"fmt"
	"gocatcli/internal/node"
	"gocatcli/internal/tree"
	"gocatcli/internal/utils"
	"strings"
)

var (
	header = []string{
		"name",
		"type",
		"path",
		"size",
		"indexed_at",
		"maccess",
		"checksum",
		"nbfiles",
		"free_space",
		"total_space",
		"meta",
		"storage",
	}
)

// CSVStringer the CSV stringer
type CSVStringer struct {
	theTree    *tree.Tree
	sep        string
	rawSize    bool
	withHeader bool
}

func (p *CSVStringer) getSize(sz uint64) string {
	if p.rawSize {
		return fmt.Sprintf("%d", sz)
	}
	return utils.SizeToHuman(sz)
}

func (p *CSVStringer) storageToString(storage *node.StorageNode) string {
	var fields []string
	fields = append(fields, storage.Name)
	fields = append(fields, string(storage.Type))
	fields = append(fields, storage.GetPath())
	fields = append(fields, p.getSize(storage.Size))
	fields = append(fields, utils.DateToString(storage.IndexedAt))
	fields = append(fields, "") // maccess
	fields = append(fields, "") // checksum
	fields = append(fields, fmt.Sprintf("%d", storage.TotalFiles))
	fields = append(fields, p.getSize(storage.Free))
	fields = append(fields, p.getSize(storage.Total))
	fields = append(fields, storage.Meta)
	fields = append(fields, storage.Name) // storage (self)
	return strings.Join(fields, p.sep)
}

func (p *CSVStringer) fileToString(n *node.FileNode) string {
	var fields []string
	fields = append(fields, n.Name)
	fields = append(fields, string(n.Type))
	fields = append(fields, n.GetPath())
	fields = append(fields, p.getSize(n.Size))
	fields = append(fields, utils.DateToString(n.IndexedAt))
	maccess := utils.DateToString(n.Maccess)
	fields = append(fields, maccess)
	fields = append(fields, string(n.Checksum))
	fields = append(fields, fmt.Sprintf("%d", len(n.Children)))
	fields = append(fields, "") // free_space
	fields = append(fields, "") // total_space
	fields = append(fields, "") // meta
	sto := p.theTree.GetStorageNode(n)
	if sto != nil {
		fields = append(fields, sto.GetName()) // storage
	} else {
		fields = append(fields, "")
	}

	return strings.Join(fields, p.sep)
}

// ToString converts node to csv for printing
func (p *CSVStringer) ToString(n node.Node, _ int, _ bool) *Entry {
	var entry Entry

	entry.Name = n.GetName()
	entry.Node = n
	if n.GetType() == node.FileTypeStorage {
		entry.Line = p.storageToString(n.(*node.StorageNode))
	} else {
		entry.Line = p.fileToString(n.(*node.FileNode))
	}
	return &entry
}

// PrintPrefix prints the header
func (p *CSVStringer) PrintPrefix() {
	if !p.withHeader {
		return
	}
	fmt.Println(strings.Join(header, p.sep))
}

// PrintSuffix unused
func (p *CSVStringer) PrintSuffix() {}

// Print prints a node
func (p *CSVStringer) Print(n node.Node, depth int, fullPath bool) {
	e := p.ToString(n, depth, fullPath)
	fmt.Println(e.Line)
}

// NewCSVStringer creates a new CSV printer
func NewCSVStringer(t *tree.Tree, sep string, withHeader bool, rawSize bool) *CSVStringer {
	p := CSVStringer{
		theTree:    t,
		sep:        sep,
		withHeader: withHeader,
		rawSize:    rawSize,
	}
	return &p
}
