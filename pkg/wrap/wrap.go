package wrap

import "strings"

func WrapText(text string, lineWidth int) string {
	// Split the text into paragraphs based on newline characters
	paragraphs := strings.Split(text, "\n")

	var wrappedText strings.Builder
	for i, paragraph := range paragraphs {
		// Wrap each paragraph individually
		wrappedParagraph := WrapParagraph(paragraph, lineWidth)
		wrappedText.WriteString(wrappedParagraph)

		// Don't add a newline character after the last paragraph
		if i < len(paragraphs)-1 {
			wrappedText.WriteString("\n")
		}
	}

	return wrappedText.String()
}

func WrapParagraph(paragraph string, lineWidth int) string {
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
