package atri

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/openai/openai-go/v3"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

// buildSystemPromptMessage 构建系统提示词消息
func (a *Atri) buildSystemPromptMessage(ctx context.Context, prompt string, userID int64, username string) (openai.ChatCompletionMessageParamUnion, error) {
	memoryRecords, err := a.loadMemories(ctx, userID)
	if err != nil {
		return openai.ChatCompletionMessageParamUnion{}, err
	}
	memories := []string{}
	for _, record := range memoryRecords {
		memories = append(memories, record.String())
	}

	prompt = strings.ReplaceAll(prompt, "{{USERNAME}}", username)
	prompt = strings.ReplaceAll(prompt, "{{MEMORIES}}", strings.Join(memories, "\n"))

	return openai.SystemMessage(prompt), nil
}

// buildUserMessage 构建用户消息
func (a *Atri) buildUserMessage(chatText string) openai.ChatCompletionMessageParamUnion {
	chatText = strings.TrimSpace(chatText)
	return openai.UserMessage(chatText)
}

// buildTimeSystemMessage 构建当前时间的系统消息
func (a *Atri) buildTimeSystemMessage() openai.ChatCompletionMessageParamUnion {
	now := time.Now()
	content := fmt.Sprintf("当前的时间是:%s", now.Format(time.DateTime))
	return openai.SystemMessage(content)
}

// assertAndGetToolArgument 断言并获取工具参数
func (a *Atri) assertAndGetToolArgument(callID string, callData string, paramPath string, expectedTypeOfParam gjson.Type) (gjson.Result, openai.ChatCompletionMessageParamUnion, bool) {
	param := gjson.Get(callData, paramPath)
	if !param.Exists() {
		return gjson.Result{}, openai.ToolMessage(fmt.Sprintf("错误: 这个工具需要参数\"%s\", 但是你并未传入.", paramPath), callID), false
	}

	if param.Type != expectedTypeOfParam {
		return gjson.Result{}, openai.ToolMessage(fmt.Sprintf("错误: 这个工具所需要的参数\"%s\"是%s类型, 但你传入的类型却是%s.", paramPath, expectedTypeOfParam, param.Type), callID), false
	}

	return param, openai.ChatCompletionMessageParamUnion{}, true
}

// getSessionOrInit 获取或初始化用户会话
func (a *Atri) getSessionOrInit(ctx context.Context, userID int64) *userSession {
	a.userSessionLock.Lock()
	session, ok := a.userSession[userID]
	if !ok {
		session = &userSession{}
		a.userSession[userID] = session

		// 加载History
		err := a.fillSessionHistoryFromDB(ctx, session, userID)
		if err != nil {
			a.logger.Error("填充History错误!", zap.Error(err))
		}
	}
	a.userSessionLock.Unlock()

	return session
}
