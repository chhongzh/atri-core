package atri

import (
	"context"
	"fmt"

	"github.com/go-telegram/bot"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/shared"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

// getTools 获取所有可用的工具定义
func (a *Atri) getTools() []openai.ChatCompletionToolUnionParam {
	return []openai.ChatCompletionToolUnionParam{
		openai.ChatCompletionFunctionTool(shared.FunctionDefinitionParam{
			Name:        "create_memory",
			Description: openai.String("创建一个记忆。它接受一个参数，参数的内容即为所需要记忆的内容。"),
			Parameters: j{
				"type": "object",
				"properties": j{
					"what": j{
						"type": "string",
					},
				},
				"required": []string{"what"},
			},
		}),
	}
}

// handleToolCall 处理工具调用分发
func (a *Atri) handleToolCall(ctx context.Context, bt *bot.Bot, userID int64, toolCall openai.FinishedChatCompletionToolCall) openai.ChatCompletionMessageParamUnion {
	handlers := map[string]func(context.Context, *bot.Bot, int64, string, string) openai.ChatCompletionMessageParamUnion{
		"create_memory": a.handleCreateMemoryTool,
	}

	callID := toolCall.ID
	callData := toolCall.Arguments

	if handler, ok := handlers[toolCall.Name]; ok {
		return handler(ctx, bt, userID, callID, callData)
	}

	a.logger.Warn("调用了一个不存在的工具", zap.String("Name", toolCall.Name))
	return openai.ToolMessage("错误: 工具不存在.", callID)
}

// handleCreateMemoryTool 处理创建记忆工具
func (a *Atri) handleCreateMemoryTool(ctx context.Context, _ *bot.Bot, userID int64, callID string, callData string) openai.ChatCompletionMessageParamUnion {
	what, message, ok := a.assertAndGetToolArgument(callID, callData, "what", gjson.String)
	if !ok {
		return message
	}
	memory := what.String()

	err := a.createMemory(ctx, userID, memory)
	if err != nil {
		a.logger.Error("存储记忆失败!", zap.Error(err))
		return openai.ToolMessage(fmt.Sprintf("错误: 记忆\"%s\"存储失败. %s", memory, err), callID)
	}
	a.logger.Info("一个记忆被存储!", zap.Int64("User ID", userID), zap.String("内容", memory))

	return openai.ToolMessage(fmt.Sprintf("成功: 记忆\"%s\"被存储.", memory), callID)
}
