package atri

import (
	"context"
	"fmt"
	"strings"

	"github.com/chhongzh/shlex"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"go.uber.org/zap"
)

func (a *Atri) handlerForTextMessage(ctx context.Context, bt *bot.Bot, update *models.Update) {
	chatID := update.Message.Chat.ID
	chatText := strings.TrimSpace(update.Message.Text)
	username := update.Message.Chat.Username
	userID := update.Message.From.ID

	if strings.ToLower(chatText) == "/start" {
		if !a.isUserInBuck(ctx, userID) {
			a.logger.Info("一名新的用户!",
				zap.Int64("Chat ID", chatID),
				zap.String("Username", username),
				zap.Int64("UserID", userID),
			)

			a.sendMessageTo(ctx, bt, chatID, fmt.Sprintf("未在白名单内, 请联系管理员, UserID=%d.", chatID), false)
			return
		}

		a.sendMessageTo(ctx, bt, chatID, fmt.Sprintf("%s, 欢迎回来!", username), false)
		return
	}

	if !a.isUserInBuck(ctx, userID) {
		// Silent 处理
		return
	}

	a.logger.Info("收到消息",
		zap.Int64("Chat ID", chatID),
		zap.String("Username", username),
		zap.Int64("UserID", userID),
		zap.String("Chat Text", chatText),
	)

	if strings.HasPrefix(chatText, "/") {
		commandLine := strings.TrimSpace(chatText[1:])
		err := a.handleCommand(ctx, bt, chatID, commandLine, userID)
		if err != nil {
			a.sendError(ctx, bt, chatID, err)
		}

		return
	}

	err := a.handleAiChat(ctx, bt, userID, username, chatID, chatText)
	if err != nil {
		a.sendError(ctx, bt, chatID, err)
		return
	}
}

func (a *Atri) handleCommand(ctx context.Context, bt *bot.Bot, chatID int64, commandLine string, userID int64) error {
	parts, err := shlex.Split(commandLine)
	if err != nil {
		return err
	}
	if len(parts) == 0 {
		return nil
	}

	args := parts[1:]

	return a.executeCommand(ctx, bt, parts[0], chatID, userID, args)
}
