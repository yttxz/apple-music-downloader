package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/fatih/color"
)

func promptForSelectionIndexes(label string, total int) ([]int, error) {
	fmt.Println("Enter one or more IDs separated by commas, use ranges like 2-4, or type 'all'")
	cyanColor := color.New(color.FgCyan)
	cyanColor.Printf("%s: ", label)
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	return parseSelectionIndexes(input, total)
}

func parseSelectionIndexes(input string, total int) ([]int, error) {
	input = strings.TrimSpace(input)
	if strings.EqualFold(input, "all") {
		selected := make([]int, total)
		for i := 0; i < total; i++ {
			selected[i] = i + 1
		}
		return selected, nil
	}
	if input == "" {
		return nil, nil
	}

	parts := strings.FieldsFunc(input, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t'
	})
	selected := []int{}
	seen := make(map[int]bool)
	for _, part := range parts {
		if strings.Contains(part, "-") {
			rangeParts := strings.SplitN(part, "-", 2)
			start, err1 := strconv.Atoi(strings.TrimSpace(rangeParts[0]))
			end, err2 := strconv.Atoi(strings.TrimSpace(rangeParts[1]))
			if err1 != nil || err2 != nil || start < 1 || end > total || start > end {
				return nil, fmt.Errorf("invalid range: %s", part)
			}
			for i := start; i <= end; i++ {
				if !seen[i] {
					selected = append(selected, i)
					seen[i] = true
				}
			}
			continue
		}
		num, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || num < 1 || num > total {
			return nil, fmt.Errorf("invalid option: %s", part)
		}
		if !seen[num] {
			selected = append(selected, num)
			seen[num] = true
		}
	}
	return selected, nil
}
