package gfs

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"github.com/richardjennings/mygit/internal/mygit/config"
	"github.com/richardjennings/mygit/internal/mygit/ignore"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	IndexNotUpdated IndexStatus = iota
	IndexUpdatedInIndex
	IndexTypeChangedInIndex
	IndexAddedInIndex
	IndexDeletedInIndex
	IndexRenamedInIndex
	IndexCopiedInIndex
	IndexUntracked
)

const (
	WDIndexAndWorkingTreeMatch WDStatus = iota
	WDWorktreeChangedSinceIndex
	WDTypeChangedInWorktreeSinceIndex
	WDDeletedInWorktree
	WDRenamedInWorktree
	WDCopiedInWorktree
	WDUntracked
)

const (
	StatusUndefined FileStatus = iota
	StatusModified             // different in working directory than Index
	StatusAdded                // in Index but not in last commit
	StatusDeleted              // in last commit but not in Index

	StatusUnchanged
	StatusUntracked // in working directory but not in Index

	StatusNotUpdated
	StatusUpdatedInIndex
	StatusTypeChangedInIndex
	StatusAddedInIndex
	StatusDeletedInIndex
	StatusRenamedInIndex
	StatusCopiedInIndex

	StatusIndexAndWorkingTreeMatch
	StatusWorktreeChangedSinceIndex
	StatusTypeChangedInWorktreeSinceIndex
	StatusDeletedInWorktree
	StatusRenamedInWorktree
	StatusCopiedInWorktree
)

type (
	File struct {
		Path      string
		IdxStatus IndexStatus
		WdStatus  WDStatus
		Sha       *Sha
		Finfo     os.FileInfo
	}
	Sha struct {
		hash [20]byte
	}
	FileStatus  uint8
	IndexStatus uint8
	WDStatus    uint8
	FileSet     struct {
		files []*File
		idx   map[string]*File
	}
	Finfo struct {
		CTimeS uint32
		CTimeN uint32
		MTimeS uint32
		MTimeN uint32
		Dev    uint32
		Ino    uint32
		MMode  uint32
		Uid    uint32
		Gid    uint32
		SSize  uint32
		Sha    [20]byte
		NName  string
	}
	Mtime struct {
		Sec  uint32
		Nsec uint32
	}
)

func NewSha(b []byte) (*Sha, error) {
	if len(b) == 40 {
		s := &Sha{}
		_, _ = hex.Decode(s.hash[:], b)
		return s, nil
	}
	if len(b) == 20 {
		s := &Sha{}
		copy(s.hash[:], b)
		return s, nil
	}
	return nil, fmt.Errorf("invalid sha %s", b)
}

func (s *Sha) Same(ss *Sha) bool {
	if ss == nil {
		return false
	}
	if s == nil {
		return false
	}
	return s.hash == ss.hash
}

func (s Sha) AsHexString() string {
	return hex.EncodeToString(s.hash[:])
}
func (s Sha) AsHexBytes() []byte {
	b := make([]byte, 40)
	hex.Encode(b, s.hash[:])
	return b
}

func (s Sha) AsArray() [20]byte {
	var r [20]byte
	copy(r[:], s.hash[:])
	return r
}

// @todo this is more AsSlice ...
func (s Sha) AsBytes() []byte {
	return s.hash[:]
}

func (is IndexStatus) String() string {
	switch is {
	case IndexNotUpdated:
		return " "
	case IndexUpdatedInIndex:
		return "M"
	case IndexTypeChangedInIndex:
		return "T"
	case IndexAddedInIndex:
		return "A"
	case IndexDeletedInIndex:
		return "D"
	case IndexRenamedInIndex:
		return "R"
	case IndexCopiedInIndex:
		return "C"
	case IndexUntracked:
		return "?"
	default:
		return ""
	}
}

func (wds WDStatus) String() string {
	switch wds {
	case WDIndexAndWorkingTreeMatch:
		return " "
	case WDWorktreeChangedSinceIndex:
		return "M"
	case WDTypeChangedInWorktreeSinceIndex:
		return "T"
	case WDDeletedInWorktree:
		return "D"
	case WDRenamedInWorktree:
		return "R"
	case WDCopiedInWorktree:
		return "C"
	case WDUntracked:
		return "?"
	default:
		return ""
	}
}

func (ist FileStatus) String() string {
	switch ist {
	case StatusModified:
		return "M"
	case StatusAdded:
		return "A"
	case StatusDeleted:
		return "D"
	case StatusUntracked:
		return "??"
	case StatusUnchanged:
		return ""
	default:
		return "x"
	}
}

// Ls recursively lists files in path
func Ls(path string) ([]*File, error) {
	var files []*File
	if err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		// do not add ignored files
		if !ignore.IsIgnored(path) {
			files = append(files, &File{
				Path:  strings.TrimPrefix(path, config.WorkingDirectory()),
				Finfo: info,
			})
		}
		return nil
	}); err != nil {
		return files, err
	}
	return files, nil
}

func NewFileSet(files []*File) *FileSet {
	fs := &FileSet{files: files}
	fs.idx = make(map[string]*File)
	for _, v := range files {
		fs.idx[v.Path] = v
	}
	return fs
}

func (fs *FileSet) Merge(fss *FileSet) {
	for _, v := range fss.idx {
		if _, ok := fs.idx[v.Path]; ok {
			if fs.idx[v.Path].IdxStatus == IndexNotUpdated {
				fs.idx[v.Path].IdxStatus = v.IdxStatus
			}
			if fs.idx[v.Path].WdStatus == WDIndexAndWorkingTreeMatch {
				fs.idx[v.Path].WdStatus = v.WdStatus
			}
		} else {
			fs.files = append(fs.files, v)
			fs.idx[v.Path] = v
		}
	}
}

func (fs *FileSet) MergeFromIndex(fss *FileSet) {
	// add files from index to set, updating the status as relevant
	for _, v := range fss.files {
		if _, ok := fs.idx[v.Path]; !ok {
			// in index but not in commit files
			fs.files = append(fs.files, v)
			fs.idx[v.Path] = v
			v.IdxStatus = IndexAddedInIndex
			continue
		}
		fs.idx[v.Path].Finfo = v.Finfo
		if !bytes.Equal(v.Sha.AsBytes(), fs.idx[v.Path].Sha.AsBytes()) {
			fs.idx[v.Path].IdxStatus = IndexUpdatedInIndex
			continue
		}
	}
	for _, v := range fs.files {
		if _, ok := fss.idx[v.Path]; !ok {
			// file exists in commit but not in index
			v.IdxStatus = IndexDeletedInIndex
		}
	}
}

func (fs *FileSet) MergeFromWD(fss *FileSet) {
	// add files from working directory to set, updating the status as relevant
	for _, v := range fss.files {
		if _, ok := fs.idx[v.Path]; !ok {
			// in working directory but not in index or commit files
			fs.files = append(fs.files, v)
			fs.idx[v.Path] = v
			v.WdStatus = WDUntracked
			v.IdxStatus = IndexUntracked
			continue
		}

		if fs.idx[v.Path].Finfo == nil {
			// this is a commit file and not in the index
			// @todo should this be able to happen ?
			fs.idx[v.Path].WdStatus = WDUntracked
			fs.idx[v.Path].IdxStatus = IndexUntracked
		} else {
			if v.Finfo.ModTime() != fs.idx[v.Path].Finfo.ModTime() {
				fs.idx[v.Path].WdStatus = WDWorktreeChangedSinceIndex
				fs.idx[v.Path].Finfo = v.Finfo
				continue
			}

		}
	}
	for _, v := range fs.files {
		if _, ok := fss.idx[v.Path]; !ok {
			// file exists in commit but not in index
			v.WdStatus = WDDeletedInWorktree
		}
	}
}

func (fs *FileSet) Add(file *File) {
	fs.idx[file.Path] = file
	fs.files = append(fs.files, file)
}

// Compliment returns files in s that are not in fs
func (fs *FileSet) Compliment(s *FileSet) *FileSet {
	r := NewFileSet(nil)
	for k, v := range s.idx {
		if _, ok := fs.idx[k]; !ok {
			r.Add(v)
		}
	}
	return r
}

func (fs *FileSet) Contains(path string) (*File, bool) {
	v, ok := fs.idx[path]
	return v, ok
}

func (fs *FileSet) Files() []*File {
	return fs.files
}

// os.FileInfo interface

func (fi *Finfo) Name() string {
	return fi.NName
}
func (fi *Finfo) Size() int64       { return int64(fi.SSize) }
func (fi *Finfo) Mode() os.FileMode { return 0 }
func (fi *Finfo) IsDir() bool       { return false }
func (fi *Finfo) Sys() any          { return nil }
func (fi *Finfo) ModTime() time.Time {
	return time.Unix(int64(fi.MTimeS), int64(fi.MTimeN))
}
