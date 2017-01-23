// Copyright (c) 2017, Daniel Martí <mvdan@mvdan.cc>
// See LICENSE for licensing information

package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	git "gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

func main() {
	branches, err := pickedBranches()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, b := range branches {
		fmt.Println(b)
	}
}

type branchInfo struct {
	ref    *plumbing.Reference
	author time.Time
}

func pickedBranches() ([]string, error) {
	r, err := git.NewFilesystemRepository(".git")
	if err != nil {
		return nil, err
	}
	all, err := allBranches(r)
	if err != nil {
		return nil, err
	}
	head, err := r.Head()
	if err != nil {
		return nil, err
	}
	// commits not yet confirmed picked
	commitsLeft := make(map[string]branchInfo, len(all)-1)
	for _, ref := range all {
		// HEAD is obviously part of itself
		if ref.Name() == head.Name() {
			continue
		}
		cm, err := r.Commit(ref.Hash())
		if err != nil {
			return nil, err
		}
		commitsLeft[commitStr(cm)] = branchInfo{
			ref:    ref,
			author: cm.Author.When.UTC(),
		}
	}
	if len(commitsLeft) == 0 {
		return nil, nil
	}
	hcm, err := r.Commit(head.Hash())
	if err != nil {
		return nil, err
	}
	stopTime := oldestTime(commitsLeft)
	picked := make([]string, 0)
	err = object.WalkCommitHistory(hcm, func(cm *object.Commit) error {
		if cm.Committer.When.Before(stopTime) {
			return reachedEnd
		}
		str := commitStr(cm)
		if bi, e := commitsLeft[str]; e {
			delete(commitsLeft, str)
			picked = append(picked, bi.ref.Name().Short())
			if len(commitsLeft) == 0 {
				return reachedEnd
			}
			stopTime = oldestTime(commitsLeft)
		}
		return nil
	})
	if err == reachedEnd {
		err = nil
	}
	return picked, err
}

func oldestTime(m map[string]branchInfo) time.Time {
	first := true
	var oldest time.Time
	for _, bi := range m {
		if first || bi.author.Before(oldest) {
			oldest = bi.author
		}
		first = false
	}
	return oldest
}

var reachedEnd = fmt.Errorf("reached end")

func commitStr(cm *object.Commit) string {
	summary := cm.Message
	if i := strings.IndexByte(summary, '\n'); i > 0 {
		summary = summary[:i]
	}
	return fmt.Sprintf("%s %s %s %s",
		cm.Author.Name,
		cm.Author.Email,
		cm.Author.When.UTC().String(),
		summary,
	)
}

func allBranches(r *git.Repository) ([]*plumbing.Reference, error) {
	refs, err := r.References()
	if err != nil {
		return nil, err
	}
	defer refs.Close()
	all := make([]*plumbing.Reference, 0)
	err = refs.ForEach(func(ref *plumbing.Reference) error {
		if ref.IsBranch() {
			all = append(all, ref)
		}
		return nil
	})
	return all, err
}
