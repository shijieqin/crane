package utils

import (
	"bufio"
	"os"
	"strings"
)

func ReadLines(file string) ([]string, error) {
	f, err := os.Open(file)
	if err != nil {
		return []string{""}, err
	}
	defer f.Close()

	b := bufio.NewReader(f)

	var ret []string

	for {
		line, err := b.ReadString('\n')
		if err != nil {
			break
		}
		ret = append(ret, strings.Trim(line, "\n"))
	}

	return ret, nil
}