package logtail

import (
	"bufio"
	"os"
)

func Lines(path string, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 200
	}
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) > limit {
			copy(lines, lines[len(lines)-limit:])
			lines = lines[:limit]
		}
	}
	return lines, scanner.Err()
}
