// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"bytes"
	"testing"

	"github.com/mendixlabs/mxcli/mdl/ast"
	"github.com/mendixlabs/mxcli/mdl/langver"
	"github.com/mendixlabs/mxcli/mdl/types"
	"github.com/mendixlabs/mxcli/mdl/visitor"
)

// DESCRIBE NAVIGATION (and DESCRIBE MENU, which shares printMenuMDL) wrote a
// menu caption between hand-placed quotes, so a caption with an apostrophe —
// "Customer's orders", "Schema's" — came out as `menu item 'Schema's'`, which
// does not parse. The emitted MDL must re-parse, under both describe
// languages, to the same captions.
func TestPrintMenuMDL_ApostropheInCaptionReparses(t *testing.T) {
	items := []*types.NavMenuItem{
		{Caption: "Schema's", Page: "M.Schedules"},
		{
			Caption: "Customer's orders",
			Items:   []*types.NavMenuItem{{Caption: "Won't ship", Page: "M.Held"}},
		},
	}
	for _, lang := range []langver.Version{langver.V0, langver.V1} {
		ctx, _ := newMockCtx(t)
		ctx.describeLang = &lang
		var b bytes.Buffer
		printMenuMDL(ctx, &b, items, 1, "CREATE NAVIGATION")
		script := "create or modify navigation Responsive {\n" + b.String() + "};"
		if lang >= langver.V1 {
			script = "mdl 1;\n" + script
		}

		prog, errs := visitor.Build(script)
		if len(errs) > 0 {
			t.Fatalf("%v: describe output does not re-parse: %v\n%s", lang, errs, script)
		}
		var stmt *ast.AlterNavigationStmt
		for _, s := range prog.Statements {
			if n, ok := s.(*ast.AlterNavigationStmt); ok {
				stmt = n
			}
		}
		if stmt == nil || len(stmt.MenuItems) != 2 || len(stmt.MenuItems[1].Items) != 1 {
			t.Fatalf("%v: menu shape changed on re-parse: %#v\n%s", lang, prog.Statements, script)
		}
		for _, c := range []struct{ got, want string }{
			{stmt.MenuItems[0].Caption, "Schema's"},
			{stmt.MenuItems[1].Caption, "Customer's orders"},
			{stmt.MenuItems[1].Items[0].Caption, "Won't ship"},
		} {
			if c.got != c.want {
				t.Errorf("%v: caption re-parsed as %q, want %q", lang, c.got, c.want)
			}
		}
	}
}
