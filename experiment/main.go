package main

import (
	"bufio"
	"fmt"
	"strings"
)

func main() {
	buffer := make([]byte, 100)
	var reader1 = strings.NewReader("Hello I am first reader")
	var bufreader = bufio.NewReader(reader1)
	n, err := bufreader.Read(buffer)
	if err != nil {
		fmt.Println("Error reading")
	}
	fmt.Println(n)
	fmt.Println(string(buffer))

}
