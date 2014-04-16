package main

import (
	"time"
	"os"
	"bufio"
	"strings"
	"strconv"
)

const (
	USER = iota
	NICE
	SYSTEM
	IDLE
)

func getStatNums() []int {
	sfi,err := os.Open("/proc/stat")
	if err != nil {
		//Who cares!
	}
	out := make([]int, 4)
	scan := bufio.NewScanner(sfi)
	scan.Scan()

	spl := strings.Split(scan.Text(), " ")
	n := 0
	if spl[1] == "" {
		n++
	}
	out[USER],_ = strconv.Atoi(spl[n + 1])
	out[NICE],_ = strconv.Atoi(spl[n + 2])
	out[SYSTEM],_ = strconv.Atoi(spl[n + 3])
	out[IDLE],_ = strconv.Atoi(spl[n + 4])
	return out
}

func GetCpuUsage() float32 {
	polla := getStatNums()
	time.Sleep(time.Second)
	pollb := getStatNums()
	sum := 0
	for i,v := range polla {
		pollb[i] -= v
		sum += pollb[i]
	}
	return float32(sum - pollb[3]) / float32(sum)
}
