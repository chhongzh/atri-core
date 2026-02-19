package atri

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-telegram/bot"
)

// executeCommand 执行命令
func (a *Atri) executeCommand(ctx context.Context, bt *bot.Bot, command string, chatID int64, userID int64, args []string) error {
	handlers := map[string]commandHandlerFunc{
		"help":   a.handleHelp,
		"info":   a.handleInfo,
		"memory": a.handleMemory,
		"user":   a.handleUserCommand,
	}

	if handler, ok := handlers[command]; ok {
		return handler(ctx, bt, chatID, userID, args)
	}

	// 默认处理未知命令
	_, err := a.sendMessageTo(ctx, bt, chatID, ">_< 不理解你在说啥喵", false)
	return err
}

func (a *Atri) handleHelp(ctx context.Context, bt *bot.Bot, chatID int64, _ int64, _ []string) error {
	help := `下面的指令是支持的喵~
/help 显示这条命令
/info 查看对话信息
/memory ls 列出所有memory
/memory rm <ID> 删除memory
/user ls 列出所有用户
/user add <ID> [admin] 添加用户
/user rm <ID> 删除用户
/user setadmin <ID> <true|false> 设置管理员`
	_, err := a.sendMessageTo(ctx, bt, chatID, help, false)
	return err
}

func (a *Atri) handleInfo(ctx context.Context, bt *bot.Bot, chatID int64, userID int64, _ []string) error {
	msg := `信息

当前内存中的轮数:%d
配置的最大轮数:%s
当前内存中的消息数量:%d
数据库中的总消息数量:%d
已经存储的记忆数量:%d
模型:%s
`
	session := a.getSessionOrInit(ctx, userID)
	roundsInMemory := countUserMessages(session.histories)
	messagesInMemory := len(session.histories)

	totalMessagesInDB, err := a.countHistoryInDB(ctx, userID)
	if err != nil {
		return err
	}

	memories, err := a.loadMemories(ctx, userID)
	if err != nil {
		return err
	}

	maxRoundsStr := "无限制"
	if a.config.MaxRounds > 0 {
		maxRoundsStr = fmt.Sprintf("%d", a.config.MaxRounds)
	}

	_, err = a.sendMessageTo(
		ctx,
		bt,
		chatID,
		fmt.Sprintf(
			msg,
			roundsInMemory,
			maxRoundsStr,
			messagesInMemory,
			totalMessagesInDB,
			len(memories),
			a.config.Model,
		),
		false,
	)
	return err
}

func (a *Atri) handleMemory(ctx context.Context, bt *bot.Bot, chatID int64, userID int64, args []string) error {
	if len(args) == 0 {
		return a.handleMemoryList(ctx, bt, chatID, userID, args)
	}

	switch strings.ToLower(args[0]) {
	case "ls", "list":
		return a.handleMemoryList(ctx, bt, chatID, userID, args[1:])
	case "rm", "remove":
		return a.handleMemoryRemove(ctx, bt, chatID, userID, args[1:])
	default:
		_, err := a.sendMessageTo(ctx, bt, chatID, "未知子命令喵~ 请使用 ls (list) 或 rm (remove)", false)
		return err
	}
}

func (a *Atri) handleMemoryList(ctx context.Context, bt *bot.Bot, chatID int64, userID int64, _ []string) error {
	memories, err := a.loadMemories(ctx, userID)
	if err != nil {
		return err
	}

	var sb strings.Builder
	for _, m := range memories {
		fmt.Fprintf(&sb, "ID: %d - %s\n", m.ID, m.Memory)
	}

	if sb.Len() == 0 {
		sb.WriteString("没有记忆喵~")
	}

	msg := `所有的记忆

%s
如果要删除某条记忆, 请输入/memory rm <ID>`

	_, err = a.sendMessageTo(ctx, bt, chatID, fmt.Sprintf(msg, sb.String()), false)
	return err
}

func (a *Atri) handleMemoryRemove(ctx context.Context, bt *bot.Bot, chatID int64, userID int64, args []string) error {
	if len(args) < 1 {
		_, err := a.sendMessageTo(ctx, bt, chatID, "请输入要删除的记忆ID喵~", false)
		return err
	}

	idStr := args[0]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		_, err := a.sendMessageTo(ctx, bt, chatID, "ID必须是数字喵~", false)
		return err
	}

	err = a.deleteMemory(ctx, userID, uint(id))
	if err != nil {
		_, sendErr := a.sendMessageTo(ctx, bt, chatID, "无法删除记忆喵~ 请确认ID是否正确且属于你自己", false)
		if sendErr != nil {
			return sendErr
		}
		return nil
	}

	_, err = a.sendMessageTo(ctx, bt, chatID, "删除成功喵!", false)
	return err
}

func (a *Atri) handleUserCommand(ctx context.Context, bt *bot.Bot, chatID int64, userID int64, args []string) error {
	if !a.isAdmin(ctx, userID) {
		_, err := a.sendMessageTo(ctx, bt, chatID, "只有管理员可以执行该命令喵~", false)
		return err
	}

	if len(args) == 0 {
		return a.handleUserList(ctx, bt, chatID, userID, args)
	}

	switch strings.ToLower(args[0]) {
	case "ls", "list":
		return a.handleUserList(ctx, bt, chatID, userID, args[1:])
	case "add":
		return a.handleUserAdd(ctx, bt, chatID, userID, args[1:])
	case "rm", "remove":
		return a.handleUserRemove(ctx, bt, chatID, userID, args[1:])
	case "setadmin":
		return a.handleUserSetAdmin(ctx, bt, chatID, userID, args[1:])
	default:
		_, err := a.sendMessageTo(ctx, bt, chatID, "未知子命令喵~ 请使用 ls/add/rm/setadmin", false)
		return err
	}
}

func (a *Atri) handleUserList(ctx context.Context, bt *bot.Bot, chatID int64, _ int64, _ []string) error {
	users, err := a.loadUsers(ctx)
	if err != nil {
		return err
	}

	var sb strings.Builder
	for _, u := range users {
		role := "User"
		if u.IsAdmin {
			role = "Admin"
		}
		fmt.Fprintf(&sb, "ID: %d - %s\n", u.UserID, role)
	}

	if sb.Len() == 0 {
		sb.WriteString("没有任何用户喵~")
	}

	msg := `所有的用户

%s`

	_, err = a.sendMessageTo(ctx, bt, chatID, fmt.Sprintf(msg, sb.String()), false)
	return err
}

func (a *Atri) handleUserAdd(ctx context.Context, bt *bot.Bot, chatID int64, _ int64, args []string) error {
	if len(args) < 1 {
		_, err := a.sendMessageTo(ctx, bt, chatID, "请输入要添加的用户ID喵~", false)
		return err
	}

	idStr := args[0]
	targetID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		_, err := a.sendMessageTo(ctx, bt, chatID, "用户ID必须是数字喵~", false)
		return err
	}

	isAdmin := false
	if len(args) >= 2 && strings.ToLower(args[1]) == "admin" {
		isAdmin = true
	}

	err = a.createUser(ctx, targetID, isAdmin)
	if err != nil {
		return err
	}

	_, err = a.sendMessageTo(ctx, bt, chatID, "添加用户成功喵!", false)
	return err
}

func (a *Atri) handleUserRemove(ctx context.Context, bt *bot.Bot, chatID int64, _ int64, args []string) error {
	if len(args) < 1 {
		_, err := a.sendMessageTo(ctx, bt, chatID, "请输入要删除的用户ID喵~", false)
		return err
	}

	idStr := args[0]
	targetID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		_, err := a.sendMessageTo(ctx, bt, chatID, "用户ID必须是数字喵~", false)
		return err
	}

	err = a.deleteUser(ctx, targetID)
	if err != nil {
		return err
	}

	_, err = a.sendMessageTo(ctx, bt, chatID, "删除用户成功喵!", false)
	return err
}

func (a *Atri) handleUserSetAdmin(ctx context.Context, bt *bot.Bot, chatID int64, userID int64, args []string) error {
	if len(args) < 2 {
		_, err := a.sendMessageTo(ctx, bt, chatID, "用法: /user setadmin <ID> <true|false>", false)
		return err
	}

	idStr := args[0]
	targetID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		_, err := a.sendMessageTo(ctx, bt, chatID, "用户ID必须是数字喵~", false)
		return err
	}

	flagStr := strings.ToLower(args[1])
	isAdmin := flagStr == "true" || flagStr == "1" || flagStr == "yes"

	if targetID == userID && !isAdmin {
		_, err := a.sendMessageTo(ctx, bt, chatID, "不可以把自己从管理员降级为普通用户喵~", false)
		return err
	}

	err = a.updateUserAdmin(ctx, targetID, isAdmin)
	if err != nil {
		return err
	}

	_, err = a.sendMessageTo(ctx, bt, chatID, "更新管理员状态成功喵!", false)
	return err
}
