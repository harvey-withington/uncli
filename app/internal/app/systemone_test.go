package app

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"uncli/internal/core"
	"uncli/internal/session"
)

// A fake System One server, answering as Kev's README says Kev does.
func fakeSystemOne(t *testing.T, answer func(req map[string]any) (int, string)) (*httptest.Server, *[]map[string]any, *[]http.Header) {
	t.Helper()
	var reqs []map[string]any
	var headers []http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/systemone" {
			http.NotFound(w, r)
			return
		}
		b, _ := io.ReadAll(r.Body)
		var req map[string]any
		if err := json.Unmarshal(b, &req); err != nil {
			t.Errorf("request isn't JSON: %s", b)
		}
		reqs = append(reqs, req)
		headers = append(headers, r.Header.Clone())
		code, body := answer(req)
		w.WriteHeader(code)
		io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv, &reqs, &headers
}

const kevAnswer = `{"model":"kev-0.8b","answers":{
	"level":{"type":"choice","choice":"routine","probabilities":{"looks":0.02,"routine":0.95,"risky":0.03},"confidence":0.95},
	"risk":{"type":"choice","choice":"none","probabilities":{"none":0.9,"deletes":0.1},"confidence":0.9}},
	"usage":{"input_tokens":101,"output_tokens":0},"latency_ms":95}`

func TestSystemOneDecider(t *testing.T) {
	srv, reqs, headers := fakeSystemOne(t, func(map[string]any) (int, string) { return 200, kevAnswer })
	d := systemOneDecider{endpoint: srv.URL + "/", model: "kev-0.8b", version: "0.1", key: "sk-test"}
	if info := d.Info(); !info.Calibrated || info.Key() != "systemone/kev-0.8b@0.1" {
		t.Errorf("info = %+v", info)
	}
	dec := core.Decision{Instructions: judgeSystem, State: "The assistant wants to run: npm test", Questions: judgeQuestions, Explain: judgeExplain}
	got, err := d.Decide(context.Background(), dec)
	if err != nil {
		t.Fatal(err)
	}
	if got.Answers["level"] != (core.Answer{Choice: "routine", Probability: 0.95}) || got.Answers["risk"].Choice != "none" || got.Reason != "" {
		t.Errorf("decided = %+v", got)
	}
	// The request: the model, the state, and each question as a choice with
	// its choices as criteria.
	req := (*reqs)[0]
	qs, _ := req["questions"].(map[string]any)
	level, _ := qs["level"].(map[string]any)
	criteria, _ := level["criteria"].(map[string]any)
	if req["model"] != "kev-0.8b" || req["state"] != dec.State || level["type"] != "choice" || len(criteria) != 3 ||
		!strings.Contains(level["instructions"].(string), "What does it do?") {
		t.Errorf("request = %v", req)
	}
	if h := (*headers)[0]; h.Get("Authorization") != "Bearer sk-test" || h.Get("Content-Type") != "application/json" {
		t.Errorf("headers = %v", h)
	}

	// The full path works as the endpoint too, and no key sends no header.
	d.endpoint, d.key = srv.URL+"/v1/systemone", ""
	if _, err := d.Decide(context.Background(), dec); err != nil || (*headers)[1].Get("Authorization") != "" {
		t.Errorf("full path: %v, %v", err, (*headers)[1])
	}

	// With the judgement on top: sure enough runs, a grey zone prompts.
	j, _ := judgementFrom(d.Info(), got, DefaultThreshold)
	if j.Level != session.RiskRoutine || j.Confidence != 0.95 {
		t.Errorf("judgement = %+v", j)
	}
}

func TestSystemOneErrors(t *testing.T) {
	dec := core.Decision{State: "x", Questions: judgeQuestions}
	cases := map[string]struct {
		code int
		body string
		want string
	}{
		"server error":   {500, `{"error":"out of memory"}`, "500"},
		"not JSON":       {200, `<html>`, "expected JSON"},
		"missing answer": {200, `{"answers":{"level":{"choice":"routine","probabilities":{"routine":0.9}}}}`, `"risk"`},
		"unknown choice": {200, `{"answers":{"level":{"choice":"fine"},"risk":{"choice":"none"}}}`, "unknown answer"},
	}
	for name, c := range cases {
		srv, _, _ := fakeSystemOne(t, func(map[string]any) (int, string) { return c.code, c.body })
		_, err := systemOneDecider{endpoint: srv.URL, model: "kev"}.Decide(context.Background(), dec)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v", name, err)
		}
	}
	// Only probabilities, no choice: the likeliest one.
	srv, _, _ := fakeSystemOne(t, func(map[string]any) (int, string) {
		return 200, `{"answers":{"level":{"probabilities":{"looks":0.1,"risky":0.7,"routine":0.2}},"risk":{"probabilities":{"none":0.2,"deletes":0.8}}}}`
	})
	got, err := systemOneDecider{endpoint: srv.URL, model: "kev"}.Decide(context.Background(), dec)
	if err != nil || got.Answers["level"] != (core.Answer{Choice: "risky", Probability: 0.7}) || got.Answers["risk"].Choice != "deletes" {
		t.Errorf("likeliest = %+v, %v", got, err)
	}
}

func TestLocalEndpoint(t *testing.T) {
	for endpoint, want := range map[string]bool{
		"http://localhost:8009":       true,
		"http://127.0.0.1:8009":       true,
		"http://192.168.1.20:8009":    true,
		"http://100.101.102.103:8009": true, // a tailnet address
		"http://kevbox:8009":          true,
		"http://kev.tail1234.ts.net":  true,
		"http://nas.local:8009":       true,
		"https://api.typesafe.ai":     false,
		"https://8.8.8.8":             false,
		"not a url":                   false,
	} {
		if got := localEndpoint(endpoint); got != want {
			t.Errorf("%s: %v, want %v", endpoint, got, want)
		}
	}
}

// Choosing a System One decision model, its key, and the sample test.
func TestSystemOnePreference(t *testing.T) {
	dir := t.TempDir()
	svc, err := New(Paths{Config: filepath.Join(dir, "c"), Cache: filepath.Join(dir, "k"), Scratch: filepath.Join(dir, "s")}, nopEmitter{})
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	srv, _, headers := fakeSystemOne(t, func(map[string]any) (int, string) { return 200, kevAnswer })
	p := svc.Preferences()
	for _, bad := range []DecisionModel{
		{Provider: DeciderSystemOne, Model: "kev"},                                                       // no address
		{Provider: DeciderSystemOne, Endpoint: srv.URL},                                                  // no model
		{Provider: DeciderSystemOne, Endpoint: "https://api.typesafe.ai", Model: "jev", LocalOnly: true}, // not local
	} {
		p.DecisionModel = bad
		if _, err := svc.SetPreferences(p); err == nil {
			t.Errorf("%+v must be refused", bad)
		}
	}
	p.DecisionModel = DecisionModel{Provider: DeciderSystemOne, Endpoint: srv.URL, Model: "kev-0.8b", Version: "0.1", LocalOnly: true}
	if _, err := svc.SetPreferences(p); err != nil {
		t.Fatal(err)
	}
	if svc.HasDecisionKey() {
		t.Error("no key yet")
	}
	if err := svc.SetDecisionKey(" sk-local "); err != nil || !svc.HasDecisionKey() {
		t.Errorf("key saved: %v", err)
	}
	if k := svc.deciderKey(); k != "systemone/kev-0.8b@0.1" {
		t.Errorf("decider key = %q", k)
	}
	// The key never travels with the preferences.
	if b, _ := json.Marshal(svc.Preferences()); strings.Contains(string(b), "sk-local") {
		t.Errorf("preferences carry the key: %s", b)
	}
	res, err := svc.TestDecisionModel(context.Background())
	if err != nil || res.Level != session.RiskRoutine || res.Verdict != session.RiskRoutine || res.Confidence != 0.95 || !res.Calibrated {
		t.Errorf("test = %+v, %v", res, err)
	}
	if (*headers)[0].Get("Authorization") != "Bearer sk-local" {
		t.Errorf("the saved key wasn't sent: %v", (*headers)[0])
	}
}
