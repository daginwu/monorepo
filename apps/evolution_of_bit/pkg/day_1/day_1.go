package day_1

import (
	"fmt"
	"time"
)

type Life struct {
}

func (l *Life) Live() {
	for {
		fmt.Println("I'm dagin wu's Life", ", and it's ", time.Now())
		time.Sleep(3 * time.Second)
	}
}
