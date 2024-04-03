package tgpt

import (
	"bufio"
	"os/exec"

	"github.com/micr0-dev/lexido/pkg/io"
)

func Generate(str_prompt string, channel chan<- string) {
	cmd := exec.Command("tgpt", str_prompt)

	// Create a pipe to the command's standard output
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		// Send the error to the channel (or handle it as needed)
		channel <- "Error creating StdoutPipe: " + err.Error()
		close(channel)
		return
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		// Send the error to the channel (or handle it as needed)
		channel <- "Error starting command: " + err.Error()
		close(channel)
		return
	}

	// Use a scanner to read the command's output line by line
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		// Send each line to the channel
		channel <- line
	}

	// Check for any errors that occurred while reading the command's output
	if err := scanner.Err(); err != nil {
		channel <- "Error reading command output: " + err.Error()
	}

	// Wait for the command to finish
	if err := cmd.Wait(); err != nil {
		channel <- "Error waiting for command to finish: " + err.Error()
	}

	// Close the channel to indicate that no more data will be sent
	close(channel)
}

func GenerateWhole(str_prompt string) (string, error) {
	return io.RunCmd("tgpt", "-w"+"\""+str_prompt+"\"")
}
