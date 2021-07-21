package main

import (
	"github.com/daginwu/monorepo/apps/evolution_of_bit/pkg/day_3"
)

func main() {
	life := day_3.NewLife(
		day_3.LifeWithName("dagin_wu_"),
	)
	life.Live()
}
