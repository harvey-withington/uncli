package session

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"uncli/internal/core"
)

// Risk: Ask mode asks only about what could do real harm, so the user
// never needs to know commands. Each tool use, or each working part of a
// command line, is one of:
//
//	looks    reads or shows information
//	routine  changes inside the session folder that can be undone or redone:
//	         edits, builds, tests, formatters, package scripts, local git
//	risky    could do real harm; Why says how, in a word the UI turns into a sentence
//	unknown  not recognised; the user's setting decides (unknownCommands)
//
// Most of it comes from tables of programs and subcommands, PowerShell
// verbs (Get- looks, Set- is routine, Remove- is risky) and a check of the
// paths a command names.
const (
	RiskLooks   = "looks"
	RiskRoutine = "routine"
	RiskRisky   = "risky"
	RiskUnknown = "unknown"
)

// Why something is risky.
const (
	WhyDeletes   = "deletes"   // deletes or overwrites for good
	WhyDiscards  = "discards"  // may throw away uncommitted changes
	WhyOutside   = "outside"   // changes files outside the session folder
	WhyPublishes = "publishes" // sends or publishes beyond this computer
	WhyInstalls  = "installs"  // installs software
	WhySystem    = "system"    // changes system settings or needs admin rights
	WhyStops     = "stops"     // stops programs or services
	WhyRemote    = "remote"    // connects to another computer
	WhySecrets   = "secrets"   // touches passwords, keys or other secrets
	WhyRunsCode  = "runs-code" // runs code it was handed
	WhyCloud     = "cloud"     // changes things in a cloud account
	WhyUnsure    = "unsure"    // the decision model wasn't sure enough it's safe
)

// Risk is the risk of one tool use or command part.
type Risk struct {
	Level string `json:"level"`
	Why   string `json:"why,omitempty"`
}

var (
	looks   = Risk{Level: RiskLooks}
	routine = Risk{Level: RiskRoutine}
	unknown = Risk{Level: RiskUnknown}
)

func risky(why string) Risk { return Risk{Level: RiskRisky, Why: why} }

// toolRisk is the risk of a tool use that isn't a shell command.
func toolRisk(a core.ToolAction, cls Class, at place) Risk {
	workdir := at.workdir
	switch {
	case asksUser(a):
		return looks // a question to the user: it always goes to them anyway
	case fileAction(a):
		p := absPath(a.Path, workdir)
		if p != "" && secretPath(p) {
			return risky(WhySecrets)
		}
		if !cls.Write {
			return looks
		}
		if p != "" && !inside(p, workdir) && !at.harmless(p) {
			return risky(WhyOutside)
		}
		return routine
	case cls.Source == SourceUnknown:
		return unknown
	case cls.Source == SourceServer && cls.Write && !cls.Destructive && !cls.OpenWorld:
		// The CLI leaves out hints that are false, so a server that said
		// nothing looks the same as one that said "writes, harmlessly":
		// treat it as not recognised.
		return unknown
	case !cls.Write:
		return looks
	case cls.Destructive:
		return risky(WhyDeletes)
	case cls.OpenWorld:
		return risky(WhyPublishes)
	}
	return routine
}

// commandRisk is the risk of one working part of a command line. piped
// says the part reads another part's output.
func commandRisk(p part, workdir string, piped bool) Risk {
	prog, ctx := partRisk(p, place{workdir: workdir})
	if ctx.Level == RiskRisky {
		return ctx
	}
	return prog
}

// partRisk is a command part's own risk (what the program does) and its
// context risk (where this use of it works).
func partRisk(p part, at place) (prog, ctx Risk) {
	prog = programRisk(p.words[0], p.words[1:], p.piped)
	if prog.Level == RiskLooks && len(p.writes) > 0 {
		prog = routine // output redirected into a file
	}
	return prog, contextRisk(p, at, prog)
}

// contextRisk is what makes this use of a command unsafe, whatever the
// command: it names secrets, code is piped (or a heredoc fed) into an
// interpreter, or it changes (or might change) things outside the session
// folder. The safe list can't override it (class.go).
func contextRisk(p part, at place, prog Risk) Risk {
	pr := pathRisk(p, at)
	switch {
	case pr.Why == WhySecrets:
		return pr
	case p.piped && interpreters[p.words[0]] && downloaders[p.from]:
		return risky(WhyRunsCode) // curl … | sh: code fetched from elsewhere
	case prog.Level != RiskLooks && pr.Level == RiskRisky:
		return pr
	}
	return Risk{}
}

var (
	buildTools = set("make", "cmake", "ninja", "tsc", "eslint", "prettier", "vitest", "jest", "mocha", "pytest", "mypy",
		"ruff", "black", "isort", "flake8", "pylint", "gofmt", "goimports", "golangci-lint", "staticcheck", "rustfmt",
		"gradle", "gradlew", "mvn", "msbuild", "wails", "vite", "webpack", "rollup", "esbuild", "svelte-check", "playwright",
		"tox", "nox", "bundle", "rake", "rspec", "phpunit", "composer", "swift", "xcodebuild", "flutter", "dart", "deno",
		"biome", "turbo", "nx", "lerna", "storybook", "tailwindcss", "sass", "dotnet-format", "npx", "pnpx", "bunx", "uvx",
		// Other stacks: JVM, Ruby, PHP, Elixir, Erlang, Haskell, Clojure, Swift/iOS, C/C++, Zig, Elm, Nim, monorepos.
		"sbt", "lein", "clj", "bundler", "mix", "rebar3", "stack", "cabal", "pod", "fastlane", "carthage", "meson",
		"bazel", "bazelisk", "buck2", "pants", "just", "mage", "zig", "elm", "nimble", "gcc", "g++", "clang", "clang++",
		"cl", "rustc", "javac", "kotlinc", "ctest", "cargo-nextest", "rush", "ng", "next", "nuxt",
		"astro", "expo", "eas", "tsx", "ts-node", "nodemon", "jasmine", "karma", "cypress", "pre-commit",
		"twine", "flit", "hatch", "pdm", "pipenv", "rye", "gem", "vsce", "ovsx", "electron-builder", "electron-forge")
	fileCommands = set("mkdir", "md", "touch", "cp", "copy", "mv", "move", "ren", "rename", "ln", "chmod", "chown",
		"new-item", "ni", "copy-item", "cpi", "move-item", "mi", "rename-item", "rni", "set-content", "add-content", "ac",
		"out-file", "expand-archive", "compress-archive", "tee-object", "tar", "zip", "unzip", "7z", "patch", "sed", "awk")
	packageManagers = set("npm", "pnpm", "yarn", "bun")
	systemCommands  = set("reg", "regedit", "sc", "schtasks", "setx", "bcdedit", "diskpart", "format", "shutdown",
		"restart-computer", "stop-computer", "set-executionpolicy", "netsh", "icacls", "takeown", "attrib", "crontab",
		"systemctl", "service", "launchctl", "mount", "umount", "chkdsk", "sfc", "dism")
	adminCommands = set("sudo", "su", "doas", "runas", "gsudo")
	osInstallers  = set("winget", "choco", "scoop", "brew", "apt", "apt-get", "yum", "dnf", "pacman", "snap", "msiexec",
		"install-module", "install-package", "install-script", "add-appxpackage")
	stopCommands   = set("kill", "taskkill", "pkill", "killall", "stop-process", "spps", "stop-service", "restart-service")
	remoteCommands = set("ssh", "scp", "sftp", "rsync", "ftp", "telnet", "nc", "ncat", "invoke-command", "enter-pssession")
	cloudCLIs      = set("kubectl", "helm", "terraform", "az", "aws", "gcloud", "pulumi", "vercel", "netlify", "fly", "heroku", "firebase", "wrangler")
	interpreters   = set("node", "python", "python3", "py", "ruby", "perl", "php", "pwsh", "powershell", "bash", "sh", "zsh", "cmd", "deno")
	codeRunners    = set("iex", "invoke-expression")
	envManagers    = set("conda", "mamba", "micromamba")
	downloaders    = set("curl", "wget", "invoke-webrequest", "iwr", "invoke-restmethod", "irm")
)

// Options that make a download send something instead.
var sendFlags = []string{"-d", "--data", "--data-raw", "--data-binary", "--data-urlencode", "-F", "--form", "-T",
	"--upload-file", "-X", "--request", "--post-data", "--post-file", "--method", "-Method", "-Body", "-InFile"}

// Options that make an interpreter run code given on the command line.
var inlineCode = []string{"-e", "--eval", "-c", "-p", "--print", "-Command", "-command", "-c", "-EncodedCommand", "-enc", "/c", "/k"}

// PowerShell verbs.
var (
	lookVerbs    = set("get", "test", "select", "measure", "format", "find", "show", "compare", "convertto", "convertfrom", "write", "resolve", "split", "join", "sort", "group", "where", "foreach", "out-string", "out-host", "trace", "search", "read", "wait", "start-sleep")
	routineVerbs = set("set", "new", "add", "copy", "move", "rename", "out", "export", "import", "expand", "compress", "convert", "push", "pop", "update", "start", "invoke-pester", "save", "merge", "checkpoint", "edit",
		"receive-job", "stop-job", "remove-job") // Claude's own background jobs
	verbWhy = map[string]string{
		"remove": WhyDeletes, "clear": WhyDeletes, "reset": WhyDeletes,
		"stop": WhyStops, "restart": WhyStops, "disable": WhyStops, "suspend": WhyStops,
		"install": WhyInstalls, "uninstall": WhyInstalls, "register": WhyInstalls, "unregister": WhyInstalls, "enable": WhySystem,
		"publish": WhyPublishes, "send": WhyPublishes, "connect": WhyRemote, "enter": WhyRemote, "grant": WhySystem, "revoke": WhySystem,
		"block": WhySystem, "unblock": WhySystem, "protect": WhySystem, "unprotect": WhySystem, "mount": WhySystem, "dismount": WhySystem,
	}
)

func programRisk(prog string, args []string, piped bool) Risk {
	sub := ""
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		sub = strings.ToLower(args[0])
	}
	switch {
	case prog == "git":
		return gitRisk(args)
	case codeRunners[prog]:
		return risky(WhyRunsCode)
	case toolchain(prog) && publishing(prog, args):
		// Any stack's build tool, package manager or task runner: what it
		// does in the folder is safe, but publishing or deploying isn't.
		return risky(WhyPublishes)
	case toolchain(prog) && installsSystemWide(prog, args):
		return risky(WhyInstalls)
	case interpreters[prog]:
		switch {
		case piped || len(args) == 0 || has(args, inlineCode...) || has(args, "-"):
			return unknown // code given inline or through a pipe: judged by reading it
		case (prog == "python" || prog == "python3" || prog == "py") && has(args, "-m") && hasWord(args, "pip"):
			return pipRisk(args)
		}
		if i := indexOf(args, "-m"); i >= 0 && publishing(prog, args[i+1:]) {
			return risky(WhyPublishes) // python -m twine upload
		}
		return routine // a script, a module, a version check
	case prog == "pip" || prog == "pip3" || prog == "pipx":
		return pipRisk(args)
	case prog == "uv":
		switch sub {
		case "pip":
			return pipRisk(args[1:])
		case "tool":
			if hasWord(args, "install", "upgrade") {
				return risky(WhyInstalls)
			}
		}
		return routine // the project's own environment: sync, add, run, lock
	case prog == "poetry" || prog == "pipenv" || prog == "pdm" || prog == "hatch" || prog == "rye" || prog == "flit":
		if sub == "self" {
			return risky(WhyInstalls)
		}
		return routine // the project's own environment
	case envManagers[prog]:
		if hasWord(args, "install", "create", "remove", "update", "uninstall") {
			return risky(WhyInstalls)
		}
		return looks
	case prog == "gem":
		if hasWord(args, "install", "uninstall", "update") {
			return risky(WhyInstalls)
		}
		return routine
	case lookCommands[prog], outputFilters[prog]:
		if prog == "find" && has(args, findActions...) {
			return risky(WhyDeletes)
		}
		if inPlace(prog, args) {
			return routine // edits its files: judged by where they are
		}
		return looks
	case deleteCommands[prog]:
		if !has(args, "-r", "-R", "-rf", "-fr", "-Recurse", "-recurse", "/s", "/S", "--recursive") && !wildcard(args) {
			return routine // single files
		}
		if targets := positional(args); len(targets) > 0 && (allGenerated(targets) || allTemp(targets)) {
			return routine // build output and caches: made again by the next build
		}
		return risky(WhyDeletes)
	case downloaders[prog]:
		if has(args, sendFlags...) {
			return risky(WhyPublishes)
		}
		return routine
	case prog == "gh":
		if sub == "" || sub == "status" || sub == "browse" || has(args[1:], "view", "list", "status", "diff", "checks") && !hasWord(args, "create", "merge", "close", "delete", "edit", "comment", "review") {
			return looks
		}
		return risky(WhyPublishes)
	case packageManagers[prog]:
		return packageRisk(sub, args)
	case prog == "go":
		if sub == "install" {
			return risky(WhyInstalls)
		}
		return routine
	case prog == "cargo":
		switch sub {
		case "install":
			return risky(WhyInstalls)
		case "publish", "yank", "owner", "login":
			return risky(WhyPublishes)
		}
		return routine
	case prog == "dotnet":
		second := ""
		if len(args) > 1 {
			second = strings.ToLower(args[1])
		}
		switch {
		case sub == "nuget" && hasWord(args, "push", "delete"):
			return risky(WhyPublishes)
		case sub == "nuget" && hasWord(args, "source") && hasWord(args, "add", "remove", "update", "enable", "disable") && !has(args, "--configfile"):
			return risky(WhyOutside) // the user's own NuGet.Config, not the project's
		case sub == "tool" && has(args, "-g", "--global"),
			sub == "workload" && second != "list" && second != "search",    // SDK-wide, often as admin
			sub == "new" && (second == "install" || second == "uninstall"): // templates for the whole account
			return risky(WhyInstalls)
		case sub == "dev-certs" && has(args, "--trust", "-t", "--clean", "-c"):
			return risky(WhySystem) // Windows' trusted certificates
		}
		return routine
	case prog == "docker" || prog == "podman":
		if has(args, "--push") || (sub == "compose" || sub == "buildx") && hasWord(args, "push") {
			return risky(WhyPublishes)
		}
		switch sub {
		case "push", "login":
			return risky(WhyPublishes)
		case "rm", "rmi", "prune", "kill":
			return risky(WhyDeletes)
		case "system", "volume", "network", "image", "container":
			if hasWord(args, "rm", "prune", "remove") {
				return risky(WhyDeletes)
			}
		}
		return routine
	case cloudCLIs[prog]:
		if hasWord(args, "get", "describe", "list", "show", "plan", "version", "status", "logs", "whoami") {
			return looks
		}
		return risky(WhyCloud)
	case buildTools[prog], fileCommands[prog]:
		return routine
	case adminCommands[prog]:
		return risky(WhySystem)
	case systemCommands[prog]:
		return risky(WhySystem)
	case osInstallers[prog]:
		return risky(WhyInstalls)
	case stopCommands[prog]:
		return risky(WhyStops)
	case remoteCommands[prog]:
		return risky(WhyRemote)
	case prog == "start-process":
		if has(args, "-Verb", "-verb") && hasWord(args, "RunAs", "runas") {
			return risky(WhySystem)
		}
		return unknown
	case prog == "set-itemproperty" || prog == "new-itemproperty" || prog == "remove-itemproperty":
		return risky(WhySystem) // the registry, mostly
	case prog == "cd" || prog == "chdir":
		return looks
	}
	return verbRisk(prog)
}

// verbRisk judges a PowerShell cmdlet by its verb.
func verbRisk(prog string) Risk {
	verb, _, ok := strings.Cut(prog, "-")
	if !ok {
		return unknown
	}
	switch {
	case lookVerbs[prog]: // a whole cmdlet name: Out-String
		return looks
	case routineVerbs[prog]: // a whole cmdlet name: Invoke-Pester
		return routine
	case lookVerbs[verb]:
		return looks
	case verb == "invoke":
		return unknown
	case routineVerbs[verb]:
		return routine
	}
	if why, ok := verbWhy[verb]; ok {
		return risky(why)
	}
	return unknown
}

func gitRisk(args []string) Risk {
	sub := ""
	if len(args) > 0 {
		sub = args[0]
	}
	switch gitClass(args) {
	case GitRead:
		return looks
	case GitLocal, GitCommit:
		return routine
	case GitPublish:
		return risky(WhyPublishes)
	case GitDestructive:
		if sub == "push" {
			return risky(WhyPublishes)
		}
		if sub == "restore" || sub == "checkout" || sub == "switch" || sub == "stash" {
			return risky(WhyDiscards)
		}
		return risky(WhyDeletes)
	}
	switch sub {
	case "clone", "init", "fetch", "pull", "lfs", "bisect", "blame", "archive", "bundle", "format-patch", "rerere", "maintenance", "fsck", "count-objects":
		return routine
	case "checkout":
		return risky(WhyDiscards) // a file restore throws away its changes
	}
	return unknown
}

// toolchain: a build tool, package manager or task runner, whatever the
// stack. Its work in the folder is safe; publishing isn't.
func toolchain(prog string) bool {
	return buildTools[prog] || packageManagers[prog] || prog == "go" || prog == "cargo" || prog == "dotnet" ||
		prog == "uv" || prog == "poetry" || prog == "pip" || prog == "pip3" || prog == "pipx" || prog == "deno"
}

// The words that name publishing or deploying, as a subcommand, task, goal
// or script: npm run deploy, mvn deploy, gradle :app:publish, mix
// hex.publish, twine upload, gem push, rake release.
var publishWords = []string{"publish", "deploy", "release", "upload", "unpublish", "yank"}

// Files a task word can't be: deploy.js and release.py are scripts, judged
// by what runs them.
var fileExts = set(".js", ".mjs", ".cjs", ".ts", ".mts", ".py", ".rb", ".sh", ".ps1", ".psm1", ".bat", ".cmd", ".go",
	".rs", ".java", ".kt", ".kts", ".php", ".pl", ".lua", ".exe", ".json", ".yaml", ".yml", ".toml", ".cfg", ".ini",
	".txt", ".md", ".csv", ".xml", ".gradle", ".mk", ".cs", ".csproj", ".sln", ".swift", ".dart", ".ex", ".exs")

// taskWord reports whether a word could name a subcommand, task or script
// rather than a file or folder.
func taskWord(w string) bool {
	return w != "" && !strings.HasPrefix(w, "-") && !strings.ContainsAny(w, "/\\~*=$") &&
		!fileExts[strings.ToLower(filepath.Ext(w))]
}

// publishing reports whether a toolchain command publishes or deploys: a
// task word made of a publish word (split at : and ., so build:release
// counts too, which errs towards prompting), or --publish, --deploy or
// --push. "release" as an option's value (-c release, --config Release) is
// a build configuration, and dotnet publish only builds locally.
func publishing(prog string, args []string) bool {
	// MSBuild deploys a web project's publish profile with DeployOnBuild,
	// whether run as msbuild or through dotnet build or publish.
	if prog == "dotnet" || prog == "msbuild" {
		for _, a := range args {
			if l := strings.ToLower(a); (strings.HasPrefix(l, "-p:") || strings.HasPrefix(l, "/p:") || strings.HasPrefix(l, "-property:") || strings.HasPrefix(l, "/property:")) &&
				strings.Contains(l, "deployonbuild=true") {
				return true
			}
		}
	}
	if prog == "dotnet" && len(args) > 0 && strings.EqualFold(args[0], "publish") {
		return false
	}
	for i, a := range args {
		l := strings.ToLower(a)
		if strings.HasPrefix(l, "-") {
			name, _, _ := strings.Cut(l, "=")
			if name == "--publish" || name == "--deploy" || name == "--push" {
				return true
			}
			continue
		}
		if !taskWord(l) {
			continue
		}
		for _, t := range strings.FieldsFunc(l, func(r rune) bool { return r == ':' || r == '.' }) {
			switch {
			case t == "release" && i > 0 && strings.HasPrefix(args[i-1], "-"):
				continue // -c release: a build configuration
			case strings.HasSuffix(t, "tomavenlocal"):
				continue // publishToMavenLocal stays on this computer
			case t == "push":
				return true
			}
			for _, w := range publishWords {
				if strings.HasPrefix(t, w) {
					return true
				}
			}
		}
	}
	return false
}

// installsSystemWide: make install, ninja install, meson install, cmake
// --install, and composer's global packages put things outside the project.
func installsSystemWide(prog string, args []string) bool {
	switch prog {
	case "make", "ninja", "meson", "just":
		return hasWord(positional(args), "install")
	case "cmake":
		return has(args, "--install")
	case "composer":
		return len(args) > 0 && strings.EqualFold(args[0], "global")
	}
	return false
}

func packageRisk(sub string, args []string) Risk {
	global := has(args, "-g", "--global", "--location=global")
	switch sub {
	case "publish", "unpublish", "deprecate", "owner", "access", "login", "adduser", "logout", "token", "dist-tag", "team":
		return risky(WhyPublishes)
	case "install", "i", "add", "ci", "uninstall", "un", "remove", "rm", "update", "up", "upgrade", "link":
		if global {
			return risky(WhyInstalls)
		}
	}
	return routine // scripts, tests, builds, local installs, information
}

func pipRisk(args []string) Risk {
	if hasWord(args, "install", "uninstall", "add", "remove", "sync", "inject") {
		return risky(WhyInstalls)
	}
	if hasWord(args, "publish", "upload") {
		return risky(WhyPublishes)
	}
	return looks
}

// Folders that builds and tools make again: deleting them loses nothing.
var generatedFolders = set("node_modules", "dist", "build", "out", "target", "coverage", ".next", ".nuxt", ".svelte-kit",
	".turbo", ".cache", "__pycache__", ".pytest_cache", ".mypy_cache", ".ruff_cache", ".parcel-cache", ".vite", "tmp",
	"temp", "bin", "obj", ".gradle", "storybook-static", "test-results", "playwright-report", ".angular",
	// Other stacks.
	".venv", "venv", ".tox", ".nox", ".eggs", "htmlcov", "vendor", "pods", ".dart_tool", "_build", "deps", ".build",
	"deriveddata", ".terraform", ".stack-work", "dist-newstyle", "elm-stuff", "zig-cache", ".zig-cache", "zig-out",
	".output", ".astro", ".docusaurus", ".expo", "bower_components", ".pnpm-store", ".bundle", ".sass-cache",
	".nyc_output")

// allTemp: every path is in the temp folder.
func allTemp(paths []string) bool {
	for _, p := range paths {
		if !harmlessPath(filepath.ToSlash(filepath.Clean(p))) {
			return false
		}
	}
	return true
}

func allGenerated(paths []string) bool {
	for _, p := range paths {
		p = strings.TrimRight(filepath.ToSlash(p), "/")
		if strings.Contains(p, "..") || filepath.IsAbs(p) || drivePath.MatchString(p) {
			return false
		}
		base := strings.ToLower(filepath.Base(p))
		if !generatedFolders[base] && !strings.HasPrefix(base, "cmake-build-") && !strings.HasSuffix(base, ".egg-info") {
			return false
		}
	}
	return true
}

func hasWord(args []string, words ...string) bool {
	for _, a := range args {
		for _, w := range words {
			if strings.EqualFold(a, w) {
				return true
			}
		}
	}
	return false
}

func wildcard(args []string) bool {
	for _, a := range args {
		if !strings.HasPrefix(a, "-") && strings.ContainsAny(a, "*?") {
			return true
		}
	}
	return false
}

var (
	drivePath   = regexp.MustCompile(`^[A-Za-z]:[\\/]`)
	secretParts = []string{".ssh", ".aws", ".gnupg", ".azure", ".kube", ".docker/config", "id_rsa", "id_ed25519", ".npmrc",
		".pypirc", ".netrc", ".git-credentials", "credentials", ".claude/.credentials", "secrets", ".password-store", "keychain"}
)

// Programs whose last argument is where they write; the others only read.
var copyCommands = set("cp", "copy", "copy-item", "cpi", "xcopy", "robocopy")

// pathRisk is risky when a part writes outside the session folder, or
// names a secret anywhere. For a copy only the destination is written;
// for other programs any path argument may be.
func pathRisk(p part, at place) Risk {
	workdir := at.workdir
	base := p.cwd
	if base == "" {
		base = workdir
	}
	args := p.words[1:]
	targets := append([]string{}, p.writes...)
	if copyCommands[p.words[0]] {
		if pos := positional(args); len(pos) > 1 {
			targets = append(targets, pos[len(pos)-1])
		}
		for i, a := range args {
			if strings.EqualFold(a, "-Destination") && i+1 < len(args) {
				targets = append(targets, args[i+1])
			}
		}
	} else {
		targets = append(targets, args...)
	}
	outside := false
	for _, w := range append(append([]string{}, args...), append(p.writes, p.reads...)...) {
		if path, ok := asPath(argValue(w), base); ok && secretPath(path) || envFile(argValue(w)) {
			return risky(WhySecrets)
		}
	}
	for _, w := range targets {
		path, ok := asPath(argValue(w), base)
		if ok && !inside(path, workdir) && !at.harmless(path) {
			outside = true
		}
	}
	if outside {
		return risky(WhyOutside)
	}
	return Risk{}
}

// argValue is the value of --opt=value, or the word itself.
func argValue(w string) string {
	if _, v, ok := strings.Cut(w, "="); ok && strings.HasPrefix(w, "-") {
		return v
	}
	return w
}

// Characters that mark a word as a pattern or code rather than a path.
const notPath = `^$*+?()[]{}|<>\`

// asPath reads a word as a file path, absolute, with forward slashes, when
// it plainly is one: a drive or root path, a home path, or one that climbs
// out with "..".
func asPath(w, base string) (string, bool) {
	home, _ := os.UserHomeDir()
	switch {
	case w == "" || strings.HasPrefix(w, "-") || strings.Contains(w, "://") || strings.HasPrefix(w, "//"):
		return "", false
	case gitBashDrive.MatchString(w): // /c/Users/… is C:/Users/…
		w = strings.ToUpper(w[1:2]) + ":" + w[2:]
	case drivePath.MatchString(w), strings.HasPrefix(w, `\\`):
	case strings.HasPrefix(w, "~"):
		w = filepath.Join(home, strings.TrimLeft(w[1:], `/\`))
	case w == "/" || strings.HasPrefix(w, "/") && strings.Count(w, "/") >= 2 && !strings.ContainsAny(w[1:], notPath+" "):
	case strings.HasPrefix(w, "$") || strings.HasPrefix(w, "%"):
		return "", false
	case strings.Contains(w, "..") && !strings.ContainsAny(w, notPath[:len(notPath)-1]+" "):
		w = filepath.Join(base, w)
	default:
		return "", false
	}
	if strings.ContainsAny(w, "*?") { // a glob: judge the folder it's in
		w = filepath.Dir(w)
	}
	return filepath.ToSlash(filepath.Clean(w)), true
}

var gitBashDrive = regexp.MustCompile(`^/[a-zA-Z]/`)

func inside(p, workdir string) bool {
	root := filepath.ToSlash(filepath.Clean(workdir))
	a, b := p, root
	if filepath.Separator == '\\' {
		a, b = strings.ToLower(a), strings.ToLower(b)
	}
	return a == b || strings.HasPrefix(a, strings.TrimSuffix(b, "/")+"/")
}

// place is where tool uses are judged: the session folder, and the
// folders the CLI keeps its own files in (core.StatePather).
type place struct {
	workdir string
	state   []string
}

// harmless: outside the session folder, but somewhere writing does no
// harm: temp folders, null devices, the CLI's own state folders.
func (at place) harmless(p string) bool {
	if harmlessPath(p) {
		return true
	}
	for _, s := range at.state {
		if inside(p, s) {
			return true
		}
	}
	return false
}

// harmlessPath: places a command may name without risk (null devices,
// temp folders).
func harmlessPath(p string) bool {
	if gitBashDrive.MatchString(p) { // /c/Users/… is C:/Users/…
		p = p[1:2] + ":" + p[2:]
	}
	l := strings.ToLower(p)
	tmp := strings.ToLower(filepath.ToSlash(os.TempDir()))
	return l == "/dev/null" || l == "nul" || strings.HasPrefix(l, "/tmp/") || tmp != "" && strings.HasPrefix(l, tmp+"/")
}

// envFile: a .env file (.env, .env.local, prod.env…), which holds a
// project's secrets; the examples checked in beside it don't.
func envFile(p string) bool {
	base := strings.ToLower(filepath.Base(filepath.ToSlash(p)))
	if !strings.HasPrefix(base, ".env") && !strings.HasSuffix(base, ".env") || base == ".envrc" {
		return false
	}
	for _, s := range []string{"example", "sample", "template", "dist", "defaults"} {
		if strings.Contains(base, s) {
			return false
		}
	}
	return base == ".env" || strings.HasPrefix(base, ".env.") || strings.HasSuffix(base, ".env")
}

func secretPath(p string) bool {
	if envFile(p) {
		return true
	}
	l := strings.ToLower(p)
	for _, s := range secretParts {
		if strings.Contains(l, s) {
			return true
		}
	}
	return false
}
