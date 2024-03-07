package main

import (
	caddycmd "github.com/caddyserver/caddy/v2/cmd"

	_ "github.com/caddyserver/caddy/v2/modules/standard"

	_ "github.com/gfx-labs/caddy-git-adapter"
)

func main() {
	caddycmd.Main()
}
