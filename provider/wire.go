package provider

import (
	"github.com/google/wire"
	"root/app"
)

var WireProviderSet = wire.NewSet(
	app.NewApp,
)
