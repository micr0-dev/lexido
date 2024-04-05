package ollama

import (
	"bufio"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/micr0-dev/lexido/pkg/io"
)

var llmModel string

func Init(model string) error {
	llmList, err := io.RunCmd("ollama", "list")
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return errors.New("ollama not installed on system, please install it first using the guide on https://github.com/micr0-dev/lexido?tab=readme-ov-file#running-locally")
		}
		return err
	}

	//  Check if model is in llm list
	if !strings.Contains(llmList, model) {
		return errors.New("Model not installed in ollama, please install it first using 'ollama run " + model + "'")
	}

	llmModel = model

	return nil
}

func GenerateContentStream(str_prompt string) (<-chan string, error) {
	// Create a command. Replace "ollama", "run", llmModel with your actual command and arguments.
	cmd := exec.Command("ollama", "run", llmModel, "\""+str_prompt+"\"")

	// Get the command's standard output pipe.
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// Start the command.
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command: %w", err)
	}

	// Create a channel to send the output.
	outputChan := make(chan string)

	// Go routine to read command's standard output.
	go func() {
		defer close(outputChan)

		// Use bufio.NewReader to read the output line by line.
		reader := bufio.NewReader(stdout)
		for {
			line, err := reader.ReadString(' ')
			if err != nil {
				if strings.Contains(err.Error(), "EOF") {
					fmt.Printf("error reading stdout: %v\n", err)
				}
				break
			}

			// Send the line to the channel.
			outputChan <- line
		}

		// Wait for the command to finish.
		if err := cmd.Wait(); err != nil {
			fmt.Printf("command finished with error: %v\n", err)
		}
	}()

	return outputChan, nil
}
