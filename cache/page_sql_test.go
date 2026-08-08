package cache

import "testing"

func TestSQLitePageRejectsUnsafeSortExpressions(t *testing.T) {
	if _, err := getOrderByClause([]PageSorter{{SortField: "name; DROP TABLE data"}}); err == nil {
		t.Fatal("expected unsafe sort field to be rejected")
	}
	if _, err := getOrderByClauseRaw([]PageSorter{{Expr: "id DESC; DROP TABLE data"}}); err == nil {
		t.Fatal("expected unsafe raw sort expression to be rejected")
	}
}

func TestBuildWhereClauseKeepsFieldInputInsideAQuotedJSONPath(t *testing.T) {
	where, args := buildWhereClause(map[string]any{"name') OR 1=1 --": "alice"})
	if len(args) != 1 || args[0] != "alice" {
		t.Fatalf("unexpected where args: %v", args)
	}
	if where == "" || where == "WHERE 1=1" {
		t.Fatalf("expected a condition, got %q", where)
	}
	if where != "WHERE json_extract(data, '$.name'') OR 1=1 --') = ?" {
		t.Fatalf("unexpected safely quoted condition: %q", where)
	}
}
