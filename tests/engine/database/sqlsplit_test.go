//go:build ignore

package database

import "testing"

func TestSplitSQLPreservesQuotedSemicolons(t *testing.T) {
	statements := SplitSQL("-- comment;\nINSERT INTO `x` VALUES ('a; b'); /* removed; */ SELECT 1;")
	if len(statements) != 2 {
		t.Fatalf("statements: %#v", statements)
	}
	if statements[0] != "INSERT INTO `x` VALUES ('a; b')" {
		t.Fatalf("first statement: %q", statements[0])
	}
	if statements[1] != "SELECT 1" {
		t.Fatalf("second statement: %q", statements[1])
	}
}

