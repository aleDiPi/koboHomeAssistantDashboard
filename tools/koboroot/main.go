// Command koboroot packs kobo/rootfs into a KoboRoot.tgz, which the Kobo
// unpacks over "/" at the next boot. Only files are stored, so that the owner
// and mode of existing system directories are left alone.
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"flag"
	"io/fs"
	"log"
	"os"
	"time"
)

func main() {
	src := flag.String("src", "kobo/rootfs", "directory to pack")
	out := flag.String("o", "dist/KoboRoot.tgz", "archive to write")
	flag.Parse()

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	root := os.DirFS(*src)
	err := fs.WalkDir(root, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := fs.ReadFile(root, path)
		if err != nil {
			return err
		}
		data = bytes.ReplaceAll(data, []byte{'\r', '\n'}, []byte{'\n'})
		err = tw.WriteHeader(&tar.Header{
			Name: "./" + path, Mode: 0o755, Size: int64(len(data)),
			Uname: "root", Gname: "root", ModTime: time.Now(), Format: tar.FormatGNU,
		})
		if err != nil {
			return err
		}
		log.Printf("adding %s", path)
		_, err = tw.Write(data)
		return err
	})
	for _, close := range []func() error{tw.Close, gz.Close} {
		if e := close(); err == nil {
			err = e
		}
	}
	if err == nil {
		err = os.WriteFile(*out, buf.Bytes(), 0o644)
	}
	if err != nil {
		log.Fatal(err)
	}
}
