package main

func main() {
	fmt.Println("Uptime Monitoring Service starting...")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	fmt.Println("Monitoring active. Press Ctrl+C to stop the server.")

	<-stop

	fmt.Println("Shutting down gracefully...")
}
