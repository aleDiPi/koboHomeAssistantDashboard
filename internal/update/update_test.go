package update

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/binary"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// armProgram is the smallest thing debug/elf accepts as an ARM executable.
func armProgram(marker string) []byte {
	h := make([]byte, 52)
	copy(h, "\x7fELF\x01\x01\x01")
	binary.LittleEndian.PutUint16(h[16:], 2)  // executable
	binary.LittleEndian.PutUint16(h[18:], 40) // ARM
	binary.LittleEndian.PutUint32(h[20:], 1)
	binary.LittleEndian.PutUint16(h[40:], 52)
	return append(h, marker...)
}

type release struct{ archive, signature, version []byte }

func serve(t *testing.T, r *release) string {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/" + ArchiveName:
			w.Write(r.archive)
		case "/" + SignatureName:
			w.Write(r.signature)
		case "/" + VersionName:
			w.Write(r.version)
		default:
			http.NotFound(w, req)
		}
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func TestInstallAndRestore(t *testing.T) {
	pub, private, _ := ed25519.GenerateKey(nil)
	files := map[string][]byte{"kobodash": armProgram("new"), "wifi.sh": []byte("new wifi")}
	archive, signature, err := Pack(files, 20, private)
	if err != nil {
		t.Fatal(err)
	}
	r := &release{archive, signature, []byte("20")}

	dir := t.TempDir()
	read := func(name string) string {
		b, _ := os.ReadFile(filepath.Join(dir, name))
		return string(b)
	}
	os.WriteFile(filepath.Join(dir, "kobodash"), []byte("old program"), 0o755)
	os.WriteFile(filepath.Join(dir, "dashboard.ini"), []byte("secret"), 0o644)

	u, err := New(serve(t, r), base64.StdEncoding.EncodeToString(pub), dir, 10)
	if err != nil {
		t.Fatal(err)
	}
	if v, newer, err := u.Check(context.Background()); err != nil || !newer || v != 20 {
		t.Fatalf("check: %v %v %v", v, newer, err)
	}
	if err := u.Install(context.Background()); err != nil {
		t.Fatal(err)
	}
	if read("kobodash") != string(files["kobodash"]) || read("kobodash.old") != "old program" ||
		read("wifi.sh") != "new wifi" || read(Pending) != "20" || read("dashboard.ini") != "secret" {
		t.Fatal("files not swapped as expected")
	}
	Restore(dir)
	if read("kobodash") != "old program" || read(Pending) != "" {
		t.Fatal("not restored")
	}
}

func TestRefusedReleases(t *testing.T) {
	pub, private, _ := ed25519.GenerateKey(nil)
	_, stranger, _ := ed25519.GenerateKey(nil)
	good := map[string][]byte{"kobodash": armProgram("x")}
	pack := func(files map[string][]byte, v int64, key ed25519.PrivateKey) *release {
		a, s, err := Pack(files, v, key)
		if err != nil {
			t.Fatal(err)
		}
		return &release{a, s, nil}
	}
	tampered := pack(good, 20, private)
	tampered.archive[len(tampered.archive)/2] ^= 1
	for name, r := range map[string]*release{
		"signed by someone else": pack(good, 20, stranger),
		"altered after signing":  tampered,
		"not newer":              pack(good, 5, private),
		"not an ARM program":     pack(map[string][]byte{"kobodash": []byte("#!/bin/sh")}, 20, private),
	} {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "kobodash"), []byte("old"), 0o755)
		u, _ := New(serve(t, r), base64.StdEncoding.EncodeToString(pub), dir, 10)
		if err := u.Install(context.Background()); err == nil {
			t.Errorf("%s: installed", name)
		}
		if b, _ := os.ReadFile(filepath.Join(dir, "kobodash")); string(b) != "old" {
			t.Errorf("%s: program replaced", name)
		}
	}
}
