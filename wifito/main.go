package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"deadnet/utils"
)

func main() {
	if os.Geteuid() != 0 {
		exe, err := os.Executable()
		if err != nil {
			log.Fatalf("must be run as root (failed to find executable: %v)", err)
		}
		args := append([]string{exe}, os.Args[1:]...)
		cmd := exec.Command("sudo", args...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			log.Fatalf("sudo failed: %v", err)
		}
		return
	}

	fmt.Printf("\n%s\nexecute with -i Interface for basic usage.\n", utils.Banner)
	utils.Printf(utils.Delim)

	args := utils.ParseArgs()
	utils.InvalidatePrint()

	attacker, err := NewDeadNet(args)
	if err != nil {
		log.Fatalf("setup failed: %v", err)
	}
	defer attacker.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		utils.Printf(utils.Delim)
		utils.Printf("%s[-]%s User requested to stop...", utils.Red, utils.White)
		cancel()
	}()

	if err := attacker.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatalf("attack failed: %v", err)
	}
}
