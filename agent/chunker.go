package agent

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Chunk struct {
	Path          string
	Heading       string
	ParentHeading string
	Content       string
	CourseID      string
	Category      string
}

func ChunkFile(path string) ([]Chunk, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	content := strings.TrimSpace(string(data))
	if content == "" {
		return nil, nil
	}

	courseID := inferCourseID(path)
	category := inferCategory(path)

	parentHeading := extractParentHeading(content)
	headingRe := regexp.MustCompile(`(?m)^## ([^#\n].*)$`)
	matches := headingRe.FindAllStringSubmatchIndex(content, -1)

	if len(matches) == 0 {
		chunks := []Chunk{{
			Path:          path,
			Heading:       "",
			ParentHeading: parentHeading,
			Content:       content,
			CourseID:      courseID,
			Category:      category,
		}}
		return splitLongChunks(chunks), nil
	}

	var chunks []Chunk

	intro := strings.TrimSpace(content[:matches[0][0]])
	if intro != "" {
		chunks = append(chunks, Chunk{
			Path:          path,
			Heading:       "",
			ParentHeading: parentHeading,
			Content:       intro,
			CourseID:      courseID,
			Category:      category,
		})
	}

	for i, m := range matches {
		heading := content[m[2]:m[3]]
		sectionStart := m[0]
		var sectionEnd int
		if i+1 < len(matches) {
			sectionEnd = matches[i+1][0]
		} else {
			sectionEnd = len(content)
		}

		sectionFull := content[sectionStart:sectionEnd]
		firstNewline := strings.IndexByte(sectionFull, '\n')
		bodyContent := ""
		if firstNewline >= 0 {
			bodyContent = strings.TrimSpace(sectionFull[firstNewline:])
		}
		if bodyContent == "" {
			continue
		}

		chunks = append(chunks, Chunk{
			Path:          path,
			Heading:       heading,
			ParentHeading: parentHeading,
			Content:       bodyContent,
			CourseID:      courseID,
			Category:      category,
		})
	}

	return splitLongChunks(chunks), nil
}

func extractParentHeading(content string) string {
	re := regexp.MustCompile(`(?m)^# ([^#\n].*)$`)
	match := re.FindStringSubmatch(content)
	if len(match) > 1 {
		return strings.TrimSpace(match[1])
	}
	return ""
}

func splitLongChunks(chunks []Chunk) []Chunk {
	var result []Chunk
	for _, c := range chunks {
		if len(c.Content) <= 1500 {
			result = append(result, c)
			continue
		}
		paragraphs := strings.Split(c.Content, "\n\n")
		var current strings.Builder
		for _, p := range paragraphs {
			if current.Len()+len(p)+2 > 1500 && current.Len() > 0 {
				result = append(result, Chunk{
					Path:          c.Path,
					Heading:       c.Heading,
					ParentHeading: c.ParentHeading,
					Content:       strings.TrimSpace(current.String()),
					CourseID:      c.CourseID,
					Category:      c.Category,
				})
				current.Reset()
			}
			if current.Len() > 0 {
				current.WriteString("\n\n")
			}
			current.WriteString(p)
		}
		if current.Len() > 0 {
			result = append(result, Chunk{
				Path:          c.Path,
				Heading:       c.Heading,
				ParentHeading: c.ParentHeading,
				Content:       strings.TrimSpace(current.String()),
				CourseID:      c.CourseID,
				Category:      c.Category,
			})
		}
	}
	return result
}

func inferCourseID(path string) string {
	p := filepath.ToSlash(path)
	if strings.Contains(p, "/courses/") {
		parts := strings.Split(p, "/courses/")
		if len(parts) > 1 {
			courseParts := strings.Split(parts[1], "/")
			if len(courseParts) > 0 {
				return courseParts[0]
			}
		}
	}
	return ""
}

func inferCategory(path string) string {
	p := filepath.ToSlash(path)
	if strings.Contains(p, "/study-methods/") {
		return "study-method"
	}
	if strings.Contains(p, "/courses/") {
		return "concept"
	}
	if strings.Contains(p, "/meta/") {
		return "study-method"
	}
	return "concept"
}
