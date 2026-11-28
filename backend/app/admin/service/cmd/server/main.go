package main

import (
	app "go-wind-uba/app/admin/service/app"
)

var version = "1.0.0"

// go build -ldflags "-X main.version=x.y.z"

func main() {
	if err := app.RunApp(version); err != nil {
		panic(err)
	}
}
