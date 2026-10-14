package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mevdschee/logitech-rgb/internal/app"
	"github.com/mevdschee/logitech-rgb/internal/config"
	"github.com/mevdschee/logitech-rgb/internal/power"
)

// daemonSoftwareID differs from the CLI/TUI one so both can talk to the
// same device at the same time without stealing each other's replies.
const daemonSoftwareID = 0x0B

func cmdDaemon(args []string) error {
	fs := flag.NewFlagSet("daemon", flag.ExitOnError)
	fs.Parse(args)
	log.SetFlags(log.Ltime)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	watcher := &config.Watcher{}
	running := map[string]bool{} // hidraw path -> handled
	done := make(chan string)
	warned := map[string]bool{}

	scanOnce := func() {
		entries, err := app.Scan(daemonSoftwareID)
		if err != nil {
			log.Printf("scan: %v", err)
			return
		}
		var denied []*app.Entry
		for _, e := range app.PermissionProblems(entries) {
			if !warned[e.Info.Path] {
				warned[e.Info.Path] = true
				denied = append(denied, e)
			}
		}
		if len(denied) > 0 {
			log.Print(app.PermissionHelp(denied))
		}
		for _, e := range entries {
			if !e.Usable() || running[e.Info.Path] {
				if e.Dev != nil {
					e.Dev.Close()
				}
				continue
			}
			running[e.Info.Path] = true
			go func(e *app.Entry) {
				defer func() { done <- e.Info.Path }()
				defer e.Dev.Close()
				cfg := func() *config.Device { return watcher.Get().Device(e.Key) }
				log.Printf("%s: connected (%s)", e.Name, e.Info.Path)
				if err := app.Apply(e, cfg()); err != nil {
					log.Printf("%s: apply: %v", e.Name, err)
				}
				if !e.Ctl.HasTimers() {
					// Nothing to manage; keep the slot so we do not
					// re-apply on every scan, and wait for unplug.
					waitGone(ctx, e.Info.Path)
					return
				}
				if err := power.New(e, cfg).Run(ctx); err != nil {
					log.Printf("%s: %v", e.Name, err)
				}
			}(e)
		}
	}

	scanOnce()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			// Let the workers notice and exit.
			for len(running) > 0 {
				delete(running, <-done)
			}
			return nil
		case p := <-done:
			delete(running, p)
		case <-ticker.C:
			scanOnce()
		}
	}
}

func waitGone(ctx context.Context, path string) {
	for ctx.Err() == nil {
		if _, err := os.Stat(path); err != nil {
			return
		}
		select {
		case <-ctx.Done():
		case <-time.After(2 * time.Second):
		}
	}
}
