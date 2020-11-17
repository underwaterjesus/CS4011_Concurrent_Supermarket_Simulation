package main

//Test Pull Request!


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
	timeAtTill time.Duration
	enterQAt time.Time
	timeInQueue time.Duration
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
				cust.enterQAt = time.Now()
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
	cust.timeInQueue = time.Since(cust.enterQAt)
	start := time.Now()
	for i := 0; i < n; i++ {
		r := rand.Intn(int(op.maxScanTime-op.minScanTime)) + int(op.minScanTime+1)
		time.Sleep(time.Duration(r))
	}
	cust.timeAtTill = time.Since(start)
}

//GLOBALS
//seconds scaled to microseconds(1e-6 seconds)
var numCheckouts = 5
var checkoutsOpen = 3
var numOperators = 5
var numCusts = 150
var minItems = 1
var maxItems = 10
var minPatience = 1
var maxPatience = 1
var maxQueLength int = 10
var minScanTime time.Duration = 5 * time.Microsecond *10000
var maxScanTime time.Duration = 10 * time.Microsecond *10000

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
		custs <- &customer{(rand.Intn(maxItems-minItems) + minItems + 1), 1, 0, time.Now(),0}
	}

	for _, till := range tills {
		if till.open {
			go func(check *checkout) {
				for {
					select {
					case c := <-check.que.customers:
						//end queue time here
						check.operator.scan(c)
						check.customersServed++
						fmt.Println("\nTill", check.id, "serving its", check.customersServed, "customer, who has", c.items, "items:", &c,
									"\nTime spent at till:", c.timeAtTill, "Time in queue:", c.timeInQueue)
					default:
						continue
					}
				}
			}(till)
		}
		
	}

	//does not need to be goroutine atm, but probably will later
SpawnLoop:
	for c := range custs {
		for {
			select {
			case <-spawner.C:
				if c.joinQue(tills) {
					continue SpawnLoop //joined que
				} else {
					continue SpawnLoop //leave store
				}
			default:
				continue
			}
		}
	}
}
