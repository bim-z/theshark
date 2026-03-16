package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"github.com/charmbracelet/log"
	"github.com/karrick/godirwalk"
	"github.com/spf13/cobra"
)

func theshark() (cmd *cobra.Command) {
	cmd = &cobra.Command{
		Use:   "theshark [path]",
		Short: "Detect and manage duplicate files",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			var hashs []struct {
				hash string
				name string
			}

			err = godirwalk.Walk(args[0], &godirwalk.Options{
				Callback: func(osPathname string, directory *godirwalk.Dirent) (err error) {
					if directory.IsDir() {
						return
					}

					desc, err := os.Open(osPathname)
					if err != nil {
						return
					}
					defer desc.Close()

					hash := sha256.New()
					if _, err = io.Copy(hash, desc); err != nil {
						return err
					}

					hashs = append(hashs, struct {
						hash string
						name string
					}{
						hash: hex.EncodeToString(hash.Sum(nil)),
						name: osPathname,
					})

					return
				},
				Unsorted: true,
				ErrorCallback: func(s string, err error) godirwalk.ErrorAction {
					log.Errorf("Error accessing %s: %v", s, err)

					return godirwalk.SkipNode
				},
			})

			if err != nil {
				return err
			}

			seen := make(map[string]struct{})
			var duplicates []string

			for _, h := range hashs {
				if _, exists := seen[h.hash]; exists {
					duplicates = append(duplicates, h.name)
				} else {
					seen[h.hash] = struct{}{}
				}
			}

			mode, _ := cmd.Flags().GetString("mode")

			if len(duplicates) == 0 {
				log.Info("No duplicate files found.")
				return
			}

			for _, file := range duplicates {
				switch mode {
				case "purge":
					if err := os.Remove(file); err != nil {
						log.Errorf("Failed to remove %s: %v", file, err)
					} else {
						log.Warnf("File purged: %s", file)
					}
				default:
					log.Infof("Duplicate found: %s", file)
				}
			}

			return
		},
	}

	_ = cmd.Flags().StringP("mode", "m", "check", "Execution mode: check or purge")
	return cmd
}
