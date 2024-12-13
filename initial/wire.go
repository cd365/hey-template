//go:build wireinject
// +build wireinject

package initial

import (
	"context"
	"github.com/cd365/hey-template/app"
	"github.com/cd365/hey-template/provider"

	"github.com/google/wire"
)

func inject(ctx context.Context, cfg *app.Config) (*app.App, error) {
	wire.Build(provider.WireProviderSet)
	return &app.App{}, nil
}
