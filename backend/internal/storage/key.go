package storage

import "strings"

func SanitizePath(filePath string) string {
	parts := strings.Split(filePath, "/")
	safeParts := make([]string, 0, len(parts))

	for _, part := range parts {
		if part == ".." {
			continue
		}
		safeParts = append(safeParts, part)
	}

	return strings.Join(safeParts, "/")
}
