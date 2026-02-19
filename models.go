package atri

import "gorm.io/gorm"

type memoryRecord struct {
	gorm.Model

	UserID int64
	Memory string
}

func (m memoryRecord) String() string {
	return m.Memory
}

type allowedUserRecord struct {
	gorm.Model

	UserID  int64
	IsAdmin bool
}

type historyRecord struct {
	gorm.Model

	UserID int64
	InJSON string
}
