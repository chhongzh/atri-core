package atri

import (
	"context"
	"fmt"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"go.uber.org/zap"
)

func (a *Atri) sendMessageTo(ctx context.Context, bt *bot.Bot, chatID int64, msg string, isMarkdown bool) (*models.Message, error) {
	param := &bot.SendMessageParams{
		ChatID: chatID,
		Text:   msg,
	}
	if isMarkdown {
		param.ParseMode = models.ParseModeMarkdown
	}
	return bt.SendMessage(ctx, param)
}

func (a *Atri) sendChatAction(ctx context.Context, bt *bot.Bot, chatID int64, newAction models.ChatAction) error {
	_, err := bt.SendChatAction(ctx, &bot.SendChatActionParams{
		ChatID: chatID,
		Action: newAction,
	})

	return err
}

func (a *Atri) sendError(ctx context.Context, bt *bot.Bot, chatID int64, err error) {
	a.logger.Info("发送错误", zap.Error(err))

	format := `>_< Fatal Error !
%s`
	formatted := fmt.Sprintf(format, err)

	_, err = a.sendMessageTo(ctx, bt, chatID, formatted, false)
	if err != nil {
		a.logger.Error("在发送错误时遇到错误! >_<", zap.Error(err))
		return
	}
}
