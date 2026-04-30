package conflict

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

type Block struct {
	FilePath           string
	OursLines          []string
	TheirsLines        []string
	BaseLines          []string
	StartLine          int
	EndLine            int
	SurroundingContext string
	Language           string
}

func ParseFile(path string) ([]Block, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}

	lang := detectLanguage(path)
	var blocks []Block
	i := 0
	for i < len(lines) {
		if strings.HasPrefix(lines[i], "<<<<<<<") {
			block := Block{FilePath: path, StartLine: i + 1, Language: lang}
			i++
			for i < len(lines) && !strings.HasPrefix(lines[i], "|||||||") && !strings.HasPrefix(lines[i], "=======") {
				block.OursLines = append(block.OursLines, lines[i])
				i++
			}
			if i < len(lines) && strings.HasPrefix(lines[i], "|||||||") {
				i++
				for i < len(lines) && !strings.HasPrefix(lines[i], "=======") {
					block.BaseLines = append(block.BaseLines, lines[i])
					i++
				}
			}
			if i < len(lines) && strings.HasPrefix(lines[i], "=======") {
				i++
			}
			for i < len(lines) && !strings.HasPrefix(lines[i], ">>>>>>>") {
				block.TheirsLines = append(block.TheirsLines, lines[i])
				i++
			}
			if i < len(lines) && strings.HasPrefix(lines[i], ">>>>>>>") {
				block.EndLine = i + 1
				i++
			}
			block.SurroundingContext = surroundingContext(lines, block.StartLine-1, block.EndLine, 20)
			blocks = append(blocks, block)
			continue
		}
		i++
	}
	return blocks, nil
}

func surroundingContext(lines []string, start, end, pad int) string {
	from := start - pad
	if from < 0 {
		from = 0
	}
	to := end + pad
	if to > len(lines) {
		to = len(lines)
	}
	return strings.Join(lines[from:to], "\n")
}

func detectLanguage(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return "go"
	case ".ts", ".tsx":
		return "typescript"
	case ".js", ".jsx":
		return "javascript"
	case ".py":
		return "python"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	case ".rb":
		return "ruby"
	case ".sql":
		return "sql"
	case ".json":
		return "json"
	case ".yaml", ".yml":
		return "yaml"
	case ".md":
		return "markdown"
	}
	return "text"
}
