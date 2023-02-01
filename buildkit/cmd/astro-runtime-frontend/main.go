package main

import (
	"github.com/astronomer/astro-runtime-frontend/internal/frontend"
	"github.com/moby/buildkit/frontend/gateway/grpcclient"
	"github.com/moby/buildkit/util/appcontext"
	"github.com/sirupsen/logrus"
)

func main() {
	if err := grpcclient.RunFromEnvironment(appcontext.Context(), frontend.Build); err != nil {
		logrus.Errorf("fatal error: %+v", err)
		panic(err)
	}
}
