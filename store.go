package atri

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/openai/openai-go/v3"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func (a *Atri) isUserInBuck(ctx context.Context, userID int64) bool {
	hasAny, err := a.hasAnyUser(ctx)
	if err != nil {
		return false
	}
	if !hasAny {
		err := a.createUser(ctx, userID, true)
		if err != nil {
			a.logger.Error("创建首个管理员失败", zap.Error(err))
			return false
		}
		a.logger.Info("首个管理员已创建", zap.Int64("UserID", userID))
		return true
	}

	_, err = gorm.G[allowedUserRecord](a.db).Where("user_id = ?", userID).Last(ctx)
	if err != nil {
		return false
	}

	return true
}

func (a *Atri) hasAnyUser(ctx context.Context) (bool, error) {
	var count int64
	records, err := gorm.G[allowedUserRecord](a.db).Limit(1).Find(ctx)
	if err != nil {
		return false, err
	}
	count = int64(len(records))
	return count > 0, nil
}

func (a *Atri) createUser(ctx context.Context, userID int64, isAdmin bool) error {
	err := gorm.G[allowedUserRecord](a.db).Create(ctx, &allowedUserRecord{
		UserID:  userID,
		IsAdmin: isAdmin,
	})
	if err != nil {
		return err
	}
	return nil
}

func (a *Atri) deleteUser(ctx context.Context, userID int64) error {
	_, err := gorm.G[allowedUserRecord](a.db).Where("user_id = ?", userID).Delete(ctx)
	if err != nil {
		return err
	}
	return nil
}

func (a *Atri) loadUsers(ctx context.Context) ([]allowedUserRecord, error) {
	records, err := gorm.G[allowedUserRecord](a.db).Find(ctx)
	if err != nil {
		return nil, err
	}
	return records, nil
}

func (a *Atri) updateUserAdmin(ctx context.Context, userID int64, isAdmin bool) error {
	_, err := gorm.G[allowedUserRecord](a.db).Where("user_id = ?", userID).Update(ctx, "is_admin", isAdmin)
	if err != nil {
		return err
	}
	return nil
}

func (a *Atri) isAdmin(ctx context.Context, userID int64) bool {
	record, err := gorm.G[allowedUserRecord](a.db).Where("user_id = ? AND is_admin = ?", userID, true).Last(ctx)
	if err != nil {
		return false
	}
	return record.IsAdmin
}

func (a *Atri) countHistoryInDB(ctx context.Context, userID int64) (int64, error) {
	var count int64
	err := a.db.WithContext(ctx).Model(&historyRecord{}).Where("user_id = ?", userID).Count(&count).Error
	if err != nil {
		return 0, err
	}
	return count, nil
}

// loadMemories 从数据库加载记忆（不会上锁）
func (a *Atri) loadMemories(ctx context.Context, userID int64) ([]memoryRecord, error) {
	records, err := gorm.G[memoryRecord](a.db).Where("user_id = ?", userID).Find(ctx)
	if err != nil {
		return nil, err
	}

	return records, nil
}

// createMemory 创建一条新记忆（不会上锁）
func (a *Atri) createMemory(ctx context.Context, userID int64, memory string) error {
	err := gorm.G[memoryRecord](a.db).Create(ctx, &memoryRecord{UserID: userID, Memory: memory})
	if err != nil {
		return err
	}

	return nil
}

func (a *Atri) deleteMemory(ctx context.Context, userID int64, memoryID uint) error {
	mem, err := gorm.G[memoryRecord](a.db).Where("id = ?", memoryID).Last(ctx)
	if err != nil {
		return err
	}
	if mem.UserID != userID {
		return gorm.ErrRecordNotFound
	}

	_, err = gorm.G[memoryRecord](a.db).Where("id = ? AND user_id = ?", memoryID, userID).Delete(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (a *Atri) fillSessionHistoryFromDB(ctx context.Context, session *userSession, userID int64) error {
	maxRounds := a.config.MaxRounds
	pageSize := 50
	historiesDesc := userChatHistroy{}
	rounds := 0
	offset := 0
	queries := 0

	for {
		historyRecords, err := gorm.G[historyRecord](a.db).
			Where("user_id = ?", userID).
			Order("id DESC").
			Offset(offset).
			Limit(pageSize).
			Find(ctx)
		if err != nil {
			return err
		}
		queries++
		if len(historyRecords) == 0 {
			break
		}

		offset += len(historyRecords)

		for _, record := range historyRecords {
			temp := openai.ChatCompletionMessageParamUnion{}
			err := json.Unmarshal([]byte(record.InJSON), &temp)
			if err != nil {
				return err
			}

			historiesDesc = append(historiesDesc, temp)
			if isUserMessage(temp) && maxRounds > 0 {
				rounds++
				if rounds >= maxRounds {
					break
				}
			}
		}

		if maxRounds > 0 && rounds >= maxRounds {
			break
		}
	}

	if len(historiesDesc) == 0 {
		session.histories = userChatHistroy{}
		return nil
	}

	slices.Reverse(historiesDesc)
	session.histories = historiesDesc

	a.logger.Info(
		"加载会话历史完成",
		zap.Int64("UserID", userID),
		zap.Int("Queries", queries),
		zap.Int("LoadedMessages", len(historiesDesc)),
		zap.Int("MaxRounds", a.config.MaxRounds),
		zap.Int("RoundsInMemory", countUserMessages(historiesDesc)),
	)

	return nil
}

// writeHistoryToDB 将新的历史记录写入数据库
func (a *Atri) writeHistoryToDB(ctx context.Context, diffed userChatHistroy, userID int64) error {
	err := a.db.Transaction(func(tx *gorm.DB) error {
		for _, msg := range diffed {
			inJSON, err := json.Marshal(msg)
			if err != nil {
				return err
			}

			err = gorm.G[historyRecord](tx).Create(ctx, &historyRecord{UserID: userID, InJSON: string(inJSON)})
			if err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return err
	}
	a.logger.Info(
		"写入会话历史到数据库",
		zap.Int64("UserID", userID),
		zap.Int("Messages", len(diffed)),
	)
	return nil
}

func countUserMessages(histories userChatHistroy) int {
	count := 0
	for _, h := range histories {
		if isUserMessage(h) {
			count++
		}
	}
	return count
}
