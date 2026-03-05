package server

import (
	"net/http/httptest"
	"testing"

	"onebookai/pkg/domain"
)

func TestParseAdminPageParams(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/admin/users?page=2&pageSize=50", nil)
	page, pageSize, err := parseAdminPageParams(req)
	if err != nil {
		t.Fatalf("parseAdminPageParams error: %v", err)
	}
	if page != 2 || pageSize != 50 {
		t.Fatalf("unexpected page/pageSize: %d/%d", page, pageSize)
	}

	badReq := httptest.NewRequest("GET", "/api/admin/users?page=0&pageSize=101", nil)
	if _, _, err := parseAdminPageParams(badReq); err == nil {
		t.Fatalf("expected invalid pagination error")
	}
}

func TestPaginateBooks(t *testing.T) {
	books := []domain.Book{
		{ID: "b1"},
		{ID: "b2"},
		{ID: "b3"},
		{ID: "b4"},
	}
	got := paginateBooks(books, 2, 2)
	if len(got) != 2 {
		t.Fatalf("expected 2 items, got %d", len(got))
	}
	if got[0].ID != "b3" || got[1].ID != "b4" {
		t.Fatalf("unexpected page items: %+v", got)
	}

	empty := paginateBooks(books, 3, 3)
	if len(empty) != 0 {
		t.Fatalf("expected empty page, got %d", len(empty))
	}
}
