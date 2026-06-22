package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"git.linux-hosting.co.il/shukivaknin/jabali2/agentwire"
	"git.linux-hosting.co.il/shukivaknin/jabali2/internal/filesafe"
)

// files.du — disk-usage view of one directory's immediate children, scoped to
// the user's home. Unlike files.list (which reports a directory's inode size),
// each subdirectory carries its RECURSIVE byte size (one `du --max-depth=1`
// call); files carry their own size. Powers the Files disk-usage tree, which
// lazily fetches one level per expand.
//
//	files.du {user_id, username, path} -> {path, total, entries[]}

type filesDuParams struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Path     string `json:"path"`
}

type filesDuEntry struct {
	Name       string `json:"name"`
	IsDir      bool   `json:"is_dir"`
	Size       int64  `json:"size"`
	HasSubdirs bool   `json:"has_subdirs"`
}

type filesDuResponse struct {
	Path    string         `json:"path"`
	Total   int64          `json:"total"`
	Entries []filesDuEntry `json:"entries"`
}

func filesDuHandler(ctx context.Context, params json.RawMessage) (any, error) {
	var p filesDuParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInvalidArgument, Message: fmt.Sprintf("parse params: %v", err)}
	}
	if p.Username == "" || p.Path == "" {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInvalidArgument, Message: "username and path required"}
	}

	homeDir := fmt.Sprintf("/home/%s", p.Username)
	scope, err := filesafe.NewScope(p.UserID, p.Username, []string{homeDir})
	if err != nil {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInvalidArgument, Message: fmt.Sprintf("scope: %v", err)}
	}
	resolved, err := scope.Resolve(p.Path)
	if err != nil {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInvalidArgument, Message: fmt.Sprintf("path validation failed: %v", err)}
	}

	// One `du` call gives every immediate subdir's recursive size (+ the dir
	// itself). -b = apparent size in bytes; --max-depth=1 = this level only.
	dirSizes := map[string]int64{}
	out, _ := exec.CommandContext(ctx, "du", "-b", "--max-depth=1", resolved).Output()
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		tab := strings.IndexByte(line, '\t')
		if tab <= 0 {
			continue
		}
		sz, perr := strconv.ParseInt(strings.TrimSpace(line[:tab]), 10, 64)
		if perr != nil {
			continue
		}
		dirSizes[strings.TrimSpace(line[tab+1:])] = sz
	}

	entries, err := os.ReadDir(resolved)
	if err != nil {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInternal, Message: fmt.Sprintf("read dir: %v", err)}
	}
	result := []filesDuEntry{}
	for _, e := range entries {
		full := filepath.Join(resolved, e.Name())
		info, lerr := os.Lstat(full)
		if lerr != nil {
			continue
		}
		isLink := (info.Mode() & os.ModeSymlink) != 0
		isDir := info.IsDir() && !isLink
		var size int64
		hasSub := false
		if isDir {
			if v, ok := dirSizes[full]; ok {
				size = v
			}
			hasSub = dirHasSubdir(full)
		} else {
			size = info.Size()
		}
		result = append(result, filesDuEntry{Name: e.Name(), IsDir: isDir, Size: size, HasSubdirs: hasSub})
	}
	// Largest first — disk-usage view wants the heavy entries up top.
	sort.SliceStable(result, func(i, j int) bool { return result[i].Size > result[j].Size })

	return &filesDuResponse{Path: resolved, Total: dirSizes[resolved], Entries: result}, nil
}

func init() {
	Default.Register("files.du", filesDuHandler)
}
