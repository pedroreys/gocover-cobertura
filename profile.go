// Imported from https://github.com/golang/go/blob/96745b980cfde139e8611772e2bc0c59a8e6cdf7/src/cmd/cover/profile.go

// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file provides support for parsing coverage profiles
// generated by "go test -coverprofile=cover.out".
// It is a copy of golang.org/x/tools/cover/profile.go.

package main

import (
	"bufio"
	"fmt"
	"github.com/dimchansky/utfbom"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Profile represents the profiling data for a specific file.
type Profile struct {
	FileName string
	Mode     string
	Blocks   []ProfileBlock
}

// ProfileBlock represents a single block of profiling data.
type ProfileBlock struct {
	StartLine, StartCol int
	EndLine, EndCol     int
	NumStmt, Count      int
}

type byFileName []*Profile

func (p byFileName) Len() int           { return len(p) }
func (p byFileName) Less(i, j int) bool { return p[i].FileName < p[j].FileName }
func (p byFileName) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// ParseProfiles parses profile data in the specified file and returns a
// Profile for each source file described therein.
func ParseProfiles(in io.Reader) ([]*Profile, error) {
	files := make(map[string]*Profile)
	// First line is "mode: foo", where foo is "set", "count", or "atomic".
	// Rest of file is in the format
	//	encoding/base64/base64.go:34.44,37.40 3 1
	// where the fields are: name.go:line.column,line.column numberOfStatements count
	reader := utfbom.SkipOnly(in)

	s := bufio.NewScanner(reader)
	mode := ""

	for s.Scan() {
		line := s.Text()
		if mode == "" {
			const p = "mode: "
			if !strings.HasPrefix(line, p) || line == p {
				return nil, fmt.Errorf("bad mode line: %q", line)
			}
			mode = line[len(p):]
			continue
		}
		m := lineRe.FindStringSubmatch(line)
		if m == nil {
			return nil, fmt.Errorf("line %q doesn't match expected format: %v", m, lineRe)
		}
		fn := m[1]
		p := files[fn]
		if p == nil {
			p = &Profile{
				FileName: fn,
				Mode:     mode,
			}
			files[fn] = p
		}
		p.Blocks = append(p.Blocks, ProfileBlock{
			StartLine: toInt(m[2]),
			StartCol:  toInt(m[3]),
			EndLine:   toInt(m[4]),
			EndCol:    toInt(m[5]),
			NumStmt:   toInt(m[6]),
			Count:     toInt(m[7]),
		})
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	for _, p := range files {
		sort.Sort(blocksByStart(p.Blocks))
		// Merge samples from the same location.
		j := 1
		for i := 1; i < len(p.Blocks); i++ {
			b := p.Blocks[i]
			last := p.Blocks[j-1]
			if b.StartLine == last.StartLine &&
				b.StartCol == last.StartCol &&
				b.EndLine == last.EndLine &&
				b.EndCol == last.EndCol {
				if b.NumStmt != last.NumStmt {
					return nil, fmt.Errorf("inconsistent NumStmt: changed from %d to %d", last.NumStmt, b.NumStmt)
				}
				if mode == "set" {
					p.Blocks[j-1].Count |= b.Count
				} else {
					p.Blocks[j-1].Count += b.Count
				}
				continue
			}
			p.Blocks[j] = b
			j++
		}
		p.Blocks = p.Blocks[:j]
	}
	// Generate a sorted slice.
	profiles := make([]*Profile, 0, len(files))
	for _, profile := range files {
		profiles = append(profiles, profile)
	}
	sort.Sort(byFileName(profiles))
	return profiles, nil
}

type blocksByStart []ProfileBlock

func (b blocksByStart) Len() int      { return len(b) }
func (b blocksByStart) Swap(i, j int) { b[i], b[j] = b[j], b[i] }
func (b blocksByStart) Less(i, j int) bool {
	bi, bj := b[i], b[j]
	return bi.StartLine < bj.StartLine || bi.StartLine == bj.StartLine && bi.StartCol < bj.StartCol
}

var lineRe = regexp.MustCompile(`^(.+):([0-9]+).([0-9]+),([0-9]+).([0-9]+) ([0-9]+) ([0-9]+)$`)

func toInt(s string) int {
	i, err := strconv.Atoi(s)
	if err != nil {
		panic(err)
	}
	return i
}
