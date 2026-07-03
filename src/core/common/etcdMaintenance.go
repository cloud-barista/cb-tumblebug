/*
Copyright 2019 The Cloud-Barista Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package common

import (
	"context"
	"os"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	"github.com/rs/zerolog/log"
)

// etcdMaintenanceInterval controls how often StartEtcdMaintenanceLoop runs
// Compact+Defragment. etcd's own --auto-compaction-* flags (if set) only
// mark old MVCC revisions as reclaimable; they never shrink the on-disk
// database file. Without periodic defrag, ordinary application writes
// accumulate revision history forever and can eventually exhaust etcd's
// default 2GiB backend quota (NOSPACE / "database space exceeded"). A value
// <= 0 disables the loop. Override with TB_ETCD_MAINTENANCE_INTERVAL (Go
// duration syntax, e.g. "12h").
var etcdMaintenanceInterval = func() time.Duration {
	if v := os.Getenv("TB_ETCD_MAINTENANCE_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			log.Warn().Str("value", v).Msg("etcd maintenance: invalid TB_ETCD_MAINTENANCE_INTERVAL, using default")
		} else {
			return d
		}
	}
	return 24 * time.Hour
}()

// StartEtcdMaintenanceLoop runs etcd Compact+Defragment once immediately and
// then on a fixed interval for the lifetime of the process. Call once, after
// the kvstore has been initialized (see setupAndWaitForInternalServices in
// main.go). Safe for a single-node etcd deployment; Defragment already
// serializes across endpoints if ever pointed at a multi-member cluster.
func StartEtcdMaintenanceLoop() {
	if etcdMaintenanceInterval <= 0 {
		log.Info().Msg("etcd maintenance: disabled (TB_ETCD_MAINTENANCE_INTERVAL <= 0)")
		return
	}

	log.Info().Dur("interval", etcdMaintenanceInterval).Msg("etcd maintenance: starting periodic compact+defrag loop")

	go func() {
		runEtcdMaintenance()

		ticker := time.NewTicker(etcdMaintenanceInterval)
		defer ticker.Stop()
		for range ticker.C {
			runEtcdMaintenance()
		}
	}()
}

func runEtcdMaintenance() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := kvstore.Compact(ctx); err != nil {
		log.Error().Err(err).Msg("etcd maintenance: compact failed")
		return
	}
	if err := kvstore.Defragment(ctx); err != nil {
		log.Error().Err(err).Msg("etcd maintenance: defragment failed")
		return
	}
	log.Info().Msg("etcd maintenance: compact+defrag completed successfully")
}
