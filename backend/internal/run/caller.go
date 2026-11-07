package run

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"testforge/internal/llm"
)

// Decision is the simulated user's next move.
type Decision struct {
	Say    *string `json:"say"`
	HangUp bool    `json:"hangUp"`
	Reason string  `json:"reason"`
}

var decisionSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"say":    map[string]any{"type": []any{"string", "null"}, "description": "What you say next; null to say nothing."},
		"hangUp": map[string]any{"type": "boolean", "description": "true to end the conversation now."},
		"reason": map[string]any{"type": "string", "description": "Why, in a few words (never said aloud)."},
	},
	"required":             []any{"say", "hangUp", "reason"},
	"additionalProperties": false,
}

var temperaments = map[string]string{
	"polite":    "polite and courteous",
	"hurried":   "hurried, short on time and a bit impatient",
	"skeptical": "skeptical; you question what you are told",
	"confused":  "easily confused; you sometimes misunderstand and ask again",
	"angry":     "upset and irritated, though not abusive",
	"friendly":  "warm and friendly",
	"calm":      "calm and patient",
	"detailed":  "detail-oriented; you ask precise follow-up questions",
}

var hangUpRules = map[string][2]string{ // [voice, text]
	"goal_or_goodbye": {
		"Hang up (hangUp true) as soon as your goal is achieved or the agent says goodbye. You may say a short goodbye in the same turn.",
		"End the chat (hangUp true) as soon as your goal is achieved or the assistant says goodbye. You may write a short goodbye in the same message.",
	},
	"goodbye": {
		"Hang up (hangUp true) only when the agent says goodbye or clearly ends the call, even if your goal is already achieved.",
		"End the chat (hangUp true) only when the assistant says goodbye or clearly closes the conversation, even if your goal is already achieved.",
	},
	"never": {
		"Never hang up on your own (hangUp always false): keep the conversation going until the call is ended for you.",
		"Never end the chat on your own (hangUp always false): keep the conversation going until it is ended for you.",
	},
}

// Caller is the simulated user: an LLM following the scenario.
type Caller struct {
	client   llm.Client
	model    string
	system   string
	voice    bool
	usageIn  int
	usageOut int
}

// NewCaller builds the simulated user for a run.
func NewCaller(client llm.Client, model string, snap Snapshot) *Caller {
	voice := snap.Agent.Modality == "voice"
	return &Caller{client: client, model: model, system: systemPrompt(snap, voice), voice: voice}
}

// Usage returns the tokens spent so far.
func (c *Caller) Usage() (in, out int) { return c.usageIn, c.usageOut }

func section(title, body string) string { return "## " + title + "\n" + strings.TrimSpace(body) }

func systemPrompt(snap Snapshot, voice bool) string {
	sc, p := snap.Scenario, snap.Persona
	other, medium := "agent", "phone call with a voice agent"
	opening := "You are the caller in a phone call with a voice agent. You play a real person, described below, and you talk with the agent to reach your goal."
	if !voice {
		other, medium = "assistant", "chat with a text assistant"
		opening = "You are a user chatting with a text assistant. You play a real person, described below, and you write to the assistant to reach your goal."
	}
	parts := []string{opening}
	if strings.TrimSpace(snap.Agent.Description) != "" {
		parts = append(parts, section("Context about the "+other+" you are talking to", snap.Agent.Description))
	}
	if strings.TrimSpace(sc.Context) != "" {
		parts = append(parts, section("Context of this conversation", sc.Context))
	}
	who := []string{"Name: " + p.Name, "Locale: " + p.Locale, fmt.Sprintf("Temperament: %s (%s)", p.Temperament, temperaments[p.Temperament])}
	if strings.TrimSpace(p.Notes) != "" {
		who = append(who, "Notes: "+strings.TrimSpace(p.Notes))
	}
	parts = append(parts, section("Who you are", strings.Join(who, "\n")), section("Your goal", sc.Goal))
	if strings.TrimSpace(sc.Instructions) != "" {
		parts = append(parts, section("How to behave", sc.Instructions))
	}
	if len(sc.Facts) > 0 {
		var facts []string
		for _, f := range sc.Facts {
			facts = append(facts, "- "+f.Key+": "+f.Value)
		}
		parts = append(parts, section("What you know (reveal only when asked)", strings.Join(facts, "\n")))
	}
	style := "Use short, spoken sentences: one or two per turn, no lists, no markup, no emojis."
	lang := "Speak only in the language of the locale " + p.Locale + ", the way a native speaker would on the phone."
	if !voice {
		style = "Write short, natural chat messages: usually one or two sentences, no lists, no markup, no headings."
		lang = "Write only in the language of the locale " + p.Locale + ", the way a native speaker would in a chat."
	}
	rule := hangUpRules[sc.EndConditions.HangUpWhen]
	if rule[0] == "" {
		rule = hangUpRules["goal_or_goodbye"]
	}
	end := rule[0]
	if !voice {
		end = rule[1]
	}
	rules := []string{
		"You are the " + map[bool]string{true: "caller", false: "user"}[voice] + ", never the " + other + ": stay in character for the whole " + medium + ".",
		lang, style,
		"Never reveal a fact unless the " + other + " asks for it or the conversation needs it; give one fact at a time, never all at once.",
		"Never say or hint that you are an AI, a test, a simulation or an evaluator.",
		"Do not repeat what the " + other + " just said back to it unless confirming a detail.",
		end,
		"Answer only with the JSON object of the response schema: `say` (your words, or null only when you end without a word), `hangUp`, and `reason` (a few words, never said aloud).",
	}
	for i, r := range rules {
		rules[i] = "- " + r
	}
	parts = append(parts, section("Rules", strings.Join(rules, "\n")))
	return strings.Join(parts, "\n\n")
}

// Decide asks the model for the next move given the transcript so far.
func (c *Caller) Decide(ctx context.Context, transcript []Turn, status string) (Decision, error) {
	you, them := "YOU", "AGENT"
	if !c.voice {
		them = "ASSISTANT"
	}
	var b strings.Builder
	if len(transcript) == 0 {
		if c.voice {
			b.WriteString("The call has just connected and nobody has spoken yet: you speak first.")
		} else {
			b.WriteString("The chat has just opened and nobody has written yet: you write first.")
		}
	} else {
		b.WriteString("Conversation so far:\n")
		for _, t := range transcript {
			who := you
			if t.Role == "agent" {
				who = them
			}
			line := who + ": " + t.Text
			if t.Interrupted {
				line += " … [you interrupted here]"
			}
			b.WriteString(line + "\n")
		}
	}
	b.WriteString("\n" + status + "\nDecide your next move.")

	res, err := c.client.Complete(ctx, llm.Request{
		Model: c.model, System: c.system, Prompt: b.String(), SchemaName: "user_decision", Schema: decisionSchema,
	})
	if err != nil {
		return Decision{}, err
	}
	c.usageIn += res.InputTokens
	c.usageOut += res.OutputTokens
	var d Decision
	if err := json.Unmarshal([]byte(strings.TrimSpace(res.JSON)), &d); err != nil {
		return Decision{}, fmt.Errorf("the simulator's answer is not valid JSON: %w", err)
	}
	if d.Say != nil {
		text := strings.TrimSpace(*d.Say)
		if len([]rune(text)) > 2000 {
			text = string([]rune(text)[:2000])
		}
		if text == "" {
			d.Say = nil
		} else {
			d.Say = &text
		}
	}
	return d, nil
}
