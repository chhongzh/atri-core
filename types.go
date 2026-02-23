package atri

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/openai/openai-go/v3"
)

type roundHistory = []openai.ChatCompletionMessageParamUnion
type j = map[string]any
type userSession struct {
	currentRole string
	histories   []roundHistory
}
type commandHandlerFunc = func(context.Context, *bot.Bot, int64, int64, []string) error
