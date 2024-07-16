![lexidoLogo](https://github.com/micr0-dev/lexido/assets/26364458/d19ad5bb-e5d2-416d-9319-408325dc1fb8)

![Badge Build] 
![Badge Version]
![Badge License]
![Badge Language] 


## Introduction
Lexido is an innovative assistant for the command line, designed to boost your productivity and efficiency. Powered by Gemini 1.5 Flash and utilizing the free API, Lexido offers smart suggestions for commands based on your prompts and importantly **your current environment**. Whether you're installing software, managing files, or configuring system settings, Lexido streamlines the process, making it faster and more intuitive.

## Examples
<p align="center">
  <img src="./demos/teamspeak_demo.gif" alt="First GIF" width="49%"/>
  <img src="./demos/drivers_demo.gif" alt="Second GIF" width="49%"/>
</p>
<p align="center">
  <img src="https://imgs.xkcd.com/comics/tar_2x.png" alt="XKCD 1168" width="100%"/>
</p>

## Features
- **Command Suggestions**: Simply type `lexido [prompt]` to get actionable command suggestions.
- **Cross-Platform**: Support for both Linux and macOS
- **Continued Conversations**: Use `lexido -c [prompt]` to continue a previous conversation, allowing for context-aware suggestions.
- **Piping Support**: Pipe commands into Lexido (e.g., `ls | lexido [prompt]`) for enhanced command list suggestions.
- **Efficiency**: Designed with efficiency in mind, Lexido helps you get things done NOW.

## Installation

[![Packaging status](https://repology.org/badge/vertical-allrepos/lexido.svg)](https://repology.org/project/lexido/versions)

### For Arch Linux:
Lexido is available on the AUR. Install it using the package manager of your choice such as:
```bash
yay -S lexido
```

### For macOS:
Lexido is available on homebrew. Install it using the `brew` command:
```bash
brew install lexido
```

### For others:
Head to the [releases](https://github.com/micr0-dev/lexido/releases) tab to pick up a binary!
Once downloaded, you'll want to make Lexido easily accessible from anywhere on your computer. Here's how:

1. **Rename the downloaded file:**

   The downloaded file might have a long and specific name, like "v1.0-lexido-linux-amd64". For easier use, consider renaming it to just "lexido". 

2. **Make the file executable:**

   Open your terminal and navigate to the folder where you downloaded the Lexido binary. Then, run the following command to make the file executable:

   ```bash
   chmod +x ./lexido
   ```

3. **Move the file to your system's path:**

   **Why do this?** By placing the renamed and executable Lexido file in your system's path, you can run it from any terminal window without needing to specify the full path to the file. It's a shortcut for convenience!

   Here's an example command assuming you downloaded Lexido to your Downloads folder:

   ```bash
   mv ~/Downloads/lexido /usr/local/bin/lexido
   ```

### Compile from source
Ensure you have Go installed on your system. Follow these steps to install Lexido:

1. Clone the Lexido repository:
```bash
git clone https://github.com/micr0-dev/lexido.git
```

2. Navigate to the Lexido directory:
```bash
cd lexido
```

3. Build the project:
```bash
go build
```

4. Optionally, move the Lexido binary to a location in your PATH for easy access.

## Running locally
If you want to run lexido completely locally you can do that as of version 1.3! This is done via [Ollama](https://github.com/ollama/ollama), a tool for easily running large language models locally. It does all the hard work of installing LLMs for you!

You can install [Ollama](https://github.com/ollama/ollama) as follows:

### Linux:
```
curl -fsSL https://ollama.com/install.sh | sh
```

### macOS:
[Download](https://ollama.com/download/Ollama-darwin.zip)

#### After you have installed Ollama
Running lexido locally is as easy as adding the `-l` flag when you want to run locally, or using `--setLocal` to run locally by default! You can also select the model you want to run with `-m` and again set it to be the default with `--setModel`. Be sure you have the model installed before attempting to run it with lexido however! 

## Running remotely

This guide provides instructions on how to create and customize the JSON configuration files necessary for API integration within lexido. Each configuration allows the application to interact with a different external API by specifying endpoints, headers, data templates, and specific fields to extract from API responses.

### Default Configuration

The default configuration template is provided as a baseline. This template includes placeholders that should be customized based on the specific API you want to integrate with.

#### Example Default Configuration

```json
{
  "api_config": {
    "url": "https://api.example.com/endpoint/v1/chat/completions",
    "headers": {
      "Content-Type": "application/json",
      "Accept": "application/json"
    },
    "data_template": {
      "model": "example-model",
      "messages": "<PROMPT>"
    },
    "field_to_extract": "response"
  }
}
```

#### Fields Explanation

- **url**: The endpoint URL of the API you are calling.
- **headers**: HTTP headers to include with your request. Common headers include `Content-Type` and `Accept`.
- **data_template**: The data body of your request. `<PROMPT>` will be replaced dynamically by the application.
- **field_to_extract**: The field within the API response from which data should be extracted.

### Configuration for oLlama

Below is an example configuration specifically set up for interacting with the oLlama API, which is assumed to run locally.

#### Example oLlama Configuration

```json
{
  "api_config": {
    "url": "http://localhost:11434/api/generate",
    "headers": {
      "Content-Type": "application/json",
      "Accept": "application/json"
    },
    "data_template": {
      "model": "llama3",
      "prompt": "<PROMPT>"
    },
    "field_to_extract": "response"
  }
}
```

#### Customization Tips

- **Model**: Depending on the capabilities of the API, you might need to change the `model` value to match the model provided by the API service.
- **Prompt**: The `<PROMPT>` placeholder in `data_template` will be replaced with the actual query or command you wish to send to the API.

### Creating Your Configuration

To create your own configuration:

1. Copy the default configuration template. The location for the config is `~/.lexido/remoteConfig.json`
2. Replace the `url`, `headers`, `data_template`, and `field_to_extract` fields as needed for your specific API.
3. Ensure all placeholders like `<PROMPT>` are appropriately positioned where dynamic content is expected to be inserted by the application.

### Conclusion

This configuration system is designed to be flexible and extendable, allowing for easy integration with various APIs by simply modifying the JSON configuration files. For advanced configurations, you may need to adjust additional parameters.

## Usage
- To get command suggestions:
```bash
lexido "install teamspeak via docker"
```

- To continue with a previous prompt:
```bash
lexido -c "add more details or follow-up"
```

- To use with piping commands:
```bash
ls | lexido "what should I do with these files?"
```

## FAQ

### Why is the binary so big?
The binary's size mainly consists of the built-in networking and encryption libraries of Go. 
A quick inspection showcases this:
```bash
> goweight
   12 MB runtime
  8.1 MB net/http
  5.3 MB google.golang.org/protobuf/internal/impl
  4.1 MB net
  4.1 MB golang.org/x/net/http2
  3.9 MB golang.org/x/sys/unix
  3.7 MB crypto/tls
  3.3 MB google.golang.org/grpc
```
### How does it know what system I am running?
Before requesting the LLM the program does what is known as prompt building or contextualization, it collects different data about your system and your current scenario to help the LLM more accurately answer. Giving the LLM context about your situation allows it to better understand what you are asking or how to reply.

If you have any more questions feel free to reach out and ask

## Contributing
Contributions are what make the open-source community such an amazing place to learn, inspire, and create. Any contributions you make are **greatly appreciated**.

1. Fork the Project
2. Create your Feature Branch (`git checkout -b feature/AmazingFeature`)
3. Commit your Changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the Branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

## ☕ Buy me a coffee
If you use and enjoy lexido, you can buy me a coffee as a thank you!
https://ko-fi.com/micr0byte

<a href='https://ko-fi.com/J3J745R96' target='_blank'><img height='36' style='border:0px;height:36px;' src='https://storage.ko-fi.com/cdn/kofi3.png?v=3' border='0' alt='Buy Me a Coffee at ko-fi.com' /></a>

## License
Distributed under the GNU Affero General Public License v3.0 or any later version. See `LICENSE` for more information.

## Acknowledgements
- [Gemini 1.5 Flash](https://deepmind.google/technologies/gemini/) for the LLM powering Lexido.

Made with 💚 by Micr0byte

                        
## Stargazers over time
[![Stargazers over time](https://starchart.cc/micr0-dev/lexido.svg?variant=adaptive)](https://starchart.cc/micr0-dev/lexido)

<!----------------------------------{ Badges }--------------------------------->

[Badge Build]: https://github.com/micr0-dev/lexido/actions/workflows/goBuild.yml/badge.svg
[Badge Issues]: https://img.shields.io/github/issues/micr0-dev/lexido
[Badge Pull Requests]: https://img.shields.io/github/issues-pr/micr0-dev/lexido
[Badge Language]: https://img.shields.io/github/languages/top/micr0-dev/lexido
[Badge License]: https://img.shields.io/github/license/micr0-dev/lexido
[Badge Version]: https://img.shields.io/github/v/release/micr0-dev/lexido
