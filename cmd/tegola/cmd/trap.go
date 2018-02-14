package cmd

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/terranodo/tegola/provider"
)

func setupTrap() {
	// Setup trap to allow us to close out the providers as needed.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGHUP, syscall.SIGINT, syscall.SIGQUIT)
	go func() {
		<-sigs
		provider.Cleanup()
	}()
}
