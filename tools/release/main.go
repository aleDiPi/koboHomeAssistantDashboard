// Command release prepares a signed release for the over-the-air updates.
//
//	go run ./tools/release -init            create the signing key (once)
//	go run ./tools/release -version N       pack dist/kobodash and the scripts
//
// The private key stays in .keys/, out of git: whoever has it can run code on
// the Kobo.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/aleDiPi/koboHomeAssistantDashboard/internal/update"
)

const keyFile = ".keys/update.key"

func main() {
	create := flag.Bool("init", false, "create the signing key and print the public one")
	version := flag.Int64("version", 0, "version number of the release")
	out := flag.String("o", "dist/release", "output folder")
	flag.Parse()
	var err error
	switch {
	case *create:
		err = initKey()
	case *version > 0:
		err = pack(*version, *out)
	default:
		err = fmt.Errorf("use -init or -version N")
	}
	if err != nil {
		log.Fatal(err)
	}
}

func readKey() (ed25519.PrivateKey, error) {
	data, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, err
	}
	seed, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(data)))
	if err != nil || len(seed) != ed25519.SeedSize {
		return nil, fmt.Errorf("%s is damaged", keyFile)
	}
	return ed25519.NewKeyFromSeed(seed), nil
}

// initKey never overwrites: a new key would lock out the installed devices.
func initKey() error {
	if _, err := os.Stat(keyFile); os.IsNotExist(err) {
		seed := make([]byte, ed25519.SeedSize)
		if _, err := rand.Read(seed); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(keyFile), 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(keyFile, []byte(base64.StdEncoding.EncodeToString(seed)), 0o600); err != nil {
			return err
		}
	}
	key, err := readKey()
	if err != nil {
		return err
	}
	fmt.Println("Put this line in [update] of dashboard.ini:")
	fmt.Println("key = " + base64.StdEncoding.EncodeToString(key.Public().(ed25519.PublicKey)))
	return nil
}

func pack(version int64, out string) error {
	key, err := readKey()
	if err != nil {
		return fmt.Errorf("no signing key (run with -init first): %w", err)
	}
	files := map[string][]byte{}
	for _, name := range update.Files {
		path := filepath.Join("kobo", name)
		if name == "kobodash" {
			path = filepath.Join("dist", name)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if name != "kobodash" {
			data = []byte(strings.ReplaceAll(string(data), "\r\n", "\n"))
		}
		files[name] = data
	}
	archive, signature, err := update.Pack(files, version, key)
	if err != nil {
		return err
	}
	// Check it the way the Kobo will.
	if _, _, err := update.Open(archive, signature, key.Public().(ed25519.PublicKey)); err != nil {
		return err
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	for name, data := range map[string][]byte{
		update.ArchiveName:   archive,
		update.SignatureName: signature,
		update.VersionName:   []byte(strconv.FormatInt(version, 10)),
	} {
		if err := os.WriteFile(filepath.Join(out, name), data, 0o644); err != nil {
			return err
		}
	}
	log.Printf("version %d packed in %s (%d bytes)", version, out, len(archive))
	return nil
}
