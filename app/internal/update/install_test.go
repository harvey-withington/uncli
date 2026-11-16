package update

import "testing"

func TestScriptMessage(t *testing.T) {
	cases := []struct{ name, out, want string }{
		{
			name: "a PowerShell error loses its position and category lines",
			out: "Register-ScheduledTask : Access is denied.\r\n" +
				"At C:\\Users\\someone\\AppData\\Local\\UNCLI\\update\\install-build.ps1:167 char:1\r\n" +
				"+ Register-ScheduledTask -TaskName $TaskName -Action $action -Principal ...\r\n" +
				"+ ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~\r\n" +
				"    + CategoryInfo          : PermissionDenied: (PS_ScheduledTask:Root/Microsoft/...S_ScheduledTask) [Register-ScheduledTask], CimException\r\n" +
				"    + FullyQualifiedErrorId : HRESULT 0x80070005,Register-ScheduledTask\r\n",
			want: "Register-ScheduledTask : Access is denied.",
		},
		{
			name: "a message over several lines is joined",
			out:  "No installer at C:\\x.exe.\nBuild one first.\nAt line:1 char:1\n+ throw\n",
			want: "No installer at C:\\x.exe. Build one first.",
		},
		{
			name: "plain output is kept",
			out:  "something went wrong\n",
			want: "something went wrong",
		},
		{name: "nothing", out: "  \r\n", want: ""},
	}
	for _, c := range cases {
		if got := scriptMessage(c.out); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
