package gemini

import (
	"context"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

var model *genai.GenerativeModel
var ctx context.Context

func Setup(apiKey string) error {
	ctx = context.Background()

	// Set up the GenAI client
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return err
	}

	// Call Gemini Pro with the user's prompt
	model = client.GenerativeModel("gemini-pro")

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
