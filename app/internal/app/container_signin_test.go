package app

import "testing"

// How setup-token draws its screen (CLI 2.1.285): the link in an OSC 8
// hyperlink, words separated by cursor moves, a reset straight after the
// token, then the CLI exits.
const esc = "\x1b"

func TestSignInURLFromTheHyperlink(t *testing.T) {
	raw := "Browser didn't open? Use the url below" + esc + "]8;id=1;https://claude.com/cai/oauth/authorize?code=true&client_id=x&state=abc" + esc + `\` +
		"https://claude.com/cai/oauth/authori\r\nze?code=true" + esc + "]8;;" + esc + `\`
	if u := signInURL(raw); u != "https://claude.com/cai/oauth/authorize?code=true&client_id=x&state=abc" {
		t.Errorf("url = %q", u)
	}
}

func TestTokenIsTakenWhole(t *testing.T) {
	tok := "sk-ant-oat01-AbCdEf_123-xyzXYZ0987654321abcdefABCDEF-QQ"
	screen := "Your OAuth token (valid for 1 year):" + esc + "[1B" + esc + "[2G" + tok + esc + "[39m" + esc + "[1CStore" + esc + "[1Cthis" + esc + "[1Ctoken"
	if got, ok := tokenIn(screen, false); !ok || got != tok {
		t.Errorf("token = %q, %v", got, ok)
	}
	// Still arriving: not yet.
	if _, ok := tokenIn("token:"+esc+"[2G"+tok[:30], false); ok {
		t.Error("a token still arriving was taken")
	}
	// At the very end once the CLI has exited.
	if got, ok := tokenIn("token: "+tok+"\r\n", true); !ok || got != tok {
		t.Errorf("at exit = %q, %v", got, ok)
	}
}
