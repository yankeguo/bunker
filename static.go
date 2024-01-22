package main

import "embed"

//go:embed ui/.output/public ui/.output/public/**/*
var STATIC embed.FS
