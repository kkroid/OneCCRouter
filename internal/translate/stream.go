package translate

import (
	"fmt"
)

// StreamContext tracks state across SSE stream chunks.
type StreamContext struct {
	MessageStartSent bool
	MessageID        string
	Model            string
	ContentBlockIdx  int
	ContentBlockOpen bool
	ActiveToolIdx    int // which tool's content block is open (-1 = none/text)
	ToolCalls        map[int]*ToolCallState
}

type ToolCallState struct {
	ID        string
	Name      string
	Args      string
	BlockSent bool
}

func TranslateStreamChunk(chunk *OpenAIStreamChunk, ctx *StreamContext) ([]SSEEvent, error) {
	var events []SSEEvent

	if len(chunk.Choices) == 0 {
		return events, nil
	}

	delta := chunk.Choices[0].Delta

	if !ctx.MessageStartSent {
		ctx.MessageStartSent = true
		ctx.ActiveToolIdx = -1 // text/no-block sentinel
		events = append(events, SSEEvent{
			Type: "message_start",
			Message: &AnthropicResponse{
				ID:      ctx.MessageID,
				Type:    "message",
				Role:    "assistant",
				Content: []AnthropicContentBlock{},
				Model:   ctx.Model,
				Usage: AnthropicUsage{
					InputTokens:  0,
					OutputTokens: 0,
				},
			},
		})
	}

	// Tool calls
	if len(delta.ToolCalls) > 0 {
		nonToolTransition := ctx.ContentBlockOpen && ctx.ActiveToolIdx < 0

		for _, tc := range delta.ToolCalls {
			idx := tc.Index

			if ctx.ToolCalls[idx] == nil {
				ctx.ToolCalls[idx] = &ToolCallState{
					ID:   tc.ID,
					Name: tc.funcName(),
				}
			}
			if tc.ID != "" {
				ctx.ToolCalls[idx].ID = tc.ID
			}
			if tc.funcName() != "" {
				ctx.ToolCalls[idx].Name = tc.funcName()
			}

			// Close non-tool (text) block when transitioning to tools
			if nonToolTransition {
				nonToolTransition = false
				ci := ctx.ContentBlockIdx
				events = append(events, SSEEvent{
					Type:  "content_block_stop",
					Index: &ci,
				})
				ctx.ContentBlockOpen = false
				ctx.ContentBlockIdx++
			}

			// Close previous tool block ONLY when switching to a DIFFERENT tool
			if ctx.ContentBlockOpen && ctx.ActiveToolIdx >= 0 && ctx.ActiveToolIdx != idx {
				ci := ctx.ContentBlockIdx
				events = append(events, SSEEvent{
					Type:  "content_block_stop",
					Index: &ci,
				})
				ctx.ContentBlockOpen = false
				ctx.ContentBlockIdx++
			}

			t := ctx.ToolCalls[idx]

			if !t.BlockSent && t.Name != "" {
				t.BlockSent = true
				ctx.ActiveToolIdx = idx
				ci := ctx.ContentBlockIdx
				events = append(events, SSEEvent{
					Type:  "content_block_start",
					Index: &ci,
					ContentBlock: map[string]interface{}{
						"type":  "tool_use",
						"id":    t.ID,
						"name":  t.Name,
						"input": map[string]interface{}{},
					},
				})
				ctx.ContentBlockOpen = true
			}

			if tc.funcArgs() != "" && t.BlockSent {
				t.Args += tc.funcArgs()
				ci := ctx.ContentBlockIdx
				events = append(events, SSEEvent{
					Type:  "content_block_delta",
					Index: &ci,
					Delta: &SSEDelta{
						Type:        "input_json_delta",
						PartialJSON: tc.funcArgs(),
					},
				})
			}
		}
		return events, nil
	}

	// Text content
	if delta.Content != "" {
		if !ctx.ContentBlockOpen {
			ctx.ActiveToolIdx = -1
			ci := ctx.ContentBlockIdx
			events = append(events, SSEEvent{
				Type:  "content_block_start",
				Index: &ci,
				ContentBlock: map[string]interface{}{
					"type": "text",
					"text": "",
				},
			})
			ctx.ContentBlockOpen = true
		}
		ci := ctx.ContentBlockIdx
		events = append(events, SSEEvent{
			Type:  "content_block_delta",
			Index: &ci,
			Delta: &SSEDelta{
				Type: "text_delta",
				Text: delta.Content,
			},
		})
	}

	finishReason := chunk.Choices[0].FinishReason
	if finishReason != nil && *finishReason != "" {
		if ctx.ContentBlockOpen {
			ci := ctx.ContentBlockIdx
			events = append(events, SSEEvent{
				Type:  "content_block_stop",
				Index: &ci,
			})
			ctx.ContentBlockOpen = false
		}
		st := mapAnthropicStopReason(*finishReason)
		events = append(events, SSEEvent{
			Type: "message_delta",
			Delta: &SSEDelta{
				StopReason: st,
			},
			Usage: &SSEUsage{OutputTokens: 0},
		})
		events = append(events, SSEEvent{
			Type: "message_stop",
		})
	}

	return events, nil
}

func mapAnthropicStopReason(finish string) string {
	switch finish {
	case "stop":
		return "end_turn"
	case "length":
		return "max_tokens"
	case "tool_calls":
		return "tool_use"
	default:
		return "end_turn"
	}
}

func (tc *OpenAIToolCallDelta) funcName() string {
	if tc.Function != nil {
		return tc.Function.Name
	}
	return ""
}

func (tc *OpenAIToolCallDelta) funcArgs() string {
	if tc.Function != nil {
		return tc.Function.Arguments
	}
	return ""
}

func GenerateMessageID() string {
	return fmt.Sprintf("msg_%s", randHex(16))
}
