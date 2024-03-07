package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

	tea "github.com/charmbracelet/bubbletea"
)

var execCmds []string

func main() {
	helpPtr := flag.Bool("help", false, "Display help information")
	hPtr := flag.Bool("h", false, "Display help information")
	cPtr := flag.Bool("c", false, "Continue previous conversation")

	flag.Parse()

	if *helpPtr || *hPtr {
		displayHelp()
		os.Exit(0)
	}

	ctx := context.Background()

	// Access your API key as an environment variable
	apiKey := os.Getenv("GOOGLE_AI_KEY")
	if apiKey == "" {
		fmt.Println("No API key found.")
		fmt.Println("Please visit https://aistudio.google.com/app/apikey to obtain your API key.")
		fmt.Print("Enter your API key here: ")

		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			apiKey = scanner.Text()
			os.Setenv("GOOGLE_AI_KEY", apiKey)
			if err := appendToShellConfig("GOOGLE_AI_KEY", apiKey); err != nil {
				fmt.Println("Failed to automatically append the API key to your shell configuration file. Please add the following line to your .bashrc, .zshrc, or equivalent file manually:")
				fmt.Printf("export GOOGLE_AI_KEY='%s'\n", apiKey)
			} else {
				fmt.Print("API key set successfully for future sessions. Please restart your terminal or source your profile for the changes to take effect.\n\n")
			}
		} else if scanner.Err() != nil {
			log.Fatalf("Error reading API key: %v", scanner.Err())
		}
	}

	// Set up the GenAI client
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Read piped input if present
	pipedInput, err := readPipedInput()
	if err != nil {
		log.Fatalf("Failed to read piped input: %v", err)
	}

	var text_prompt string

	if *cPtr {
		// Read previous conversation from cache if -c is present
		cachedConversation, err := readConversationCache()
		if err != nil {
			log.Printf("Warning: Could not read cache. Starting a new conversation. Error: %v", err)
		}
		text_prompt = cachedConversation + "\n"
	}

	text_prompt += strings.Join(os.Args[1:], " ")

	if text_prompt == "" {
		text_prompt = "The user did not provide a prompt."
	}

	// Append piped input to the prompt if available
	if pipedInput != "" {
		text_prompt += "\n\nUser also attached via pipe the following input:\n" + pipedInput
	}

	// Get some information about the user's system
	// Get the user's username
	userpath, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	username := userpath[strings.LastIndex(userpath, "/")+1:]

	// Get the user's hostname
	hostname, err := os.Hostname()
	if err != nil {
		log.Fatal(err)
	}

	// Get the user's current working directory
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	// Detect Operating System (MacOS or Linux)
	osname, err := runCmd("uname", "-s")
	if err != nil {
		log.Fatal(err)
	}

	var opperatingSystem string

	if strings.Contains(strings.ToLower(osname), "darwin") {
		opperatingSystem = "macOS"
	} else {
		// Get the user's full operating system if not MacOS
		opperatingSystem, err = extractHostnameCtlValue("Operating System")
		if err != nil {
			log.Fatal(err)
		}
	}

	pre_prompt := defaultPrePrompt

	// Set the default post-prompt
	pre_prompt += " The user, " + username + ", is currently running " + opperatingSystem + " on " + hostname + " in " + cwd + "."

	// Call Gemini Pro with the user's prompt
	model := client.GenerativeModel("gemini-pro")
	prompt := genai.Text(pre_prompt + "\n User: " + text_prompt)

	model.SetTemperature(0.7)
	model.SetTopK(1)

	model.SafetySettings = []*genai.SafetySetting{
		{
			Category:  genai.HarmCategoryHarassment,
			Threshold: genai.HarmBlockNone,
		},
		{
			Category:  genai.HarmCategoryHateSpeech,
			Threshold: genai.HarmBlockNone,
		},
		{
			Category:  genai.HarmCategorySexuallyExplicit,
			Threshold: genai.HarmBlockNone,
		},
		{
			Category:  genai.HarmCategoryDangerousContent,
			Threshold: genai.HarmBlockNone,
		},
	}

	// Run the Bubble Tea program

	wg := &sync.WaitGroup{}

	p = tea.NewProgram(initialModel())
	wg.Add(1)
	go func() {
		defer wg.Done()
		if _, err := p.Run(); err != nil {
			log.Printf("Alas, there's been an error: %v", err)
			os.Exit(1)
		}
	}()

	iter := model.GenerateContentStream(ctx, prompt)

	var responseContent string
	totalresponse := 1

	for {
		resp, err := iter.Next()
		if err != nil {
			if err == iterator.Done {
				break // End of stream
			}
			log.Println("An error occurred:", err)

			// Check if the error is due to safety filter activation
			if strings.Contains(err.Error(), "FinishReasonSafety") {
				fmt.Println("The content generation was blocked for safety reasons. Please try a different prompt.")
				fmt.Println(resp.PromptFeedback.BlockReason.String())
				return // Exit the loop and potentially allow for a new attempt
			}

			log.Fatal(err) // For any other type of error, terminate
		}

		for _, part := range resp.Candidates[0].Content.Parts {
			totalresponse += len(fmt.Sprintf("%v", part))
			p.Send(appendResponseMsg(fmt.Sprintf("%v", part)))

			responseContent += fmt.Sprintf("%v", part)
		}
	}

	p.Send(generationDoneMsg{})

	if err := cacheConversation(text_prompt + "\n" + responseContent); err != nil {
		log.Printf("Warning: Failed to cache conversation. Error: %v", err)
	}

	wg.Wait()

	// Run the commands
	runCommands(execCmds)
}

// Function to parse commands from the response @run[<COMMAND>]
func parseCommands(responseContent string) []string {
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
func highlightCommands(responseContent string) string {
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
func runCommands(commands []string) {
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
