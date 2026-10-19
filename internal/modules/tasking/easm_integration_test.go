package tasking

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/red-copilot/red-x/internal/domain"
	"github.com/red-copilot/red-x/internal/platform/config"
	"github.com/red-copilot/red-x/internal/platform/jobs"
	"github.com/red-copilot/red-x/internal/platform/logx"
	"github.com/red-copilot/red-x/internal/store"
)

func TestSubmitPersistsEASMLineage(t *testing.T) {
	uri := os.Getenv("ARL_TEST_MONGO_URI")
	if uri == "" {
		t.Skip("set ARL_TEST_MONGO_URI to run MongoDB integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	st, err := store.Open(ctx, uri, fmt.Sprintf("arl_tasking_easm_test_%d", time.Now().UnixNano()))
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
	queue := jobs.NewMemory(logx.New("error", false))
	defer queue.Close()
	cfg := config.Default()
	cfg.Scan.MaxHosts = 10
	engine := &Engine{Store: st, Queue: queue, Config: cfg, Log: logx.New("error", false)}
	orgID := "507f1f77bcf86cd799439011"
	created, err := engine.Submit(ctx, SubmitInput{
		Name: "EASM lineage", Kind: domain.TaskDomain, Target: "guc.edu.cn",
		OrganizationID: orgID, RootDomain: "guc.edu.cn", Origin: domain.TaskOriginEASM,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(created) != 1 {
		t.Fatalf("created %d tasks", len(created))
	}
	reloaded, err := st.GetTask(ctx, created[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.OrganizationID != orgID || reloaded.RootDomain != "guc.edu.cn" || reloaded.Origin != domain.TaskOriginEASM {
		t.Fatalf("lineage not persisted: %+v", reloaded)
	}
}
