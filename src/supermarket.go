package main

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

//STRUCTS
type customer struct {
	items       int
	queue		string
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
	name            string
	cappedCheckRate int
	itemLimit       int
	isSmart         bool
	isQuikCheck     bool
}

type checkout struct {
	operator           *operator
	queue              *queue
	id                 int
	itemLimit          int
	customersServed    int
	customersLost      int
	itemsProcessed     int
	startTime          time.Time
	endTime            time.Time
	open               bool
	totalQueueWait     time.Duration
	totalScanTime      time.Duration
	percentTotalCusts  float32
	percentTimeWorking float32
	timePerCust        float32
	numInQ             int32
}

type weather struct {
	weatherCondition float32
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
func (cust *customer) joinQue(tills []*checkout) bool {

	if smartCusts {
		mutex.Lock()
		sort.Sort(byQLength(tills))
		mutex.Unlock()
	}

	for _, till := range tills {
		if till.open && till.operator != nil && cust.items < till.itemLimit {
			select {
			case till.queue.customers <- cust:
				cust.enterQAt = time.Now()
				cust.queue = strconv.Itoa(till.id)
				if smartCusts {
					atomic.AddInt32(&till.numInQ, 1)
				}

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
	if timeWaited > (cust.patience) {
		return false
	}
	return true
}

func (op *operator) scan(cust *customer) {
	n := cust.items
	cust.timeInQueue = time.Since(cust.enterQAt)
	start := time.Now()
	for i := 0; i < n; i++ {
		time.Sleep(op.scanTime)
	}
	cust.timeAtTill = time.Since(start)
	servedCusts <- cust
}

//GLOBALS
//seconds scaled to microseconds(1e-6 seconds)
const maxItem = 2147483647

//Array of strings not implemented yet
var setWeather = 1 // Change from 0 - 4 to see changes in customerArrival Rate
var weatherStrings = [5]string{"Stormy", "Rainy", "Mild", "Sunny", "Heatwave"}
var weatherScale = [5]float64{0.4, 0.8, 1, 1.2, 0.6}

//setting user input to Mild => 2 => 1
var weatherConditions weather

var scale int64 = 1000
var numCheckouts = 8
var checkoutsOpen = 8
var numOperators = 7
var numCusts = 200
var custsLost = 0
var minItems = 1
var maxItems = 90
var minPatience = 0
var maxPatience = 1
var maxQueueLength = 6
var smartCusts = false
var minScanTime time.Duration = 3 * time.Microsecond
var maxScanTime time.Duration = 10 * time.Microsecond
var custArrivalRate time.Duration = 180 * time.Microsecond //
var totalItemsProcessed = 0
var averageItemsPerTrolley = 0

var spawner = time.NewTicker(custArrivalRate)
var tick = time.NewTicker(custArrivalRate / 10)

var mutex = &sync.Mutex{}
var tills = make([]*checkout, numCheckouts)
var ops = make([]*operator, numOperators)
var custs = make(chan *customer, numCusts)
var servedCusts = make(chan *customer, numCusts)
var mrManager manager

var wg = &sync.WaitGroup{}

func main() {
	//SETUP
	rand.Seed(time.Now().UTC().UnixNano())
	//This seems like an appropriate place for the time mark,
	//like when the manager first opens the door to the market at the start of the day.
	mrManager.name = "Mr. Manager"
	mrManager.cappedCheckRate = rand.Intn(int(checkoutsOpen / 2))
	mrManager.itemLimit = 5
	mrManager.isSmart = true
	mrManager.isQuikCheck = true

	//Modify the customer arrival rate based on the weather
	custArrivalRate = time.Duration(float64(custArrivalRate) * float64(weatherScale[setWeather]))

	//checkout setup
	for i := range tills {
		q := make(chan *customer, maxQueueLength)

		//checkout(operator, queue, id, itemLimit, customersServed, customersLost startTime, endTime, open, totalQueueWait,
		//		   totalScanTime, percentTotalCusts, percentTimeWorking, timePerCust)
		if i < checkoutsOpen {
			if i < mrManager.cappedCheckRate {
				tills[i] = &checkout{nil, &queue{q}, i + 1, mrManager.itemLimit, 0, 0, 0, time.Time{}, time.Time{}, true, 0, 0, 0.0, 0.0, 0.0, 0}
			} else {
				tills[i] = &checkout{nil, &queue{q}, i + 1, maxItem, 0, 0, 0, time.Time{}, time.Time{}, true, 0, 0, 0.0, 0.0, 0.0, 0}
			}
		} else {

			tills[i] = &checkout{nil, &queue{q}, i + 1, maxItem, 0, 0, 0, time.Time{}, time.Time{}, false, 0, 0, 0.0, 0.0, 0.0, 0}

		}

	}

	//checkout operator setup
	for i := range ops {
		ops[i] = &operator{time.Duration(rand.Intn(int(maxScanTime-minScanTime) + int(minScanTime+1)))}
	}

	//Mr Manager is a good manager and makes sure to always pick the quickest available operator.
	if mrManager.isSmart {
		mrManager.sortOperators()
	}

	for i := 0; i < len(ops); i++ {
		if tills[i].open {
			tills[i].operator = ops[i]
			wg.Add(1)
		}
	}

	//create customers and send them to the cust channel
	for i := 0; i < cap(custs); i++ {
		custs <- &customer{(rand.Intn((maxItems-minItems)+1) + minItems), "0", time.Now(), 0, 0, time.Second}
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
						if smartCusts {
							atomic.AddInt32(&check.numInQ, -1)
						}

						check.operator.scan(c)
						check.totalQueueWait += c.timeInQueue
						check.totalScanTime += c.timeAtTill
						check.itemsProcessed += c.items
						check.customersServed++
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
				if !c.joinQue(tills) {
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

	close(servedCusts)
	fmt.Println()
	totalCusts := 0
	tillUseTime := 0 * time.Microsecond
	tillOpenTime := 0 * time.Microsecond
	waitTime := 0 * time.Microsecond
	runningUtilization := 0.0
	weatherToday := weatherStrings[setWeather]
	fmt.Printf("The weather today is %s !\n", weatherToday)
	fmt.Println("Customer arrival rate:", custArrivalRate)

	fmt.Println("Manager Name:", mrManager.name, "\nItem Limit:", mrManager.itemLimit, "\nIs smart?:", mrManager.isSmart, "\nItem Limited Checkouts?:", mrManager.isQuikCheck, "\nQuikCheckChance:", mrManager.cappedCheckRate)
	if smartCusts {
		sort.Sort(byTillID(tills))
	}

	fmt.Println("**********\nSIM REPORT\n**********")
	fmt.Println("\nINDIVIDUAL TILLS:")

	for _, till := range tills {
		fmt.Printf("\nTILL %d:\n", till.id)
		if !till.open {
			fmt.Println("TILL CLOSED")
			continue
		}
		if till.operator == nil {
			fmt.Println("NO OPERATOR ASSIGNED")
			continue
		}

		if till.itemLimit < math.MaxInt32 {
			fmt.Println("__________________________")
			fmt.Println(till.itemLimit, "item limit on this till")
			fmt.Println("‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾")
		}

		totalCusts += till.customersServed
		totalItemsProcessed += till.itemsProcessed
		open := time.Duration(till.endTime.Sub(till.startTime))
		tillOpenTime += open
		tillUseTime += till.totalScanTime
		waitTime += till.totalQueueWait
		utilization := (float64(till.totalScanTime) / float64(open)) * 100.0
		runningUtilization += utilization
		meanItems := float64(till.itemsProcessed) / float64(till.customersServed)
		meanWait := time.Duration(0)
		if math.IsNaN(meanItems) {
			meanItems = 0.0
		}
		if till.customersServed > 0 {
			meanWait = time.Duration(float64(till.totalQueueWait*1_000) / float64(till.customersServed)).Truncate(time.Second)
		}

		fmt.Println(" Time Open                              :", (open * 1_000).Truncate(time.Second))
		fmt.Println(" Total Scanning                         :", (till.totalScanTime * 1_000).Truncate(time.Second))
		fmt.Println(" Customers Served                       :", till.customersServed)
		fmt.Println(" Items Processed                        :", till.itemsProcessed)
		fmt.Printf(" Mean Items Per Customer                : %.2f\n", meanItems)
		fmt.Printf(" Utilization                            : %.2f%%\n", utilization)
		fmt.Println(" Mean Customer Wait Time                :", meanWait)
		fmt.Println(" Total time waited by customers in queue:", (till.totalQueueWait * 1_000).Truncate(time.Second))
	}

	divisor := checkoutsOpen
	if numOperators < checkoutsOpen {
		divisor = numOperators
	}

	fmt.Println("\n\nTOTALS:")
	fmt.Println(" Total Customers Served          :", totalCusts)
	fmt.Println(" Total Customers Lost            :", custsLost)
	fmt.Println(" Total Items Processed           :", totalItemsProcessed)
	fmt.Printf(" Mean Number Items per Customer  : %.2f\n", (float64(totalItemsProcessed) / float64(totalCusts)))

	fmt.Printf("\n Total Till Utilization          : %.2f%%\n", (float64(tillUseTime)/float64(tillOpenTime))*100.0)
	fmt.Printf(" Mean Till Utilization           : %.2f%%\n", runningUtilization/float64(divisor))
	fmt.Println(" Mean Customer Wait Time         :", time.Duration(float64(waitTime*1_000)/float64(totalCusts)).Truncate(time.Second))
	fmt.Println(" Store Processed a customer every:", time.Duration(float64(tillUseTime*1_000)/float64(totalCusts)).Truncate(time.Second))

	fmt.Println("\n\nSim RunTime:", simRunTime)

	fmt.Println("\n\nCUSTOMER INFO:")

	servedCustsArr := make([]*customer, totalCusts)
	idx := 0
	for i := range servedCusts {
		servedCustsArr[idx] = i
		idx++
	}

	for j, cust := range servedCustsArr {
		fmt.Println("\nCustomer:", j+1)
		fmt.Println(" Served at till number:", cust.queue)
		fmt.Println(" Items in Trolley     :", cust.items)
		fmt.Println(" Time in Queue        :", (cust.timeInQueue * 1_000))
		fmt.Println(" Time at Till         :", (cust.timeAtTill * 1_000))
	}
}
