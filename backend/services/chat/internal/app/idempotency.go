package app

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"onebookai/internal/util"
	"onebookai/pkg/domain"
)

const idempotencyScopeAskQuestion = "chat.ask"

func (a *App) beginChatIdempotency(userID, bookID, conversationID, question, key string) (domain.IdempotencyRecord, domain.Answer, bool, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return domain.IdempotencyRecord{}, domain.Answer{}, false, nil
	}
	requestHash := util.HashStrings(strings.TrimSpace(bookID), strings.TrimSpace(conversationID), strings.TrimSpace(question))
	record, ok, err := a.store.GetIdempotencyRecord(idempotencyScopeAskQuestion, strings.TrimSpace(userID), key)
	if err != nil {
		return domain.IdempotencyRecord{}, domain.Answer{}, false, err
	}
	if ok {
		if record.RequestHash != requestHash {
			return domain.IdempotencyRecord{}, domain.Answer{}, false, fmt.Errorf("idempotency key reused with different request")
		}
		switch record.State {
		case domain.IdempotencyStateCompleted:
			answer, err := unmarshalAnswerResponse(record.ResponseJSON)
			return record, answer, true, err
		case domain.IdempotencyStateProcessing:
			return domain.IdempotencyRecord{}, domain.Answer{}, false, fmt.Errorf("idempotent request already in progress")
		case domain.IdempotencyStateFailed:
			record.State = domain.IdempotencyStateProcessing
			record.StatusCode = 0
			record.ResourceID = ""
			record.ResourceType = ""
			record.ResponseJSON = nil
			record.UpdatedAt = time.Now().UTC()
			if err := a.store.SaveIdempotencyRecord(record); err != nil {
				return domain.IdempotencyRecord{}, domain.Answer{}, false, err
			}
			return record, domain.Answer{}, false, nil
		}
	}
	now := time.Now().UTC()
	record = domain.IdempotencyRecord{
		ID:             util.NewID(),
		Scope:          idempotencyScopeAskQuestion,
		ActorID:        strings.TrimSpace(userID),
		IdempotencyKey: key,
		RequestHash:    requestHash,
		State:          domain.IdempotencyStateProcessing,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := a.store.SaveIdempotencyRecord(record); err != nil {
		current, found, getErr := a.store.GetIdempotencyRecord(idempotencyScopeAskQuestion, userID, key)
		if getErr != nil {
			return domain.IdempotencyRecord{}, domain.Answer{}, false, err
		}
		if !found {
			return domain.IdempotencyRecord{}, domain.Answer{}, false, err
		}
		if current.RequestHash != requestHash {
			return domain.IdempotencyRecord{}, domain.Answer{}, false, fmt.Errorf("idempotency key reused with different request")
		}
		if current.State == domain.IdempotencyStateCompleted {
			answer, err := unmarshalAnswerResponse(current.ResponseJSON)
			return current, answer, true, err
		}
		return domain.IdempotencyRecord{}, domain.Answer{}, false, fmt.Errorf("idempotent request already in progress")
	}
	return record, domain.Answer{}, false, nil
}

func marshalAnswerResponse(answer domain.Answer) (json.RawMessage, error) {
	raw, err := json.Marshal(answer)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func unmarshalAnswerResponse(raw json.RawMessage) (domain.Answer, error) {
	if len(raw) == 0 {
		return domain.Answer{}, fmt.Errorf("idempotent response not found")
	}
	var answer domain.Answer
	if err := json.Unmarshal(raw, &answer); err != nil {
		return domain.Answer{}, err
	}
	return answer, nil
}
