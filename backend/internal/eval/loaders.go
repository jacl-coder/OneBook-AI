package eval

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ReadChunksJSONL loads chunks from JSONL and normalizes key variants.
func ReadChunksJSONL(path string) ([]ChunkRecord, error) {
	rows, err := readJSONLMaps(path)
	if err != nil {
		return nil, err
	}
	out := make([]ChunkRecord, 0, len(rows))
	for i, row := range rows {
		chunkID := firstString(row, "chunk_id", "chunkId", "id")
		docID := firstString(row, "doc_id", "docId", "book_id", "bookId")
		text := firstString(row, "text", "content")
		if strings.TrimSpace(chunkID) == "" {
			chunkID = fmt.Sprintf("row_%d", i+1)
		}
		meta := mapStringString(row, "metadata", "meta")
		out = append(out, ChunkRecord{ChunkID: chunkID, DocID: docID, Text: text, Metadata: meta})
	}
	return out, nil
}

// ReadQueriesJSONL loads retrieval/e2e queries from JSONL.
func ReadQueriesJSONL(path string) ([]QueryRecord, error) {
	rows, err := readJSONLMaps(path)
	if err != nil {
		return nil, err
	}
	out := make([]QueryRecord, 0, len(rows))
	for i, row := range rows {
		qid := firstString(row, "qid", "query_id", "id")
		if strings.TrimSpace(qid) == "" {
			qid = fmt.Sprintf("q_%d", i+1)
		}
		out = append(out, QueryRecord{
			QID:            qid,
			Query:          firstString(row, "query", "question", "text"),
			BookID:         firstString(row, "book_id", "bookId"),
			ExpectedAnswer: firstString(row, "expected_answer", "gold_answer", "reference_answer"),
			ExpectAbstain:  firstBool(row, "expect_abstain", "expected_abstain"),
		})
	}
	return out, nil
}

// ReadQrels loads qrels from .tsv/.txt or JSONL.
func ReadQrels(path string) ([]QRel, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".tsv" || ext == ".txt" {
		return readQrelsTSV(path)
	}
	rows, err := readJSONLMaps(path)
	if err != nil {
		return nil, err
	}
	out := make([]QRel, 0, len(rows))
	for _, row := range rows {
		qid := firstString(row, "qid", "query_id")
		docID := firstString(row, "doc_id", "chunk_id", "id")
		rel := firstInt(row, "relevance", "rel", "score")
		if strings.TrimSpace(qid) == "" || strings.TrimSpace(docID) == "" {
			continue
		}
		out = append(out, QRel{QID: qid, DocID: docID, Relevance: rel})
	}
	return out, nil
}

// ReadRunJSONL loads run results from JSONL.
func ReadRunJSONL(path string) ([]RunEntry, error) {
	rows, err := readJSONLMaps(path)
	if err != nil {
		return nil, err
	}
	out := make([]RunEntry, 0, len(rows))
	for _, row := range rows {
		qid := firstString(row, "qid", "query_id")
		if strings.TrimSpace(qid) == "" {
			continue
		}
		results := parseResults(row)
		out = append(out, RunEntry{QID: qid, Results: results})
	}
	return out, nil
}

// ReadPredictionsJSONL loads model output predictions.
func ReadPredictionsJSONL(path string) ([]PredictionRecord, error) {
	rows, err := readJSONLMaps(path)
	if err != nil {
		return nil, err
	}
	out := make([]PredictionRecord, 0, len(rows))
	for _, row := range rows {
		qid := firstString(row, "qid", "query_id")
		if strings.TrimSpace(qid) == "" {
			continue
		}
		out = append(out, PredictionRecord{
			QID:       qid,
			Answer:    firstString(row, "answer", "response"),
			Citations: firstStringSlice(row, "citations", "source_ids", "sources"),
			Abstained: firstBool(row, "abstained", "is_abstain"),
		})
	}
	return out, nil
}

// ReadEmbeddingsJSONL loads embedding vectors from JSONL.
func ReadEmbeddingsJSONL(path string) ([]EmbeddingRecord, error) {
	rows, err := readJSONLMaps(path)
	if err != nil {
		return nil, err
	}
	out := make([]EmbeddingRecord, 0, len(rows))
	for _, row := range rows {
		id := firstString(row, "id", "chunk_id", "doc_id")
		if id == "" {
			continue
		}
		vector, ok := decodeFloat32Slice(row["vector"])
		if !ok {
			if v, ok := decodeFloat32Slice(row["embedding"]); ok {
				vector = v
			}
		}
		out = append(out, EmbeddingRecord{ID: id, Vector: vector})
	}
	return out, nil
}

func readQrelsTSV(path string) ([]QRel, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	out := make([]QRel, 0, 1024)
	s := bufio.NewScanner(f)
	buf := make([]byte, 0, 1024*1024)
	s.Buffer(buf, 8*1024*1024)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}
		var qid, docID, relRaw string
		if len(parts) >= 4 {
			qid, docID, relRaw = parts[0], parts[2], parts[3]
		} else {
			qid, docID, relRaw = parts[0], parts[1], parts[2]
		}
		rel, err := strconv.Atoi(relRaw)
		if err != nil {
			continue
		}
		out = append(out, QRel{QID: qid, DocID: docID, Relevance: rel})
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func readJSONLMaps(path string) ([]map[string]json.RawMessage, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	rows := make([]map[string]json.RawMessage, 0, 1024)
	s := bufio.NewScanner(f)
	buf := make([]byte, 0, 1024*1024)
	s.Buffer(buf, 8*1024*1024)
	lineNo := 0
	for s.Scan() {
		lineNo++
		line := strings.TrimSpace(s.Text())
		if line == "" {
			continue
		}
		var row map[string]json.RawMessage
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			return nil, fmt.Errorf("parse %s line %d: %w", path, lineNo, err)
		}
		rows = append(rows, row)
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return rows, nil
}

func firstString(row map[string]json.RawMessage, keys ...string) string {
	for _, key := range keys {
		raw, ok := row[key]
		if !ok {
			continue
		}
		if v, ok := decodeString(raw); ok {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func firstInt(row map[string]json.RawMessage, keys ...string) int {
	for _, key := range keys {
		raw, ok := row[key]
		if !ok {
			continue
		}
		if v, ok := decodeInt(raw); ok {
			return v
		}
	}
	return 0
}

func firstBool(row map[string]json.RawMessage, keys ...string) bool {
	for _, key := range keys {
		raw, ok := row[key]
		if !ok {
			continue
		}
		if v, ok := decodeBool(raw); ok {
			return v
		}
	}
	return false
}

func firstStringSlice(row map[string]json.RawMessage, keys ...string) []string {
	for _, key := range keys {
		raw, ok := row[key]
		if !ok {
			continue
		}
		if arr, ok := decodeStringSlice(raw); ok {
			return arr
		}
	}
	return nil
}

func mapStringString(row map[string]json.RawMessage, keys ...string) map[string]string {
	for _, key := range keys {
		raw, ok := row[key]
		if !ok {
			continue
		}
		var obj map[string]interface{}
		if err := json.Unmarshal(raw, &obj); err != nil {
			continue
		}
		out := make(map[string]string, len(obj))
		for k, v := range obj {
			out[k] = fmt.Sprint(v)
		}
		return out
	}
	return nil
}

func parseResults(row map[string]json.RawMessage) []RunHit {
	for _, key := range []string{"results", "hits", "retrieved"} {
		raw, ok := row[key]
		if !ok {
			continue
		}
		var arr []map[string]json.RawMessage
		if err := json.Unmarshal(raw, &arr); err == nil {
			out := make([]RunHit, 0, len(arr))
			for _, item := range arr {
				docID := firstString(item, "doc_id", "chunk_id", "id")
				if docID == "" {
					continue
				}
				score, _ := firstFloat(item, "score", "similarity", "distance")
				out = append(out, RunHit{DocID: docID, Score: score})
			}
			return out
		}
		var ids []string
		if err := json.Unmarshal(raw, &ids); err == nil {
			out := make([]RunHit, 0, len(ids))
			for _, id := range ids {
				id = strings.TrimSpace(id)
				if id == "" {
					continue
				}
				out = append(out, RunHit{DocID: id})
			}
			return out
		}
	}
	return nil
}

func firstFloat(row map[string]json.RawMessage, keys ...string) (float64, bool) {
	for _, key := range keys {
		raw, ok := row[key]
		if !ok {
			continue
		}
		if v, ok := decodeFloat(raw); ok {
			return v, true
		}
	}
	return 0, false
}

func decodeString(raw json.RawMessage) (string, bool) {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s, true
	}
	var n float64
	if err := json.Unmarshal(raw, &n); err == nil {
		return strconv.FormatFloat(n, 'f', -1, 64), true
	}
	return "", false
}

func decodeInt(raw json.RawMessage) (int, bool) {
	var n int
	if err := json.Unmarshal(raw, &n); err == nil {
		return n, true
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return int(f), true
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		v, err := strconv.Atoi(strings.TrimSpace(s))
		if err == nil {
			return v, true
		}
	}
	return 0, false
}

func decodeBool(raw json.RawMessage) (bool, bool) {
	var b bool
	if err := json.Unmarshal(raw, &b); err == nil {
		return b, true
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		s = strings.ToLower(strings.TrimSpace(s))
		if s == "true" || s == "1" || s == "yes" {
			return true, true
		}
		if s == "false" || s == "0" || s == "no" {
			return false, true
		}
	}
	return false, false
}

func decodeStringSlice(raw json.RawMessage) ([]string, bool) {
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		out := make([]string, 0, len(arr))
		for _, v := range arr {
			v = strings.TrimSpace(v)
			if v != "" {
				out = append(out, v)
			}
		}
		return out, true
	}
	return nil, false
}

func decodeFloat(raw json.RawMessage) (float64, bool) {
	var v float64
	if err := json.Unmarshal(raw, &v); err == nil {
		return v, true
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		n, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
		if err == nil {
			return n, true
		}
	}
	return 0, false
}

func decodeFloat32Slice(raw json.RawMessage) ([]float32, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	var arr []float32
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr, true
	}
	var arr64 []float64
	if err := json.Unmarshal(raw, &arr64); err == nil {
		out := make([]float32, 0, len(arr64))
		for _, v := range arr64 {
			out = append(out, float32(v))
		}
		return out, true
	}
	return nil, false
}
