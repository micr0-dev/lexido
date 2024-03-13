package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/google/generative-ai-go/genai"
	"github.com/micr0-dev/lexido/pkg/commands"
	"github.com/micr0-dev/lexido/pkg/io"
	"github.com/micr0-dev/lexido/pkg/prompt"
	"github.com/micr0-dev/lexido/pkg/tea"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

	tearaw "github.com/charmbracelet/bubbletea"
)

var p *tearaw.Program

func main() {
	helpPtr := flag.Bool("help", false, "Display help information")
	hPtr := flag.Bool("h", false, "Display help information")
	cPtr := flag.Bool("c", false, "Continue previous conversation")

	flag.Parse()

	if *helpPtr || *hPtr {
		io.DisplayHelp()
		os.Exit(0)
	}

	ctx := context.Background()

	// Access your API key from keyring or environment variable (backwards compatible with previous versions)
	apiKey := os.Getenv("GOOGLE_AI_KEY")

	if apiKey == "" {
		apiKey, _ = io.ReadFromKeyring("GOOGLE_AI_KEY")
	}

	// If no API key is found, prompt the user to enter it
	if apiKey == "" {
		fmt.Println("No API key found.")
		fmt.Println("Please visit https://aistudio.google.com/app/apikey to obtain your API key.")
		fmt.Print("Enter your API key here: ")

		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			apiKey = scanner.Text()
			os.Setenv("GOOGLE_AI_KEY", apiKey)
			if err := io.SaveToKeyring("GOOGLE_AI_KEY", apiKey); err != nil {
				fmt.Println("Failed to automatically append the API key to keyring. Please add the following line to your .bashrc, .zshrc, or equivalent file manually:")
				fmt.Printf("export GOOGLE_AI_KEY='%s'\n", apiKey)
			} else {
				fmt.Print("API key set successfully for future sessions. \n\n")
			}
		} else if scanner.Err() != nil {
			log.Printf("Error reading API key: %v\n", scanner.Err())
			os.Exit(1)
		}
	}

	// Set up the GenAI client
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		log.Printf("Failed to create client: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	// Read piped input if present
	pipedInput, err := io.ReadPipedInput()
	if err != nil {
		log.Printf("Failed to read piped input: %v\n", err)
		pipedInput = ""
	}

	var text_prompt string

	if *cPtr {
		// Read previous conversation from cache if -c is present
		cachedConversation, err := io.ReadConversationCache()
		if err != nil {
			log.Printf("Warning: Could not read cache. Starting a new conversation. Error: %v\n", err)
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
	username := "Unknown"
	userpath, err := os.UserHomeDir()
	if err != nil {
		log.Println(err)
	} else {
		username = userpath[strings.LastIndex(userpath, "/")+1:]
	}

	// Get the user's hostname
	hostname, err := os.Hostname()
	if err != nil {
		log.Println(err)
		hostname = "Unknown"
	}

	// Get the user's current working directory
	cwd, err := os.Getwd()
	if err != nil {
		log.Println(err)
		cwd = "Unknown"
	}

	// Detect Operating System (MacOS or Linux)
	osname, err := io.RunCmd("uname", "-s")
	if err != nil {
		log.Println(err)
		osname = "Unknown"
	}

	var opperatingSystem string

	if strings.Contains(strings.ToLower(osname), "darwin") {
		opperatingSystem = "macOS"
	} else {
		// Get the user's full operating system if not MacOS
		opperatingSystem, err = io.ExtractHostnameCtlValue("Operating System")
		if err != nil {
			log.Println(err)
			opperatingSystem = "Linux"
		}
	}

	pre_prompt := prompt.DefaultPrePrompt

	// Set the default post-prompt
	pre_prompt += " The user, " + username + ", is currently running " + opperatingSystem + " on " + hostname + " in " + cwd + "."

	// Detect all installed package managers
	installedManagers := io.DetectPackageManagers()
	pre_prompt += " The user has the following package managers installed: " + strings.Join(installedManagers, ", ") + "."

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

	cmds := new([]string)

	p = tearaw.NewProgram(tea.InitialModel(cmds))
	wg.Add(1)

	// Properly close the program if something goes wrong
	defer p.Quit()

	go func() {
		defer wg.Done()
		if _, err := p.Run(); err != nil {
			log.Printf("Alas, there's been a Bubble Tea error: %v\n", err)
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
				os.Exit(1)
			}

			log.Println(err) // For any other type of error, terminate
			os.Exit(1)
		}

		for _, part := range resp.Candidates[0].Content.Parts {
			totalresponse += len(fmt.Sprintf("%v", part))
			p.Send(tea.AppendResponseMsg(fmt.Sprintf("%v", part)))

			responseContent += fmt.Sprintf("%v", part)
		}
	}

	p.Send(tea.GenerationDoneMsg{})

	err = io.CacheConversation(text_prompt + "\n" + responseContent)
	if err != nil {
		log.Printf("Warning: Failed to cache conversation. Error: %v", err)
	}

	wg.Wait()

	// Run the commands
	commands.RunCommands(*cmds)
}
