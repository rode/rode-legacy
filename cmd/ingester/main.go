package main

import "runtime"

func main() {
	println("hello world!")
	for {
		runtime.Gosched()
	}
}
