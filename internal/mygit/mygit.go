package mygit

import (
	"errors"
	"fmt"
	"github.com/richardjennings/mygit/internal/mygit/commits"
	"github.com/richardjennings/mygit/internal/mygit/config"
	"github.com/richardjennings/mygit/internal/mygit/index"
	"github.com/richardjennings/mygit/internal/mygit/objects"
	"github.com/richardjennings/mygit/internal/mygit/refs"
	"io"
	"log"
	"os"
	"time"
)

func Init() error {
	path := config.GitPath()
	if err := os.MkdirAll(path, 0755); err != nil {
		return err
	}
	for _, v := range []string{
		config.ObjectPath(),
		config.RefsDirectory(),
		config.RefsHeadsDirectory(),
	} {
		if err := os.MkdirAll(v, 0755); err != nil {
			log.Fatalln(err)
		}
	}
	// set default main branch
	return os.WriteFile(config.GitHeadPath(), []byte(fmt.Sprintf("ref: %s\n", config.Config.DefaultBranch)), 0644)
}

func Add(paths ...string) error {
	idx, err := index.ReadIndex()
	if err != nil {
		return err
	}
	// get working directory files with idx status
	wdFiles, err := index.WdStatus()
	if err != nil {
		return err
	}
	var updates []*index.File
	for _, p := range paths {
		if p == "." {
			// special case meaning add everything
			for _, v := range wdFiles {
				switch v.Status {
				case index.StatusUntracked, index.StatusModified, index.StatusDeleted:
					updates = append(updates, v)
				}
			}
		} else {
			// @todo add support for paths other than just '.'
			return errors.New("only supports '.' currently ")
		}
	}
	for _, v := range updates {
		switch v.Status {
		case index.StatusUntracked, index.StatusModified:
			// add the file to the object store
			obj, err := objects.WriteBlob(v.Path)
			if err != nil {
				return err
			}
			v.Sha = obj.Sha
		}
		if err := idx.AddWdFile(v); err != nil {
			return err
		}
	}
	// once all wdFiles are added to idx struct, write it out
	return index.WriteIndex(idx)
}

// LsFiles returns a list of files in the index
func LsFiles() ([]string, error) {
	idx, err := index.ReadIndex()
	if err != nil {
		return nil, err
	}
	var files []string
	for _, v := range idx.IdxFiles() {
		files = append(files, v.Path)
	}
	return files, nil
}

func Commit() ([]byte, error) {
	idx, err := index.ReadIndex()
	if err != nil {
		return nil, err
	}
	root := index.ObjectTree(idx.IdxFiles())
	tree, err := root.WriteTree()
	if err != nil {
		return nil, err
	}

	previousCommits, err := commits.PreviousCommits()
	if err != nil {
		return nil, err
	}
	return commits.Write(
		&commits.Commit{
			Tree:          tree,
			Parents:       previousCommits,
			Author:        "Richard Jennings <richardjennings@gmail.com>",
			AuthoredTime:  time.Now(),
			Committer:     "Richard Jennings <richardjennings@gmail.com>",
			CommittedTime: time.Now(),
			Message:       "test",
		},
	)
}

// Status currently displays the
func Status(o io.Writer) error {
	var err error
	// index
	idx, err := index.ReadIndex()
	if err != nil {
		return err
	}
	commitSha, err := refs.LastCommit()
	if err != nil {
		return err
	}
	files, err := idx.CommitStatus(commitSha)
	if err != nil {
		return err
	}
	for _, v := range files {
		if v.Status == index.StatusUnchanged {
			continue
		}
		if _, err := fmt.Fprintf(o, "%s %s\n", v.Status, v.Path); err != nil {
			return err
		}
	}

	// working directory
	files, err = index.WdStatus()
	if err != nil {
		return err
	}
	for _, v := range files {
		if v.Status == index.StatusUnchanged {
			continue
		}
		if _, err := fmt.Fprintf(o, "%s %s\n", v.Status, v.Path); err != nil {
			return err
		}
	}
	return nil
}
