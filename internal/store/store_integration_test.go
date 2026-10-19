package store

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Aabyss-Team/ARL/internal/domain"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestUpsertAssetWithScope(t *testing.T) {
	uri := os.Getenv("ARL_TEST_MONGO_URI")
	if uri == "" {
		t.Skip("set ARL_TEST_MONGO_URI to run MongoDB integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	dbName := "arl_test_" + time.Now().Format("20060102150405")
	st, err := Open(ctx, uri, dbName)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = st.DB().Drop(context.Background())
		_ = st.Close(context.Background())
	}()
	if err := st.EnsureIndexes(ctx); err != nil {
		t.Fatal(err)
	}
	scopeID := bson.NewObjectID()
	if err := st.CreateScope(ctx, &domain.Scope{ID: scopeID, Name: "test", Type: "domain", Targets: []string{"example.com"}}); err != nil {
		t.Fatal(err)
	}
	taskID := bson.NewObjectID()
	for i := 0; i < 2; i++ {
		asset, err := st.UpsertAsset(ctx, domain.AssetDomain, "example.com", map[string]any{"domain": "example.com"}, taskID.Hex(), scopeID.Hex())
		if err != nil {
			t.Fatalf("upsert %d: %v", i, err)
		}
		if err := st.ObserveAsset(ctx, taskID, asset, "test", map[string]any{"domain": "example.com"}); err != nil {
			t.Fatal(err)
		}
		if err := st.LinkAssetToScope(ctx, scopeID, asset, []string{"smoke"}); err != nil {
			t.Fatal(err)
		}
	}
	items, total, err := st.ListScopeAssets(ctx, scopeID, "domain", PageQuery{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("unexpected scope assets: total=%d len=%d", total, len(items))
	}
}
