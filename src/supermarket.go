package main

import (
	"fmt"
	"math"
	"math/rand"
	"time"
)

//STUCTS
type customer struct {
	items    int
	patience int
}

type operator struct {
	minScanTime time.Duration
	maxScanTime time.Duration
}

type que struct {
	customers chan *customer
}

type checkout struct {
	operator        *operator
	que             *que
	id              int
	itemLimit       int
	customersServed int
	idleTime        int
	useTime         int
	open            bool
}

//RECEIVER FUNCTIONS
func (cust *customer) joinQue(tills []*checkout) bool {
	for _, till := range tills {
		if till.open {
			select {
			case till.que.customers <- cust:
				return true
			default:
				continue
			}
		}
	}
	return false
}

func (till *que) moveAlong() {
	<-till.customers
}

func (op *operator) scan(cust *customer) {
	n := cust.items
	for i := 0; i < n; i++ {
		r := rand.Intn(int(op.maxScanTime-op.minScanTime)) + int(op.minScanTime+1)
		time.Sleep(time.Duration(r))
	}
}

//GLOBALS
//seconds scaled to microseconds(1e-6 seconds)
var numCheckouts = 5
var checkoutsOpen = 3
var numOperators = 5
var numCusts = 100
var minItems = 1
var maxItems = 10
var minPatience = 1
var maxPatience = 1
var maxQueLength int = 5
var minScanTime time.Duration = 5 * time.Microsecond
var maxScanTime time.Duration = 10 * time.Microsecond

var custArrivalRate time.Duration = 300 * time.Microsecond //5mins scaled secs->microsecs
var spawner = time.NewTicker(custArrivalRate)

var tills = make([]*checkout, numCheckouts)
var ops = make([]*operator, numOperators)
var custs = make(chan *customer, numCusts)

type manager struct {
	staff []*operator
}

func main() {
	//SETUP
	for i, _ := range tills {
		q := make(chan *customer, maxQueLength)

		if i < checkoutsOpen {
			tills[i] = &checkout{nil, &que{q}, i + 1, math.MaxInt32, 0, 0, 0, true}
		} else {
			tills[i] = &checkout{nil, &que{q}, i + 1, math.MaxInt32, 0, 0, 0, false}
		}
	}

	for i, _ := range ops {
		ops[i] = &operator{minScanTime, maxScanTime}

		if i < numCheckouts {
			if tills[i].open {
				tills[i].operator = ops[i]
			}
		}
	}

	for i := 0; i < cap(custs); i++ {
		custs <- &customer{(rand.Intn(maxItems-minItems) + minItems + 1), 1}
	}

	for _, till := range tills {
		if till.open {
			//fmt.Println("outer for loop")
			go func(check *checkout) {
				//fmt.Println("goroutine")
				for { //figure out how to stop loop
					//fmt.Println("inner for loop")
					select {
					case c := <-check.que.customers:
						check.operator.scan(c)
						check.customersServed++
						fmt.Println("Till", check.id, "serving its", check.customersServed, "customer, who has", c.items, "items:", &c)
					default:
						//fmt.Println("default")
						continue
					}
				}
			}(till)
		}
	}

	//does not need to be goroutine atm, but probably will later
	//go func() {
	//fmt.Println("goroutine")
SpawnLoop:
	for c := range custs {
		//fmt.Println("outer for loop")
		for {
			//fmt.Println("inner for loop")
			select {
			case <-spawner.C:
				//fmt.Println("Tick")
				if c.joinQue(tills) {
					continue SpawnLoop //joined que
				} else {
					continue SpawnLoop //leave store
				}
			default:
				//fmt.Println("default")
				continue
			}
		}
	}
	//}()
}
