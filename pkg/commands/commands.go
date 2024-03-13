package commands

import (
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// Function to parse commands from the response @run[<COMMAND>]
func ParseCommands(responseContent string) []string {
	// Regular expression to find @run[<COMMAND>]
	re := regexp.MustCompile(`@run\[(.*?)\]`)
	matches := re.FindAllStringSubmatch(responseContent, -1)

	var commands []string
	for _, match := range matches {
		// Each match is a slice where the first element is the whole match
		// and the second element is the captured group (the command)
		if len(match) > 1 {
			commands = append(commands, match[1]) // Append the command
		}
	}

	return commands
}

// Function to highlight all occurrences of @run[<COMMAND>] in the responseContent
func HighlightCommands(responseContent string) string {
	// Regular expression to find @run[<COMMAND>]
	re := regexp.MustCompile(`(@run\[(.*?)\])`)

	// ANSI color codes for highlighting
	startHighlight := "\033[34m"
	endHighlight := "\033[0m"

	// Replace matches with highlighted version
	highlightedContent := re.ReplaceAllStringFunc(responseContent, func(match string) string {
		return startHighlight + match[5:len(match)-1] + endHighlight
	})

	return highlightedContent
}

// Run commands from model
func RunCommands(commands []string) {
	for _, cmdStr := range commands {
		parts := strings.Fields(cmdStr)
		cmd := exec.Command(parts[0], parts[1:]...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			log.Printf("Error running command %q: %v", cmdStr, err)
			continue
		}
	}
}

// Function to detect if any of the commands are being ran as sudo
func ContainsSudo(commands []string) bool {
	for _, cmdStr := range commands {
		if strings.HasPrefix(cmdStr, "sudo") {
			return true
		}
	}
	return false
}
