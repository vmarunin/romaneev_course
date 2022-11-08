package main

import (
	"sort"
	"strconv"
	"strings"
	"sync"
)

// сюда писать код

var md5Mutex *sync.Mutex = &sync.Mutex{}

func crc32Helper(in string, out *[]string, outIdx int, wg *sync.WaitGroup) {
	(*out)[outIdx] = DataSignerCrc32(in)
	wg.Done()
}

func SingleHashWorker(s string, out chan interface{}, wg *sync.WaitGroup) {
	md5Mutex.Lock()
	md5s := DataSignerMd5(s)
	md5Mutex.Unlock()
	wg2 := &sync.WaitGroup{}
	crc := make([]string, 2)
	wg2.Add(1)
	go crc32Helper(s, &crc, 0, wg2)
	wg2.Add(1)
	go crc32Helper(md5s, &crc, 1, wg2)
	wg2.Wait()
	out <- crc[0] + "~" + crc[1]
	wg.Done()

}
func SingleHash(in, out chan interface{}) {
	wg := &sync.WaitGroup{}
	for v := range in {
		s := strconv.Itoa(v.(int))
		wg.Add(1)
		go SingleHashWorker(s, out, wg)
	}
	wg.Wait()
}

func MultiHashWorker(s string, out chan interface{}, wg *sync.WaitGroup) {
	wg2 := &sync.WaitGroup{}
	crc := make([]string, 6)
	for i := 0; i <= 5; i++ {
		wg2.Add(1)
		go crc32Helper(string(byte(i)+'0')+s, &crc, i, wg2)
	}
	wg2.Wait()
	out <- strings.Join(crc, "")
	wg.Done()

}
func MultiHash(in, out chan interface{}) {
	wg := &sync.WaitGroup{}
	for v := range in {
		s := v.(string)
		wg.Add(1)
		go MultiHashWorker(s, out, wg)
	}
	wg.Wait()
}

func CombineResults(in, out chan interface{}) {
	data := []string{}
	for v := range in {
		data = append(data, v.(string))
	}
	sort.Strings(data)
	out <- strings.Join(data, "_")
}

func ExecutePipeline(jobs ...job) {
	prevCh := make(chan interface{})
	wg := &sync.WaitGroup{}

	for _, j := range jobs {
		currCh := make(chan interface{})
		wg.Add(1)
		go func(j job, in, out chan interface{}) {
			defer close(out)
			defer wg.Done()
			j(in, out)
		}(j, prevCh, currCh)
		prevCh = currCh
	}
	wg.Wait()
}
