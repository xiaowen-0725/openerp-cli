package cmd

import (
	"bytes"
	"io"
	"testing"

	"github.com/zhoujw/openerp-cli/internal/cmdutil"
)

func TestRootHelp(t *testing.T) {
	f := cmdutil.NewFactory()
	var out bytes.Buffer
	root := NewRootCmd(f)
	root.SetArgs([]string{"--help"})
	root.SetOut(&out)
	root.SetErr(&out)
	if err := root.Execute(); err != nil {
		t.Fatalf("help: %v", err)
	}
	for _, want := range []string{"openerp", "config", "auth", "doctor", "query", "bom"} {
		if !bytes.Contains(out.Bytes(), []byte(want)) {
			t.Errorf("help output missing %q", want)
		}
	}
}

func TestUnknownSubcommandGuard(t *testing.T) {
	f := cmdutil.NewFactory()
	root := NewRootCmd(f)
	root.SetArgs([]string{"config", "bogus"})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	if err := root.Execute(); err == nil {
		t.Fatal("expected an error for an unknown subcommand")
	}
}
