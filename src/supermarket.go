package main

import (
	"fmt"
	"math"
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
		time.Sleep(op.scanTime)
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
var managerItemLimit int
var smartCusts bool
var smartManager bool
var minScanTime time.Duration = 3 * time.Microsecond
var maxScanTime time.Duration = 10 * time.Microsecond
var simRunTime time.Duration
var custArrivalRate time.Duration = 180 * time.Microsecond //5mins scaled secs->microsecs
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
	label02 := widget.NewLabel("Checkouts Open:")
	label03 := widget.NewLabel("Number of Checkout Operartors:")
	label04 := widget.NewLabel("Number of Customers:")
	label05 := widget.NewLabel("Minimum Items:")
	label06 := widget.NewLabel("Maximum Items:")
	label07 := widget.NewLabel("Max Queue Length:")
	label08 := widget.NewLabel("Manager Checkout Item Limit:")

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
	entry08 := widget.NewEntry()

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
		managerItemLimit, _ = strconv.Atoi(entry08.Text)

		if runSim() == 1 {

			outputLabel := widget.NewLabelWithStyle(postProcesses(), fyne.TextAlignLeading, fyne.TextStyle{false, false, true})
			outputLabel.Wrapping = fyne.TextWrapOff
			cd1 := widget.NewCard("SIMULATION REPORT", "", outputLabel)
			scrllCont := widget.NewScrollContainer(cd1)
			content2 := fyne.NewContainerWithLayout(layout.NewGridLayout(1), scrllCont)
			window.SetContent(content2)
		}
	})

	content := fyne.NewContainerWithLayout(layout.NewFormLayout(),
		label01, entry01, label02, entry02, label03, entry03, label04, entry04, label05, entry05, label06, entry06, label07, entry07, label08, entry08,
		labelfiller01, labelfiller02,
		checkbox01, checkbox02,
		labelfiller03, labelfiller04,
		labelfiller05, button01,
	)

	window.SetContent(content)
	window.Resize(fyne.NewSize(300, 300))
	window.ShowAndRun()

}

func postProcesses() string {
	totalCusts := 0
	tillUseTime := 0 * time.Microsecond
	tillOpenTime := 0 * time.Microsecond
	waitTime := 0 * time.Microsecond
	runningUtilization := 0.0
	output := ("INDIVIDUAL TILLS:\n")

	for _, till := range tills {
		output += fmt.Sprintf("\nTILL %d:\n", till.id)
		if !till.open {
			output += ("TILL CLOSED\n")
			continue
		}
		if till.operator == nil {
			output += ("NO OPERATOR ASSIGNED\n")
			continue
		}

		if till.itemLimit < math.MaxInt32 {
			output += ("_________________________\n")
			output += fmt.Sprintf("%d item limit on this till\n", till.itemLimit)
			output += ("‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾\n")
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

		output += fmt.Sprintf(" Time Open                              : %s\n", (open * 1_000).Truncate(time.Second).String())
		output += fmt.Sprintf(" Total Scanning                         : %s\n", (till.totalScanTime * 1_000).Truncate(time.Second).String())
		output += fmt.Sprintf(" Customers Served                       : %d\n", till.customersServed)
		output += fmt.Sprintf(" Items Processed                        : %d\n", till.itemsProcessed)
		output += fmt.Sprintf(" Mean Items Per Customer                : %.2f\n", meanItems)
		output += fmt.Sprintf(" Utilization                            : %.2f%%\n", utilization)
		output += fmt.Sprintf(" Mean Customer Wait Time                : %s\n", meanWait.String())
		output += fmt.Sprintf(" Total time waited by customers in queue: %s\n", (till.totalQueueWait * 1_000).Truncate(time.Second).String())
	}

	divisor := checkoutsOpen
	if numOperators < checkoutsOpen {
		divisor = numOperators
	}

	output += fmt.Sprintf("\n\nTOTALS:\n")
	output += fmt.Sprintf(" Total Customers Served          : %d\n", totalCusts)
	output += fmt.Sprintf(" Total Customers Lost            : %d\n", custsLost)
	output += fmt.Sprintf(" Total Items Processed           : %d\n", totalItemsProcessed)
	output += fmt.Sprintf(" Mean Number Items per Customer  : %.2f\n", (float64(totalItemsProcessed) / float64(totalCusts)))
	output += fmt.Sprintf(" Total Till Utilization          : %.2f%%\n", (float64(tillUseTime)/float64(tillOpenTime))*100.0)
	output += fmt.Sprintf(" Mean Till Utilization           : %.2f%%\n", runningUtilization/float64(divisor))
	output += fmt.Sprintf(" Mean Customer Wait Time         : %s\n", time.Duration(float64(waitTime*1_000)/float64(totalCusts)).Truncate(time.Second).String())
	output += fmt.Sprintf(" Store Processed a customer every: %s\n", time.Duration(float64(tillUseTime*1_000)/float64(totalCusts)).Truncate(time.Second).String())

	output += fmt.Sprintf("\n\nSim RunTime: %s", simRunTime.String())

	return output
}

func runSim() int {

	tills = make([]*checkout, numCheckouts)
	ops = make([]*operator, numOperators)
	custs = make(chan *customer, numCusts)
	//SETUP
	rand.Seed(time.Now().UTC().UnixNano())

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
						check.itemsProcessed += c.items
						check.customersServed++
						check.itemsProcessed += c.items
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

	return 1
}

func main() {

	gui()

}
