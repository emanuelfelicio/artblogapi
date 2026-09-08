package db_test

import (
	"context"
	"testing"

	"github.com/emanuelfelicio/artblogapi/db"
)

func TestContextWithTx_Empty(t *testing.T) {
	ctx := context.Background()
	tx, ok := db.TxFromContext(ctx)
	if ok || tx != nil {
		t.Fatalf("expected nil tx, got %v", tx)
	}
}
