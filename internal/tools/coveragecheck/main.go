package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type block struct {
	statements int
	covered    bool
}

func main() {
	profile := flag.String("profile", "coverage.out", "Go cover profile")
	minimum := flag.Float64("min", 80, "minimum statement coverage percentage")
	flag.Parse()
	percentage, covered, total, err := coverage(*profile)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	fmt.Printf("production statement coverage: %.2f%% (%d/%d)\n", percentage, covered, total)
	if percentage+1e-9 < *minimum {
		fmt.Fprintf(os.Stderr, "coverage %.2f%% is below required %.2f%%\n", percentage, *minimum)
		os.Exit(1)
	}
}

func coverage(path string) (float64, int, int, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, 0, 0, err
	}
	defer file.Close()
	blocks := map[string]block{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "mode:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 3 {
			return 0, 0, 0, fmt.Errorf("invalid coverage line %q", line)
		}
		location := fields[0]
		if excluded(location) {
			continue
		}
		statements, statementErr := strconv.Atoi(fields[1])
		hits, hitsErr := strconv.Atoi(fields[2])
		if statementErr != nil || hitsErr != nil {
			return 0, 0, 0, fmt.Errorf("invalid coverage counters in %q", line)
		}
		value := blocks[location]
		value.statements = statements
		value.covered = value.covered || hits > 0
		blocks[location] = value
	}
	if err := scanner.Err(); err != nil {
		return 0, 0, 0, err
	}
	covered, total := 0, 0
	for _, value := range blocks {
		total += value.statements
		if value.covered {
			covered += value.statements
		}
	}
	if total == 0 {
		return 0, 0, 0, fmt.Errorf("coverage profile contains no production statements")
	}
	return float64(covered) * 100 / float64(total), covered, total, nil
}

func excluded(location string) bool {
	return strings.Contains(location, "/internal/testfixture/") || strings.Contains(location, "/internal/tools/coveragecheck/") || strings.Contains(location, "/taskflow/main.go:")
}
