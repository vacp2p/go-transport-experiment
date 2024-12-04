package main

//go:generate protoc -I. --go_opt=paths=source_relative --go_opt=Mmessage.proto=./  --go_out=. ./message.proto
