package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strings"
)

const cacheFilePath = "/tmp/lexido_conversation_cache.txt"

// Writes conversation to cache file
func cacheConversation(conversation string) error {
	return os.WriteFile(cacheFilePath, []byte(conversation), 0644)
}

// Reads conversation from cache file
func readConversationCache() (string, error) {
	content, err := os.ReadFile(cacheFilePath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// Helper function to check and read piped input
func readPipedInput() (string, error) {
	fileInfo, err := os.Stdin.Stat()
	if err != nil {
		return "", err
	}

	// Check if data is being piped into stdin
	if fileInfo.Mode()&os.ModeNamedPipe != 0 {
		var inputData string
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			inputData += scanner.Text() + "\n"
		}
		if err := scanner.Err(); err != nil {
			return "", err
		}
		return inputData, nil
	}
	return "", nil // No piped data
}

func appendToShellConfig(varName, apiKey string) error {
	usr, err := user.Current()
	if err != nil {
		return err
	}
	shellConfigPath := ""
	if pathExists(usr.HomeDir + "/.zshrc") {
		shellConfigPath = usr.HomeDir + "/.zshrc"
	} else if pathExists(usr.HomeDir + "/.bashrc") {
		shellConfigPath = usr.HomeDir + "/.bashrc"
	} else {
		return fmt.Errorf("could not find a supported shell configuration file")
	}

	// Attempt to append the variable
	line := fmt.Sprintf("\nexport %s='%s'\n", varName, apiKey)
	file, err := os.OpenFile(shellConfigPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(line)
	return err
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// Helper function to run command and return trimmed output string
func runCmd(command string, args ...string) (string, error) {
	cmd := exec.Command(command, args...)
	data, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// Helper function to extract value from hostnamectl output
func extractHostnameCtlValue(field string) (string, error) {
	txtcmd := fmt.Sprintf("hostnamectl | grep \"%s\"", field)
	data, err := runCmd("bash", "-c", txtcmd)
	if err != nil {
		return "", err
	}
	// Replace field name and remove leading and trailing white spaces
	return strings.TrimSpace(strings.ReplaceAll(data, field+":", "")), nil
}
