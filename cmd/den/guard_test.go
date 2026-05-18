package main

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/us/den/internal/config"
	"github.com/us/den/internal/runtime/docker"
)

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// --- guard-ordering seam -----------------------------------------------------
//
// The one safe order is Ping → guards → Reconcile → EnsureNetwork. A guard
// refusal MUST short-circuit before Reconcile/EnsureNetwork so a refused
// posture never mutates the managed network. These tests pin that with spies
// instead of a live daemon.

func TestRunStartup_OrderHappyPath(t *testing.T) {
	var order []string
	step := func(name string) func(context.Context) error {
		return func(context.Context) error { order = append(order, name); return nil }
	}
	err := runStartup(context.Background(), startupSteps{
		ping:      step("ping"),
		guard:     step("guard"),
		reconcile: step("reconcile"),
		ensureNet: step("ensureNet"),
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"ping", "guard", "reconcile", "ensureNet"}, order,
		"startup must run in exactly this order")
}

func TestRunStartup_GuardRefusalAbortsBeforeMutation(t *testing.T) {
	var order []string
	step := func(name string) func(context.Context) error {
		return func(context.Context) error { order = append(order, name); return nil }
	}
	sentinel := errors.New("refusing to start: bind guard")
	err := runStartup(context.Background(), startupSteps{
		ping:      step("ping"),
		guard:     func(context.Context) error { order = append(order, "guard"); return sentinel },
		reconcile: step("reconcile"),
		ensureNet: step("ensureNet"),
	})
	require.ErrorIs(t, err, sentinel, "guard error must propagate unwrapped")
	assert.Equal(t, []string{"ping", "guard"}, order,
		"a guard refusal must abort BEFORE Reconcile/EnsureNetwork")
}

func TestRunStartup_PingFailureAbortsBeforeGuard(t *testing.T) {
	var order []string
	step := func(name string) func(context.Context) error {
		return func(context.Context) error { order = append(order, name); return nil }
	}
	err := runStartup(context.Background(), startupSteps{
		ping:      func(context.Context) error { order = append(order, "ping"); return errors.New("no daemon") },
		guard:     step("guard"),
		reconcile: step("reconcile"),
		ensureNet: step("ensureNet"),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "docker not available")
	assert.Equal(t, []string{"ping"}, order, "a ping failure must abort before the guard runs")
}

// --- bridge-refusal runs in MCP mode (httpListener=false) --------------------
//
// MCP is stdio-only so the bind guard is a no-op, but a bridge sandbox has
// unfiltered egress regardless of whether den exposes an HTTP API. The
// bridge-refusal guard MUST still fire. docker.New() is lazy (no daemon
// contacted) and the bridge check returns before rt is ever touched.

func TestApplyNetworkGuards_BridgeRefusalRunsInMCP(t *testing.T) {
	rt, err := docker.New()
	require.NoError(t, err)

	cfg := config.DefaultConfig()
	cfg.Runtime.DefaultNetworkMode = "bridge"
	cfg.Runtime.AllowUnsafeBridge = false
	// httpListener=false ⇒ MCP stdio mode.
	err = applyNetworkGuards(context.Background(), rt, cfg, quietLogger(), false)
	require.Error(t, err, "bridge without allow_unsafe_bridge must refuse even in MCP mode")
	assert.Contains(t, err.Error(), "bridge")

	// Opt-in flips it to allowed; the bridge guard no longer refuses.
	cfg.Runtime.AllowUnsafeBridge = true
	require.NoError(t, applyNetworkGuards(context.Background(), rt, cfg, quietLogger(), false))
}

func TestApplyNetworkGuards_BindGuardIsNoOpInMCP(t *testing.T) {
	rt, err := docker.New()
	require.NoError(t, err)

	// Auth off + loopback bind + internal default would REFUSE under an HTTP
	// listener; in MCP stdio mode the bind guard must NOT run, so this starts.
	cfg := config.DefaultConfig()
	cfg.Auth.Enabled = false
	cfg.Server.Host = "127.0.0.1"
	cfg.Runtime.DefaultNetworkMode = "internal"
	require.NoError(t, applyNetworkGuards(context.Background(), rt, cfg, quietLogger(), false),
		"the bind guard must be a no-op in MCP stdio mode")
}

// --- MCP is stdio-only: structural no-network-listener invariant -------------
//
// mcp.go must never open a network listener. This is enforced structurally
// (go/parser) rather than by hoping a reviewer notices, plus a comment
// tripwire so deleting the rationale also fails the build.

func TestMCP_IsStdioOnly_Structural(t *testing.T) {
	src, err := os.ReadFile("mcp.go")
	require.NoError(t, err)

	// Tripwire: the SECURITY-INVARIANT rationale must stay in the file.
	assert.Contains(t, string(src), "SECURITY-INVARIANT: mcp mode is stdio-only",
		"the no-listener rationale comment was removed from mcp.go")

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "mcp.go", src, parser.ParseComments)
	require.NoError(t, err)

	// (1) mcp.go must not even import a network-server package.
	for _, imp := range f.Imports {
		path, _ := strconv.Unquote(imp.Path.Value)
		switch path {
		case "net", "net/http", "net/http/httptest":
			t.Fatalf("mcp.go must not import %q — MCP is stdio-only", path)
		}
	}

	// (2) Defense-in-depth: no listener/server selector even via a dot-aliased
	// or transitively reachable identifier.
	netForbidden := map[string]bool{
		"ListenAndServe": true, "ListenAndServeTLS": true,
		"Serve": true, "ServeTLS": true, "Server": true, "FileServer": true,
	}
	ast.Inspect(f, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		switch pkg.Name {
		case "net":
			if strings.HasPrefix(sel.Sel.Name, "Listen") {
				t.Fatalf("mcp.go references net.%s — MCP must not open a listener", sel.Sel.Name)
			}
		case "http":
			if netForbidden[sel.Sel.Name] {
				t.Fatalf("mcp.go references http.%s — MCP must not run an HTTP server", sel.Sel.Name)
			}
		case "httptest":
			t.Fatalf("mcp.go references httptest.%s — MCP must not open a listener", sel.Sel.Name)
		}
		return true
	})
}
