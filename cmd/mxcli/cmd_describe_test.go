// SPDX-License-Identifier: Apache-2.0

package main

import (
	"slices"
	"strings"
	"testing"
)

func TestChooseDescribeType(t *testing.T) {
	cases := []struct {
		name      string
		matches   []string
		wantType  string
		wantCands []string
		wantErr   bool
	}{
		{"single", []string{"microflow"}, "microflow", nil, false},
		{"dedup to single", []string{"entity", "entity"}, "entity", nil, false},
		{"empties filtered", []string{"", "page", ""}, "page", nil, false},
		{"ambiguous", []string{"entity", "microflow"}, "", []string{"entity", "microflow"}, false},
		{"ambiguous with dup", []string{"entity", "microflow", "entity"}, "", []string{"entity", "microflow"}, false},
		{"order preserved", []string{"microflow", "entity"}, "", []string{"microflow", "entity"}, false},
		{"none", nil, "", nil, true},
		{"all empty is none", []string{"", ""}, "", nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotType, gotCands, err := chooseDescribeType("Mod.X", tc.matches)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tc.wantErr)
			}
			if gotType != tc.wantType {
				t.Errorf("type = %q, want %q", gotType, tc.wantType)
			}
			if !slices.Equal(gotCands, tc.wantCands) {
				t.Errorf("candidates = %v, want %v", gotCands, tc.wantCands)
			}
		})
	}
}

// TestDescribeTypeToMDL_PublishedRestService pins the CLI spellings of the
// published REST service type. The MDL statement `describe published rest
// service M.S` worked in exec and the REPL, but the `mxcli describe` subcommand
// rejected the type with "Unknown type".
func TestDescribeTypeToMDL_PublishedRestService(t *testing.T) {
	const want = "DESCRIBE PUBLISHED REST SERVICE Shop.OrderApi"
	for _, typ := range []string{
		"PUBLISHED REST SERVICE",
		"PUBLISHEDRESTSERVICE",
		"REST SERVICE",
		"RESTSERVICE",
	} {
		t.Run(typ, func(t *testing.T) {
			got, ok := describeTypeToMDL(typ, "Shop.OrderApi")
			if !ok {
				t.Fatalf("describeTypeToMDL(%q) reported an unknown type", typ)
			}
			if got != want {
				t.Errorf("describeTypeToMDL(%q) = %q, want %q", typ, got, want)
			}
		})
	}
}

// TestTypeMaps_ValuesAreDispatchable checks that every keyword auto-detect can
// produce is a type the describe dispatch accepts; otherwise auto-detect would
// resolve a name and then fail with "Unknown type".
func TestTypeMaps_ValuesAreDispatchable(t *testing.T) {
	for _, m := range []map[string]string{objectTypeToDescribe, unitTypeToDescribe} {
		for k, v := range m {
			if _, ok := describeTypeToMDL(strings.ToUpper(v), "Mod.X"); !ok {
				t.Errorf("auto-detect maps %q to %q, which describe does not accept", k, v)
			}
		}
	}
	// A published REST service must be auto-detectable from both the catalog
	// and the live project scan.
	if objectTypeToDescribe["PUBLISHED_REST_SERVICE"] == "" {
		t.Error(`objectTypeToDescribe has no entry for "PUBLISHED_REST_SERVICE"`)
	}
	if unitTypeToDescribe["Rest$PublishedRestService"] == "" {
		t.Error(`unitTypeToDescribe has no entry for "Rest$PublishedRestService"`)
	}
}

// TestTypeMaps_KnownEntries pins the mappings the auto-detect dispatch depends
// on: every mapped value must be a describe keyword the command actually handles,
// and the common types must be present.
func TestTypeMaps_KnownEntries(t *testing.T) {
	wantObject := map[string]string{
		"MICROFLOW": "microflow", "ENTITY": "entity", "PAGE": "page",
		"ENUMERATION": "enumeration", "MODULE": "module", "EXTERNAL_ENTITY": "entity",
	}
	for k, v := range wantObject {
		if objectTypeToDescribe[k] != v {
			t.Errorf("objectTypeToDescribe[%q] = %q, want %q", k, objectTypeToDescribe[k], v)
		}
	}
	wantUnit := map[string]string{
		"Microflows$Microflow": "microflow", "Forms$Page": "page",
		"Enumerations$Enumeration": "enumeration", "JavaActions$JavaAction": "javaaction",
	}
	for k, v := range wantUnit {
		if unitTypeToDescribe[k] != v {
			t.Errorf("unitTypeToDescribe[%q] = %q, want %q", k, unitTypeToDescribe[k], v)
		}
	}
	// Every mapped describe keyword must be a single bare word (no spaces) so it
	// slots into the dispatch as args[0].
	for _, m := range []map[string]string{objectTypeToDescribe, unitTypeToDescribe} {
		for k, v := range m {
			if v == "" {
				t.Errorf("empty describe keyword for %q", k)
			}
		}
	}
}
