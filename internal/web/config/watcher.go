// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package config

import (
	"context"
	"fmt"
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	toolscache "k8s.io/client-go/tools/cache"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlcache "sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

const (
	// syncPeriod defines how often the secret cache
	// should re-sync to recover from missed events.
	// Note: Currently, the minimum interval applied
	// internally by controller-runtime is 5 minutes
	// to avoid excessive load on the API server. We
	// set it to 1 minute here to declare our intent,
	// even if the effective value will be higher. If
	// the internal behavior changes in the future,
	// this will ensure a more responsive watcher.
	syncPeriod = time.Minute
)

// WatchSecret monitors the given secret in the specified namespace for changes
// and sends the updated configuration on the returned channel. It also returns
// a channel that is closed when the watcher stops.
func WatchSecret(ctx context.Context, name, namespace string,
	restConfig *rest.Config) (<-chan *fluxcdv1.WebConfigSpec, <-chan struct{}, error) {

	l := ctrl.Log.WithName("web-config-watcher").WithValues("secretRef", map[string]any{
		"name":      name,
		"namespace": namespace,
	})

	// Build a cache to watch the secret.
	cache, err := ctrlcache.New(restConfig, ctrlcache.Options{
		SyncPeriod: new(syncPeriod),
		ByObject: map[client.Object]ctrlcache.ByObject{
			&corev1.Secret{}: {
				Field: fields.SelectorFromSet(fields.Set{
					"metadata.name":      name,
					"metadata.namespace": namespace,
				}),
			},
		},
	})
	if err != nil {
		return nil, nil, err
	}

	// Register event handlers to watch for secret changes.
	getBytesAndVersion := func(obj any) ([]byte, string) {
		secret, ok := obj.(*corev1.Secret)
		if !ok {
			return nil, ""
		}
		if secret.Namespace != namespace || secret.Name != name {
			return nil, ""
		}
		b, exists := secret.Data["config.yaml"]
		if !exists || len(b) == 0 {
			l.Error(
				fmt.Errorf("'config.yaml' key is empty or not present in the web configuration secret"),
				"failed to process web configuration secret")
			return nil, ""
		}
		return b, secret.ResourceVersion
	}
	setupCtx, cancelSetupCtx := context.WithTimeout(ctx, 10*time.Second)
	defer cancelSetupCtx()
	secretKind := schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Secret"}
	informer, err := cache.GetInformerForKind(setupCtx, secretKind)
	if err != nil {
		return nil, nil, err
	}
	confChannel := make(chan *fluxcdv1.WebConfigSpec, 10)
	_, err = informer.AddEventHandler(toolscache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			b, version := getBytesAndVersion(obj)
			if len(b) == 0 {
				return
			}
			conf, err := parse(b)
			if err != nil {
				l.Error(err, "failed to load web configuration from secret")
				return
			}
			conf.Version = version
			confChannel <- conf
		},
		UpdateFunc: func(oldObj, newObj any) {
			oldBytes, _ := getBytesAndVersion(oldObj)
			newBytes, newVersion := getBytesAndVersion(newObj)
			if len(newBytes) == 0 || string(oldBytes) == string(newBytes) {
				return
			}
			conf, err := parse(newBytes)
			if err != nil {
				l.Error(err, "failed to load web configuration from secret")
				return
			}
			conf.Version = newVersion
			confChannel <- conf
		},
	})
	if err != nil {
		return nil, nil, err
	}

	// Start the cache on a separate goroutine.
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		l.Info("starting watcher for web configuration secret")
		if err := cache.Start(ctx); err != nil {
			l.Error(err, "unable to start watcher for web configuration secret")
			os.Exit(1)
		}
	}()

	return confChannel, stopped, nil
}
