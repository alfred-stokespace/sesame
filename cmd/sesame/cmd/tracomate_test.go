package cmd

import (
	"fmt"
	"github.com/madflojo/tasks"
	"testing"
	"time"
)

var countChan = make(chan bool, 10)

var x = 10
var count = 1
var task = func() error {

	fmt.Printf("doing my thing %d\n", count)
	count++
	countChan <- true
	return nil
}

func TestSchedular(t *testing.T) {

	// Start the Scheduler
	scheduler := tasks.New()
	defer scheduler.Stop()

	// Add a task
	id, err := scheduler.Add(&tasks.Task{
		Interval: time.Duration(1 * time.Second),
		TaskFunc: task,
	})
	if err != nil {
		t.Errorf("%s", err)
	}
	fmt.Printf("%s", id)

	for i := 1; i < x-1; i++ {
		fmt.Print("Checking")
		<-countChan
		fmt.Printf(" Something ran! %d \n", i)
	}
	fmt.Println("Stopping")
	scheduler.Del(id)
}
