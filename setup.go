package atri

import "github.com/go-telegram/bot"

func (a *Atri) setupBot() error {
	opts := []bot.Option{
		bot.WithDefaultHandler(a.handlerForTextMessage),
	}

	if a.config.CheckInitTimeout != 0 {
		opts = append(opts, bot.WithCheckInitTimeout(a.config.CheckInitTimeout))
	}

	bt, err := bot.New(a.botToken, opts...)
	if err != nil {
		return err
	}

	a.bot = bt
	a.logger.Info("初始化Bot成功")

	return nil
}

func (a *Atri) setupDB() error {
	return a.db.AutoMigrate(&memoryRecord{}, &allowedUserRecord{}, &historyRecord{})
}
