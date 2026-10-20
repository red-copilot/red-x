package tasking

import (
	"testing"

	"github.com/red-copilot/red-x/internal/domain"
	"github.com/red-copilot/red-x/internal/platform/config"
	"github.com/red-copilot/red-x/internal/platform/jobs"
	"github.com/red-copilot/red-x/internal/platform/logx"
)

func TestSubmitPersistsEASMLineage(t *testing.T) {
	st, ctx := newTaskingPostgresStore(t)
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
