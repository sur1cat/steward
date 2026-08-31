package bashedit

import "testing"

func TestDetectsFileWrites(t *testing.T) {
	writes := []string{
		`sed -i '' 's/a/b/' file.go`,
		`sed -i.bak s/a/b/ file.go`,
		`perl -pi -e 's/a/b/' file.go`,
		`echo hi > out.txt`,
		`echo hi >> out.txt`,
		`cat file | tee copy.txt`,
		`cat file | tee -a copy.txt`,
		`dd if=/dev/zero of=disk.img`,
		`truncate -s 0 log.txt`,
		// hidden behind a separator, which is where it is easiest to miss
		`go build ./... && sed -i '' 's/x/y/' main.go`,
		`make test; echo done > status.txt`,
	}
	for _, c := range writes {
		if !Writes(c) {
			t.Errorf("Writes(%q) = false, want true", c)
		}
	}
}

func TestLeavesReadsAndHarmlessRedirectsAlone(t *testing.T) {
	reads := []string{
		`sed 's/a/b/' file.go`, // no -i: prints, does not write
		`grep -r pattern .`,
		`go test ./... 2>/dev/null`, // discards, writes nothing
		`ls -la >/dev/null 2>&1`,
		`cat file.go`,
		`git diff`,
		`npm run build`,
		`echo hi | grep h`,
		`command 2>&1`, // a descriptor, not a path
	}
	for _, c := range reads {
		if fs := Detect(c); len(fs) > 0 {
			t.Errorf("Detect(%q) flagged %+v, want nothing", c, fs)
		}
	}
}

func TestExplainNamesEveryKindOnce(t *testing.T) {
	fs := Detect(`sed -i '' s/a/b/ x.go && echo y > z.txt && sed -i '' s/c/d/ w.go`)
	got := Explain(fs)
	if got == "" {
		t.Fatal("a flagged command must be explainable")
	}
	// "sed" appears twice in the command but should be named once.
	if n := countSubstr(got, "sed"); n != 1 {
		t.Errorf("kind listed %d times, want once: %s", n, got)
	}
	if countSubstr(got, "redirect") != 1 {
		t.Errorf("the redirect should be named too: %s", got)
	}
	if Explain(nil) != "" {
		t.Error("nothing found means nothing to explain")
	}
}

func countSubstr(s, sub string) int {
	n := 0
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			n++
		}
	}
	return n
}
