package day_2

import (
	"bufio"
	"fmt"
	"os"
	"time"

	"github.com/daginwu/monorepo/apps/evolution_of_bit/pkg/day_1"
)

type Life struct {
	day_1.Life
	StdinSensor
}

func (l *Life) Live() {
	go l.StdinSensor.Run()
	for {
		// fmt.Println("I'm dagin wu's Life", ", and it's ", time.Now())
		time.Sleep(10 * time.Second)
	}
}

type StdinSensor struct {
}

func (ss *StdinSensor) Run() {
	for {
		// Request
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Stdin to Dagin wu Life > ")
		cmd, _ := reader.ReadString('\n')

		// Response
		fmt.Println(cmd)
	}
}
