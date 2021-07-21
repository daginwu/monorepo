package day_3

import "github.com/daginwu/monorepo/apps/evolution_of_bit/pkg/day_2"

type Life struct {
	day_2.Life
	Name string
	ID   string
}

type LifeOption func(*Life)

func LifeWithName(name string) LifeOption {
	return func(l *Life) {
		l.Name = name
	}
}

func NewLife(opts ...LifeOption) *Life {

	l := &Life{}

	// Loop through each option
	for _, opt := range opts {
		// Call the option giving the instantiated
		// *House as the argument
		opt(l)
	}

	// return the modified house instance
	return l
}
