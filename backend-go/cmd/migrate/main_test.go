package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestSplitSQLStatements_SplitsCommentsAndStatements(t *testing.T) {
	sql := `-- heading comment
CREATE TABLE foo (
    id TEXT PRIMARY KEY,
    note TEXT DEFAULT 'semi;colon'
);

-- between statements
CREATE INDEX foo_note_idx ON foo (note);
`

	got, err := splitSQLStatements(sql)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"-- heading comment\nCREATE TABLE foo (\n    id TEXT PRIMARY KEY,\n    note TEXT DEFAULT 'semi;colon'\n)",
		"-- between statements\nCREATE INDEX foo_note_idx ON foo (note)",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("split mismatch\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestSplitSQLStatements_KeepsDollarQuotedBlocks(t *testing.T) {
	sql := `DO $$
BEGIN
    PERFORM 1;
END $$;

CREATE TABLE bar (id INT PRIMARY KEY);
`

	got, err := splitSQLStatements(sql)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"DO $$\nBEGIN\n    PERFORM 1;\nEND $$",
		"CREATE TABLE bar (id INT PRIMARY KEY)",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("split mismatch\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestStripSQLComments_RemovesInlineAndLeadingComments(t *testing.T) {
	sql := `-- header
CREATE TABLE organizations (
    slug TEXT NOT NULL UNIQUE, -- inline comment
    note TEXT DEFAULT 'keep -- literal'
);
`

	got := stripSQLComments(sql)
	if reflect.DeepEqual(got, sql) {
		t.Fatal("expected comments to be removed")
	}
	if want := "'keep -- literal'"; !strings.Contains(got, want) {
		t.Fatalf("string literal was not preserved: %q", got)
	}
	if strings.Contains(got, "inline comment") || strings.Contains(got, "header") {
		t.Fatalf("comments were not stripped: %q", got)
	}
}

func TestSplitSQLStatements_AllowsUTF8BOMPrefix(t *testing.T) {
	sql := "\ufeffCREATE TABLE foo (id INT PRIMARY KEY);"

	got, err := splitSQLStatements(strings.TrimPrefix(sql, "\ufeff"))
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"CREATE TABLE foo (id INT PRIMARY KEY)"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("split mismatch\nwant: %#v\ngot:  %#v", want, got)
	}
}
