# cmd
Go library to encapsulate the signal handling pattern for termination signal so that I can properly clean up.

```go
package main

import (
	"fmt"
	"time"

	"github.com/gdey/cmd"
)

func cleanup(s string) {
	fmt.Printf("Cleaning up: %s!\n", s)
}

func main() {
	defer cmd.New().Complete()
	// Main code here
	fmt.Println("Runing 3 times.")
	for i := 0; i < 3; i++ {
		fmt.Println("Going to nap for a second.")
		select {
		case <-time.After(5 * time.Second):
			fmt.Println("Ahhh that was a good nap!")
		case <-cmd.Cancelled():
			fmt.Println("Ho! I got Ctr-C")
			cleanup("for and return")
			return
		}
		fmt.Println("Going sleep some more")
		<-time.After(2 * time.Second)
		fmt.Println("Good sleep!")
		if cmd.IsCancelled() {
			fmt.Println("Ctr-C got called.")
			cleanup("for and break")
			break
		}
		// do some chunk off work.
		<-time.After(2 * time.Second)
	}
	fmt.Println("All done!")
}
```
