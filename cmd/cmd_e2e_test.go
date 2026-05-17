package cmd

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// useTempDB points wedevctl at a fresh temp-file database for the test.
func useTempDB(t *testing.T) {
	t.Helper()
	t.Setenv("WEDEVCTL_DB_PATH", t.TempDir())
}

// runCLI executes the root command with the given args. If stdin is non-empty
// it is fed to os.Stdin (for confirmation prompts). Stdout is captured and
// returned alongside the execution error.
func runCLI(t *testing.T, stdin string, args ...string) (string, error) {
	t.Helper()

	if stdin != "" {
		origStdin := os.Stdin
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatalf("os.Pipe() error = %v", err)
		}
		os.Stdin = r
		defer func() {
			os.Stdin = origStdin
			r.Close()
		}()
		if _, err := w.WriteString(stdin); err != nil {
			t.Fatalf("write to stdin pipe error = %v", err)
		}
		w.Close()
	}

	origStdout := os.Stdout
	tmp, err := os.CreateTemp(t.TempDir(), "stdout-*")
	if err != nil {
		t.Fatalf("os.CreateTemp() error = %v", err)
	}
	os.Stdout = tmp

	root := NewRootCommand()
	root.SetArgs(args)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	execErr := root.Execute()

	os.Stdout = origStdout
	tmp.Close()
	data, _ := os.ReadFile(tmp.Name())
	return string(data), execErr
}

// TestCLIFullFlow walks a complete lifecycle through the CLI against one DB.
func TestCLIFullFlow(t *testing.T) {
	useTempDB(t)
	outDir := t.TempDir()

	steps := []struct {
		name     string
		stdin    string
		args     []string
		wantErr  bool
		contains string
	}{
		{"vn help", "", []string{"vn"}, false, ""},
		{"vn add", "y\n", []string{"vn", "add", "testnet", "10.0.0.0/24"}, false, "created successfully"},
		{"vn list", "", []string{"vn", "list"}, false, "testnet"},
		{"server add", "", []string{"vn", "testnet", "server", "add", "srv1", "vpn.example.com", "51820"}, false, "created successfully"},
		{"server info", "", []string{"vn", "testnet", "server", "info"}, false, "srv1"},
		{"server edit", "", []string{"vn", "testnet", "server", "edit", "--port", "51821"}, false, "updated successfully"},
		{"node add peer", "", []string{"vn", "testnet", "node", "add", "peerA", "peer", "1.2.3.4"}, false, "created successfully"},
		{"node add route", "", []string{"vn", "testnet", "node", "add", "routeA", "route"}, false, "created successfully"},
		{"node list", "", []string{"vn", "testnet", "node", "list"}, false, "peerA"},
		{"node edit", "", []string{"vn", "testnet", "node", "edit", "peerA", "--port", "51830"}, false, "updated successfully"},
		{"config generate", "", []string{"vn", "testnet", "config", "generate", "--output-dir", outDir, "--force"}, false, "version 1 saved"},
		{"config info", "", []string{"vn", "testnet", "config", "info"}, false, "Configuration Version: 1"},
		{"config info v1", "", []string{"vn", "testnet", "config", "info", "1"}, false, "Content Hash:"},
		{"config history", "", []string{"vn", "testnet", "config", "history"}, false, "Version"},
		{"node delete", "y\n", []string{"vn", "testnet", "node", "delete", "peerA"}, false, "deleted successfully"},
		{"server delete", "y\n", []string{"vn", "testnet", "server", "delete"}, false, "deleted successfully"},
		{"vn delete", "y\n", []string{"vn", "delete", "testnet"}, false, "deleted successfully"},
	}

	for _, s := range steps {
		out, err := runCLI(t, s.stdin, s.args...)
		if (err != nil) != s.wantErr {
			t.Errorf("[%s] error = %v, wantErr %v (out: %s)", s.name, err, s.wantErr, out)
			continue
		}
		if s.contains != "" && !strings.Contains(out, s.contains) {
			t.Errorf("[%s] output %q does not contain %q", s.name, out, s.contains)
		}
	}

	// The generated server config file should exist on disk.
	if _, err := os.Stat(filepath.Join(outDir, "srv1.conf")); err != nil {
		t.Errorf("expected generated config file srv1.conf: %v", err)
	}
}

func TestCLICancelledConfirmations(t *testing.T) {
	useTempDB(t)

	// Cancelled network creation: no error, prints "Cancelled".
	out, err := runCLI(t, "n\n", "vn", "add", "cancelnet", "10.0.0.0/24")
	if err != nil {
		t.Errorf("cancelled vn add should not error: %v", err)
	}
	if !strings.Contains(out, "Cancelled") {
		t.Errorf("expected 'Cancelled' in output, got %q", out)
	}

	// The network must not have been created.
	out, _ = runCLI(t, "", "vn", "list")
	if strings.Contains(out, "cancelnet") {
		t.Error("cancelled network should not appear in 'vn list'")
	}

	// Create one, then cancel its deletion.
	if _, err := runCLI(t, "y\n", "vn", "add", "keepnet", "10.0.0.0/24"); err != nil {
		t.Fatalf("vn add keepnet error = %v", err)
	}
	if _, err := runCLI(t, "n\n", "vn", "delete", "keepnet"); err != nil {
		t.Errorf("cancelled vn delete should not error: %v", err)
	}
	out, _ = runCLI(t, "", "vn", "list")
	if !strings.Contains(out, "keepnet") {
		t.Error("network should still exist after cancelled delete")
	}
}

func TestCLIErrorCases(t *testing.T) {
	useTempDB(t)

	// Seed a network + server for the node error cases.
	if _, err := runCLI(t, "y\n", "vn", "add", "enet", "10.0.0.0/24"); err != nil {
		t.Fatalf("seed vn add error = %v", err)
	}
	if _, err := runCLI(t, "", "vn", "enet", "server", "add", "srv", "vpn.example.com"); err != nil {
		t.Fatalf("seed server add error = %v", err)
	}

	cases := []struct {
		name string
		args []string
	}{
		{"vn add missing args", []string{"vn", "add", "onlyname"}},
		{"vn add duplicate", []string{"vn", "add", "enet", "10.9.0.0/24"}},
		{"vn add invalid cidr", []string{"vn", "add", "badcidr", "not-a-cidr"}},
		{"unknown network", []string{"vn", "ghostnet", "server", "info"}},
		{"node invalid type", []string{"vn", "enet", "node", "add", "n1", "bogus", "1.2.3.4"}},
		{"peer without address", []string{"vn", "enet", "node", "add", "n2", "peer"}},
		{"node invalid port", []string{"vn", "enet", "node", "add", "n3", "peer", "1.2.3.4", "notaport"}},
		{"server invalid port", []string{"vn", "enet", "server", "add", "s2", "vpn.example.com", "notaport"}},
		{"server edit no flags", []string{"vn", "enet", "server", "edit"}},
		{"config info bad version", []string{"vn", "enet", "config", "info", "999"}},
		{"node delete missing", []string{"vn", "enet", "node", "delete", "doesnotexist"}},
	}

	for _, c := range cases {
		stdin := ""
		// Commands that reach a confirmation prompt need an answer.
		if c.name == "vn add duplicate" || c.name == "vn add invalid cidr" || c.name == "node delete missing" {
			stdin = "y\n"
		}
		_, err := runCLI(t, stdin, c.args...)
		if err == nil {
			t.Errorf("[%s] expected an error, got nil", c.name)
		}
	}
}

func TestCLINodeAndServerEditBranches(t *testing.T) {
	useTempDB(t)
	if _, err := runCLI(t, "y\n", "vn", "add", "ed", "10.0.0.0/24"); err != nil {
		t.Fatalf("vn add error = %v", err)
	}
	if _, err := runCLI(t, "", "vn", "ed", "server", "add", "srv", "vpn.example.com"); err != nil {
		t.Fatalf("server add error = %v", err)
	}
	if _, err := runCLI(t, "", "vn", "ed", "node", "add", "p1", "peer", "1.2.3.4", "51820"); err != nil {
		t.Fatalf("node add p1 error = %v", err)
	}
	if _, err := runCLI(t, "", "vn", "ed", "node", "add", "r1", "route"); err != nil {
		t.Fatalf("node add r1 error = %v", err)
	}

	// Server edit by public address.
	if out, err := runCLI(t, "", "vn", "ed", "server", "edit", "--public-address", "new.example.com"); err != nil {
		t.Errorf("server edit --public-address error = %v (out: %s)", err, out)
	}

	// Promote a route node to peer (type change requires an address).
	if out, err := runCLI(t, "", "vn", "ed", "node", "edit", "r1", "--type", "peer", "--public-address", "5.6.7.8"); err != nil {
		t.Errorf("node edit r1 ->peer error = %v (out: %s)", err, out)
	}
	// Promoting to peer without an address must fail.
	if _, err := runCLI(t, "", "vn", "ed", "node", "edit", "p1", "--type", "route"); err != nil {
		t.Errorf("node edit p1 ->route error = %v", err)
	}
	// Invalid type is rejected.
	if _, err := runCLI(t, "", "vn", "ed", "node", "edit", "p1", "--type", "bogus"); err == nil {
		t.Error("node edit with invalid type should fail")
	}
	// Clearing the address of a peer node (kept as peer) is rejected.
	if _, err := runCLI(t, "", "vn", "ed", "node", "edit", "r1", "--public-address", ""); err == nil {
		t.Error("clearing a peer node's address should fail")
	}
}

func TestCLIConfigGenerateOverwrite(t *testing.T) {
	useTempDB(t)
	outDir := t.TempDir()
	if _, err := runCLI(t, "y\n", "vn", "add", "ov", "10.0.0.0/24"); err != nil {
		t.Fatalf("vn add error = %v", err)
	}
	if _, err := runCLI(t, "", "vn", "ov", "server", "add", "srv", "vpn.example.com"); err != nil {
		t.Fatalf("server add error = %v", err)
	}
	// First generation populates the directory.
	if _, err := runCLI(t, "", "vn", "ov", "config", "generate", "--output-dir", outDir, "--force"); err != nil {
		t.Fatalf("first config generate error = %v", err)
	}
	// Second generation without --force prompts; declining cancels.
	out, err := runCLI(t, "n\n", "vn", "ov", "config", "generate", "--output-dir", outDir)
	if err != nil {
		t.Errorf("declined overwrite should not error: %v", err)
	}
	if !strings.Contains(out, "Cancelled") {
		t.Errorf("expected 'Cancelled' on declined overwrite, got %q", out)
	}
	// Accepting the prompt overwrites the files.
	if out, err := runCLI(t, "y\n", "vn", "ov", "config", "generate", "--output-dir", outDir); err != nil {
		t.Errorf("accepted overwrite error = %v (out: %s)", err, out)
	}
}

func TestCLIDeleteCancellations(t *testing.T) {
	useTempDB(t)
	if _, err := runCLI(t, "y\n", "vn", "add", "dc", "10.0.0.0/24"); err != nil {
		t.Fatalf("vn add error = %v", err)
	}
	if _, err := runCLI(t, "", "vn", "dc", "server", "add", "srv", "vpn.example.com"); err != nil {
		t.Fatalf("server add error = %v", err)
	}
	if _, err := runCLI(t, "", "vn", "dc", "node", "add", "n1", "route"); err != nil {
		t.Fatalf("node add error = %v", err)
	}

	// Declined node deletion: no error, node survives.
	if out, err := runCLI(t, "n\n", "vn", "dc", "node", "delete", "n1"); err != nil {
		t.Errorf("declined node delete error = %v (out: %s)", err, out)
	}
	if out, _ := runCLI(t, "", "vn", "dc", "node", "list"); !strings.Contains(out, "n1") {
		t.Error("node should survive a declined deletion")
	}
	// Declined server deletion: no error, server survives.
	if out, err := runCLI(t, "n\n", "vn", "dc", "server", "delete"); err != nil {
		t.Errorf("declined server delete error = %v (out: %s)", err, out)
	}
	if out, _ := runCLI(t, "", "vn", "dc", "server", "info"); !strings.Contains(out, "srv") {
		t.Error("server should survive a declined deletion")
	}
}

func TestCLIConfigGenerateNoServer(t *testing.T) {
	useTempDB(t)
	if _, err := runCLI(t, "y\n", "vn", "add", "emptynet", "10.0.0.0/24"); err != nil {
		t.Fatalf("vn add error = %v", err)
	}
	// Generating configs for a network without a server must fail.
	if _, err := runCLI(t, "", "vn", "emptynet", "config", "generate", "--output-dir", t.TempDir(), "--force"); err == nil {
		t.Error("config generate without a server should fail")
	}
}

func TestCLIEmptyListings(t *testing.T) {
	useTempDB(t)

	out, err := runCLI(t, "", "vn", "list")
	if err != nil {
		t.Fatalf("vn list error = %v", err)
	}
	if !strings.Contains(out, "No virtual networks") {
		t.Errorf("expected empty-list message, got %q", out)
	}

	if _, err := runCLI(t, "y\n", "vn", "add", "ln", "10.0.0.0/24"); err != nil {
		t.Fatalf("vn add error = %v", err)
	}
	out, err = runCLI(t, "", "vn", "ln", "node", "list")
	if err != nil {
		t.Fatalf("node list error = %v", err)
	}
	if !strings.Contains(out, "No nodes") {
		t.Errorf("expected empty node-list message, got %q", out)
	}

	out, err = runCLI(t, "", "vn", "ln", "config", "history")
	if err != nil {
		t.Fatalf("config history error = %v", err)
	}
	if !strings.Contains(out, "No configuration versions") {
		t.Errorf("expected empty history message, got %q", out)
	}
}
