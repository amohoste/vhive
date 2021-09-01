package main

import (
	"fmt"
	"github.com/google/nftables"
)

func main() {
	conn := nftables.Conn{}

	tables, err := conn.ListTables()
	if err != nil {
		fmt.Println(err)
	}

	for _, table := range tables {
		fmt.Println(table)
	}

	chains, err := conn.ListChains()
	if err != nil {
		fmt.Println(err)
	}

	for _, chain := range chains {
		fmt.Println(chain)
	}

}