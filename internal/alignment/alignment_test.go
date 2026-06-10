package alignment

import "testing"

func TestNormalizeTargets(t *testing.T) {
	got := NormalizeTargets([]string{" 宋 ", "唐", "", "宋", "元"})
	want := []string{"元", "唐", "宋"}
	if len(got) != len(want) {
		t.Fatalf("unexpected targets length: %#v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected targets: %#v", got)
		}
	}
}

func TestChunkStrings(t *testing.T) {
	got := ChunkStrings([]string{"a", "b", "c"}, 2)
	if len(got) != 2 {
		t.Fatalf("unexpected chunks: %#v", got)
	}
	if len(got[0]) != 2 || got[0][0] != "a" || got[0][1] != "b" {
		t.Fatalf("unexpected first chunk: %#v", got)
	}
	if len(got[1]) != 1 || got[1][0] != "c" {
		t.Fatalf("unexpected second chunk: %#v", got)
	}
}
