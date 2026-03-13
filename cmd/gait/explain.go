package main

import (
	"fmt"
	"strings"
)

func hasExplainFlag(arguments []string) bool {
	for _, argument := range arguments {
		if strings.TrimSpace(argument) == "--explain" {
			return true
		}
	}
	return false
}

func hasJSONFlag(arguments []string) bool {
	for _, argument := range arguments {
		if strings.TrimSpace(argument) == "--json" {
			return true
		}
	}
	return false
}

func writeExplain(text string) int {
	fmt.Println(text)
	return exitOK
}
