package main

import (
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"
)

//STUCTS
type customer struct {
	items       int
	tillJoined  int
	enterQAt    time.Time
	patience    time.Duration
	timeAtTill  time.Duration
	timeInQueue time.Duration
}

type operator struct {
	minScanTime time.Duration
	maxScanTime time.Duration
}

type queue struct {
	customers chan *customer
}

type checkout struct {
	operator           *operator
	queue              *queue
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

//RECEIVER FUNCTIONS
func (cust *customer) joinQue(tills []*checkout) bool {
	for _, till := range tills {
		if till.open && till.operator != nil {
			select {
			case till.queue.customers <- cust:
				cust.enterQAt = time.Now()
				return true
			default:
				continue
			}
		}
	}
	return false
}

func (till *queue) moveAlong() {
	<-till.customers
}

func (cust *customer) checkPatience() bool {
	timeWaited := time.Since(cust.enterQAt)
	if timeWaited > (cust.patience) { //need to address the int issue here
		return false
	}
	return true
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
var custsLost = 0
var minItems = 1
var maxItems = 15
var minPatience = 0
var maxPatience = 1
var maxQueueLength = 10
var minScanTime time.Duration = 5 * time.Microsecond
var maxScanTime time.Duration = 10 * time.Microsecond

var custArrivalRate time.Duration = 300 * time.Microsecond //5mins scaled secs->microsecs
var spawner = time.NewTicker(custArrivalRate)
var tick = time.NewTicker(custArrivalRate / 10)

var tills = make([]*checkout, numCheckouts)
var ops = make([]*operator, numOperators)
var custs = make(chan *customer, numCusts)

var wg = &sync.WaitGroup{}

type manager struct {
	staff []*operator
}

func main() {
	//SETUP
	rand.Seed(time.Now().UTC().UnixNano())

	//This seems like an appropriate place for the time mark,
	//like when the manager first opens the door to the market at the start of the day.
	simStart := time.Now()

	//checkout setup
	for i := range tills {
		q := make(chan *customer, maxQueueLength)

		//checkout(operator, queue, id, itemLimit, customersServed, startTime, endTime, open, totalQueueWait,
		//		   totalScanTime, percentTotalCusts, percentTimeWorking, timePerCust)
		if i < checkoutsOpen {
			tills[i] = &checkout{nil, &queue{q}, i + 1, math.MaxInt32, 0, 0, time.Time{}, time.Time{}, true, 0, 0, 0.0, 0.0, 0.0}
		} else {
			tills[i] = &checkout{nil, &queue{q}, i + 1, math.MaxInt32, 0, 0, time.Time{}, time.Time{}, false, 0, 0, 0.0, 0.0, 0.0}
		}
	}

	//checkout operator setup
	for i := range ops {
		ops[i] = &operator{minScanTime, maxScanTime}

		if i < numCheckouts {
			if tills[i].open {
				tills[i].operator = ops[i]
				wg.Add(1)
			}
		}
	}

	//create customers and send them to the cust channel
	for i := 0; i < cap(custs); i++ {
		custs <- &customer{(rand.Intn(maxItems-minItems) + minItems + 1), 3, time.Now(), 0, 0, time.Second}
	}

	//process customers at tills.
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
					case c, ok := <-check.queue.customers:
						if !ok {
							break Spin
						}

						check.operator.scan(c)

						check.totalQueueWait += c.timeInQueue
						check.totalScanTime += c.timeAtTill
						check.customersServed++
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
				if !c.joinQue(tills) {
					custsLost++
					fmt.Println("A customer left")
				}

			default:
				break SpawnLoop
			}
		default:
			continue
		}
	}

	spawner.Stop()

	for _, till := range tills {
		close(till.queue.customers)
	}

	wg.Wait()
	simRunTime := time.Since(simStart)
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

	fmt.Println("\nTotal Customers Served:", totalCusts)
	fmt.Println("\nTotal Customers Lost  :", custsLost)
	fmt.Println("\nSim RunTime", simRunTime.Truncate(time.Second))
}
