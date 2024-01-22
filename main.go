package main

import (
	"github.com/saiset-co/saiCosmosInteraction/internal"
	"github.com/saiset-co/saiService"
)

func main() {
	svc := saiService.NewService("saiCosmosInteraction")
	is := internal.InternalService{Context: svc.Context}

	svc.RegisterConfig("config.yml")

	svc.RegisterInitTask(is.Init)

	svc.RegisterHandlers(
		is.NewHandler(),
	)

	svc.Start()
}
