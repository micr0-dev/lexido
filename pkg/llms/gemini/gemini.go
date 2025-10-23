package gemini

import (
	"context"
	"errors"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

var model *genai.GenerativeModel
var ctx context.Context

func IsKeyValid(apiKey string) (bool, error) {
	if Setup(apiKey) != nil {
		return false, nil
	}

	prompt := genai.Text("Say Hello World!")
	_, err := model.GenerateContent(ctx, prompt)

	if err != nil {
		// Check if the error contains invalid API key (Error 400)
		if strings.Contains(err.Error(), "Error 400") {
			return false, nil
		}
		return false, errors.New("Error setting up GenAI client: " + err.Error())
	}

	return true, nil
}

func Setup(apiKey string) error {
	ctx = context.Background()

	// Set up the GenAI client
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return err
	}

	// Call Gemini Pro with the user's prompt
	model = client.GenerativeModel("gemini-2.0-flash")

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

	return nil
}

func Generate(str_prompt string) *genai.GenerateContentResponseIterator {
	prompt := genai.Text(str_prompt)
	return model.GenerateContentStream(ctx, prompt)
}
