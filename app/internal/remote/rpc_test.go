package remote

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"testing"
)

// The methods a paired device may call. Adding one here is a deliberate
// widening of what the network can do: check it belongs (decision 0021).
var pinnedMethods = []string{
	"AnswerApproval", "AnswerAsk", "Archive", "ArtifactFiles", "ArtifactLink",
	"Bootstrap", "BuildContainer", "CLIChannels", "CLIStatus", "CancelSignIn", "CheckForUpdates", "Commands", "Containers",
	"CreateSession", "Delete", "DeleteSafeEntry", "Editors", "ExplainCommand", "ExplainPage", "ExplainSize",
	"Focus", "HasDecisionKey", "ImportContainer", "ImportTranscript", "InstallUpdate", "Interrupt", "KnownTools",
	"ListFolder", "MCPCallback", "MCPCancel", "MCPReconnect", "MCPSignIn",
	"MCPSignOut", "MCPToggle", "MarkSafe", "MoveSafeEntry", "PageChanges", "Pages", "Pinned",
	"PreviewClasses", "ProviderChannels", "ProviderStatus", "PushKey", "PushState", "PushSubscribe", "PushUnsubscribe", "Queue", "ReadArtifact",
	"Rename", "Restart", "SafeList", "Search", "Send", "SendQueued", "SendWith", "SessionAllowlist", "SetBookmark",
	"SetMode", "SetModel", "SetPinned", "SetPushNotifications", "SetResumeCutOff", "SetSafeEntry", "SetSafeLabel",
	"SetShares", "SetSortOrder", "SetUnattended", "SignIn", "SubmitLoginCode", "SuggestedContainers", "SummarisePage", "Teach", "TestPush",
	"ToggleModifier", "Transcripts", "Unqueue", "UpdateStatus", "UploadLink", "Usage", "UsageReport",
	"WatchFiles", "WatchRead",
}

func TestRPCMethods(t *testing.T) {
	if got := RPCMethods(); !slices.Equal(got, pinnedMethods) {
		t.Errorf("RPC methods changed:\n got %q\nwant %q", got, pinnedMethods)
	}
	ct := reflect.TypeOf(&client{})
	for _, m := range rpcMethods {
		if _, ok := ct.MethodByName(m); !ok {
			t.Errorf("%s is listed but the client has no such method", m)
		}
	}
	// What stays at the server's own screen.
	for _, m := range []string{"OpenFolder", "OpenFile", "OpenPath", "RevealFile", "RevealCLI",
		"SetPreferences", "SetDecisionKey", "TestDecisionModel", "TestNotification",
		"InstallCLI", "InstallProvider", "SetCLIVersion", "SetProviderVersion",
		"SignInTerminal", "CancelDeviceSignIn", "EnablePlugin", "DisablePlugin",
		"InstallWSL", "StopContainers", "SaveContainer", "RemoveContainer",
		"RemoveContainerConfig", "StartContainerSignIn", "SignOutContainers", "Theme",
		"ArtifactPath", "ClipboardFiles", "DescribePaths", "DescribeAttachments"} {
		if rpcNames[m] {
			t.Errorf("%s must not be reachable remotely", m)
		}
	}
}

// A host's own UNCLI offers its page what a device may call, less push and
// updates, plus setting up the CLIs; never what needs the server's screen.
func TestEmbeddedMethods(t *testing.T) {
	ct := reflect.TypeOf(&client{})
	got := EmbeddedMethods()
	for _, m := range got {
		if _, ok := ct.MethodByName(m); !ok {
			t.Errorf("%s is offered but the client has no such method", m)
		}
	}
	for _, m := range []string{"InstallCLI", "SignIn", "SubmitLoginCode", "StartDeviceSignIn", "Send", "AnswerApproval"} {
		if !slices.Contains(got, m) {
			t.Errorf("a host's page can't call %s", m)
		}
	}
	for _, m := range []string{"PushSubscribe", "TestPush", "InstallUpdate", "CheckForUpdates", "SignInTerminal", "OpenFolder", "SetPreferences", "BuildContainer"} {
		if slices.Contains(got, m) {
			t.Errorf("%s must not be offered to a host's page", m)
		}
	}
	// Devices still can't install the CLIs or sign in the others; they can
	// sign the first in again by link and code (decision 0034).
	for _, m := range setupMethods {
		again := m == "SignIn" || m == "SubmitLoginCode" || m == "CancelSignIn"
		if rpcNames[m] != again {
			t.Errorf("%s reachable from a device: %v, want %v", m, rpcNames[m], again)
		}
	}
	if n := len(got); n != len(slices.Compact(slices.Sorted(slices.Values(got)))) {
		t.Errorf("a host's page is offered a method twice: %q", got)
	}
}

type target struct{ calls []string }

func (t *target) Echo(s string, n int) (string, error) { return s + "!", nil }
func (t *target) Fail() error                          { return errors.New("it broke") }
func (t *target) Many() (int, string)                  { return 1, "a" }
func (t *target) Ctx(ctx context.Context, s string) string {
	if ctx == nil {
		return "no ctx"
	}
	return s
}
func (t *target) Boom()   { panic("boom") }
func (t *target) Secret() {}

func raw(vs ...any) []json.RawMessage {
	out := []json.RawMessage{}
	for _, v := range vs {
		b, _ := json.Marshal(v)
		out = append(out, b)
	}
	return out
}

func TestDispatch(t *testing.T) {
	names := map[string]bool{"Echo": true, "Fail": true, "Many": true, "Ctx": true, "Boom": true}
	tg := &target{}
	ctx := context.Background()
	if res, err := dispatch(ctx, tg, names, "Echo", raw("hi", 2)); err != nil || res != "hi!" {
		t.Errorf("Echo = %v, %v", res, err)
	}
	if _, err := dispatch(ctx, tg, names, "Echo", raw("hi")); err == nil || err.Code != errInvalidParams {
		t.Errorf("wrong count = %v", err)
	}
	if _, err := dispatch(ctx, tg, names, "Echo", raw(1, 2)); err == nil || err.Code != errInvalidParams {
		t.Errorf("wrong type = %v", err)
	}
	if _, err := dispatch(ctx, tg, names, "Fail", nil); err == nil || err.Code != errApp || err.Message != "it broke" {
		t.Errorf("Fail = %v", err)
	}
	if res, err := dispatch(ctx, tg, names, "Many", nil); err != nil || !reflect.DeepEqual(res, []any{1, "a"}) {
		t.Errorf("Many = %v, %v", res, err)
	}
	if res, err := dispatch(ctx, tg, names, "Ctx", raw("x")); err != nil || res != "x" {
		t.Errorf("Ctx = %v, %v", res, err)
	}
	if _, err := dispatch(ctx, tg, names, "Boom", nil); err == nil || err.Code != errInternal {
		t.Errorf("Boom = %v", err)
	}
	// Exported but not listed: as good as missing.
	if _, err := dispatch(ctx, tg, names, "Secret", nil); err == nil || err.Code != errMethodNotFound {
		t.Errorf("Secret = %v", err)
	}
}
