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
	"strings"

	"os"
	"path/filepath"

	"gopkg.in/alecthomas/kingpin.v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	ca "k8s.io/client-go/tools/cache"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

	// IBM Patch: reduce cluster permission
	// we want to restrict cache to watch only a given list of namespaces
	// instead of all (cluster scoped). List of namespaces is read
	// from NamespaceScope ConfigMap, if it exists. Changes in this ConfigMap
	// should restart Provider's pod.
	watchNamespace := os.Getenv("WATCH_NAMESPACE")
	if watchNamespace == "" {
		kingpin.FatalIfError(err, "Empty WATCH_NAMESPACE env variable")
	}
	// By default set at least watchNamespace
	namespaces := []string{watchNamespace}
	// Name of ConfigMap created by NamespaceScope Operator
	// as the result of aggregating NamespaceScope custom resources
	nssCM := "namespace-scope"

	cfn, err := client.New(cfg, client.Options{})
	kingpin.FatalIfError(err, "Cannot create client for reading NamespaceScope ConfigMap")

	nfn, err := namespacesFromNssConfigMap(cfn, watchNamespace, nssCM)
	kingpin.FatalIfError(err, "Cannot read namespaces from NamespaceScope ConfigMap")

	namespaces = append(namespaces, nfn...)
	// Start informer to watch for changes in ConfigMap
	dc, err := dynamic.NewForConfig(cfg)
	kingpin.FatalIfError(err, "Cannot create client for observing NamespaceScope ConfigMap")

	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(dc, 0, watchNamespace, nil)
	informer := factory.ForResource(schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "configmaps",
	})
	stopper := make(chan struct{})
	defer close(stopper)
	// Handle each update causing the pod to restart.
	handlers := ca.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj, newObj interface{}) {
			if newObj.(*unstructured.Unstructured).GetName() == nssCM {
				log.Debug("Observed NamespaceScope ConfigMap has been updated, restarting")
				os.Exit(1)
			}
		},
	}
	informer.Informer().AddEventHandler(handlers)
	go informer.Informer().Run(stopper)
	log.Debug(fmt.Sprintf("Starting watch on NamespaceScope ConfigMap %s", nssCM))

	log.Debug(fmt.Sprintf("Creating multinamespaced cache with namespaces: %+q", namespaces))

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		LeaderElection:   *leaderElection,
		LeaderElectionID: "crossplane-leader-election-provider-kubernetes",
		SyncPeriod:       syncInterval,
		NewCache:         cache.MultiNamespacedCacheBuilder(namespaces),
	})
	// IBM Patch end: reduce cluster permission
	kingpin.FatalIfError(err, "Cannot create controller manager")

	rl := ratelimiter.NewDefaultProviderRateLimiter(ratelimiter.DefaultProviderRPS)
	kingpin.FatalIfError(apis.AddToScheme(mgr.GetScheme()), "Cannot add Template APIs to scheme")
	kingpin.FatalIfError(controller.Setup(mgr, log, rl, *pollInterval), "Cannot setup Template controllers")
	kingpin.FatalIfError(mgr.Start(ctrl.SetupSignalHandler()), "Cannot start controller manager")
}

func namespacesFromNssConfigMap(cfn client.Client, watchNamespace string, nssCM string) ([]string, error) {
	var namespaces []string
	cm := &unstructured.Unstructured{}
	cm.SetGroupVersionKind(schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"})
	if err := cfn.Get(context.Background(), types.NamespacedName{Namespace: watchNamespace, Name: nssCM}, cm); err != nil {
		return namespaces, err
	}
	data := cm.Object["data"].(map[string]interface{})
	members := data["namespaces"]
	if members != nil {
		ms := strings.Split(members.(string), ",")
		for _, m := range ms {
			if m == watchNamespace {
				continue
			}
			namespaces = append(namespaces, m)
		}
	}
	return namespaces, nil
}
