// Command mediasync copies the photos saved on this computer (MEDIA_DIR, default var/media)
// into the Cloudflare R2 bucket, so the live shop has the photos you uploaded locally.
// It's safe to run again: files are simply uploaded again.
//
//	cd backend
//	go run ./cmd/mediasync -dry-run     # list what would be copied
//	go run ./cmd/mediasync              # copy (needs the R2_* settings in backend/.env)
package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"

	"yetibyrash/backend/internal/config"
	"yetibyrash/backend/internal/media"
)

func main() {
	dryRun := flag.Bool("dry-run", false, "list the files without copying them")
	flag.Parse()
	cfg := config.Load()
	ctx := context.Background()

	var files []string
	err := filepath.WalkDir(cfg.MediaDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			files = append(files, p)
		}
		return nil
	})
	if err != nil {
		log.Fatalf("read %s: %v", cfg.MediaDir, err)
	}
	fmt.Printf("%d files in %s\n", len(files), cfg.MediaDir)
	if *dryRun {
		for _, f := range files {
			fmt.Println(" ", f)
		}
		return
	}

	store, err := media.NewR2Storage(media.R2Config{
		AccountID: cfg.R2AccountID, AccessKeyID: cfg.R2AccessKeyID, SecretAccessKey: cfg.R2SecretAccessKey,
		Bucket: cfg.R2Bucket, PublicURL: cfg.MediaPublicURL,
	})
	if err != nil {
		log.Fatal(err)
	}
	if err := store.Check(ctx); err != nil {
		log.Fatal(err)
	}
	for i, p := range files {
		rel, _ := filepath.Rel(cfg.MediaDir, p)
		key := filepath.ToSlash(rel)
		f, err := os.Open(p)
		if err != nil {
			log.Fatal(err)
		}
		err = store.Put(ctx, key, f)
		f.Close()
		if err != nil {
			log.Fatalf("copy %s: %v", key, err)
		}
		fmt.Printf("[%d/%d] %s\n", i+1, len(files), key)
	}
	fmt.Println("Done. Photos are now in the R2 bucket.")
}
