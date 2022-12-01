package minconntransport

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestMinConn(*testing.T) {
	client := &http.Client{Transport: New()}

	for i := 0; i < 5; i++ {
		resp, err := client.Get("https://management.azure.com")
		if err != nil {
			fmt.Println(err)
		} else {
			defer resp.Body.Close()
			fmt.Println(resp.StatusCode)
		}
	}

	time.Sleep(10 * time.Second)
	for i := 0; i < 60; i++ {
		resp, err := client.Get("https://management.azure.com")
		if err != nil {
			fmt.Println(err)
		} else {
			defer resp.Body.Close()
			fmt.Println(resp.StatusCode)
		}
	}
}
