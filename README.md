## Atri Core

Atri Core 是一个用于实现聊天机器人的 Go 库.

### 最小示例

```go
package main

import (
	"context"

	"github.com/chhongzh/atri-core"
	"github.com/glebarez/sqlite"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func main() {
	botToken := "YOUR_TELEGRAM_BOT_TOKEN"
	apiBaseURL := "https://your-openai-compatible-endpoint"
	apiKey := "YOUR_OPENAI_API_KEY"
	apiModel := "gpt-4.1"

	ctx := context.Background()
	logger := zap.Must(zap.NewDevelopment())

	openaiClient := openai.NewClient(
		option.WithBaseURL(apiBaseURL),
		option.WithAPIKey(apiKey),
	)

	db, err := gorm.Open(sqlite.Open("atri-core.db"))
	if err != nil {
		logger.Fatal("初始化DB失败", zap.Error(err))
	}

	cfg := atri.Config{
		Model:        apiModel,
		MaxRounds:    16,                 // 0 表示不限制
		SystemPrompt: "你是一个聊天机器人",  // 支持的占位符 {{USERNAME}} / {{MEMORIES}}
	}

	core := atri.New(ctx, logger, &openaiClient, db, botToken, cfg)
	ch, err := core.Start()
	if err != nil {
		logger.Fatal("启动Atri失败", zap.Error(err))
	}

	<-ch
}
```

启动后，在聊天中输入 `/help` 可以查看所有可用命令和功能说明。
