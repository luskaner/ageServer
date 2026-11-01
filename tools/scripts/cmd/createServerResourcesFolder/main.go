package main

import (
	i "scripts/internal"
)

func main() {
	dst := i.BuildResourcePath("server")
	i.MkdirP(dst)
}
