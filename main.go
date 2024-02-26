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
	"syscall"
	"time"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

const cacheFilePath = "/tmp/lexido_conversation_cache.txt"
const maxWidth = 200

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

var p *tea.Program

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

	// Run the Bubble Tea program

	p = tea.NewProgram(initialModel())
	go func() {
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

	if err := cacheConversation(text_prompt + "\n" + responseContent); err != nil {
		log.Printf("Warning: Failed to cache conversation. Error: %v", err)
	}

	select {}
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
func runCommands(m model) {
	// Filter and concatenate selected commands
	var commands []string
	for i, selected := range m.selected {
		if selected {
			commands = append(commands, m.choices[i])
		}
	}

	for _, cmdStr := range commands {
		// Prepare the command to be executed
		// Note: This example uses '/bin/sh', but adjust according to your needs
		bin, err := exec.LookPath("/bin/sh")
		if err != nil {
			println("Failed to find '/bin/sh':", err.Error())
			return
		}

		// Prepare arguments for execution
		// The first argument is conventionally the name of the command being executed
		args := []string{"sh", "-c", cmdStr}

		// Use syscall.Exec to replace the current process with the new command
		err = syscall.Exec(bin, args, os.Environ())
		if err != nil {
			println("Failed to execute command:", err.Error())
		}
	}
}

// Tea code

type model struct {
	spinner                spinner.Model
	response               string
	choices                []string
	selected               []bool
	cursor                 int
	editing                bool
	input                  string
	width                  int
	height                 int
	displayedContentLength int
	commandless            bool
}

type appendResponseMsg string

func initialModel() model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	return model{
		spinner:                s,
		response:               "",
		choices:                make([]string, 0),
		selected:               make([]bool, 0),
		cursor:                 0,
		editing:                false,
		input:                  "",
		width:                  0,
		height:                 0,
		displayedContentLength: 0,
		commandless:            true,
	}
}

func tickCmd(interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

type tickMsg time.Time

func (m model) Init() tea.Cmd {
	return tea.Batch(tickCmd(100*time.Millisecond), m.spinner.Tick)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case appendResponseMsg:
		m.response += string(msg)
		m.choices = parseCommands(m.response)
		m.selected = make([]bool, len(m.choices)+1)
		m.commandless = m.choices == nil || len(m.choices) == 0
	case tickMsg:
		totalResponseLength := len(m.response)
		// Logic to increment displayedContentLength
		chunkSize := rand.Intn(7) + 2 // Random chunk size between 1 and 5
		m.displayedContentLength += chunkSize

		// Ensure we don't exceed the total content length
		if m.displayedContentLength > totalResponseLength {
			m.displayedContentLength = totalResponseLength
		}

		// Adjust the timing based on the proportion of the content displayed
		sleepMs := int(math.Max(float64(m.displayedContentLength)/float64(totalResponseLength)*30, 1))
		interval := time.Duration(sleepMs) * time.Millisecond

		return m, tickCmd(interval)

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "q" || msg.String() == "esc" {
			os.Exit(0)
		}
		if m.commandless {
			return m, nil
		}
		if m.editing {
			switch msg.String() {
			case "esc":
				m.editing = false
				m.input = ""
			case "enter":
				m.choices[m.cursor] = m.input
				m.editing = false
				m.input = ""
			case "backspace":
				if len(m.input) > 0 {
					m.input = m.input[:len(m.input)-1]
				}
			default:
				m.input += msg.String()
			}
		} else {
			switch msg.String() {
			case "enter":
				if m.cursor != len(m.choices) {
					m.selected[m.cursor] = !m.selected[m.cursor]
				} else {
					fmt.Print("\n\n")
					runCommands(m)
					os.Exit(0)
				}
			case "j", "down":
				if m.cursor < len(m.choices) {
					m.cursor++
				}
			case "k", "up":
				if m.cursor > 0 {
					m.cursor--
				}
			case "e":
				if m.cursor != len(m.choices) {
					m.editing = true
					m.input = m.choices[m.cursor]
				}
			}
		}
	case tea.WindowSizeMsg:
		// Optionally store the new dimensions
		m.width = msg.Width
		m.height = msg.Height
	default:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	}
	return m, nil
}

func (m model) View() string {
	var s strings.Builder

	s.WriteString("\033[0m")

	if m.response == "" {
		s.WriteString(fmt.Sprintf("%s Initializing...", m.spinner.View()))
		return s.String()
	}

	displayContent := m.response
	if len(displayContent) > m.displayedContentLength {
		displayContent = displayContent[:m.displayedContentLength]
	}

	wrappedResponse := wrapText(highlightCommands(displayContent), min(m.width, maxWidth))
	s.WriteString(wrappedResponse)

	if m.commandless {
		return s.String()
	}

	s.WriteString("\n—————————————————————\n")

	s.WriteString("Command List:\n\n")
	for i, todo := range m.choices {
		var selected, color string

		if m.selected[i] {
			selected = "x"
			color = "\033[32m"
		} else {
			selected = " "
			color = "\033[0m"
		}
		if m.cursor == i {
			if m.editing {
				s.WriteString(fmt.Sprintf("> "+color+"["+selected+"] %s█ (editing)\n", m.input))
			} else {
				s.WriteString(fmt.Sprintf("> "+color+"["+selected+"] %s\n", todo))
			}
		} else {
			s.WriteString(fmt.Sprintf("  "+color+"["+selected+"] %s\n", todo))
		}
		s.WriteString("\033[0m")
	}

	if m.cursor == len(m.choices) {
		s.WriteString(">   \033[32m[RUN]\033[0m\n")
	} else {
		s.WriteString("    [RUN]\n")
	}

	if !m.editing {
		s.WriteString(wrapText("\nPlease select the tasks to run. e to edit a task. q to quit. up/down to select", min(m.width, maxWidth)))
	} else {
		s.WriteString(wrapText(("\nEditing: Use normal keys to add text, backspace to delete, enter to save, esc to cancel."), min(m.width, maxWidth)))
	}

	return s.String()
}

func wrapText(text string, lineWidth int) string {
	// Split the text into paragraphs based on newline characters
	paragraphs := strings.Split(text, "\n")

	var wrappedText strings.Builder
	for i, paragraph := range paragraphs {
		// Wrap each paragraph individually
		wrappedParagraph := wrapParagraph(paragraph, lineWidth)
		wrappedText.WriteString(wrappedParagraph)

		// Don't add a newline character after the last paragraph
		if i < len(paragraphs)-1 {
			wrappedText.WriteString("\n")
		}
	}

	return wrappedText.String()
}

func wrapParagraph(paragraph string, lineWidth int) string {
	var result strings.Builder
	words := strings.Fields(strings.TrimSpace(paragraph))
	if len(words) < 1 {
		return ""
	}
	result.WriteString(words[0])
	spaceLeft := lineWidth - len(words[0])
	for _, word := range words[1:] {
		if len(word)+1 > spaceLeft {
			result.WriteString("\n" + word)
			spaceLeft = lineWidth - len(word)
		} else {
			result.WriteString(" " + word)
			spaceLeft -= (1 + len(word))
		}
	}
	return result.String()
}
