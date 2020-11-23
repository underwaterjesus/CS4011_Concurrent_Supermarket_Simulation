package main

import (
	"fmt"
	"sort"
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
	scanTime time.Duration
	
}

type queue struct {
	customers chan *customer
}

type manager struct {
	name 			string
	cappedCheckRate	int
	itemLimit		int
	isSmart			bool
	isQuikCheck		bool
	
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
	numInQ			   int32
}

type byQLength []*checkout

func (a byQLength) Len() int           { return len(a) }
func (a byQLength) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byQLength) Less(i, j int) bool { return a[i].numInQ < a[j].numInQ }

type byTillID []*checkout

func (a byTillID) Len() int           { return len(a) }
func (a byTillID) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byTillID) Less(i, j int) bool { return a[i].id < a[j].id }

type byScanTime []*operator

func (a byScanTime) Len() int           { return len(a) }
func (a byScanTime) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byScanTime) Less(i, j int) bool { return int(a[i].scanTime) < int(a[j].scanTime) }

//RECEIVER FUNCTIONS
func (cust *customer) joinQue(tills []*checkout, items int) bool {

	if(smartCusts){
		mutex.Lock()
		sort.Sort(byQLength(tills))
		mutex.Unlock()
	}
	
	for _, till := range tills {
		if till.open && till.operator != nil && items < till.itemLimit {
			select {
			case till.queue.customers <- cust:
				cust.enterQAt = time.Now()
				till.numInQ++			
				return true

			default:
				continue
			}
		}
	}

	return false
}

func (manager *manager) sortOperators() {
	sort.Sort(byScanTime(ops))
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
		r := op.scanTime
		time.Sleep(time.Duration(r))
	}
	cust.timeAtTill = time.Since(start)
}

//GLOBALS
//seconds scaled to microseconds(1e-6 seconds)
const maxItem = 2147483647
var scale int64 = 1000
var numCheckouts = 8
var checkoutsOpen = 8
var numOperators = 8
var numCusts = 100
var custsLost = 0
var minItems = 1
var maxItems = 90
var minPatience = 0
var maxPatience = 1
var maxQueueLength = 6
var smartCusts = false
var minScanTime time.Duration = 5 * time.Microsecond * 1000
var maxScanTime time.Duration = 60 * time.Microsecond * 1000
var custArrivalRate time.Duration = 30 * time.Microsecond * 1000 //5mins scaled secs->microsecs
var spawner = time.NewTicker(custArrivalRate)
var tick = time.NewTicker(custArrivalRate / 10)

var mutex = &sync.Mutex{}
var tills = make([]*checkout, numCheckouts)
var ops = make([]*operator, numOperators)
var custs = make(chan *customer, numCusts)
var mrManager manager


var wg = &sync.WaitGroup{}

func main() {
	//SETUP
	rand.Seed(time.Now().UTC().UnixNano())

	//This seems like an appropriate place for the time mark,
	//like when the manager first opens the door to the market at the start of the day.
	mrManager.name = "Mr. Manager"
	mrManager.cappedCheckRate = rand.Intn(int(checkoutsOpen/2))
	mrManager.itemLimit = 5
	mrManager.isSmart = true
	mrManager.isQuikCheck = true
	
	
	

	//checkout setup
	for i := range tills {
		q := make(chan *customer, maxQueueLength)

		//checkout(operator, queue, id, itemLimit, customersServed, startTime, endTime, open, totalQueueWait,
		//		   totalScanTime, percentTotalCusts, percentTimeWorking, timePerCust)
		if i < checkoutsOpen {
			if i < mrManager.cappedCheckRate {
				tills[i] = &checkout{nil, &queue{q}, i + 1, mrManager.itemLimit, 0, 0, time.Time{}, time.Time{}, true, 0, 0, 0.0, 0.0, 0.0, 0}
			} else {
				tills[i] = &checkout{nil, &queue{q}, i + 1, maxItem, 0, 0, time.Time{}, time.Time{}, true, 0, 0, 0.0, 0.0, 0.0, 0}
			}
		} else {

			tills[i] = &checkout{nil, &queue{q}, i + 1, maxItem, 0, 0, time.Time{}, time.Time{}, true, 0, 0, 0.0, 0.0, 0.0, 0}
			
		}
		
	}

	//checkout operator setup
	for i := range ops {
		ops[i] = &operator{ time.Duration(rand.Intn(int(maxScanTime - minScanTime) + int(minScanTime+1)) ) }

		if i < numCheckouts {
			if tills[i].open {
				tills[i].operator = ops[i]
				wg.Add(1)
			}
		}
	}

	//Mr Manager is a good manager and makes sure to always pick the quickest available operator.
	if(mrManager.isSmart){
		mrManager.sortOperators()
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



						check.numInQ--
						//Keep this in mind ^^^

						check.operator.scan(c)
						check.totalQueueWait += c.timeInQueue
						check.totalScanTime += c.timeAtTill
						check.customersServed++
						//fmt.Println("\nTill", check.id, "serving its", check.customersServed, "customer, who has", c.items, "items:", &c,
						//	"\nTime spent at till:", c.timeAtTill, "Time in queue:", c.timeInQueue)
						//fmt.Println("Average wait time in queue", check.id, "=", time.Duration(int64(check.totalQueueWait)/int64(check.customersServed)))
						//fmt.Println("Currently", check.numInQ, "in queue", check.id)

			
					}
				}
			}(till, wg)
		}

	}

	//does not need to be goroutine atm, but probably will later
	simStart := time.Now()
SpawnLoop:
	for {
		select {
		case <-spawner.C:
			select {
			case c, ok := <-custs:
				if !ok {
					break SpawnLoop
				}
				if !c.joinQue(tills, c.items) {
					custsLost++
					//fmt.Println("A customer left")
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

	fmt.Println("Manager Name:", mrManager.name,"\nItem Limit:", mrManager.itemLimit,"\nIs smart?:", mrManager.isSmart,"\nItem Limited Checkouts?:", mrManager.isQuikCheck,"\nQuikCheckChance:", mrManager.cappedCheckRate)

	sort.Sort(byTillID(tills))
	for _, till := range tills {
		totalCusts += till.customersServed
		fmt.Println("\nTILL", till.id, "")
		fmt.Println("  Time Open:", till.endTime.Sub(till.startTime).Truncate(time.Second))
		fmt.Println("  Max Item Limit:", till.itemLimit)
		fmt.Println("  Customers Served:", till.customersServed)
		fmt.Println("  Total time waited by customers in queue:", till.totalQueueWait.Truncate(time.Second))
		fmt.Println("  Total time scanning:", till.totalScanTime.Truncate(time.Second))
	}

	fmt.Println("\nTotal Customers Served:", totalCusts)
	fmt.Println("\nTotal Customers Lost  :", custsLost)
	fmt.Println("\nSim RunTime", simRunTime.Truncate(time.Second))
}
