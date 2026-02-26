package app

import "errors"

var (
	// ErrBookNotReady indicates a book is not yet processed for chat.
	ErrBookNotReady          = errors.New("book not ready")
	ErrConversationNotFound  = errors.New("conversation not found")
	ErrConversationForbidden = errors.New("conversation forbidden")
)
