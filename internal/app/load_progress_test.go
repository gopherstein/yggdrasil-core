package app

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/yeixio/toskar-core/internal/config"
	"github.com/yeixio/toskar-core/internal/runlog"
	"github.com/yeixio/toskar-core/internal/store"
	"github.com/yeixio/toskar-core/pkg/pluginapi"
)

// model.load.started says whether this computer has to load the model and
// about how long that took before (#498).
func TestLoadStartedSaysColdAndHowLong(t *testing.T) {
	a, _ := tokenApp(pluginapi.RunningModel{ModelID: "warm", Endpoint: "http://warm", Status: "running"})
	cfg, err := config.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a.Config = cfg
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	a.RunLog = runlog.NewStore(db.SQL)
	node := a.nodeDisplayName("")
	c := runlog.New("r1", "c1", "", "chat")
	c.Loaded("cold", 12*time.Second)
	c.ModelCall("cold", "assistant", node, 13*time.Second, nil)
	if err := a.RunLog.Save(context.Background(), c.Finish(runlog.StatusCompleted, nil)); err != nil {
		t.Fatal(err)
	}

	p := a.loadStartedPayload(context.Background(), "c2", "cold", "", "assistant")
	if p["cold"] != true || p["expected_ms"] != 12000.0 || p["conversation_id"] != "c2" || p["model_name"] != "cold" {
		t.Fatalf("cold payload = %v", p)
	}
	p = a.loadStartedPayload(context.Background(), "", "warm", "", "assistant")
	if p["cold"] != false || p["expected_ms"] != nil || p["conversation_id"] != nil {
		t.Fatalf("warm payload = %v", p)
	}
}
