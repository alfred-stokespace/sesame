package cmd

import (
	"fmt"
	"testing"
	"time"

	"github.com/go-co-op/gocron"
)

var countChan = make(chan bool, 10)

var count = 1
var task = func (ch chan<- bool)  {
	
	fmt.Printf("doing my thing %d\n", count)
	count++
	ch <- true
}

func TestSchedular(t *testing.T) {

	s := gocron.NewScheduler(time.UTC)

	x := 10
	_, err := s.Every("1s").LimitRunsTo(x).SingletonMode().Do(task, countChan)
	
	if err != nil {
		t.Errorf("%s", err)
	}
	
	go func(){
		s.StartAsync()
	}()
	
	for i :=0 ; i<x-1; i++ {
		fmt.Print("Checking")
		<-countChan
		fmt.Printf(" Something ran! %d \n", i)
	}
	fmt.Println("Stopping")
	s.Stop()

}