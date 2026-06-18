package alignment

import "testing"

func TestParseQualifiedName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    qualifiedName
		wantErr bool
	}{
		{name: "schema table", input: "public.artifact_raw", want: qualifiedName{Schema: "public", Name: "artifact_raw"}},
		{name: "default schema", input: "artifact_raw", want: qualifiedName{Schema: defaultSchema, Name: "artifact_raw"}},
		{name: "too many parts", input: "a.b.c", wantErr: true},
		{name: "invalid identifier", input: "public.artifact-raw", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseQualifiedName(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseQualifiedName: %v", err)
			}
			if got != tt.want {
				t.Fatalf("unexpected name: %#v", got)
			}
		})
	}
}

func TestBuildRangeClause(t *testing.T) {
	start := int64(10)
	end := int64(20)
	tests := []struct {
		name      string
		startID   *int64
		endID     *int64
		wantSQL   string
		wantCount int
	}{
		{name: "no range", wantSQL: "", wantCount: 0},
		{name: "start only", startID: &start, wantSQL: " AND s.\"id\" >= $1", wantCount: 1},
		{name: "end only", endID: &end, wantSQL: " AND s.\"id\" <= $1", wantCount: 1},
		{name: "full range", startID: &start, endID: &end, wantSQL: " AND s.\"id\" >= $1 AND s.\"id\" <= $2", wantCount: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSQL, gotArgs := buildRangeClause(`s."id"`, tt.startID, tt.endID, 1)
			if gotSQL != tt.wantSQL {
				t.Fatalf("unexpected sql: %q", gotSQL)
			}
			if len(gotArgs) != tt.wantCount {
				t.Fatalf("unexpected args length: %d", len(gotArgs))
			}
		})
	}
}

func TestMappingTableNameForUsesDefaultSchema(t *testing.T) {
	got, err := mappingTableNameFor("biz.artifact_raw")
	if err != nil {
		t.Fatalf("mapping table name: %v", err)
	}
	want := qualifiedName{Schema: defaultSchema, Name: mappingTableName}
	if got != want {
		t.Fatalf("unexpected mapping table: %#v", got)
	}
}

func TestBuildWaitingTargetReviewClause(t *testing.T) {
	req := ExecuteRequest{SourceTable: "public.artifact_raw"}
	gotSQL, gotArgs := buildWaitingTargetReviewClause(req, `s."id"`, 3)
	if gotSQL != "" || gotArgs != nil {
		t.Fatalf("expected empty clause, got %q %#v", gotSQL, gotArgs)
	}

	req.OnlyWaitingTargetReview = true
	gotSQL, gotArgs = buildWaitingTargetReviewClause(req, `s."id"`, 3)
	wantSQL := ` AND EXISTS (
		SELECT 1
		FROM data_process_records dpr
		WHERE dpr.source_table = $3
		  AND dpr.source_key = s."id"
		  AND dpr.process_type = $4
		  AND dpr.status = $5
	)`
	if gotSQL != wantSQL {
		t.Fatalf("unexpected clause:\n%s", gotSQL)
	}
	if len(gotArgs) != 3 || gotArgs[0] != "public.artifact_raw" ||
		gotArgs[1] != ProcessTypeEntityAlignment ||
		gotArgs[2] != "waiting_target_review" {
		t.Fatalf("unexpected args: %#v", gotArgs)
	}
}

func TestBuildOutputPages(t *testing.T) {
	tests := []struct {
		name     string
		start    int64
		end      int64
		pageSize int64
		want     []outputPage
	}{
		{
			name:     "multiple pages",
			start:    1,
			end:      25,
			pageSize: 10,
			want: []outputPage{
				{Start: 1, End: 10},
				{Start: 11, End: 20},
				{Start: 21, End: 25},
			},
		},
		{
			name:     "single page",
			start:    1,
			end:      10,
			pageSize: 10,
			want:     []outputPage{{Start: 1, End: 10}},
		},
		{
			name:     "same start and end",
			start:    7,
			end:      7,
			pageSize: 10,
			want:     []outputPage{{Start: 7, End: 7}},
		},
		{
			name:     "invalid range",
			start:    10,
			end:      1,
			pageSize: 10,
			want:     nil,
		},
		{
			name:     "non positive page size uses default",
			start:    1,
			end:      defaultOutputPageSize + 1,
			pageSize: 0,
			want: []outputPage{
				{Start: 1, End: defaultOutputPageSize},
				{Start: defaultOutputPageSize + 1, End: defaultOutputPageSize + 1},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildOutputPages(tt.start, tt.end, tt.pageSize)
			if len(got) != len(tt.want) {
				t.Fatalf("unexpected page count: got %d want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("page[%d] = %#v, want %#v", i, got[i], tt.want[i])
				}
			}
		})
	}
}
