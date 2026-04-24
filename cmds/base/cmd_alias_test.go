package base

import (
	"bytes"
	"io"
	"strings"
	"sync"
	"testing"
)

var registerAliasTestCommands sync.Once

func ensureAliasTestCommands() {
	registerAliasTestCommands.Do(func() {
		Register(NewCommand("zz_test_alias_cmd", "test alias command", func(args []string, stdout io.Writer) error {
			return nil
		}))
		Register(NewCommand("zz_test_alias_extra", "test alias extra command", func(args []string, stdout io.Writer) error {
			return nil
		}))
	})
}

func TestAliasCmd(t *testing.T) {
	ensureAliasTestCommands()

	var out bytes.Buffer

	if err := aliasCmd(nil, &out); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "export gobox_alias_type=bash") {
		t.Fatalf("expected alias type export, got %q", output)
	}
	if !strings.Contains(output, "alias zz_test_alias_cmd='gobox zz_test_alias_cmd'") {
		t.Fatalf("expected registered command alias, got %q", output)
	}
	if strings.Contains(output, "alias alias='gobox alias'") {
		t.Fatalf("did not expect alias command to alias itself, got %q", output)
	}
}

func TestAliasCmdUnalias(t *testing.T) {
	ensureAliasTestCommands()

	var out bytes.Buffer

	if err := aliasCmd([]string{"-u"}, &out); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "unalias zz_test_alias_cmd 2>/dev/null || true") {
		t.Fatalf("expected registered command unalias, got %q", output)
	}
	if !strings.Contains(output, "unset gobox_alias_type") {
		t.Fatalf("expected alias type unset, got %q", output)
	}
}

func TestAliasCmdUsage(t *testing.T) {
	var out bytes.Buffer

	if err := aliasCmd([]string{"-h"}, &out); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !strings.Contains(out.String(), "Usage: gobox alias [-u]") {
		t.Fatalf("expected alias usage, got %q", out.String())
	}
}
