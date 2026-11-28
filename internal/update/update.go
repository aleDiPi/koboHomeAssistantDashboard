// Package update fetches new versions of the dashboard and swaps them in.
//
// A release is three files at one address: version.txt, kobodash.zip and
// kobodash.zip.sig, an Ed25519 signature of the archive. Only the signature is
// trusted, so the address (a GitHub release, or a server at home) does not
// need to be.
package update

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/ed25519"
	"debug/elf"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

const (
	ArchiveName   = "kobodash.zip"
	SignatureName = ArchiveName + ".sig"
	VersionName   = "version.txt"

	versionEntry = "VERSION"
	// Pending marks a version that has not yet proved to work; start.sh puts
	// the previous files back if it finds it after a failure.
	Pending = "update-pending"

	maxArchive = 40 << 20
	maxFile    = 30 << 20
)

// Files are the only ones a release may replace. The configuration, and so the
// token and the public key, is never among them.
var Files = []string{"kobodash", "start.sh", "wifi.sh", "autostart.sh"}

var client = &http.Client{Timeout: 5 * time.Minute}

// Updater checks one address and installs into one folder.
type Updater struct {
	URL     string
	Key     ed25519.PublicKey
	Dir     string
	Current int64
}

// New parses the public key, in base64.
func New(url, key, dir string, current int64) (*Updater, error) {
	pub, err := base64.StdEncoding.DecodeString(key)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return nil, errors.New("the update key is not an Ed25519 public key")
	}
	return &Updater{URL: strings.TrimRight(url, "/"), Key: pub, Dir: dir, Current: current}, nil
}

func (u *Updater) get(ctx context.Context, name string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.URL+"/"+name, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", name, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err == nil && int64(len(data)) > limit {
		err = fmt.Errorf("%s: too large", name)
	}
	return data, err
}

// Check returns the published version and whether it is newer than ours.
func (u *Updater) Check(ctx context.Context) (int64, bool, error) {
	data, err := u.get(ctx, VersionName, 64)
	if err != nil {
		return 0, false, err
	}
	v, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0, false, fmt.Errorf("%s: %w", VersionName, err)
	}
	return v, v > u.Current, nil
}

// Install downloads the release, verifies it and replaces the files, keeping
// the previous ones as *.old.
func (u *Updater) Install(ctx context.Context) error {
	archive, err := u.get(ctx, ArchiveName, maxArchive)
	if err != nil {
		return err
	}
	signature, err := u.get(ctx, SignatureName, 256)
	if err != nil {
		return err
	}
	files, version, err := Open(archive, signature, u.Key)
	if err != nil {
		return err
	}
	// The signature covers the version, so an old release cannot be replayed.
	if version <= u.Current {
		return fmt.Errorf("version %d is not newer than %d", version, u.Current)
	}
	return swap(u.Dir, files, version)
}

// Open verifies a release and returns its files.
func Open(archive, signature []byte, key ed25519.PublicKey) (map[string][]byte, int64, error) {
	sig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(signature)))
	if err != nil || !ed25519.Verify(key, archive, sig) {
		return nil, 0, errors.New("the signature does not match: release refused")
	}
	z, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, 0, err
	}
	files := map[string][]byte{}
	var version int64
	for _, entry := range z.File {
		if entry.Name != versionEntry && !slices.Contains(Files, entry.Name) {
			return nil, 0, fmt.Errorf("unexpected file %q in the release", entry.Name)
		}
		r, err := entry.Open()
		if err != nil {
			return nil, 0, err
		}
		data, err := io.ReadAll(io.LimitReader(r, maxFile+1))
		r.Close()
		if err != nil || len(data) > maxFile {
			return nil, 0, fmt.Errorf("%s: unreadable or too large", entry.Name)
		}
		if entry.Name == versionEntry {
			if version, err = strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64); err != nil {
				return nil, 0, err
			}
			continue
		}
		files[entry.Name] = data
	}
	if version <= 0 || files["kobodash"] == nil {
		return nil, 0, errors.New("incomplete release")
	}
	program, err := elf.NewFile(bytes.NewReader(files["kobodash"]))
	if err != nil || program.Machine != elf.EM_ARM {
		return nil, 0, errors.New("the program in the release is not for the Kobo")
	}
	return files, version, nil
}

func writeSynced(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if e := f.Sync(); err == nil {
		err = e
	}
	if e := f.Close(); err == nil {
		err = e
	}
	return err
}

// swap writes every new file first, then renames them into place. The marker
// is written before the first rename, so that an interrupted swap is undone.
func swap(dir string, files map[string][]byte, version int64) error {
	path := func(name string) string { return filepath.Join(dir, name) }
	for name, data := range files {
		if err := writeSynced(path(name+".new"), data); err != nil {
			return err
		}
	}
	// Leftovers of an earlier update would be "restored" over newer files.
	for _, name := range Files {
		if err := os.Remove(path(name + ".old")); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	if err := writeSynced(path(Pending), []byte(strconv.FormatInt(version, 10))); err != nil {
		return err
	}
	for name := range files {
		if err := os.Rename(path(name), path(name+".old")); err != nil && !os.IsNotExist(err) {
			Restore(dir)
			return err
		}
		if err := os.Rename(path(name+".new"), path(name)); err != nil {
			Restore(dir)
			return err
		}
	}
	return nil
}

// Restore puts the previous files back.
func Restore(dir string) {
	for _, name := range Files {
		old := filepath.Join(dir, name+".old")
		if _, err := os.Stat(old); err == nil {
			os.Rename(old, filepath.Join(dir, name))
		}
	}
	os.Remove(filepath.Join(dir, Pending))
}

// Confirm records that the running version works.
func (u *Updater) Confirm() error {
	err := os.Remove(filepath.Join(u.Dir, Pending))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// Pack builds the signed archive of a release.
func Pack(files map[string][]byte, version int64, key ed25519.PrivateKey) (archive, signature []byte, err error) {
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	add := func(name string, data []byte) error {
		w, err := z.Create(name)
		if err == nil {
			_, err = w.Write(data)
		}
		return err
	}
	if err := add(versionEntry, []byte(strconv.FormatInt(version, 10))); err != nil {
		return nil, nil, err
	}
	for _, name := range Files {
		if data, ok := files[name]; ok {
			if err := add(name, data); err != nil {
				return nil, nil, err
			}
		}
	}
	if err := z.Close(); err != nil {
		return nil, nil, err
	}
	sig := base64.StdEncoding.EncodeToString(ed25519.Sign(key, buf.Bytes()))
	return buf.Bytes(), []byte(sig), nil
}
