// Package atri 是Atri的实现
package atri

import (
	"context"
	"sync"
	"time"

	"github.com/go-telegram/bot"
	"github.com/openai/openai-go/v3"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Config 用于配置Atri实例的模型、最大保留轮数和系统提示词
type Config struct {
	Model            string
	MaxRounds        int
	SystemPrompt     string
	CheckInitTimeout time.Duration
}

// Atri 是Atri的实例
type Atri struct {
	ctx             context.Context
	logger          *zap.Logger
	db              *gorm.DB
	openaiClient    *openai.Client
	bot             *bot.Bot
	botToken        string
	config          Config
	userSession     map[int64]*userSession
	userSessionLock sync.Mutex
}

// New 创建一个新的Atri实例
func New(ctx context.Context, logger *zap.Logger, openaiClient *openai.Client, db *gorm.DB, botToken string, cfg Config) *Atri {
	return &Atri{
		ctx:          ctx,
		logger:       logger.Named("Atri"),
		db:           db,
		openaiClient: openaiClient,
		botToken:     botToken,
		config:       cfg,
		userSession:  make(map[int64]*userSession),
	}
}

// Start 启动Telegram Bot并返回一个在停止时关闭的通道
func (a *Atri) Start() (<-chan struct{}, error) {
	if err := a.setupBot(); err != nil {
		return nil, err
	}

	if err := a.setupDB(); err != nil {
		return nil, err
	}

	closeCh := make(chan struct{})
	go func() {
		a.bot.Start(a.ctx)
		close(closeCh)
	}()

	return closeCh, nil
}
