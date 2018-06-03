package main

import (
	"flag"
	"fmt"
	"sync"

	mgt "./management"
)

func main() {

	dns := flag.String("dns", "8.8.8.8", "The DNS SERVER")
	flag.Parse()

	mgt.SetDNS(*dns)
	fmt.Println("DNS SERVER = " + *dns)

	var wg sync.WaitGroup

	ch := make(chan bool)

	mgt.StartManagementServer(ch, &wg)
	wg.Wait()
}
