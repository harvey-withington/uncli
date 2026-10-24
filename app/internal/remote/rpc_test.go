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
	"Bootstrap", "CLIChannels", "CLIStatus", "Commands", "Containers",
	"CreateSession", "Delete", "DeleteSafeEntry", "Editors", "ExplainCommand",
	"Focus", "HasDecisionKey", "ImportTranscript", "Interrupt", "KnownTools",
	"ListFolder", "MCPCallback", "MCPCancel", "MCPReconnect", "MCPSignIn",
	"MCPSignOut", "MarkSafe", "MoveSafeEntry", "Pages", "Pinned",
	"PreviewClasses", "ProviderChannels", "ProviderStatus", "ReadArtifact",
	"Rename", "SafeList", "Search", "Send", "SessionAllowlist", "SetBookmark",
	"SetMode", "SetModel", "SetPinned", "SetSafeEntry", "SetSafeLabel",
	"SetShares", "SetSortOrder", "SetUnattended", "SummarisePage", "Teach",
	"ToggleModifier", "Transcripts", "UploadLink", "Usage", "UsageReport",
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
		"InstallCLI", "InstallProvider", "SetCLIVersion", "SetProviderVersion", "SubmitLoginCode",
		"SignInTerminal", "CancelDeviceSignIn", "EnablePlugin", "DisablePlugin",
		"InstallWSL", "StopContainers", "BuildContainer", "SaveContainer", "RemoveContainer",
		"RemoveContainerConfig", "StartContainerSignIn", "SignOutContainers", "Theme",
		"ArtifactPath", "ClipboardFiles", "DescribePaths", "DescribeAttachments"} {
		if rpcNames[m] {
			t.Errorf("%s must not be reachable remotely", m)
		}
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
