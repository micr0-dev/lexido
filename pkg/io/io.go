package io

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const cacheDir = ".lexido"
const cacheFile = "lexido_conversation_cache.txt"
const keyringFile = "keyring.json"

// Writes conversation to cache file
func CacheConversation(conversation string) error {
	filePath, err := GetFilePath(cacheFile)
	if err != nil {
		return err
	}

	// Ensure the .lexido directory exists
	err = os.MkdirAll(filepath.Dir(filePath), 0700)
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, []byte(conversation), 0644)
}

// Reads conversation from cache file
func ReadConversationCache() (string, error) {
	filePath, err := GetFilePath(cacheFile)
	if err != nil {
		return "", err
	}

	// Ensure the .lexido directory exists
	err = os.MkdirAll(filepath.Dir(filePath), 0700)
	if err != nil {
		return "", err
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// Helper function to check and read piped input
func ReadPipedInput() (string, error) {
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

func GetFilePath(file string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, cacheDir, file), nil
}

func SaveToKeyring(field string, val string) error {
	filePath, err := GetFilePath(keyringFile)
	if err != nil {
		return err
	}

	// Ensure the .lexido directory exists
	err = os.MkdirAll(filepath.Dir(filePath), 0700)
	if err != nil {
		return err
	}

	// Load existing data
	data := make(map[string]string)
	file, err := os.ReadFile(filePath)
	if err == nil {
		json.Unmarshal(file, &data)
	}

	// Update the data with the new value
	data[field] = val

	// Write updated data back to file
	updatedData, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, updatedData, 0600)
}

func ReadFromKeyring(field string) (string, error) {
	filePath, err := GetFilePath(keyringFile)
	if err != nil {
		return "", err
	}

	// Read the file
	file, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	// Unmarshal the data into a map
	data := make(map[string]string)
	err = json.Unmarshal(file, &data)
	if err != nil {
		return "", err
	}

	// Retrieve the value for the specified field
	val, ok := data[field]
	if !ok {
		return "", errors.New("field not found")
	}

	return val, nil
}

// Helper function to run command and return trimmed output string
func RunCmd(command string, args ...string) (string, error) {
	cmd := exec.Command(command, args...)
	data, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// Helper function to extract value from hostnamectl output
func ExtractHostnameCtlValue(field string) (string, error) {
	txtcmd := fmt.Sprintf("hostnamectl | grep \"%s\"", field)
	data, err := RunCmd("bash", "-c", txtcmd)
	if err != nil {
		return "", err
	}
	// Replace field name and remove leading and trailing white spaces
	return strings.TrimSpace(strings.ReplaceAll(data, field+":", "")), nil
}

func DisplayHelp() {
	fmt.Println(`Lexido Command Line Tool Usage:

Usage:
    To get command suggestions:
        lexido "install teamspeak via docker"

    To continue with a previous prompt:
        lexido -c "add more details or follow-up"

    To use with piping commands:
        ls | lexido "what should I do with these files?"
    
Options:
    -h, --help          Display help information
    -c                  Continue with a previous prompt or add more details to it`)
}
