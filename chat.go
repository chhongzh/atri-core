package atri

import (
	"context"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/openai/openai-go/v3"
	"go.uber.org/zap"
)

// handleAiChat 处理 AI 聊天逻辑
func (a *Atri) handleAiChat(ctx context.Context, bt *bot.Bot, userID int64, username string, chatID int64, chatText string) error {
	session := a.getSessionOrInit(ctx, userID)

	a.userSessionLock.Lock()
	defer a.userSessionLock.Unlock()

	systemPromptMessage, err := a.buildSystemPromptMessage(ctx, a.config.SystemPrompt, userID, username)
	if err != nil {
		return err
	}

	thisRoundStartAt := len(session.histories)
	session.histories = append(
		session.histories,
		a.buildTimeSystemMessage(),
		a.buildUserMessage(chatText),
	)

	stopTyping := a.startTypingLoop(ctx, bt, chatID)
	defer stopTyping()

	// 循环处理，直到没有工具调用
	for {
		fullContent, finishedToolCalls, err := a.processStreamResponse(ctx, bt, chatID, session.histories, systemPromptMessage)
		if err != nil {
			return err
		}

		assistantMsg := openai.AssistantMessage(fullContent)

		// 如果有工具调用
		if len(finishedToolCalls) > 0 {
			if assistantMsg.OfAssistant != nil {
				toolCallsParam := make([]openai.ChatCompletionMessageToolCallUnionParam, len(finishedToolCalls))
				for i, ftc := range finishedToolCalls {
					toolCallsParam[i] = openai.ChatCompletionMessageToolCallUnionParam{
						OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
							ID: ftc.ID,
							Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
								Name:      ftc.Name,
								Arguments: ftc.Arguments,
							},
						},
					}
				}
				assistantMsg.OfAssistant.ToolCalls = toolCallsParam
			}
			session.histories = append(session.histories, assistantMsg)

			for _, toolCall := range finishedToolCalls {
				res := a.handleToolCall(ctx, bt, userID, toolCall)
				session.histories = append(session.histories, res)
			}
			// 继续循环，将 Tool Call 的结果发给 AI
			continue
		}

		// 没有工具调用，结束对话
		session.histories = append(session.histories, assistantMsg)
		break
	}

	// 保存历史
	diffed := session.histories[thisRoundStartAt:]
	err = a.writeHistoryToDB(ctx, diffed, userID)
	if err != nil {
		return err
	}

	session.histories = a.trimHistoryToMaxRounds(session.histories)

	a.logger.Info(
		"会话完成",
		zap.Int64("UserID", userID),
		zap.Int("NewMessages", len(diffed)),
		zap.Int("TotalMessages", len(session.histories)),
		zap.Int("RoundsInMemory", countUserMessages(session.histories)),
	)

	return nil
}

func isUserMessage(msg openai.ChatCompletionMessageParamUnion) bool {
	return msg.OfUser != nil
}

func (a *Atri) trimHistoryToMaxRounds(histories userChatHistroy) userChatHistroy {
	max := a.config.MaxRounds
	if max <= 0 {
		return histories
	}
	rounds := 0
	start := 0
	for i := len(histories) - 1; i >= 0; i-- {
		if isUserMessage(histories[i]) {
			rounds++
			if rounds == max {
				start = i
				break
			}
		}
	}
	if rounds < max {
		return histories
	}
	return histories[start:]
}

// startTypingLoop 开启一个 goroutine 持续发送 Typing 状态，返回一个停止函数
func (a *Atri) startTypingLoop(ctx context.Context, bt *bot.Bot, chatID int64) func() {
	done := make(chan struct{})
	ticker := time.NewTicker(time.Second * 6)

	go func() {
		fn := func() {
			err := a.sendChatAction(ctx, bt, chatID, models.ChatActionTyping)
			if err != nil {
				a.logger.Error("Action Routine Error", zap.Error(err))
			}
		}
		fn()
		for {
			select {
			case <-done:
				ticker.Stop()
				return
			case <-ticker.C:
				fn()
			}
		}
	}()

	return func() {
		close(done)
	}
}

// processStreamResponse 处理流式响应，返回完整内容和工具调用
func (a *Atri) processStreamResponse(
	ctx context.Context,
	bt *bot.Bot,
	chatID int64,
	histories []openai.ChatCompletionMessageParamUnion,
	systemPrompt openai.ChatCompletionMessageParamUnion,
) (string, []openai.FinishedChatCompletionToolCall, error) {

	cached := []rune{}
	sendAndResetCached := func(theCached []rune) error {
		if len(theCached) == 0 {
			return nil
		}
		msg := string(theCached)
		_, err := a.sendMessageTo(ctx, bt, chatID, msg, false)
		return err
	}

	waitingForFirstToken := true
	a.logger.Debug("调用API")
	acc := &openai.ChatCompletionAccumulator{}
	var fullContent strings.Builder
	var finishedToolCalls []openai.FinishedChatCompletionToolCall

	stream := a.openaiClient.Chat.Completions.NewStreaming(ctx, openai.ChatCompletionNewParams{
		Messages: append(userChatHistroy{systemPrompt}, histories...),
		Model:    a.config.Model,
		Tools:    a.getTools(),
	})

	for stream.Next() {
		chunk := stream.Current()
		acc.AddChunk(chunk)

		if len(chunk.Choices) == 0 {
			continue
		}

		deltaContent := chunk.Choices[0].Delta.Content
		fullContent.WriteString(deltaContent)

		if toolCall, ok := acc.JustFinishedToolCall(); ok {
			finishedToolCalls = append(finishedToolCalls, toolCall)
		}

		if deltaContent == "" {
			continue
		}

		if waitingForFirstToken {
			waitingForFirstToken = false
			a.logger.Debug("Received the first token", zap.String("Delta", deltaContent))
		}

		for _, char := range deltaContent {
			cached = append(cached, char)
			lCached := len(cached)

			// 简单的缓冲策略：遇到两个换行符就发送
			if lCached >= 2 && cached[lCached-1] == '\n' && cached[lCached-2] == '\n' {
				err := sendAndResetCached(cached)
				if err != nil {
					return "", nil, err
				}
				cached = []rune{}
			}
		}
	}

	if err := stream.Err(); err != nil {
		return "", nil, err
	}

	// 发送剩余的内容
	if len(cached) > 0 {
		if err := sendAndResetCached(cached); err != nil {
			return "", nil, err
		}
	}

	return fullContent.String(), finishedToolCalls, nil
}
