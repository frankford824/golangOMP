package mysqlrepo

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestBatchProjectionBindsEmbeddingVersionBeforeDerivedIDs(t *testing.T) {
	tests := []struct {
		name       string
		entityType string
		apply      func(context.Context, *AIRetrievalProjector) error
	}{
		{
			name:       "task",
			entityType: "task",
			apply: func(ctx context.Context, projector *AIRetrievalProjector) error {
				return projector.projectIDs(ctx, "task", []int64{11, 12}, "embed:v1")
			},
		},
		{
			name:       "resource group",
			entityType: "task_resource_group",
			apply: func(ctx context.Context, projector *AIRetrievalProjector) error {
				return projector.projectIDs(ctx, "task_resource_group", []int64{11, 12}, "embed:v1")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			firstID := retrievalDocumentID(tt.entityType, 11)
			secondID := retrievalDocumentID(tt.entityType, 12)
			mock.ExpectBegin()
			mock.ExpectExec("INSERT INTO ai_retrieval_documents").
				WithArgs("embed:v1", int64(11), firstID, int64(12), secondID).
				WillReturnResult(sqlmock.NewResult(0, 2))
			mock.ExpectExec("INSERT INTO ai_retrieval_outbox").
				WithArgs(firstID, secondID).
				WillReturnResult(sqlmock.NewResult(0, 2))
			mock.ExpectCommit()
			if err := tt.apply(context.Background(), NewAIRetrievalProjector(New(db))); err != nil {
				t.Fatalf("apply batch projection: %v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestExternalBatchProjectionBindsEmbeddingVersionBeforeDerivedIDs(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	firstID := retrievalDocumentID("external_asset", 21)
	secondID := retrievalDocumentID("external_asset", 22)
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO ai_retrieval_documents").
		WithArgs("embed:v1", int64(21), firstID, int64(22), secondID).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("INSERT INTO ai_retrieval_outbox").
		WithArgs(firstID, secondID).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectCommit()
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := upsertAIExternalAssetProjectionBatch(context.Background(), tx, []int64{21, 22}, "embed:v1"); err != nil {
		t.Fatalf("apply external projection: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
