// Package qualitydup measures repeated ten-line windows in authored production sources.
package qualitydup

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func Windows(source string) []string {
	var lines []string
	for _, line := range strings.Split(source, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		lines = append(lines, strings.Join(strings.Fields(line), " "))
	}
	var result []string
	for i := 0; i+10 <= len(lines); i++ {
		result = append(result, strings.Join(lines[i:i+10], "\n"))
	}
	return result
}
func Measure(roots ...string) (int, error) {
	seen := map[string]int{}
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			if strings.Contains(path, ".test.") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			ext := filepath.Ext(path)
			if ext != ".go" && ext != ".ts" && ext != ".tsx" {
				return nil
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for _, window := range Windows(string(content)) {
				seen[window]++
			}
			return nil
		})
		if err != nil {
			return 0, err
		}
	}
	total := 0
	for _, count := range seen {
		total += count - 1
	}
	return total, nil
}
func Exceeds(current, ceiling int) bool { return current > ceiling }
