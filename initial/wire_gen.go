// Code generated by Wire. DO NOT EDIT.

//go:generate go run -mod=mod github.com/google/wire/cmd/wire
//go:build !wireinject
// +build !wireinject

package initial

import (
	"context"
	"github.com/cd365/hey-template/app"
)

// Injectors from wire.go:

func inject(ctx context.Context, cfg *app.Config) (*app.App, error) {
	appApp := app.NewApp(ctx, cfg)
	return appApp, nil
}
