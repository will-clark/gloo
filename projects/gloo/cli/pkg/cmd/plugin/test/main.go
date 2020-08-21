package main

import (
	"context"

	"github.com/solo-io/gloo/projects/gloo/cli/pkg/cmd/plugin/common"
	"github.com/solo-io/gloo/projects/gloo/cli/pkg/constants"
)

func main() {
	registry, err := common.NewGcsRegistry(context.TODO(), constants.GlooctlPluginBucket)
	if err != nil {
		panic(err)
	}

	registry.Search(context.TODO(), "")
}
