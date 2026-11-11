package infra

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvtest"
)

func fakeTemplates(t *testing.T) {
	t.Helper()
	prev := initScriptTemplateLookup
	initScriptTemplateLookup = func(nsId, id string) (model.InitScriptTemplateInfo, error) {
		switch id {
		case "base":
			return model.InitScriptTemplateInfo{Id: "base", Scripts: []model.InitScriptStep{{Script: "echo base\n"}}, Env: map[string]string{"ROLE": "tmpl", "TIER": "t1"}}, nil
		case "profile":
			return model.InitScriptTemplateInfo{Id: "profile", Scripts: []model.InitScriptStep{{Script: "echo p1\n"}, {Name: "agent", Script: "echo agent\n"}}}, nil
		}
		return model.InitScriptTemplateInfo{}, fmt.Errorf("initScript template '%s' not found", id)
	}
	t.Cleanup(func() { initScriptTemplateLookup = prev })
}

func TestResolveInitScript(t *testing.T) {
	fakeTemplates(t)
	req := &model.InitScriptReq{Mode: "custom", TemplateId: "base", Script: "echo inline\n",
		Scripts: []model.InitScriptStep{{Name: "otel", Script: "echo otel\n"}, {Script: "echo last\n"}},
		Env:     map[string]string{"ROLE": "req"}}
	got, err := resolveInitScript("ns1", req)
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, s := range got.Scripts {
		names = append(names, s.Name+":"+strings.TrimSpace(s.Script))
	}
	if want := "base:echo base,inline:echo inline,otel:echo otel,step-4:echo last"; strings.Join(names, ",") != want {
		t.Fatalf("steps = %v", names)
	}
	if got.Env["ROLE"] != "req" || got.Env["TIER"] != "t1" || got.FromTemplateId != "base" {
		t.Fatalf("env merge or source wrong: %+v", got)
	}
	if got.TemplateId != "" || got.Script != "" || req.TemplateId != "base" {
		t.Fatal("shorthand fields must be folded into Scripts on the copy only")
	}
	again, err := resolveInitScript("ns1", got)
	if err != nil || len(again.Scripts) != 4 || again.Env["ROLE"] != "req" {
		t.Fatalf("resolution must be idempotent: %v %v", again, err)
	}
	// a multi-step template expands to its steps, named after the template unless the step has a name
	prof, err := resolveInitScript("ns1", &model.InitScriptReq{TemplateId: "profile", Mode: "custom"})
	if err != nil || len(prof.Scripts) != 2 || prof.Scripts[0].Name != "profile-1" || prof.Scripts[1].Name != "agent" {
		t.Fatalf("profile expansion wrong: %+v %v", prof, err)
	}
	if _, err := resolveInitScript("ns1", &model.InitScriptReq{TemplateId: "missing"}); err == nil {
		t.Fatal("missing template must fail")
	}
	none, err := resolveInitScript("ns1", &model.InitScriptReq{Mode: "none", TemplateId: "missing"})
	if err != nil || none.TemplateId != "missing" {
		t.Fatal("mode none must not touch templates")
	}
}

func TestBuildInitScriptCustomMode(t *testing.T) {
	fakeTemplates(t)
	NotifierProvider = func() *model.NotifierInfo {
		return &model.NotifierInfo{Status: model.NotifierStatusActive, Url: "https://ntfy.example.com", Topic: "tb-x", WriteToken: "tk_w"}
	}
	defer func() { NotifierProvider = nil }()
	node := &model.NodeInfo{Id: "g1-1", NodeGroupId: "g1", InitScript: &model.InitScriptReq{
		Mode: "custom", UserName: "ubuntu", TemplateId: "base", Script: "#!/bin/bash\necho hi > /tmp/x\n", Env: map[string]string{"ROLE": "worker"},
	}}
	node.ConnectionConfig.ProviderName = "aws"
	script, err := buildInitScript("ns1", "infra1", node, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"export TB_INIT_ACCESS='0'", "export TB_INIT_USER='ubuntu'", "export ROLE='worker'", "export TIER='t1'",
		"export TB_NOTIFY_URL='https://ntfy.example.com'", "export TB_NODE_ID='g1-1'", "export TB_CSP='aws'",
		"export TB_INIT_SCRIPT_COUNT='2'", "export TB_INIT_SCRIPT_1_NAME='base'", "export TB_INIT_SCRIPT_2_NAME='inline'",
		"export TB_INIT_SCRIPT_2_B64='" + base64.StdEncoding.EncodeToString([]byte("#!/bin/bash\necho hi > /tmp/x\n")) + "'"} {
		if !strings.Contains(script, want) {
			t.Fatalf("missing %q", want)
		}
	}
	if !strings.HasPrefix(script, "#!/bin/bash\nexport ") {
		t.Fatalf("script must start with the shebang followed by exports, got %q", script[:40])
	}
	cmd := exec.Command("bash", "-n")
	cmd.Stdin = strings.NewReader(script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("syntax error: %v\n%s", err, out)
	}
	if s, err := buildInitScript("ns1", "infra1", node, true); err != nil || s != "" {
		t.Fatalf("windows should get no user data: %q %v", s, err)
	}
	node.InitScript.Mode = "none"
	if s, _ := buildInitScript("ns1", "infra1", node, false); s != "" {
		t.Fatal("mode none should get no user data")
	}
}

// TestInitScriptWrapperRuns executes the rendered user data locally (no account creation) against a fake ntfy
// and checks the status marker, the step order and the stop-on-failure behaviour.
func TestInitScriptWrapperRuns(t *testing.T) {
	for _, bin := range []string{"bash", "curl", "base64"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("%s not available", bin)
		}
	}
	var reports []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var report map[string]any
		if err := json.Unmarshal(body, &report); err != nil {
			t.Errorf("report is not JSON: %v\n%s", err, body)
		}
		reports = append(reports, report)
		if r.Header.Get("Authorization") != "Bearer tk_w" || r.URL.Path != "/tb-x" {
			t.Errorf("unexpected request %s %s", r.URL.Path, r.Header.Get("Authorization"))
		}
	}))
	defer srv.Close()
	NotifierProvider = func() *model.NotifierInfo {
		return &model.NotifierInfo{Status: model.NotifierStatusActive, Url: srv.URL, Topic: "tb-x", WriteToken: "tk_w"}
	}
	defer func() { NotifierProvider = nil }()
	dir := t.TempDir()
	req := &model.InitScriptReq{Mode: "custom", Scripts: []model.InitScriptStep{
		{Name: "one", Script: "echo one-$ROLE > " + filepath.Join(dir, "one") + "\n"},
		{Name: "two", Script: "echo failing; exit 3\n"},
		{Name: "three", Script: "touch " + filepath.Join(dir, "three") + "\n"},
	}, Env: map[string]string{"ROLE": "w"}}
	node := &model.NodeInfo{Id: "g1-1", NodeGroupId: "g1", InitScript: req}
	script, err := buildInitScript("ns1", "infra1", node, false)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", "-c", script)
	cmd.Env = append(os.Environ(), "TB_DIR="+dir, "TB_LOG="+filepath.Join(dir, "log"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("wrapper failed: %v\n%s", err, out)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "one")); strings.TrimSpace(string(b)) != "one-w" {
		t.Fatalf("step one output = %q", b)
	}
	if _, err := os.Stat(filepath.Join(dir, "three")); err == nil {
		t.Fatal("step three must not run after step two failed")
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "init-script.status")); !strings.HasPrefix(string(b), "fail 3 ") {
		t.Fatalf("status marker = %q", b)
	}
	// one "step" event per started step, then one "done" event
	if len(reports) != 3 || reports[0]["event"] != "step" || reports[0]["step"] != "one" || reports[0]["status"] != "initializing" ||
		reports[1]["step"] != "two" || reports[2]["event"] != "done" {
		t.Fatalf("unexpected event sequence: %v", reports)
	}
	if st := reports[1]["steps"].([]any); st[0].(map[string]any)["status"] != "ok" || st[1].(map[string]any)["status"] != "running" || st[2].(map[string]any)["status"] != "pending" {
		t.Fatalf("second snapshot steps = %v", st)
	}
	report := reports[2]
	if report["status"] != "fail" || report["exitCode"] != float64(3) {
		t.Fatalf("report = %v", report)
	}
	steps, _ := report["steps"].([]any)
	if len(steps) != 3 || steps[1].(map[string]any)["name"] != "two" || steps[1].(map[string]any)["status"] != "fail" || steps[2].(map[string]any)["status"] != "pending" {
		t.Fatalf("steps = %v", report["steps"])
	}
	// a second run must be a no-op because the marker exists
	cmd = exec.Command("bash", "-c", script)
	cmd.Env = append(os.Environ(), "TB_DIR="+dir, "TB_LOG="+filepath.Join(dir, "log"))
	os.Remove(filepath.Join(dir, "one"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("second run failed: %v\n%s", err, out)
	}
	if _, err := os.Stat(filepath.Join(dir, "one")); err == nil {
		t.Fatal("wrapper must not run again once the status marker exists")
	}
}

func TestReviewInitScript(t *testing.T) {
	fakeTemplates(t)
	info, warnings, errs := reviewInitScript("ns1", "infra1", "g1", &model.InitScriptReq{TemplateId: "base"})
	if len(errs) != 0 || len(info) != 1 || !strings.Contains(info[0], "1 step(s) [base]") || len(warnings) != 1 {
		t.Fatalf("info=%v warnings=%v errs=%v", info, warnings, errs)
	}
	big := &model.InitScriptReq{Script: strings.Repeat("x", model.InitScriptMaxBytes)}
	if _, _, errs := reviewInitScript("ns1", "infra1", "g1", big); len(errs) != 1 || !strings.Contains(errs[0], "limit") {
		t.Fatalf("oversized script must be an error: %v", errs)
	}
	if _, _, errs := reviewInitScript("ns1", "infra1", "g1", &model.InitScriptReq{TemplateId: "missing"}); len(errs) != 1 {
		t.Fatalf("missing template must be an error: %v", errs)
	}
	review := model.ReviewNodeGroupDynamicReqInfo{Status: "Ready", CanCreate: true}
	if viable, _ := applyInitScriptReview("ns1", "infra1", &model.CreateNodeGroupDynamicReq{Name: "g1"}, big, &review); viable || review.Status != "Error" || review.CanCreate {
		t.Fatalf("review must turn into Error: %+v", review)
	}
}

func TestValidateInitScriptReq(t *testing.T) {
	bad := []*model.InitScriptReq{
		{Mode: "weird"},
		{UserName: "has space"},
		{Env: map[string]string{"TB_X": "v"}},
		{Env: map[string]string{"1ABC": "v"}},
		{Env: map[string]string{"OK": "it's"}},
		{Scripts: []model.InitScriptStep{{Name: "x"}}}, // no script
		{Scripts: []model.InitScriptStep{{Name: "bad name", Script: "a"}}},
	}
	for i, r := range bad {
		if validateInitScriptReq(r) == nil {
			t.Fatalf("case %d should fail", i)
		}
	}
	if err := validateInitScriptReq(&model.InitScriptReq{Mode: "default", Env: map[string]string{"ROLE": "a b"}}); err != nil {
		t.Fatal(err)
	}
}

func TestDefaultInitScriptAndNotify(t *testing.T) {
	fakeTemplates(t)
	NotifierProvider = func() *model.NotifierInfo {
		return &model.NotifierInfo{Status: model.NotifierStatusActive, Url: "https://n", Topic: "t", WriteToken: "w"}
	}
	defer func() { NotifierProvider = nil }()
	d := defaultInitScript(nil)
	if !d.Implicit || d.Mode != InitScriptModeDefault || !initScriptEnabled(d) {
		t.Fatalf("implicit default wrong: %+v", d)
	}
	explicit := &model.InitScriptReq{Mode: "custom"}
	if defaultInitScript(explicit) != explicit {
		t.Fatal("explicit request must be kept")
	}
	off := false
	script := renderInitScript(map[string]string{}, &model.InitScriptReq{Mode: "custom", Notify: &off})
	if strings.Contains(script, "export TB_NOTIFY_URL=") {
		t.Fatal("notify=false must not export the notifier")
	}
	if !strings.Contains(renderInitScript(map[string]string{}, &model.InitScriptReq{Mode: "custom"}), "export TB_NOTIFY_URL=") {
		t.Fatal("notify unset must export the notifier")
	}
	if _, w, _ := reviewInitScript("ns1", "i", "g1", &model.InitScriptReq{Notify: &off, Mode: "custom"}); len(w) != 0 {
		t.Fatalf("no notifier warning expected with notify=false: %v", w)
	}
}

func TestInitScriptResultStore(t *testing.T) {
	restore := kvstore.SetTestStore(kvtest.NewMemoryStore())
	defer restore()
	NotifierProvider = func() *model.NotifierInfo {
		return &model.NotifierInfo{Status: model.NotifierStatusActive, Url: "https://n", Topic: "t", WriteToken: "w"}
	}
	defer func() { NotifierProvider = nil }()
	ns, infraId := "ns1", "infra-res"
	put := func(key string, v any) {
		b, _ := json.Marshal(v)
		if err := kvstore.Put(key, string(b)); err != nil {
			t.Fatal(err)
		}
	}
	put(common.GenInfraKey(ns, infraId, ""), model.InfraInfo{Id: infraId})
	put(common.GenInfraNodeGroupKey(ns, infraId, "g1"), model.NodeGroupInfo{Id: "g1", InitScript: &model.InitScriptReq{Mode: "default"}})
	put(common.GenInfraNodeGroupKey(ns, infraId, "g1-win"), model.NodeGroupInfo{Id: "g1-win", InitScript: &model.InitScriptReq{Mode: "none"}})
	for _, id := range []string{"g1-1", "g1-2", "g1-3", "g1-win-1"} {
		put(common.GenInfraKey(ns, infraId, id), model.CompactNodeInfo{Id: id})
	}
	if err := RecordInitScriptResult(ns, infraId, "missing", model.InitScriptResult{Status: "ok"}); err == nil {
		t.Fatal("a report for an unknown node must be rejected")
	}
	ok := model.InitScriptResult{Status: "ok", Steps: []model.InitScriptStepResult{{Name: "a", Status: "ok"}}}
	fail := model.InitScriptResult{Status: "fail", ExitCode: 7, Message: "boom", Steps: []model.InitScriptStepResult{{Name: "a", Status: "ok"}, {Name: "b", Status: "fail", ExitCode: 7}}}
	if err := RecordInitScriptResult(ns, infraId, "g1-1", ok); err != nil {
		t.Fatal(err)
	}
	if err := RecordInitScriptResult(ns, infraId, "g1-2", fail); err != nil {
		t.Fatal(err)
	}
	running := model.InitScriptResult{Event: "step", Status: "initializing", Step: "b", Steps: []model.InitScriptStepResult{{Name: "a", Status: "ok"}, {Name: "b", Status: "running"}, {Name: "c", Status: "pending"}}}
	put(common.GenInfraKey(ns, infraId, "g1-9"), model.CompactNodeInfo{Id: "g1-9"})
	if err := RecordInitScriptResult(ns, infraId, "g1-9", running); err != nil {
		t.Fatal(err)
	}
	if running.Summary() != "initializing: b (2/3)" {
		t.Fatalf("running summary = %q", running.Summary())
	}
	st, err := GetInfraInitScriptStatus(ns, infraId, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if st.Total != 5 || st.Reported != 3 || st.Ok != 1 || st.Fail != 1 || st.Initializing != 1 || st.Pending != 1 || st.NotApplicable != 1 || len(st.Nodes) != 5 {
		t.Fatalf("summary = %+v", st)
	}
	byId := map[string]model.NodeInitScriptStatus{}
	for _, n := range st.Nodes {
		byId[n.NodeId] = n
	}
	if byId["g1-2"].NodeGroupId != "g1" || byId["g1-win-1"].NodeGroupId != "g1-win" || byId["g1-win-1"].Status != "notApplicable" || byId["g1-3"].Status != "pending" {
		t.Fatalf("nodes = %+v", byId)
	}
	if byId["g1-2"].Summary() != "b exit 7: boom" {
		t.Fatalf("summary message = %q", byId["g1-2"].Summary())
	}
	only, _ := GetInfraInitScriptStatus(ns, infraId, "", "fail")
	if len(only.Nodes) != 1 || only.Nodes[0].NodeId != "g1-2" {
		t.Fatalf("status filter wrong: %+v", only.Nodes)
	}
	summary, _ := GetInfraInitScriptStatus(ns, infraId, "summary", "")
	if summary.Nodes != nil || summary.Ok != 1 {
		t.Fatalf("summary option must omit nodes: %+v", summary)
	}
	off := false
	nodes := []model.NodeInfo{{Id: "g1-1"}, {Id: "g1-2"}, {Id: "g1-3", InitScript: &model.InitScriptReq{Mode: "default"}},
		{Id: "g1-4", InitScript: &model.InitScriptReq{Mode: "default", Notify: &off}}, {Id: "g1-5"}}
	attachInitScriptResults(ns, infraId, nodes)
	if nodes[0].InitScriptStatus != "ok" || nodes[1].InitScriptMessage != "b exit 7: boom" || nodes[2].InitScriptStatus != "pending" ||
		nodes[3].InitScriptStatus != "" || nodes[4].InitScriptStatus != "" {
		t.Fatalf("attach wrong: %+v", nodes)
	}
	single := model.NodeInfo{Id: "g1-3", InitScript: &model.InitScriptReq{Mode: "default"}}
	attachInitScriptResult(ns, infraId, &single)
	if single.InitScriptStatus != "pending" {
		t.Fatalf("single read must be pending, got %q", single.InitScriptStatus)
	}
	// list view: results from the memory index, step names from the NodeGroup cache (only while not ok)
	sums := []model.NodeSummary{{Id: "g1-1", NodeGroupId: "g1"}, {Id: "g1-9", NodeGroupId: "g1"}, {Id: "g1-3", NodeGroupId: "g1"}, {Id: "g1-win-1", NodeGroupId: "g1-win"}}
	attachInitScriptSummaries(ns, infraId, sums)
	if sums[0].InitScriptStatus != "ok" || sums[0].InitScriptSteps != nil || sums[1].InitScriptStatus != "initializing" ||
		sums[1].InitScriptMessage != "initializing: b (2/3)" || sums[2].InitScriptStatus != "pending" || sums[3].InitScriptStatus != "" {
		t.Fatalf("list view attach wrong: %+v", sums)
	}
	deleteInitScriptResult(ns, infraId, "g1-2")
	if res, _ := listInitScriptResults(ns, infraId); len(res) != 2 {
		t.Fatalf("index must drop deleted results, got %d", len(res))
	}
	var one model.NodeInfo
	one.Id = "g1-2"
	attachInitScriptResult(ns, infraId, &one)
	if one.InitScriptStatus != "" {
		t.Fatal("deleted result must not be attached")
	}
	NotifierProvider = nil // no notifier: nothing can report, so no node is pending
	none := model.NodeInfo{Id: "g1-3", InitScript: &model.InitScriptReq{Mode: "default"}}
	attachInitScriptResult(ns, infraId, &none)
	if none.InitScriptStatus != "" {
		t.Fatalf("without a notifier the status must stay empty, got %q", none.InitScriptStatus)
	}
}
