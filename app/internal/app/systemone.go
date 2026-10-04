package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"uncli/internal/core"
)

// A decider on the System One wire (POST /v1/systemone): hosted Jev, or a
// self-hosted Kev, which serves the same contract. Written from Kev's
// README (github.com/jaredpalmer/kev), not yet tested against a live
// server: the request and response shapes below are the spec.
//
// Request:  {"model", "state", "questions": {id: {"type": "choice", "instructions", "criteria": {value: meaning}}}}
// Response: {"model", "answers": {id: {"type", "choice", "probabilities": {value: p}, "confidence"}}, "usage", "latency_ms"}

// DeciderSystemOne is a decider on the System One wire.
const DeciderSystemOne = "systemone"

// settingDecisionKey holds the decider's API key, apart from the
// preferences so it never travels to the UI.
const settingDecisionKey = "decisionModel.apiKey"

type systemOneDecider struct {
	endpoint string // the server's base URL, or the full /v1/systemone URL
	model    string
	version  string
	key      string
	client   *http.Client
}

func (d systemOneDecider) Info() core.DeciderInfo {
	return core.DeciderInfo{ID: DeciderSystemOne, Model: d.model, Version: d.version, Calibrated: true}
}

type soQuestion struct {
	Type         string            `json:"type"`
	Instructions string            `json:"instructions"`
	Criteria     map[string]string `json:"criteria"`
}

type soAnswer struct {
	Type          string             `json:"type"`
	Choice        string             `json:"choice"`
	Probabilities map[string]float64 `json:"probabilities"`
	Confidence    float64            `json:"confidence"`
}

type soResponse struct {
	Model     string              `json:"model"`
	Answers   map[string]soAnswer `json:"answers"`
	LatencyMS float64             `json:"latency_ms"`
}

// url is where requests go: the endpoint as given if it already names
// /v1/systemone, else that path under it.
func (d systemOneDecider) url() string {
	e := strings.TrimRight(d.endpoint, "/")
	if strings.HasSuffix(e, "/v1/systemone") {
		return e
	}
	return e + "/v1/systemone"
}

// Decide asks every question in one request. Each becomes a "choice"
// question whose criteria are the choices and what they mean; the
// decision's instructions lead each question's. A pure decider writes no
// prose, so Reason stays empty and the card says the risk in plain words.
func (d systemOneDecider) Decide(ctx context.Context, dec core.Decision) (core.Decided, error) {
	qs := map[string]soQuestion{}
	for _, q := range dec.Questions {
		criteria := map[string]string{}
		for _, c := range q.Choices {
			criteria[c.Value] = c.Meaning
		}
		qs[q.ID] = soQuestion{Type: "choice", Instructions: strings.TrimSpace(dec.Instructions + "\n\n" + q.Text), Criteria: criteria}
	}
	body, err := json.Marshal(map[string]any{"model": d.model, "state": dec.State, "questions": qs})
	if err != nil {
		return core.Decided{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.url(), bytes.NewReader(body))
	if err != nil {
		return core.Decided{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if d.key != "" {
		req.Header.Set("Authorization", "Bearer "+d.key)
	}
	client := d.client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return core.Decided{}, fmt.Errorf("the decision model didn't answer: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return core.Decided{}, fmt.Errorf("the decision model said %s: %s", resp.Status, clip(strings.TrimSpace(string(raw)), 300))
	}
	var got soResponse
	if err := json.Unmarshal(raw, &got); err != nil {
		return core.Decided{}, fmt.Errorf("the decision model's answer wasn't the expected JSON: %w", err)
	}
	out := core.Decided{Answers: map[string]core.Answer{}}
	for _, q := range dec.Questions {
		a, ok := got.Answers[q.ID]
		if !ok {
			return core.Decided{}, fmt.Errorf("the decision model didn't answer %q", q.ID)
		}
		choice := a.Choice
		if choice == "" { // only probabilities: take the likeliest
			for v, p := range a.Probabilities {
				if choice == "" || p > a.Probabilities[choice] {
					choice = v
				}
			}
		}
		if !hasChoice(q.Choices, choice) {
			return core.Decided{}, fmt.Errorf("the decision model gave an unknown answer %q to %q", choice, q.ID)
		}
		p, ok := a.Probabilities[choice]
		if !ok {
			p = a.Confidence
		}
		out.Answers[q.ID] = core.Answer{Choice: choice, Probability: p}
	}
	return out, nil
}

// localEndpoint reports whether a decider's address is on this computer or
// a private network (a LAN, or a tailnet), for the local-only setting.
// Names that need DNS to tell count as remote, except the usual local
// suffixes and single-label hosts.
func localEndpoint(endpoint string) bool {
	u, err := url.Parse(endpoint)
	if err != nil || u.Hostname() == "" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || tailnet.Contains(ip)
	}
	if host == "localhost" || !strings.Contains(host, ".") {
		return true
	}
	for _, suffix := range []string{".local", ".lan", ".internal", ".home.arpa", ".ts.net"} {
		if strings.HasSuffix(host, suffix) {
			return true
		}
	}
	return false
}

// Tailscale's address range (CGNAT, 100.64.0.0/10).
var tailnet = func() *net.IPNet { _, n, _ := net.ParseCIDR("100.64.0.0/10"); return n }()

// validSystemOne checks a System One decision model's settings.
func validSystemOne(m DecisionModel) error {
	u, err := url.Parse(m.Endpoint)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return errors.New("enter the decision model's address, such as http://localhost:8009")
	}
	if strings.TrimSpace(m.Model) == "" {
		return errors.New("enter the decision model's name, such as kev-latest or jev-1.13")
	}
	if m.LocalOnly && !localEndpoint(m.Endpoint) {
		return errors.New("that address isn't on this computer or a private network, and the decision model is set to local only")
	}
	return nil
}
