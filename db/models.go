// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.28.0

package db

import (
	"time"
)

type Note struct {
	ID         string
	Title      string
	Note       string
	Archive    bool
	Favorite   bool
	CreatedAt  time.Time
	ModifiedAt time.Time
	Tags       []string
}

type TagSummary struct {
	TagName   interface{}
	NoteCount int64
}
