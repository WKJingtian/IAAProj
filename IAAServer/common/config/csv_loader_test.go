package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadCSVParsesScalarAndSliceFields(t *testing.T) {
	type rewardRow struct {
		ID     int      `csv:"id"`
		Name   string   `csv:"name"`
		Tags   []string `csv:"tags"`
		Values []int    `csv:"values"`
	}

	path := writeCSVTestFile(t, strings.Join([]string{
		"id,name,tags,values",
		"1,alpha,\"[a,b,c]\",\"[1,2,3]\"",
		"2,beta,[],[]",
	}, "\n"))

	rows, err := LoadCSV[rewardRow](path)
	if err != nil {
		t.Fatalf("LoadCSV returned unexpected error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0].ID != 1 || rows[0].Name != "alpha" {
		t.Fatalf("unexpected first row scalars: %+v", rows[0])
	}
	if got := strings.Join(rows[0].Tags, ","); got != "a,b,c" {
		t.Fatalf("unexpected first row tags: %q", got)
	}
	if len(rows[0].Values) != 3 || rows[0].Values[0] != 1 || rows[0].Values[2] != 3 {
		t.Fatalf("unexpected first row values: %+v", rows[0].Values)
	}
	if rows[1].Tags == nil || len(rows[1].Tags) != 0 {
		t.Fatalf("expected empty non-nil tags slice, got %+v", rows[1].Tags)
	}
	if rows[1].Values == nil || len(rows[1].Values) != 0 {
		t.Fatalf("expected empty non-nil values slice, got %+v", rows[1].Values)
	}
}

func TestLoadCSVReturnsRowsAndAggregatesSliceConversionErrors(t *testing.T) {
	type rewardRow struct {
		ID     int      `csv:"id"`
		Values []int    `csv:"values"`
		Tags   []string `csv:"tags"`
	}

	path := writeCSVTestFile(t, strings.Join([]string{
		"id,values,tags",
		"1,\"[1,2,3]\",\"[a,b,c]\"",
		"2,\"[a,b,c]\",\"[x,y,z]\"",
		"3,\"[4,x,6]\",[]",
	}, "\n"))

	rows, err := LoadCSV[rewardRow](path)
	if err == nil {
		t.Fatal("expected aggregated error, got nil")
	}

	var loadErr *CSVLoadError
	if !errors.As(err, &loadErr) {
		t.Fatalf("expected *CSVLoadError, got %T", err)
	}
	if len(loadErr.Issues) != 2 {
		t.Fatalf("expected 2 issues, got %d", len(loadErr.Issues))
	}
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}
	if len(rows[0].Values) != 3 || rows[0].Values[1] != 2 {
		t.Fatalf("unexpected successful row values: %+v", rows[0].Values)
	}
	if rows[1].Values == nil || len(rows[1].Values) != 0 {
		t.Fatalf("expected empty non-nil slice for invalid list, got %+v", rows[1].Values)
	}
	if rows[2].Values == nil || len(rows[2].Values) != 0 {
		t.Fatalf("expected empty non-nil slice for partially invalid list, got %+v", rows[2].Values)
	}
	if loadErr.Issues[0].Row != 3 || loadErr.Issues[0].Header != "values" {
		t.Fatalf("unexpected first issue metadata: %+v", loadErr.Issues[0])
	}
	if loadErr.Issues[1].Row != 4 || loadErr.Issues[1].Header != "values" {
		t.Fatalf("unexpected second issue metadata: %+v", loadErr.Issues[1])
	}
}

func TestLoadCSVRejectsUnknownHeader(t *testing.T) {
	type rewardRow struct {
		ID int `csv:"id"`
	}

	path := writeCSVTestFile(t, strings.Join([]string{
		"id,unknown",
		"1,2",
	}, "\n"))

	_, err := LoadCSV[rewardRow](path)
	if err == nil {
		t.Fatal("expected error for unknown header, got nil")
	}
	if !strings.Contains(err.Error(), `header "unknown" has no matching exported field`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func writeCSVTestFile(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.csv")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write csv test file failed: %v", err)
	}
	return path
}
