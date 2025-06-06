// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package json

import (
	"fmt"
	"io"
	"os"

	"github.com/hashicorp/hcl/v2"
)

// Parse attempts to parse the given buffer as JSON and, if successful, returns
// a hcl.File for the HCL configuration represented by it.
//
// This is not a generic JSON parser. Instead, it deals only with the profile
// of JSON used to express HCL configuration.
//
// The returned file is valid only if the returned diagnostics returns false
// from its HasErrors method. If HasErrors returns true, the file represents
// the subset of data that was able to be parsed, which may be none.
func Parse(src []byte, filename string) (*hcl.File, hcl.Diagnostics) {
	return ParseWithStartPos(src, filename, hcl.Pos{Byte: 0, Line: 1, Column: 1})
}

// ParseWithStartPos attempts to parse like json.Parse, but unlike json.Parse
// you can pass a start position of the given JSON as a hcl.Pos.
//
// In most cases json.Parse should be sufficient, but it can be useful for parsing
// a part of JSON with correct positions.
func ParseWithStartPos(src []byte, filename string, start hcl.Pos) (*hcl.File, hcl.Diagnostics) {
	rootNode, diags := parseFileContent(src, filename, start)

	switch rootNode.(type) {
	case *objectVal, *arrayVal:
		// okay
	default:
		diags = diags.Append(&hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Root value must be object",
			Detail:   "The root value in a JSON-based configuration must be either a JSON object or a JSON array of objects.",
			Subject:  rootNode.StartRange().Ptr(),
		})

		// Since we've already produced an error message for this being
		// invalid, we'll return an empty placeholder here so that trying to
		// extract content from our root body won't produce a redundant
		// error saying the same thing again in more general terms.
		fakePos := hcl.Pos{
			Byte:   0,
			Line:   1,
			Column: 1,
		}
		fakeRange := hcl.Range{
			Filename: filename,
			Start:    fakePos,
			End:      fakePos,
		}
		rootNode = &objectVal{
			Attrs:     []*objectAttr{},
			SrcRange:  fakeRange,
			OpenRange: fakeRange,
		}
	}

	file := &hcl.File{
		Body: &body{
			val: rootNode,
		},
		Bytes: src,
		Nav:   navigation{rootNode},
	}
	return file, diags
}

// ParseExpression parses the given buffer as a standalone JSON expression,
// returning it as an instance of Expression.
func ParseExpression(src []byte, filename string) (hcl.Expression, hcl.Diagnostics) {
	return ParseExpressionWithStartPos(src, filename, hcl.Pos{Byte: 0, Line: 1, Column: 1})
}

// ParseExpressionWithStartPos parses like json.ParseExpression, but unlike
// json.ParseExpression you can pass a start position of the given JSON
// expression as a hcl.Pos.
func ParseExpressionWithStartPos(src []byte, filename string, start hcl.Pos) (hcl.Expression, hcl.Diagnostics) {
	node, diags := parseExpression(src, filename, start)
	return &expression{src: node}, diags
}

// ParseFile is a convenience wrapper around Parse that first attempts to load
// data from the given filename, passing the result to Parse if successful.
//
// If the file cannot be read, an error diagnostic with nil context is returned.
func ParseFile(filename string) (rf *hcl.File, diags hcl.Diagnostics) {
	f, err := os.Open(filename)
	if err != nil {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Failed to open file",
			Detail:   fmt.Sprintf("The file %q could not be opened.", filename),
		})
		return
	}
	defer func() {
		err := f.Close()
		if err != nil {
			diags = append(diags, &hcl.Diagnostic{
				Severity: hcl.DiagWarning,
				Summary:  "Failed to close file",
				Detail:   fmt.Sprintf("The file %q was opened, but an error occured while closing it.", filename),
			})
		}
	}()

	src, err := io.ReadAll(f)
	if err != nil {
		return nil, hcl.Diagnostics{
			{
				Severity: hcl.DiagError,
				Summary:  "Failed to read file",
				Detail:   fmt.Sprintf("The file %q was opened, but an error occured while reading it.", filename),
			},
		}
	}

	return Parse(src, filename)
}
