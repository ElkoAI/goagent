package memory

import (
	"encoding/json"
	"log"
	"strings"

	"goagent/internal/llm"
)

const extractionPrompt = `Extract important facts, user preferences, and decisions from this conversation exchange.
Return a JSON array of short, factual strings. Each fact should be self-contained and useful for future conversations.
Focus on: user preferences, stated facts about the user, key decisions, and important context.
If there is nothing worth remembering, return an empty array [].
Do NOT include greetings, pleasantries, or trivial information.
Return ONLY the JSON array, no other text.`

// ExtractAndSave runs a background extraction of facts from a conversation turn,
// then saves each fact to the memory provider. This is fire-and-forget.
func ExtractAndSave(client *llm.Client, provider Provider, userMessage, assistantResponse string) {
	if userMessage == "" || assistantResponse == "" {
		return
	}

	conversation := "User: " + userMessage + "\nAssistant: " + assistantResponse

	msgs := []llm.Message{
		{Role: "user", Content: conversation},
	}

	result, err := client.Chat(extractionPrompt, msgs)
	if err != nil {
		log.Printf("memory extraction LLM error: %v", err)
		return
	}

	// Parse the JSON array of facts from the LLM response.
	content := strings.TrimSpace(result)
	// Strip markdown code fences if the LLM wraps the response.
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var facts []string
	if err := json.Unmarshal([]byte(content), &facts); err != nil {
		log.Printf("memory extraction parse error: %v (response: %q)", err, content)
		return
	}

	saved := 0
	for _, fact := range facts {
		fact = strings.TrimSpace(fact)
		if fact == "" {
			continue
		}
		if err := provider.Save(fact); err != nil {
			log.Printf("memory extraction save error: %v", err)
			continue
		}
		saved++
	}

	if saved > 0 {
		log.Printf("memory: extracted %d facts from conversation", saved)
	}
}
