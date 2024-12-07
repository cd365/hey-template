//go:build wireinject
// +build wireinject

package initial

import (
	"context"
	"root/app"
	"root/provider"

	"github.com/google/wire"
)

func inject(ctx context.Context, cfg *app.Config) (*app.App, error) {
	wire.Build(provider.WireProviderSet)
	return &app.App{}, nil
}
