package ollama

import (
	"errors"
	"strings"

	"github.com/micr0-dev/lexido/pkg/io"
)

var llmModel string

func Init(model string) error {
	llmList, err := io.RunCmd("ollama", "list")
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return errors.New("ollama not installed on system, please install it first using the guide on github.com/micr0-dev/lexido")
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

func Generate(str_prompt string) (string, error) {
	return io.RunCmd("ollama", "run", llmModel, "\""+str_prompt+"\"")
}
