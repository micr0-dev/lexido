package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/micr0-dev/lexido/pkg/commands"
	"github.com/micr0-dev/lexido/pkg/io"
	gemini "github.com/micr0-dev/lexido/pkg/llms/gemini"
	ollama "github.com/micr0-dev/lexido/pkg/llms/ollama"
	"github.com/micr0-dev/lexido/pkg/prompt"
	"github.com/micr0-dev/lexido/pkg/tea"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/iterator"

	tearaw "github.com/charmbracelet/bubbletea"
)

var p *tearaw.Program

const version = "1.2.1" // Program version

func main() {
	helpPtr := flag.Bool("help", false, "Display help information")
	hPtr := flag.Bool("h", false, "Display help information")
	cPtr := flag.Bool("c", false, "Continue previous conversation")
	vPtr := flag.Bool("v", false, "Display version information")
	versionPtr := flag.Bool("version", false, "Display version information")

	lPtr := flag.Bool("l", false, "Utilize a local LLM via ollama instead of Gemini Pro")
	mPtr := flag.String("m", "", "Specify the model to use with ollama, only required if -l is used")

	setMPtr := flag.String("setModel", "", "Set the default model to use with ollama")
	setLPtr := flag.Bool("setLocal", false, "Toggle the default to use a local LLM via ollama instead of Gemini Pro")

	flag.Parse()

	if *helpPtr || *hPtr {
		io.DisplayHelp()
		os.Exit(0)
	}

	if *vPtr || *versionPtr {
		io.DisplayVersion(version)
		os.Exit(0)
	}

	_, err := io.ReadFromKeyring("OLLAMA_MODEL")
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			err = io.SaveToKeyring("OLLAMA_MODEL", "llama2")
			if err != nil {
				log.Printf("Error saving model: %v\n", err)
				os.Exit(1)
			}
		}
	}
	if *setMPtr != "" {
		err := io.SaveToKeyring("OLLAMA_MODEL", *setMPtr)
		if err != nil {
			log.Printf("Error saving model: %v\n", err)
			os.Exit(1)
		}
	}

	local, err := io.ReadFromKeyring("OLLAMA_LOCAL")
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			err = io.SaveToKeyring("OLLAMA_LOCAL", "false")
			if err != nil {
				log.Printf("Error saving model: %v\n", err)
				os.Exit(1)
			}
		}
	}
	if *setLPtr {
		isLocal, err := strconv.ParseBool(local)
		if err != nil {
			log.Printf("Error reading model: %v\n", err)
			os.Exit(1)
		}

		err = io.SaveToKeyring("OLLAMA_LOCAL", strconv.FormatBool(!isLocal))
		if err != nil {
			log.Printf("Error saving model: %v\n", err)
			os.Exit(1)
		}
	}

	local, err = io.ReadFromKeyring("OLLAMA_LOCAL")
	if err != nil {
		log.Printf("Error reading model: %v\n", err)
		os.Exit(1)
	}
	isLocal, err := strconv.ParseBool(local)
	if err != nil {
		log.Printf("Error reading model: %v\n", err)
		os.Exit(1)
	}

	isGemini := true

	if *lPtr || isLocal {
		isGemini = false
	}

	if isGemini {

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
					fmt.Println("Failed to automatically append the API key to keyring. Please add the following line to your .bashrc, .zshrc, or equivalent file manually (replace the {API_KEY_HERE} with your API key):")
					fmt.Println("export GOOGLE_AI_KEY={API_KEY_HERE}")
				} else {
					fmt.Print("API key set successfully for future sessions. \n\n")
				}
			} else if scanner.Err() != nil {
				log.Printf("Error reading API key: %v\n", scanner.Err())
				os.Exit(1)
			}
		}

		err = gemini.Setup(apiKey)
		if err != nil {
			log.Printf("Error setting up gemini: %v\n", err)
			os.Exit(1)
		}
	} else {
		model := *mPtr
		if *mPtr == "" {
			model, err = io.ReadFromKeyring("OLLAMA_MODEL")
			if err != nil {
				log.Printf("Error reading model: %v\n", err)
				os.Exit(1)
			}
		}

		err := ollama.Init(model)
		if err != nil {
			log.Printf("Error initializing ollama: %v\n", err)
			os.Exit(1)
		}
	}
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
	str_prompt := pre_prompt + "\n User: " + text_prompt

	// Run the Bubble Tea program

	wg := &sync.WaitGroup{}

	cmds := new([]string)

	p = tearaw.NewProgram(tea.InitialModel(cmds, !isGemini))
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

	var responseContent string
	totalresponse := 1
	if isGemini {
		iter := gemini.Generate(str_prompt)
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

				var gerr *googleapi.Error
				if !errors.As(err, &gerr) {
					log.Printf("error: %s\n", err)
					os.Exit(1)
				} else {
					log.Printf("error details: %s\n", gerr)
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
	} else {
		response, err := ollama.Generate(str_prompt)
		if err != nil {
			log.Printf("Error generating response: %v\n", err)
			os.Exit(1)
		}
		responseContent += response
		p.Send(tea.AppendResponseMsg(response))
		totalresponse += len(response)

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
