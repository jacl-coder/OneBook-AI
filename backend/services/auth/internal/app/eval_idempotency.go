package app

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"onebookai/internal/util"
	"onebookai/pkg/domain"
)

const idempotencyScopeEvalRun = "auth.eval.run.create"

func buildEvalRunFingerprint(datasetID string, mode domain.EvalRunMode, retrievalMode domain.EvalRetrievalMode, gateMode string, params map[string]any) string {
	return util.HashStrings(
		strings.TrimSpace(datasetID),
		string(mode),
		string(retrievalMode),
		strings.TrimSpace(gateMode),
		util.HashJSON(params),
	)
}

func (a *App) beginEvalIdempotency(actorID, key, fingerprint string, input EvalRunCreateInput) (domain.IdempotencyRecord, domain.EvalRun, bool, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return domain.IdempotencyRecord{}, domain.EvalRun{}, false, fmt.Errorf("idempotency key required")
	}
	requestHash := util.HashStrings(fingerprint, util.HashJSON(map[string]any{
		"datasetId":     strings.TrimSpace(input.DatasetID),
		"mode":          string(input.Mode),
		"retrievalMode": string(input.RetrievalMode),
		"gateMode":      strings.TrimSpace(input.GateMode),
		"params":        input.Params,
	}))
	record, ok, err := a.store.GetIdempotencyRecord(idempotencyScopeEvalRun, actorID, key)
	if err != nil {
		return domain.IdempotencyRecord{}, domain.EvalRun{}, false, err
	}
	if ok {
		if record.RequestHash != requestHash {
			return domain.IdempotencyRecord{}, domain.EvalRun{}, false, fmt.Errorf("idempotency key reused with different request")
		}
		switch record.State {
		case domain.IdempotencyStateCompleted:
			run, found, getErr := a.store.GetEvalRun(record.ResourceID)
			if getErr != nil {
				return domain.IdempotencyRecord{}, domain.EvalRun{}, false, getErr
			}
			if !found {
				return domain.IdempotencyRecord{}, domain.EvalRun{}, false, fmt.Errorf("idempotent resource not found")
			}
			return record, run, true, nil
		case domain.IdempotencyStateProcessing:
			return domain.IdempotencyRecord{}, domain.EvalRun{}, false, fmt.Errorf("idempotent request already in progress")
		case domain.IdempotencyStateFailed:
			record.State = domain.IdempotencyStateProcessing
			record.StatusCode = 0
			record.ResourceID = ""
			record.ResourceType = ""
			record.UpdatedAt = time.Now().UTC()
			if err := a.store.SaveIdempotencyRecord(record); err != nil {
				return domain.IdempotencyRecord{}, domain.EvalRun{}, false, err
			}
			return record, domain.EvalRun{}, false, nil
		}
	}
	now := time.Now().UTC()
	record = domain.IdempotencyRecord{
		ID:             util.NewID(),
		Scope:          idempotencyScopeEvalRun,
		ActorID:        strings.TrimSpace(actorID),
		IdempotencyKey: key,
		RequestHash:    requestHash,
		State:          domain.IdempotencyStateProcessing,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := a.store.SaveIdempotencyRecord(record); err != nil {
		current, found, getErr := a.store.GetIdempotencyRecord(idempotencyScopeEvalRun, actorID, key)
		if getErr != nil {
			return domain.IdempotencyRecord{}, domain.EvalRun{}, false, err
		}
		if !found {
			return domain.IdempotencyRecord{}, domain.EvalRun{}, false, err
		}
		if current.RequestHash != requestHash {
			return domain.IdempotencyRecord{}, domain.EvalRun{}, false, fmt.Errorf("idempotency key reused with different request")
		}
		if current.State == domain.IdempotencyStateCompleted {
			run, found, getErr := a.store.GetEvalRun(current.ResourceID)
			if getErr != nil {
				return domain.IdempotencyRecord{}, domain.EvalRun{}, false, getErr
			}
			if !found {
				return domain.IdempotencyRecord{}, domain.EvalRun{}, false, fmt.Errorf("idempotent resource not found")
			}
			return current, run, true, nil
		}
		return domain.IdempotencyRecord{}, domain.EvalRun{}, false, fmt.Errorf("idempotent request already in progress")
	}
	return record, domain.EvalRun{}, false, nil
}

func (a *App) completeEvalIdempotency(record domain.IdempotencyRecord, runID string, statusCode int) error {
	record.State = domain.IdempotencyStateCompleted
	record.ResourceType = "eval_run"
	record.ResourceID = strings.TrimSpace(runID)
	record.StatusCode = statusCode
	record.UpdatedAt = time.Now().UTC()
	return a.store.SaveIdempotencyRecord(record)
}

func (a *App) markEvalIdempotencyFailed(record domain.IdempotencyRecord, statusCode int) error {
	if strings.TrimSpace(record.ID) == "" {
		return nil
	}
	record.State = domain.IdempotencyStateFailed
	record.StatusCode = statusCode
	record.UpdatedAt = time.Now().UTC()
	return a.store.SaveIdempotencyRecord(record)
}

func statusCodeForEvalErr(err error) int {
	if err == nil {
		return http.StatusOK
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(message, "not found"):
		return http.StatusNotFound
	case strings.Contains(message, "invalid"), strings.Contains(message, "required"):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}
