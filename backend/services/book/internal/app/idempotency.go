package app

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"onebookai/internal/util"
	"onebookai/pkg/domain"
)

const (
	idempotencyScopeUpload    = "book.upload"
	idempotencyScopeReprocess = "book.reprocess"
)

func uploadRequestHash(ownerID, filename string, size int64) string {
	return util.HashStrings(ownerID, filepath.Base(strings.TrimSpace(filename)), strconv.FormatInt(size, 10))
}

func (a *App) beginBookIdempotency(scope, actorID, key, requestHash string) (domain.IdempotencyRecord, domain.Book, bool, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return domain.IdempotencyRecord{}, domain.Book{}, false, fmt.Errorf("idempotency key required")
	}
	record, ok, err := a.store.GetIdempotencyRecord(scope, actorID, key)
	if err != nil {
		return domain.IdempotencyRecord{}, domain.Book{}, false, err
	}
	if ok {
		if record.RequestHash != requestHash {
			return domain.IdempotencyRecord{}, domain.Book{}, false, fmt.Errorf("idempotency key reused with different request")
		}
		switch record.State {
		case domain.IdempotencyStateCompleted:
			book, found, getErr := a.store.GetBook(record.ResourceID)
			if getErr != nil {
				return domain.IdempotencyRecord{}, domain.Book{}, false, getErr
			}
			if !found {
				return domain.IdempotencyRecord{}, domain.Book{}, false, fmt.Errorf("idempotent resource not found")
			}
			return record, book, true, nil
		case domain.IdempotencyStateProcessing:
			return domain.IdempotencyRecord{}, domain.Book{}, false, fmt.Errorf("idempotent request already in progress")
		case domain.IdempotencyStateFailed:
			record.State = domain.IdempotencyStateProcessing
			record.StatusCode = 0
			record.UpdatedAt = time.Now().UTC()
			record.ResourceID = ""
			record.ResourceType = ""
			if err := a.store.SaveIdempotencyRecord(record); err != nil {
				return domain.IdempotencyRecord{}, domain.Book{}, false, err
			}
			return record, domain.Book{}, false, nil
		}
	}
	now := time.Now().UTC()
	record = domain.IdempotencyRecord{
		ID:             util.NewID(),
		Scope:          scope,
		ActorID:        strings.TrimSpace(actorID),
		IdempotencyKey: key,
		RequestHash:    requestHash,
		State:          domain.IdempotencyStateProcessing,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := a.store.SaveIdempotencyRecord(record); err != nil {
		current, found, getErr := a.store.GetIdempotencyRecord(scope, actorID, key)
		if getErr != nil {
			return domain.IdempotencyRecord{}, domain.Book{}, false, err
		}
		if !found {
			return domain.IdempotencyRecord{}, domain.Book{}, false, err
		}
		if current.RequestHash != requestHash {
			return domain.IdempotencyRecord{}, domain.Book{}, false, fmt.Errorf("idempotency key reused with different request")
		}
		if current.State == domain.IdempotencyStateCompleted {
			book, found, getErr := a.store.GetBook(current.ResourceID)
			if getErr != nil {
				return domain.IdempotencyRecord{}, domain.Book{}, false, getErr
			}
			if !found {
				return domain.IdempotencyRecord{}, domain.Book{}, false, fmt.Errorf("idempotent resource not found")
			}
			return current, book, true, nil
		}
		return domain.IdempotencyRecord{}, domain.Book{}, false, fmt.Errorf("idempotent request already in progress")
	}
	return record, domain.Book{}, false, nil
}

func (a *App) completeBookIdempotency(record domain.IdempotencyRecord, resourceType, resourceID string, statusCode int) error {
	record.State = domain.IdempotencyStateCompleted
	record.ResourceType = strings.TrimSpace(resourceType)
	record.ResourceID = strings.TrimSpace(resourceID)
	record.StatusCode = statusCode
	record.UpdatedAt = time.Now().UTC()
	return a.store.SaveIdempotencyRecord(record)
}

func (a *App) markBookIdempotencyFailed(record domain.IdempotencyRecord, statusCode int) error {
	if strings.TrimSpace(record.ID) == "" {
		return nil
	}
	record.State = domain.IdempotencyStateFailed
	record.StatusCode = statusCode
	record.UpdatedAt = time.Now().UTC()
	return a.store.SaveIdempotencyRecord(record)
}

func httpStatusFromErr(err error) int {
	if err == nil {
		return http.StatusOK
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(message, "required"), strings.Contains(message, "invalid"), strings.Contains(message, "unsupported"), strings.Contains(message, "too large"):
		return http.StatusBadRequest
	case strings.Contains(message, "not found"):
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}
