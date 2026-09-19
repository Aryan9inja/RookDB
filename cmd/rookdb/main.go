package main

import (
	"encoding/json"
	"fmt"

	"github.com/Aryan9inja/RookDB/internal/operation"
)

func checkMarshal() {
	op1 := operation.Operation{OpType: operation.Set, Key: "name", Value: "Aryan"}
	op2 := operation.Operation{OpType: operation.Delete, Key: "name"}

	data1, err := json.Marshal(op1)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(string(data1))
	// {"type":"SET","key":"name","value":"Aryan"}

	data2, err := json.Marshal(op2)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(string(data2))
	// {"type":"DELETE","key":"name"}  ← "value" omitted (omitempty + zero value)
}

func main() {
	checkMarshal()
}
