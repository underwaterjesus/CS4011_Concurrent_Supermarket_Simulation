package main

import (
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"sync"
	"time"

	"fyne.io/fyne"
	"fyne.io/fyne/app"
	"fyne.io/fyne/layout"
	"fyne.io/fyne/widget"
)

//STRUCTS
type customer struct {
	items       int
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

	if smartCusts {
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
		r := op.scanTime
		time.Sleep(time.Duration(r))
	}
	cust.timeAtTill = time.Since(start)
}

//GLOBALS
//seconds scaled to microseconds(1e-6 seconds)
const maxItem = 2147483647

var scale int64 = 1000
var numCheckouts int
var checkoutsOpen int
var numOperators int
var numCusts int
var totalCustsServed int
var custsLost = 0
var minItems int
var maxItems int
var maxQueueLength int
var smartCusts bool
var smartManager bool
var minScanTime time.Duration = 5 * time.Microsecond * 1000
var maxScanTime time.Duration = 60 * time.Microsecond * 1000
var simRunTime time.Duration
var custArrivalRate time.Duration = 30 * time.Microsecond * 1000 //5mins scaled secs->microsecs
var totalItemsProcessed = 0
var averageItemsPerTrolley = 0

var spawner = time.NewTicker(custArrivalRate)
var tick = time.NewTicker(custArrivalRate / 10)

var mutex = &sync.Mutex{}
var tills []*checkout
var ops []*operator
var custs chan *customer
var mrManager manager

var wg = &sync.WaitGroup{}

//GUI function
func gui() {
	app := app.New()
	window := app.NewWindow("Supermarket Simulator Param Input")
	label01 := widget.NewLabel("Number of Checkouts:")
	label02 := widget.NewLabel("Checkouts open:")
	label03 := widget.NewLabel("Number of Checkout operartors:")
	label04 := widget.NewLabel("Number of customers:")
	label05 := widget.NewLabel("Minimum items:")
	label06 := widget.NewLabel("Maximum items:")
	label07 := widget.NewLabel("Max Queue Length:")

	labelfiller01 := widget.NewLabel("")
	labelfiller02 := widget.NewLabel("")
	labelfiller03 := widget.NewLabel("")
	labelfiller04 := widget.NewLabel("")
	labelfiller05 := widget.NewLabel("")
	entry01 := widget.NewEntry()
	entry02 := widget.NewEntry()
	entry03 := widget.NewEntry()
	entry04 := widget.NewEntry()
	entry05 := widget.NewEntry()
	entry06 := widget.NewEntry()
	entry07 := widget.NewEntry()

	checkbox01 := widget.NewCheck("Smart Manager", func(value bool) {
		smartManager = value

	})
	checkbox02 := widget.NewCheck("Smart Customers", func(value bool) {
		smartCusts = value
	})

	button01 := widget.NewButton("Begin simulation", func() {
		numCheckouts, _ = strconv.Atoi(entry01.Text)
		checkoutsOpen, _ = strconv.Atoi(entry02.Text)
		numOperators, _ = strconv.Atoi(entry03.Text)
		numCusts, _ = strconv.Atoi(entry04.Text)
		minItems, _ = strconv.Atoi(entry05.Text)
		maxItems, _ = strconv.Atoi(entry06.Text)
		maxQueueLength, _ = strconv.Atoi(entry07.Text)

		if runSim() == 1 {

			output := "Output:\n"
			fmt.Println("Manager Name:", mrManager.name, "\nItem Limit:", mrManager.itemLimit, "\nIs smart?:", mrManager.isSmart, "\nItem Limited Checkouts?:", mrManager.isQuikCheck, "\nQuikCheckChance:", mrManager.cappedCheckRate)
			if smartCusts {
				sort.Sort(byTillID(tills))
			}

			
			label08 := widget.NewLabel("Output")

			avgItem := 0.0
			for _, till := range tills {
				totalCustsServed += till.customersServed
				totalItemsProcessed += till.itemsProcessed
				
	
				output += "\n\nTILL " +  strconv.Itoa(till.id) + ""

				output += "\n  Time Open: " + till.endTime.Sub(till.startTime).Truncate(time.Second).String()
				output += "\n  Max Item Limit: " + strconv.Itoa(till.itemLimit)
				output += "\n  Customers Served: " + strconv.Itoa(till.customersServed)
				output += "\n  Total time waited by customers in queue: " + till.totalQueueWait.Truncate(time.Second).String()
				output += "\n  Total time scanning: " + till.totalScanTime.Truncate(time.Second).String()

				label08.SetText(output)

			}

			output += "\n\nTotal Customers Served: " + strconv.Itoa(totalCustsServed)
			output += "\nTotal Customers Lost: " +  strconv.Itoa(custsLost)
			output += "\nSim RunTime: " + simRunTime.Truncate(time.Second).String()
			output += "\nTotal Items Processed: " + strconv.Itoa(totalItemsProcessed)
			avgItem = (float64(totalItemsProcessed) / float64(totalCustsServed))
			output += "\nMean Average Item per customer: " + strconv.FormatFloat(avgItem,'E', -1, 32)
			


			output +="\n\nTotal Customers Served:" + strconv.Itoa(totalCustsServed)
			
			//label08 := widget.NewLabel(output)

			// button02 := widget.NewButton("Close", func() {
			//  	window.Close()
			// })
		
			cd1 := widget.NewCard("Output Info", "aaaa", label08)
			scrllCont := widget.NewScrollContainer(cd1)

			content2 := fyne.NewContainerWithLayout(layout.NewGridLayout(1), scrllCont)

			window.SetContent(content2)
		}
		//window.Close()
	})

	content := fyne.NewContainerWithLayout(layout.NewFormLayout(),
		label01, entry01,
		label02, entry02,
		label03, entry03,
		label04, entry04,
		label05, entry05,
		label06, entry06,
		label07, entry07,
		labelfiller01, labelfiller02,
		checkbox01, checkbox02,
		labelfiller03, labelfiller04,
		labelfiller05, button01,
	)

	window.SetContent(content)
	window.ShowAndRun()

}

func runSim() int {

	tills = make([]*checkout, numCheckouts)
	ops = make([]*operator, numOperators)
	custs = make(chan *customer, numCusts)
	//SETUP
	rand.Seed(time.Now().UTC().UnixNano())

	//This seems like an appropriate place for the time mark,
	//like when the manager first opens the door to the market at the start of the day.
	mrManager.name = "Mr. Manager"
	mrManager.cappedCheckRate = rand.Intn(int(checkoutsOpen / 2))
	mrManager.itemLimit = 5
	mrManager.isSmart = smartManager
	mrManager.isQuikCheck = true

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

			tills[i] = &checkout{nil, &queue{q}, i + 1, maxItem, 0, 0, 0, time.Time{}, time.Time{}, true, 0, 0, 0.0, 0.0, 0.0, 0}

		}

	}

	//checkout operator setup
	for i := range ops {
		ops[i] = &operator{time.Duration(rand.Intn(int(maxScanTime-minScanTime) + int(minScanTime+1)))}

		if i < numCheckouts {
			if tills[i].open {
				tills[i].operator = ops[i]
				wg.Add(1)
			}
		}
	}

	//Mr Manager is a good manager and makes sure to always pick the quickest available operator.
	if mrManager.isSmart {
		mrManager.sortOperators()
	}

	//create customers and send them to the cust channel
	for i := 0; i < cap(custs); i++ {
		custs <- &customer{(rand.Intn(maxItems-minItems) + minItems + 1), time.Now(), 0, 0, time.Second}
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
						check.itemsProcessed += c.items
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
	simRunTime = time.Since(simStart)
	fmt.Println()
	totalCustsServed = 0

	//output :="\nTotal Customers Served:" + strconv.Itoa(totalCustsServed)
	//+"\nTotal Customers Lost  :"+custsLost+"\nSim RunTime"+simRunTime.Truncate(time.Second)+"Total Items Processed:"+totalItemsProcessed+"Mean Average Item per customer"+float32(totalItemsProcessed) / float32(totalCusts)

	return 1
}

func main() {

	gui()

}
