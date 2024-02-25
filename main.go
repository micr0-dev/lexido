package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"os/user"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

	tea "github.com/charmbracelet/bubbletea"
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
	if pathExists(usr.HomeDir + "/.bashrc") {
		shellConfigPath = usr.HomeDir + "/.bashrc"
	} else if pathExists(usr.HomeDir + "/.zshrc") {
		shellConfigPath = usr.HomeDir + "/.zshrc"
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

func main() {
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
				fmt.Println("API key set successfully for future sessions. Please restart your terminal or source your profile for the changes to take effect.\n")
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

	continuePrevious := false
	for _, arg := range os.Args[1:] {
		if arg == "-c" {
			continuePrevious = true
			break
		}
	}

	// Read piped input if present
	pipedInput, err := readPipedInput()
	if err != nil {
		log.Fatalf("Failed to read piped input: %v", err)
	}

	var text_prompt string

	if continuePrevious {
		// Read previous conversation from cache if -c is present
		cachedConversation, err := readConversationCache()
		if err != nil {
			log.Printf("Warning: Could not read cache. Starting a new conversation. Error: %v", err)
		}
		text_prompt = cachedConversation + "\n"
	}

	//take all args after the first one and join them into a single string
	text_prompt += strings.Join(os.Args[1:], " ")

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

	opperatingSystem, err := extractHostnameCtlValue("Operating System")
	if err != nil {
		log.Fatal(err)
	}

	// import default pre-prompt from the instructions.txt file

	default_pre_prompt := "You are lexido, an AI tool for the linux command line. You are helpful and clever. You know a lot about UNIX and Linux commands, and you are always ready to get things done. Your goal is to do what the user wants. Just do it, don't talk to much, only say crucial information. Explain the basics of what you are doing. Do not use latex or markdown, always answer in plain text. Do not use emojis or emoticons unless told otherwise. Assume that the user would prefer a terminal answer, not GUI instructions. You have to ability to suggest running commands and scripts to the user. The syntax to run a command is @run[<CODE HERE>] all commands are to be in bash. Use it after explaing to the user what it will do. ALWAYS explain to the user what you are doing, ALWAYS. Here are some examples of what you can do: @run[ls -l] or @run[echo 'Hello World']. You can also write multiple lines of code in the command such as @run[echo 'Hello'; echo 'World']. You can also run scripts such as @run[./script.sh]. You can also run commands that require user input such as @run[read -p 'Enter your name: ' name; echo 'Hello, $name!']. Don’t ask the user questions, make educated guesses or put the question into the command. Such as @run[read -p Where would you like to make a directory?' directory; mkdir $directory] Only put functional code into the command. Do not put code that is not functional or is hypothetical."

	// Set the default post-prompt
	default_pre_prompt += " The user, " + username + ", is currently running " + opperatingSystem + " on " + hostname + " in " + cwd + "."

	// Call Gemini Pro with the user's prompt
	model := client.GenerativeModel("gemini-pro")
	prompt := genai.Text(default_pre_prompt + "\n User: " + text_prompt)

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

	var wg sync.WaitGroup
	contentChannel := make(chan string)

	wg.Add(1)

	iter := model.GenerateContentStream(ctx, prompt)

	var responseContent string
	totalresponse := 1

	go printContentSmoothly(contentChannel, &wg, &totalresponse)

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
			contentChannel <- fmt.Sprintf("%v", part)
			totalresponse += len(fmt.Sprintf("%v", part))
			// fmt.Print(part)
			responseContent += fmt.Sprintf("%v", part)
		}
	}
	close(contentChannel)

	if err := cacheConversation(text_prompt + "\n" + responseContent); err != nil {
		log.Printf("Warning: Failed to cache conversation. Error: %v", err)
	}

	wg.Wait()

	commands := parseCommands(responseContent)

	fmt.Println()

	// Run the Bubble Tea program

	p := tea.NewProgram(initialModel(commands))
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
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

func printContentSmoothly(contentChannel <-chan string, wg *sync.WaitGroup, totalresponse *int) {
	defer wg.Done()
	var totalprint int

	for content := range contentChannel {
		length := len(content)

		var chunkSize, sleepMs int

		for i := 0; i < length; i += chunkSize {
			chunkSize = rand.Intn(5) + 1
			sleepMs = int(math.Max(float64(float64(totalprint)/float64(*totalresponse))*20, 0))

			end := i + chunkSize
			if end > length {
				end = length
			}
			fmt.Print(content[i:end])
			totalprint += end - i
			time.Sleep(time.Duration(sleepMs) * time.Millisecond)
		}
	}
}

// Tea code

type model struct {
	choices  []string
	cursor   int
	selected map[int]struct{}
}

func initialModel(commands []string) model {
	return model{
		// Our to-do list is a grocery list
		choices: commands,

		// A map which indicates which choices are selected. We're using
		// the  map like a mathematical set. The keys refer to the indexes
		// of the `choices` slice, above.
		selected: make(map[int]struct{}),
	}
}

func (m model) Init() tea.Cmd {
	// Just return `nil`, which means "no I/O right now, please."
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	// Is it a key press?
	case tea.KeyMsg:

		// Cool, what was the actual key pressed?
		switch msg.String() {

		// These keys should exit the program.
		case "ctrl+c", "q":
			return m, tea.Quit

		// The "up" and "k" keys move the cursor up
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		// The "down" and "j" keys move the cursor down
		case "down", "j":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			}

		// The "enter" key and the spacebar (a literal space) toggle
		// the selected state for the item that the cursor is pointing at.
		case "enter", " ":
			_, ok := m.selected[m.cursor]
			if ok {
				delete(m.selected, m.cursor)
			} else {
				m.selected[m.cursor] = struct{}{}
			}
		}
	}

	// Return the updated model to the Bubble Tea runtime for processing.
	// Note that we're not returning a command.
	return m, nil
}

func (m model) View() string {
	// The header
	s := "\nWhich of the provided commands would you like to run?\n\n"

	// Iterate over our choices
	for i, choice := range m.choices {

		// Is the cursor pointing at this choice?
		cursor := " " // no cursor
		if m.cursor == i {
			cursor = ">" // cursor!
		}

		// Is this choice selected?
		checked := " " // not selected
		if _, ok := m.selected[i]; ok {
			checked = "x" // selected!
		}

		// Render the row
		s += fmt.Sprintf("%s [%s] %s\n", cursor, checked, choice)
	}

	// The footer
	s += "\nPress q to quit.\n"

	// Send the UI for rendering
	return s
}
