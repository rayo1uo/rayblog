package main

import (
	"fmt"

	"golang.org/x/time/rate"
)

func main() {
	r := rate.NewLimiter(1, 5)
	for i := 0; i < 10; i++ {
		if r.Allow() {
			fmt.Println("Handle event", i)
		} else {
			fmt.Println("Rate limited", i)
		}
	}
}
