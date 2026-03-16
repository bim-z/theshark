//go:build mage

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/charmbracelet/log"
	"github.com/google/go-github/v84/github"
	"github.com/karrick/godirwalk"
	"github.com/magefile/mage/mg"
	"golang.org/x/oauth2"
)

type target struct {
	OS   string
	Arch string
}

var targets = []target{
	// Linux
	{"linux", "amd64"}, {"linux", "386"},
	{"linux", "arm64"}, {"linux", "arm"},
	{"linux", "riscv64"}, {"linux", "ppc64le"}, {"linux", "s390x"},

	// Windows
	{"windows", "amd64"}, {"windows", "386"},
	{"windows", "arm64"}, {"windows", "arm"},

	// Darwin (macOS)
	{"darwin", "amd64"}, {"darwin", "arm64"},

	// BSD Family
	{"freebsd", "amd64"}, {"freebsd", "386"}, {"freebsd", "arm64"},
	{"openbsd", "amd64"}, {"openbsd", "386"}, {"openbsd", "arm64"},
	{"netbsd", "amd64"}, {"netbsd", "386"}, {"netbsd", "arm64"},
	{"dragonfly", "amd64"},

	// Mobile & Others
	// {"android", "amd64"}, {"android", "arm64"}, {"android", "386"}, {"android", "arm"},
	{"illumos", "amd64"},
	{"solaris", "amd64"},
}

const name = "theshark"

// Build compiles the application for all defined targets.
func Build() (err error) {
	log.Info("Starting build process...")

	// Ensure dist directory exists
	if err := os.MkdirAll("dist", 0755); err != nil {
		return fmt.Errorf("failed to create dist directory: %v", err)
	}

	for _, t := range targets {
		ext := ""
		if t.OS == "windows" {
			ext = ".exe"
		}

		output := fmt.Sprintf("dist/%s-%s-%s%s", name, t.OS, t.Arch, ext)
		log.Info("Compiling", "os", t.OS, "arch", t.Arch)

		cmd := exec.Command("go", "build", "-o", output, ".")
		cmd.Env = append(os.Environ(),
			"CGO_ENABLED=0",
			"GOOS="+t.OS,
			"GOARCH="+t.Arch,
		)

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to build for %s/%s: %v", t.OS, t.Arch, err)
		}
	}

	log.Info("Build completed successfully")
	return
}

// Release handles the atomic GitHub release process.
func Release(tag string) (err error) {
	mg.Deps(Build)

	ctx := context.Background()
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return fmt.Errorf("GITHUB_TOKEN environment variable is not set")
	}

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	gh := github.NewClient(tc)

	owner := "bim-z"
	repo := "mathrock"

	log.Info("Creating draft release", "tag", tag)
	data := &github.RepositoryRelease{
		TagName: github.Ptr(tag),
		Name:    github.Ptr("Release " + tag),
		Draft:   github.Ptr(true),
	}

	release, _, err := gh.Repositories.CreateRelease(ctx, owner, repo, data)
	if err != nil {
		return fmt.Errorf("failed to create release: %v", err)
	}

	// Rollback logic for atomicity
	var success bool
	defer func() {
		if !success {
			log.Warn("Operation failed. Cleaning up incomplete release...", "id", *release.ID)
			_, _ = client.Repositories.DeleteRelease(ctx, owner, repo, *release.ID)
		}
	}()

	err = godirwalk.Walk("./dist", &godirwalk.Options{
		Callback: func(osPathname string, de *godirwalk.Dirent) error {
			if de.IsDir() {
				return nil
			}

			file, err := os.Open(osPathname)
			if err != nil {
				return err
			}
			defer file.Close()

			log.Info("Uploading asset", "file", de.Name())
			_, _, err = client.Repositories.UploadReleaseAsset(ctx, owner, repo, *release.ID, &github.UploadOptions{
				Name: de.Name(),
			}, file)
			return err
		},
	})

	if err != nil {
		return fmt.Errorf("failed to upload assets: %v", err)
	}

	// Finalizing the release
	log.Info("Publishing release...")
	_, _, err = client.Repositories.EditRelease(ctx, owner, repo, *release.ID, &github.RepositoryRelease{
		Draft: github.Ptr(false),
	})

	if err == nil {
		success = true
		log.Info("Release successfully published!")
	}

	return err
}

// Clean removes the build artifacts.
func Clean() {
	log.Info("Cleaning up dist directory...")
	if err := os.RemoveAll("dist"); err != nil {
		log.Error("Failed to clean dist directory", "error", err.Error())
	}
}
