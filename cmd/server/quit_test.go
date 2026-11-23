package main

import (
	"sync"
	"testing"
	"time"
)

func TestQuitSignalDelivers(t *testing.T) {
	request, ch := newQuitSignal()
	request()
	select {
	case <-ch:
	default:
		t.Fatal("quit request was not delivered")
	}
}

func TestQuitSignalSecondRequestDoesNotBlock(t *testing.T) {
	request, ch := newQuitSignal()
	done := make(chan struct{})
	go func() {
		request()
		request()
		request()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("a repeated quit request blocked")
	}
	<-ch
	select {
	case <-ch:
		t.Fatal("repeated requests must collapse into one signal")
	default:
	}
}

func TestQuitSignalConcurrentRequestsDoNotBlock(t *testing.T) {
	request, ch := newQuitSignal()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); request() }()
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("concurrent quit requests blocked")
	}
	<-ch
}
