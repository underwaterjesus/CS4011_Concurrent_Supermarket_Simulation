package main

//Test Pull Request!

import (
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"
)

//STUCTS
type metric struct {
	id                  int
	customersServed     int
	customersLeft       int
	numCustomers        int
	totalQueueWait      time.Duration
	totalCheckoutTime   time.Duration
	checkoutUtilisation float32
}

type customer struct {
	items       int
	patience    int
	tillJoined  int
	timeAtTill  time.Duration
	enterQAt    time.Time
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
var checkoutsOpen = 5
var numOperators = 5
var numCusts = 12
var minItems = 1
var maxItems = 10
var minPatience = 0
var maxPatience = 1
var maxQueLength int = 10
var minScanTime time.Duration = 5 * time.Microsecond * 10000
var maxScanTime time.Duration = 10 * time.Microsecond * 10000

var custArrivalRate time.Duration = 300 * time.Microsecond //5mins scaled secs->microsecs
var spawner = time.NewTicker(custArrivalRate)

var tills = make([]*checkout, numCheckouts)
var ops = make([]*operator, numOperators)
var custs = make(chan *customer, numCusts)

var wg = &sync.WaitGroup{}

//thinking about a table per till for the moment to keep track of stats.
//can combine them for final output?
//Probably a way to do this with channels...
var metrics = make([]*metric, numCheckouts)

type manager struct {
	staff []*operator
}

func main() {
	//SETUP
	rand.Seed(time.Now().UTC().UnixNano())
	wg.Add(numCheckouts)

	for i := range tills {
		q := make(chan *customer, maxQueLength)

		if i < checkoutsOpen {
			tills[i] = &checkout{nil, &que{q}, i + 1, math.MaxInt32, 0, 0, 0, true}
			metrics[i] = &metric{i + 1, 0, 0, 0, 0, 0, 0.0}
		} else {
			tills[i] = &checkout{nil, &que{q}, i + 1, math.MaxInt32, 0, 0, 0, false}
			metrics[i] = &metric{i + 1, 0, 0, 0, 0, 0, 0.0}
		}
	}

	for i := range ops {
		ops[i] = &operator{minScanTime, maxScanTime}

		if i < numCheckouts {
			if tills[i].open {
				tills[i].operator = ops[i]
			}
		}
	}

	for i := 0; i < cap(custs); i++ {
		custs <- &customer{(rand.Intn(maxItems-minItems) + minItems + 1), 3, 0, 0, time.Now(), 0}
	}

	for _, till := range tills {
		if till.open {
			go func(check *checkout, wg *sync.WaitGroup) {
				defer wg.Done()
			Spin:
				for {
					select {
					case c, ok := <-check.que.customers:
						if !ok {
							break Spin
						}
						check.operator.scan(c)
						metrics[check.id-1].totalQueueWait += c.timeInQueue
						metrics[check.id-1].totalCheckoutTime += c.timeAtTill
						metrics[check.id-1].numCustomers++
						check.customersServed++
						fmt.Println("\nTill", check.id, "serving its", check.customersServed, "customer, who has", c.items, "items:", &c,
							"\nTime spent at till:", c.timeAtTill, "Time in queue:", c.timeInQueue)
						fmt.Println("Average wait time in queue", check.id, "=", time.Duration(int64(metrics[check.id-1].totalQueueWait)/int64(metrics[check.id-1].numCustomers)))
					default:
						continue
					}
				}
			}(till, wg)
		}

	}

	//does not need to be goroutine atm, but probably will later
SpawnLoop:
	for {
		select {
		case <-spawner.C:
			select {
			case c, ok := <-custs:
				if !ok {
					break SpawnLoop
				}
				for !c.joinQue(tills) {

				}
			default:
				break SpawnLoop
			}
		default:
			continue
		}
	}

	for _, till := range tills {
		close(till.que.customers)
	}

	wg.Wait()
	fmt.Println()
	totalCusts := 0
	for _, m := range metrics {
		totalCusts += m.numCustomers
		fmt.Println("TILL", m.id, "")
		fmt.Println(" Customers Served:", m.numCustomers)
		fmt.Println(" Total time waited by customers in queue:", m.totalQueueWait)
		fmt.Println(" Total time scanning:", m.totalCheckoutTime, "\n")
	}

	fmt.Println("\nTotal Customers Served:", totalCusts)
}
