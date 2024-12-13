package provider

import (
	"github.com/cd365/hey-template/app"
	"github.com/google/wire"
)

var WireProviderSet = wire.NewSet(
	app.NewApp,
)
