/*
Copyright 2021 The Kubernetes Authors.

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
	"flag"
	"io/ioutil"
	"time"

	"github.com/GoogleCloudPlatform/testgrid/pkg/merger"
	"github.com/GoogleCloudPlatform/testgrid/util/gcs"

	"github.com/sirupsen/logrus"
)

type options struct {
	listPath     string
	creds        string
	confirm      bool
	wait         time.Duration
	skipValidate bool
}

func (o *options) validate(log logrus.FieldLogger) {
	if o.listPath == "" {
		log.Fatal("--config-list of configurations to merge required")
	}
	if !o.confirm {
		log.Info("--confirm=false (DRY-RUN): will not write to gcs")
	}
	if o.skipValidate {
		log.Info("--allow-invalid-configs: result may not validate either")
	}
}

func gatherOptions() options {
	var o options
	flag.StringVar(&o.listPath, "config-list", "", "List of configurations to merge")
	flag.StringVar(&o.creds, "gcp-service-account", "", "/path/to/gcp/creds (use local creds if empty)")
	flag.BoolVar(&o.confirm, "confirm", false, "Upload data if set")
	flag.DurationVar(&o.wait, "wait", 0, "Ensure at least this much time ahs passed since the last loop. (Run only once if zero)")
	flag.BoolVar(&o.skipValidate, "allow-invalid-configs", false, "Allows merging of configs that don't validate. Usually skips invalid configs")
	flag.Parse()
	return o
}

func main() {
	log := logrus.WithField("component", "config-merger")

	opt := gatherOptions()
	opt.validate(log)
	file, err := ioutil.ReadFile(opt.listPath)
	if err != nil {
		log.WithField("--config-list", opt.listPath).WithError(err).Fatalf("Can't find --config-list")
	}

	list, err := merger.ParseAndCheck(file)
	if err != nil {
		log.WithField("--config-list", opt.listPath).WithError(err).Fatal("Can't parse --config-list")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	storageClient, err := gcs.ClientWithCreds(ctx, opt.creds)
	if err != nil {
		log.WithError(err).Fatalf("Can't make storage client")
	}

	client := gcs.NewClient(storageClient)

	updateOnce := func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
		defer cancel()
		return merger.MergeAndUpdate(ctx, client, list, opt.skipValidate, opt.confirm)
	}

	if err := updateOnce(ctx); err != nil {
		log.WithError(err).Error("Failed update")
	}
	if opt.wait == 0 {
		return
	}
	timer := time.NewTimer(opt.wait)
	defer timer.Stop()
	for range timer.C {
		timer.Reset(opt.wait)
		if err := updateOnce(ctx); err != nil {
			log.WithError(err).Error("Failed update")
		}
		log.WithField("--wait", opt.wait).Info("Sleeping")
	}
}
