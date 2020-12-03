//CS4011 - Supermarket ABB
//Authored by:
//Adam Aherne - 12159603, Eoin Purtill - 17185467, Ronan McMullen - 0451657, Tedis Stumbrs - 17208475.

package main

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"fyne.io/fyne"
	"fyne.io/fyne/app"
	"fyne.io/fyne/layout"
	"fyne.io/fyne/widget"
)

//STRUCTS
type customer struct {
	items       int
	queue       string
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
	isItemLimit     bool
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

var weatherStrings = []string{"Stormy - x0.4", "Rainy - x0.8", "Mild - x1.0", "Sunny - x1.2", "Heatwave - x0.6"}
var weatherScale float64

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
var limitedCheckoutRate int
var smartCusts bool
var smartManager bool
var isItemLimit bool
var minST float64
var maxST float64
var minScanTime time.Duration
var maxScanTime time.Duration
var simRunTime time.Duration
var arrivalRateScale float64
var custArrivalRate time.Duration = 60 * time.Millisecond
var totalItemsProcessed = 0
var averageItemsPerTrolley = 0

var mutex = &sync.Mutex{}
var tills []*checkout
var ops []*operator
var custs chan *customer
var servedCusts chan *customer
var mrManager manager

var wg = &sync.WaitGroup{}

func gui() {
	app := app.New()
	window := app.NewWindow("Supermarket Simulator CS4011")
	label01 := widget.NewLabel("Number of Checkouts:")
	label02 := widget.NewLabel("Checkouts Open:")
	label03 := widget.NewLabel("Number of Checkout Operators:")
	label04 := widget.NewLabel("Number of Customers:")
	label05 := widget.NewLabel("Minimum Items:")
	label06 := widget.NewLabel("Maximum Items:")
	label07 := widget.NewLabel("Max Queue Length:")
	label08 := widget.NewLabel("Manager Checkout Item Limit:")
	label09 := widget.NewLabel("Item Limited Till Rate:")
	label10 := widget.NewLabel("Min Scan Time:")
	label11 := widget.NewLabel("Max Scan Time:")
	label12 := widget.NewLabel("Customer Arrival Rate:")
	label13 := widget.NewLabel("Weather (affects C.A.R.)")
	labelfiller := widget.NewLabel("")
	selectWeather := widget.NewSelect(weatherStrings, func(selected string) {

		if selected == "Stormy - x0.4" {

			weatherScale = 0.4

		} else if selected == "Rainy - x0.8" {

			weatherScale = 0.8

		} else if selected == "Sunny - x1.2" {

			weatherScale = 1.2

		} else if selected == "Heatwave - x0.6" {

			weatherScale = 0.6

		} else {

			weatherScale = 1.0

		}

	})

	entry01 := widget.NewEntry()
	entry01.SetPlaceHolder("- - Integer expected (1-8) - -")
	entry02 := widget.NewEntry()
	entry02.SetPlaceHolder("- - Integer expected (1-8) - -")
	entry03 := widget.NewEntry()
	entry03.SetPlaceHolder("- - Integer expected (1-8) - -")
	entry04 := widget.NewEntry()
	entry04.SetPlaceHolder("- - Integer expected (1-200) - -")
	entry05 := widget.NewEntry()
	entry05.SetPlaceHolder("- - Integer expected (1-200) - -")
	entry06 := widget.NewEntry()
	entry06.SetPlaceHolder("- - Integer expected (1-200) - -")
	entry07 := widget.NewEntry()
	entry07.SetPlaceHolder("- - Integer expected (1-12) - -")
	entry08 := widget.NewEntry()
	entry08.SetPlaceHolder("- - Integer expected (1-20) - -")
	entry09 := widget.NewEntry()
	entry09.SetPlaceHolder("- - Float expected (0.5 - 6.0) - -")
	entry10 := widget.NewEntry()
	entry10.SetPlaceHolder("- - Float expected (0.5 - 6.0) - -")
	entry11 := widget.NewEntry()
	entry11.SetPlaceHolder("- - Float expected (1.0 - 60.0) (1 = slowest) - -")

	checkbox01 := widget.NewCheck("Smart Manager", func(value bool) {
		smartManager = value
	})
	checkbox02 := widget.NewCheck("Smart Customers", func(value bool) {
		smartCusts = value
	})
	checkbox03 := widget.NewCheck("Item Limit Tills?", func(value bool) {
		isItemLimit = value
	})
	radio := widget.NewRadio([]string{"10%", "25%", "50%"}, func(value string) {
		limitedCheckoutRate = 10
		if strings.Compare(value, "25%") == 0 {
			limitedCheckoutRate = 4
		}
		if strings.Compare(value, "50%") == 0 {
			limitedCheckoutRate = 2
		}
	})
	button01 := widget.NewButton("Begin simulation", func() {
		var ok error
		valid := true
		errorString := ""
		numCheckouts, ok = strconv.Atoi(entry01.Text)
		if ok != nil {
			valid = false
			errorString += fmt.Sprintf("\nInvalid Input for checkout number!")
		} else {
			if numCheckouts < 1 || numCheckouts > 8 {
				valid = false
				errorString += fmt.Sprintf("\nNumber of checkouts outside range!")
			}
		}
		checkoutsOpen, ok = strconv.Atoi(entry02.Text)
		if ok != nil {
			valid = false
			errorString += fmt.Sprintf("\nInvalid Input for checkouts open!")
		} else {
			if checkoutsOpen < 1 || checkoutsOpen > 8 {
				valid = false
				errorString += fmt.Sprintf("\nCheckouts open outside range!")
			}
		}
		numOperators, ok = strconv.Atoi(entry03.Text)
		if ok != nil {
			valid = false
			errorString += fmt.Sprintf("\nInvalid Input for operator number!")
		} else {
			if numOperators < 1 || numOperators > 8 {
				valid = false
				errorString += fmt.Sprintf("\nCheckout operators outside range!")
			}
		}
		numCusts, ok = strconv.Atoi(entry04.Text)
		if ok != nil {
			valid = false
			errorString += fmt.Sprintf("\nInvalid Input for customer number!")
		} else {
			if numCusts < 1 || numCusts > 200 {
				valid = false
				errorString += fmt.Sprintf("\nNumber of customers outside range!")
			}
		}
		minItems, ok = strconv.Atoi(entry05.Text)
		if ok != nil {
			valid = false
			errorString += fmt.Sprintf("\nInvalid Input for item minimum!")
		} else {
			if minItems < 1 || minItems > 200 {
				valid = false
				errorString += fmt.Sprintf("\nMin items out of range!")
			}
		}
		maxItems, ok = strconv.Atoi(entry06.Text)
		if ok != nil {
			valid = false
			errorString += fmt.Sprintf("\nInvalid Input for item maximum!")
		} else {
			if maxItems < 1 || maxItems > 200 {
				valid = false
				errorString += fmt.Sprintf("\nMax items out of range!")
			}
		}
		maxQueueLength, ok = strconv.Atoi(entry07.Text)
		if ok != nil {
			valid = false
			errorString += fmt.Sprintf("\nInvalid Input for max queue length!")
		} else {
			if maxQueueLength < 1 || maxQueueLength > 12 {
				valid = false
				errorString += fmt.Sprintf("\nMax queue length out of range!")
			}
		}
		managerItemLimit, ok = strconv.Atoi(entry08.Text)
		if ok != nil {
			valid = false
			errorString += fmt.Sprintf("\nInvalid Input for manager item limit!")
		} else {
			if managerItemLimit < 1 || managerItemLimit > 20 {
				valid = false
				errorString += fmt.Sprintf("\nItem limit outside of range!")
				isItemLimit = false
			}
		}
		minST, ok = strconv.ParseFloat(entry09.Text, 64)
		if ok != nil {
			valid = false
			errorString += fmt.Sprintf("\nInvalid Input for minimum scan time!")
		} else {
			if minST < 0.5 || minST > 6.0 {
				valid = false
				errorString += fmt.Sprintf("\nMin scan time out of range!")
			}
		}
		maxST, ok = strconv.ParseFloat(entry10.Text, 64)
		if ok != nil {
			valid = false
			errorString += fmt.Sprintf("\nInvalid Input for maximum scan time!")
		} else {
			if maxST < 0.5 || maxST > 6.0 {
				valid = false
				errorString += fmt.Sprintf("\nMax scan time out of range!")
			}
		}
		arrivalRateScale, ok = strconv.ParseFloat(entry11.Text, 64)
		if ok != nil {
			valid = false
			errorString += fmt.Sprintf("\nInvalid Input for arrival rate!")
		} else {
			if arrivalRateScale < 1.0 || arrivalRateScale > 60.0 {
				valid = false
				errorString += fmt.Sprintf("\nCustomer arrival rate out of range!")
			}
		}

		if valid {
			if minST > maxST {
				valid = false
				errorString += fmt.Sprintf("\nMin scan time is greater than max scan time!")
			}
			if minItems > maxItems {
				valid = false
				errorString += fmt.Sprintf("\nMin items is greater than max items!")
			}
			if checkoutsOpen > numCheckouts {
				valid = false
				errorString += fmt.Sprintf("\nCheckouts open is greater than number of checkouts!")
			}
		}
		if !valid {
			errorOutputLabel := widget.NewLabel(errorString)
			errorContent := fyne.NewContainerWithoutLayout(errorOutputLabel)
			window.SetContent(errorContent)
			window.Resize(fyne.Size{500, 300})
		} else {
			minScanTime = time.Duration(minST * float64(time.Millisecond))
			maxScanTime = time.Duration(maxST * float64(time.Millisecond))
			custArrivalRate = time.Duration(float64(custArrivalRate) / arrivalRateScale)
			custArrivalRate = time.Duration(float64(custArrivalRate) * weatherScale)

			if runSim() == 1 {
				outputLabel := widget.NewLabelWithStyle(postProcesses(), fyne.TextAlignLeading, fyne.TextStyle{false, false, true})
				outputLabel.Wrapping = fyne.TextWrapOff
				cd1 := widget.NewCard("SIMULATION REPORT", "", outputLabel)
				scrllCont := widget.NewScrollContainer(cd1)
				content2 := fyne.NewContainerWithLayout(layout.NewGridLayout(1), scrllCont)
				window.SetContent(content2)
			}
		}
	})

	content := fyne.NewContainerWithLayout(layout.NewFormLayout(),
		label01, entry01,
		label02, entry02,
		label03, entry03,
		label04, entry04,
		label05, entry05,
		label06, entry06,
		label07, entry07,
		label08, entry08,
		label10, entry09,
		label11, entry10,
		label12, entry11,
		label13, labelfiller,
		selectWeather, checkbox03,
		labelfiller, label09,
		labelfiller, radio,
		labelfiller, labelfiller,
		checkbox01, checkbox02,
		labelfiller, button01,
	)

	window.SetContent(content)
	window.Resize(fyne.Size{600, 700})
	window.ShowAndRun()

}

func postProcesses() string {
	if smartCusts {
		sort.Sort(byTillID(tills))
	}

	totalCustsServed = 0
	totalCusts := 0
	tillUseTime := 0 * time.Millisecond
	tillOpenTime := 0 * time.Millisecond
	waitTime := 0 * time.Millisecond
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
			output += ("__________________________\n")
			output += fmt.Sprintf("%d item limit on this till\n", till.itemLimit)
			output += ("‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾\n")
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

		output += fmt.Sprintf(" Scan Rate (per item)                        : %s\n", (till.operator.scanTime * 1_000).Truncate(time.Millisecond).String())
		output += fmt.Sprintf(" Time Open                                   : %s\n", (open * 1_000).Truncate(time.Second).String())
		output += fmt.Sprintf(" Total Scanning                              : %s\n", (till.totalScanTime * 1_000).Truncate(time.Second).String())
		output += fmt.Sprintf(" Customers Served                            : %d\n", till.customersServed)
		output += fmt.Sprintf(" Items Processed                             : %d\n", till.itemsProcessed)
		output += fmt.Sprintf(" Mean Items Per Customer                     : %.2f\n", meanItems)
		output += fmt.Sprintf(" Utilization                                 : %.2f%%\n", utilization)
		output += fmt.Sprintf(" Mean Customer Wait Time                     : %s\n", meanWait.String())
		output += fmt.Sprintf(" Cumulative time waited by customers in queue: %s\n", (till.totalQueueWait * 1_000).Truncate(time.Second).String())
	}

	output += fmt.Sprintf("\n\nTOTALS:\n")
	output += fmt.Sprintf(" Base Customer Arrival Rate      : 1 every %s\n", (time.Duration(float64(custArrivalRate)/weatherScale) * 1_000).Truncate(time.Millisecond).String())
	output += fmt.Sprintf(" Arrival Rate After Weather      : 1 every %s\n", (custArrivalRate * 1_000).Truncate(time.Millisecond).String())
	output += fmt.Sprintf(" Total Customers Served          : %d\n", totalCusts)
	output += fmt.Sprintf(" Total Customers Lost            : %d\n", custsLost)
	output += fmt.Sprintf(" Total Items Processed           : %d\n", totalItemsProcessed)
	output += fmt.Sprintf(" Mean Number Items per Customer  : %.2f\n", (float64(totalItemsProcessed) / float64(totalCusts)))
	output += fmt.Sprintf(" Till Utilization                : %.2f%%\n", (float64(tillUseTime)/float64(tillOpenTime))*100.0)
	output += fmt.Sprintf(" Mean Customer Wait Time         : %s\n", time.Duration(float64(waitTime*1_000)/float64(totalCusts)).Truncate(time.Second).String())
	output += fmt.Sprintf(" Store Processed a customer every: %s\n", time.Duration(float64(tillUseTime*1_000)/float64(totalCusts)).Truncate(time.Second).String())

	output += fmt.Sprintf("\n\nSim RunTime: %s", simRunTime.String())

	servedCustsArr := make([]*customer, totalCusts)
	idx := 0
	for i := range servedCusts {
		servedCustsArr[idx] = i
		idx++
	}

	for j, cust := range servedCustsArr {
		output += fmt.Sprintf("\n\nCustomer %d:\n", j+1)
		output += fmt.Sprintf(" Till Used       : %s\n", cust.queue)
		output += fmt.Sprintf(" Items in Trolley: %d\n", cust.items)
		output += fmt.Sprintf(" Time in Queue   : %s\n", (cust.timeInQueue * 1_000).Truncate(time.Second).String())
		output += fmt.Sprintf(" Time at Till    : %s\n", (cust.timeAtTill * 1_000).Truncate(time.Second).String())
	}

	return output
}

func runSim() int {

	tills = make([]*checkout, numCheckouts)
	ops = make([]*operator, numOperators)
	custs = make(chan *customer, numCusts)
	servedCusts = make(chan *customer, numCusts)
	spawner := time.NewTicker(custArrivalRate)

	//SETUP
	rand.Seed(time.Now().UTC().UnixNano())
	mrManager.name = "Mr. Manager"
	var num int
	if isItemLimit && limitedCheckoutRate > 0 {
		if checkoutsOpen%2 == 0 {
			num = checkoutsOpen / limitedCheckoutRate
			if num <= 0 {
				num = 1
			}
			mrManager.cappedCheckRate = num
		} else {
			num = (checkoutsOpen - 1) / limitedCheckoutRate
			if num <= 0 {
				num = 1
			}
			mrManager.cappedCheckRate = num
		}
	} else {
		mrManager.cappedCheckRate = 0
	}
	mrManager.itemLimit = managerItemLimit
	mrManager.isSmart = smartManager
	mrManager.isItemLimit = isItemLimit

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
		ops[i] = &operator{time.Duration(rand.Intn(int((maxScanTime-minScanTime)+1)) + int(minScanTime))}
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

	for _, till := range tills {
		till.endTime = time.Now()
	}

	simRunTime = time.Since(simStart)
	close(servedCusts)

	return 1
}

func main() {

	gui()

}
