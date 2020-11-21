package main

//Test Pull Request!

import (
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"
)

//STRUCTS
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
	operator           *operator
	que                *que
	id                 int
	itemLimit          int
	customersServed    int
	customersLost      int
	startTime          time.Time
	endTime            time.Time
	open               bool
	totalQueueWait     time.Duration
	totalScanTime      time.Duration
	percentTotalCusts  float32
	percentTimeWorking float32
	timePerCust        float32
}
type manager struct {
	restrictedTills []*checkout
}

//RECEIVER FUNCTIONS
func (cust *customer) joinQue(tills []*checkout) bool {
	for _, till := range tills {
		if till.open && till.operator != nil {
			select {
			case till.que.customers <- cust:
				cust.enterQAt = time.Now()
				return true
			default: //forces reiteration of the loop
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
var numCheckouts = 8
var checkoutsOpen = 5
var numOperators = 4
var numCusts = 150
var minItems = 1
var maxItems = 10
var minPatience = 0
var maxPatience = 1
var maxQueLength = 10
var minScanTime time.Duration = 5 * time.Microsecond * 10000
var maxScanTime time.Duration = 10 * time.Microsecond * 10000
var totalItemsProcessed = 0
var averageItemsPerTrolley = 0

var custArrivalRate time.Duration = 300 * time.Microsecond //5mins scaled secs->microsecs
var spawner = time.NewTicker(custArrivalRate)

var tills = make([]*checkout, numCheckouts)
var ops = make([]*operator, numOperators)
var custs = make(chan *customer, numCusts) //buffer length of numCusts

var wg = &sync.WaitGroup{}

func main() {
	//SETUP
	rand.Seed(time.Now().UTC().UnixNano())

	for i := range tills {
		q := make(chan *customer, maxQueLength)

		if i < checkoutsOpen {
			tills[i] = &checkout{nil, &que{q}, i + 1, math.MaxInt32, 0, 0, time.Time{}, time.Time{}, true, 0, 0, 0.0, 0.0, 0.0}
		} else {
			tills[i] = &checkout{nil, &que{q}, i + 1, math.MaxInt32, 0, 0, time.Time{}, time.Time{}, false, 0, 0, 0.0, 0.0, 0.0}
		}
	}

	for i := range ops {
		ops[i] = &operator{minScanTime, maxScanTime}

		if i < numCheckouts {
			if tills[i].open {
				tills[i].operator = ops[i]
				wg.Add(1)
			}
		}
	}

	for i := 0; i < cap(custs); i++ {
		custs <- &customer{(rand.Intn(maxItems-minItems) + minItems + 1), 3, 0, 0, time.Now(), 0}
	}

	for _, till := range tills {
		if till.open && till.operator != nil {
			go func(check *checkout, wg *sync.WaitGroup) {
				defer func() {
					wg.Done()
					check.endTime = time.Now()
				}()
				check.startTime = time.Now()
			Spin:
				for {
					select {
					case c, ok := <-check.que.customers:
						if !ok {
							break Spin
						}
						check.operator.scan(c)
						check.totalQueueWait += c.timeInQueue
						check.totalScanTime += c.timeAtTill
						check.customersServed++
						totalItemsProcessed += c.items
						fmt.Println("\nTill", check.id, "serving its", check.customersServed, "customer, who has", c.items, "items:", &c,
							"\nTime spent at till:", c.timeAtTill, "Time in queue:", c.timeInQueue)
						fmt.Println("Average wait time in queue", check.id, "=", time.Duration(int64(check.totalQueueWait)/int64(check.customersServed)))
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
	for _, till := range tills {
		totalCusts += till.customersServed
		fmt.Println("TILL", till.id, "")
		fmt.Println("  Time Open:", till.endTime.Sub(till.startTime).Truncate(time.Second))
		fmt.Println("  Customers Served:", till.customersServed)
		fmt.Println("  Total time waited by customers in queue:", till.totalQueueWait.Truncate(time.Second))
		fmt.Println("  Total time scanning:", till.totalScanTime.Truncate(time.Second), "\n")
	}

	fmt.Println("\nTotal Customers Served:", totalCusts, "\nTotal Items Processed", totalItemsProcessed, "\nAverage Items Per Trolley", int(totalItemsProcessed/totalCusts))
}
