package reporting

import (
	"encoding/json"
	"fmt"
	"log"

	"goagent/internal/llm"
)

const sectionNarrativePrompt = `You are a concise financial analyst. Given the following market data for the "%s" section, write 3-5 bullet points summarizing key movements.

Rules:
- Reference exact numbers (prices, percentages)
- Note standout movers (biggest gains/losses)
- Keep each bullet to 1-2 sentences
- No disclaimers or preamble — just the bullets
- Use markdown bullet format (- )

Data:
%s`

const macroViewPrompt = `You are a concise financial analyst. Given the following daily market data across all sectors, write a 2-3 paragraph macro overview.

Rules:
- Identify broad themes (risk-on/off, sector rotation, etc.)
- Reference specific numbers to support your observations
- Note any notable divergences between sectors
- Keep it concise and actionable
- No disclaimers or preamble

Data:
%s`

// GenerateSectionNarrative asks the LLM to narrate a section's data.
func GenerateSectionNarrative(client *llm.Client, sectionName string, data []SymbolData) string {
	factsJSON, err := json.Marshal(data)
	if err != nil {
		log.Printf("narrative: marshal facts: %v", err)
		return ""
	}

	prompt := fmt.Sprintf(sectionNarrativePrompt, sectionName, string(factsJSON))
	msg := llm.Message{Role: "user", Content: prompt}

	resp, err := client.Chat("You are a financial market analyst.", []llm.Message{msg})
	if err != nil {
		log.Printf("narrative: LLM error for %s: %v", sectionName, err)
		return "*Narrative unavailable (LLM rate limit or error)*"
	}
	return resp
}

// GenerateMacroView asks the LLM for a cross-sector macro overview.
func GenerateMacroView(client *llm.Client, report *DailyReport) string {
	// Build a compact summary of all sections for the LLM.
	type sectionSummary struct {
		Name string       `json:"section"`
		Data []SymbolData `json:"data"`
	}

	var summaries []sectionSummary
	for _, sec := range report.Sections {
		summaries = append(summaries, sectionSummary{
			Name: sec.Name,
			Data: sec.Data,
		})
	}

	factsJSON, err := json.Marshal(summaries)
	if err != nil {
		log.Printf("macro: marshal error: %v", err)
		return ""
	}

	prompt := fmt.Sprintf(macroViewPrompt, string(factsJSON))
	msg := llm.Message{Role: "user", Content: prompt}

	resp, err := client.Chat("You are a financial market analyst.", []llm.Message{msg})
	if err != nil {
		log.Printf("macro: LLM error: %v", err)
		return "*Macro overview unavailable (LLM rate limit or error)*"
	}
	return resp
}
