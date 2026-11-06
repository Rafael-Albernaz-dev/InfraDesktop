package qualityrules

import (
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

func gitOutput(args ...string) ([]byte, error) { return exec.Command("git", args...).Output() }
func ChangedSources() (map[string][]Span, error) {
	raw, err := gitOutput("diff", "HEAD", "--unified=0", "--no-ext-diff", "--", "*.go", "*.tsx", "*.ts")
	if err != nil {
		return nil, err
	}
	result := map[string][]Span{}
	path := ""
	hunk := regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@`)
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "+++ b/") {
			path = strings.TrimPrefix(line, "+++ b/")
			// ignored: map membership does not need the stored spans.
			if _, ok := result[path]; !ok {
				result[path] = nil
			}
		}
		match := hunk.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		start, err := strconv.Atoi(match[1])
		if err != nil {
			return nil, err
		}
		count := 1
		if match[2] != "" {
			count, err = strconv.Atoi(match[2])
			if err != nil {
				return nil, err
			}
		}
		result[path] = append(result[path], Span{start, start + max(count, 1) - 1})
	}
	raw, err = gitOutput("ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil, err
	}
	for _, path := range strings.Split(string(raw), "\x00") {
		if strings.HasSuffix(path, ".go") || strings.HasSuffix(path, ".tsx") || strings.HasSuffix(path, ".ts") {
			result[path] = []Span{{1, int(^uint(0) >> 1)}}
		}
	}
	return result, nil
}
