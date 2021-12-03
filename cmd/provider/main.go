/*
Copyright 2021 The Crossplane Authors.

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

package main

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"os"
	"path/filepath"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"gopkg.in/alecthomas/kingpin.v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"

	"github.com/crossplane-contrib/provider-kubernetes/apis"
	"github.com/crossplane-contrib/provider-kubernetes/internal/controller"
)

func main() {
	var (
		app            = kingpin.New(filepath.Base(os.Args[0]), "Template support for Crossplane.").DefaultEnvars()
		debug          = app.Flag("debug", "Run with debug logging.").Short('d').Bool()
		syncInterval   = app.Flag("sync", "Controller manager sync period such as 300ms, 1.5h, or 2h45m").Short('s').Default("1h").Duration()
		pollInterval   = app.Flag("poll", "Poll interval controls how often an individual resource should be checked for drift.").Default("1m").Duration()
		leaderElection = app.Flag("leader-election", "Use leader election for the controller manager.").Short('l').Default("false").OverrideDefaultFromEnvar("LEADER_ELECTION").Bool()
	)
	kingpin.MustParse(app.Parse(os.Args[1:]))

	zl := zap.New(zap.UseDevMode(*debug))
	log := logging.NewLogrLogger(zl.WithName("provider-kubernetes"))
	if *debug {
		// The controller-runtime runs with a no-op logger by default. It is
		// *very* verbose even at info level, so we only provide it a real
		// logger when we're running in debug mode.
		ctrl.SetLogger(zl)
	}

	log.Debug("Starting", "sync-period", syncInterval.String())

	cfg, err := ctrl.GetConfig()
	kingpin.FatalIfError(err, "Cannot get API server rest config")

	var namespaces []string
	cfn, err := client.New(cfg, client.Options{})
	if err != nil {
		log.Debug("Cannot create client for nss", "error", err)
	}
	nss := &unstructured.Unstructured{}
	nss.SetGroupVersionKind(schema.GroupVersionKind{Version: "operator.ibm.com/v1", Kind: "NamespaceScope"})
	if err := cfn.Get(context.Background(), types.NamespacedName{Namespace: "ibm-common-services", Name: "common-service"}, nss); err != nil {
		kingpin.FatalIfError(err, "Cannot get NamespaceScope common-service")
	}

	spec := nss.Object["spec"].(map[string]interface{})
	ms := spec["namespaceMembers"]
	if ms != nil {
		for _, m := range ms.([]interface{}) {
			namespaces = append(namespaces, fmt.Sprintf("%v", m))
		}
	}

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		LeaderElection:   *leaderElection,
		LeaderElectionID: "crossplane-leader-election-provider-kubernetes",
		SyncPeriod:       syncInterval,
		NewCache:		  cache.MultiNamespacedCacheBuilder(namespaces),
	})
	kingpin.FatalIfError(err, "Cannot create controller manager")

	rl := ratelimiter.NewDefaultProviderRateLimiter(ratelimiter.DefaultProviderRPS)
	kingpin.FatalIfError(apis.AddToScheme(mgr.GetScheme()), "Cannot add Template APIs to scheme")
	kingpin.FatalIfError(controller.Setup(mgr, log, rl, *pollInterval), "Cannot setup Template controllers")
	kingpin.FatalIfError(mgr.Start(ctrl.SetupSignalHandler()), "Cannot start controller manager")
}
