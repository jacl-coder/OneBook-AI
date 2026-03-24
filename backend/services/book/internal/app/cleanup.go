package app

import (
	"context"
	"time"

	"onebookai/pkg/domain"
)

const bookCleanupPollInterval = 5 * time.Second

func (a *App) startCleanupWorker() {
	go func() {
		ticker := time.NewTicker(bookCleanupPollInterval)
		defer ticker.Stop()
		for range ticker.C {
			a.cleanupDeletedBooks(context.Background())
		}
	}()
}

func (a *App) cleanupDeletedBooks(ctx context.Context) {
	books, err := a.store.ClaimBooksPendingCleanup(10)
	if err != nil {
		return
	}
	for _, book := range books {
		a.cleanupDeletedBook(ctx, book)
	}
}

func (a *App) cleanupDeletedBook(ctx context.Context, book domain.Book) {
	if a.search != nil {
		if err := a.search.DeleteByBook(ctx, book.ID); err != nil {
			_ = a.store.UpdateBookCleanup(book.ID, domain.BookCleanupStatusFailed, err.Error(), false)
			return
		}
	}
	if err := a.objects.Delete(ctx, book.StorageKey); err != nil {
		_ = a.store.UpdateBookCleanup(book.ID, domain.BookCleanupStatusFailed, err.Error(), false)
		return
	}
	if err := a.store.DeleteBook(book.ID); err != nil {
		_ = a.store.UpdateBookCleanup(book.ID, domain.BookCleanupStatusFailed, err.Error(), false)
	}
}
