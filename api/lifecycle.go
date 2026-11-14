// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 WittyWilla and contributors

package api

import (
	"context"
	"encoding/json"
	"os"
	"sync"
	"time"

	"github.com/wittywilla/wittymodule"
	"github.com/wittywilla/wittymodule/host"
)

// Lifecycle answers the lifecycle calls integration.json says WittyPhotos answers: taken off without
// keeping its data, its folder (the albums' database) goes.
func Lifecycle(ctx context.Context, call string, args wittymodule.LifecycleArgs) error {
	if call == wittymodule.CallUninstall && !args.KeepData {
		if a, err := Get(); err == nil {
			a.DB.Close()
			return os.RemoveAll(a.Dir)
		}
	}
	return nil
}

var watchOnce sync.Once

// Watch follows the server's events while the service runs: an account that goes takes its albums
// with it, and so does a library.
func Watch() {
	watchOnce.Do(func() {
		go func() {
			var after int64
			for {
				a, err := Get()
				if err != nil {
					time.Sleep(30 * time.Second)
					continue
				}
				evs, next, err := a.Host.Events(context.Background(), after, 50*time.Second)
				if err != nil {
					time.Sleep(10 * time.Second)
					continue
				}
				after = next
				a.Heard(context.Background(), evs)
			}
		}()
	})
}

// Heard acts on the server's events.
func (a *App) Heard(ctx context.Context, evs []host.Event) {
	for _, e := range evs {
		var d struct {
			User    int64 `json:"user"`
			Library int64 `json:"library"`
		}
		if json.Unmarshal(e.Data, &d) != nil {
			continue
		}
		switch {
		case e.Type == "user.deleted" && d.User > 0:
			if err := a.DB.ForgetUser(ctx, d.User); err == nil {
				a.Log.Info("wittyphotos: an account's albums went with it", "cat", "library", "user", d.User)
			}
		case e.Type == "library.removed" && d.Library > 0:
			if err := a.DB.ForgetLibrary(ctx, d.Library); err == nil {
				a.Log.Info("wittyphotos: a library's albums went with it", "cat", "library", "library", d.Library)
			}
		}
	}
}
