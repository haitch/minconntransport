package minconntransport

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestMinConn(*testing.T) {
	// max connection allowed is 5
	transport := NewFromHttpTransport(5, http.DefaultTransport.(*http.Transport))
	client := &http.Client{Transport: transport}

	minConnTransport := transport.(*roundTripperWithConnectionManager)

	// first 5 request will create new connection
	for i := 0; i < 5; i++ {
		resp, err := client.Get("https://management.azure.com")
		if err != nil {
			fmt.Println(err)
		} else {
			defer resp.Body.Close()
			fmt.Println(resp.StatusCode)
		}
	}

	perHostCM := minConnTransport.cm.connections["management.azure.com:443"]
	// TODO: assert on unique connection count
	for _, conn := range perHostCM.connections {
		fmt.Printf("Local: %s \t --> \t Remote: %s\n", conn.conn.LocalAddr(), conn.conn.RemoteAddr())
	}

	// after 5 request, it will start to reuse the connection, and it will do round-robin on all pooled connection
	time.Sleep(2 * time.Second)
	for i := 0; i < 60; i++ {
		resp, err := client.Get("https://management.azure.com")
		if err != nil {
			fmt.Println(err)
		} else {
			defer resp.Body.Close()
			fmt.Println(resp.StatusCode)
		}
	}

	// sleep 30s, so server will close some of the connection, and we should recover from broken connection.
	time.Sleep(153 * time.Second)
	for i := 0; i < 60000; i++ {
		resp, err := client.Get("https://management.azure.com")
		if err != nil {
			fmt.Println(err)
		} else {
			defer resp.Body.Close()
			fmt.Println(resp.StatusCode)
		}
	}
}
