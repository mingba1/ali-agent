package retrieval

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/gogf/gf/v2/frame/g"

	"ali-agent/internal/model"
	"ali-agent/internal/utility/keyword"
)

// FileSearch 在文件系统中执行 grep 风格检索，对应架构图中的 file system / branch state。
// branch state 由 retrieval.fs.root 指定（工作区根目录）。
func FileSearch(ctx context.Context, query string, maxResults int) ([]model.Hit, error) {
	root := g.Cfg().MustGet(ctx, "retrieval.fs.root", "./data/workspace").String()
	root, _ = filepath.Abs(root)
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return []model.Hit{}, nil
	}
	includeExt := g.Cfg().MustGet(ctx, "retrieval.fs.includeExt").Strings()
	maxFiles := int(g.Cfg().MustGet(ctx, "retrieval.fs.maxFiles", 2000).Int())

	// 中文用关键词抽取，英文/数字用空格切分。
	patterns := keyword.Extract(query)
	if len(patterns) == 0 {
		patterns = strings.Fields(strings.ToLower(query))
	}
	hits := make([]model.Hit, 0, maxResults)
	scanned := 0
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if scanned >= maxFiles {
			return filepath.SkipAll
		}
		ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
		if len(includeExt) > 0 && !contains(includeExt, ext) {
			return nil
		}
		scanned++
		matches, err := grepFile(path, patterns)
		if err != nil {
			return nil
		}
		for _, m := range matches {
			rel, _ := filepath.Rel(root, path)
			hits = append(hits, model.Hit{
				Source: "fs", FilePath: rel, LineNo: m.lineNo,
				Content: m.text, Ref: "file:" + rel + ":" + itoa(int64(m.lineNo)),
				Score: float64(m.matched),
			})
			if len(hits) >= maxResults {
				return filepath.SkipAll
			}
		}
		return nil
	})
	return hits, err
}

type match struct {
	lineNo  int
	text    string
	matched int
}

func grepFile(path string, patterns []string) ([]match, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []match
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		lower := strings.ToLower(line)
		matched := 0
		for _, p := range patterns {
			if p != "" && strings.Contains(lower, p) {
				matched++
			}
		}
		if matched > 0 {
			t := strings.TrimSpace(line)
			if len([]rune(t)) > 200 {
				t = string([]rune(t)[:200]) + "..."
			}
			out = append(out, match{lineNo: lineNo, text: t, matched: matched})
		}
	}
	return out, scanner.Err()
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
