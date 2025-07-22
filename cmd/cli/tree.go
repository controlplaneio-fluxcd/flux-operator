// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"strings"

	"github.com/fluxcd/cli-utils/pkg/object"
	ssautil "github.com/fluxcd/pkg/ssa/utils"
	"github.com/spf13/cobra"
)

var treeCmd = &cobra.Command{
	Use:   "tree",
	Short: "Print a tree view of resources reconciled by Flux Operator",
}

func init() {
	rootCmd.AddCommand(treeCmd)
}

const (
	newLine      = "\n"
	emptySpace   = "    "
	middleItem   = "├── "
	continueItem = "│   "
	lastItem     = "└── "
)

type (
	objMetadataTree struct {
		Resource     object.ObjMetadata `json:"resource"`
		ResourceTree []ObjMetadataTree  `json:"resources,omitempty"`
	}

	ObjMetadataTree interface {
		Add(objMetadata object.ObjMetadata) ObjMetadataTree
		AddTree(tree ObjMetadataTree)
		Items() []ObjMetadataTree
		Text() string
		Print() string
	}

	treePrinter struct {
	}

	TreePrinter interface {
		Print(ObjMetadataTree) string
	}
)

func NewTree(objMetadata object.ObjMetadata) ObjMetadataTree {
	return &objMetadataTree{
		Resource:     objMetadata,
		ResourceTree: []ObjMetadataTree{},
	}
}

func (t *objMetadataTree) Add(objMetadata object.ObjMetadata) ObjMetadataTree {
	n := NewTree(objMetadata)
	t.ResourceTree = append(t.ResourceTree, n)
	return n
}

func (t *objMetadataTree) AddTree(tree ObjMetadataTree) {
	t.ResourceTree = append(t.ResourceTree, tree)
}

func (t *objMetadataTree) Text() string {
	return ssautil.FmtObjMetadata(t.Resource)
}

func (t *objMetadataTree) Items() []ObjMetadataTree {
	return t.ResourceTree
}

func (t *objMetadataTree) Print() string {
	return newPrinter().Print(t)
}

func newPrinter() TreePrinter {
	return &treePrinter{}
}

func (p *treePrinter) Print(t ObjMetadataTree) string {
	return t.Text() + newLine + p.printItems(t.Items(), []bool{})
}

func (p *treePrinter) printText(text string, spaces []bool, last bool) string {
	var result string
	for _, space := range spaces {
		if space {
			result += emptySpace
		} else {
			result += continueItem
		}
	}

	indicator := middleItem
	if last {
		indicator = lastItem
	}

	var out string
	lines := strings.Split(text, "\n")
	for i := range lines {
		text := lines[i]
		if i == 0 {
			out += result + indicator + text + newLine
			continue
		}
		if last {
			indicator = emptySpace
		} else {
			indicator = continueItem
		}
		out += result + indicator + text + newLine
	}

	return out
}

func (p *treePrinter) printItems(t []ObjMetadataTree, spaces []bool) string {
	var result string
	for i, f := range t {
		last := i == len(t)-1
		result += p.printText(f.Text(), spaces, last)
		if len(f.Items()) > 0 {
			spacesChild := append(spaces, last)
			result += p.printItems(f.Items(), spacesChild)
		}
	}
	return result
}
