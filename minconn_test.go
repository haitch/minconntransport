package minconntransport

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMinConn(t *testing.T) {
	// max connection allowed is 5
	transport := NewFromHttpTransport(http.DefaultTransport.(*http.Transport), map[string]int32{"management.azure.com:443": 5})
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

	conns := listConnections(minConnTransport, "management.azure.com:443")
	fmt.Println("Connections: \n" + strings.Join(conns, "\n"))
	assert.Equal(t, 5, len(conns))

	// bing is not configured in hostLimit, so it will use default transport
	_, err := client.Get("https//www.bing.com")
	assert.NoError(t, err)
	connsBing := listConnections(minConnTransport, "www.bing.com:443")
	assert.Equal(t, 0, len(connsBing))

	// after 5 request, it will start to reuse the connection, and it will do round-robin on all pooled connection
	time.Sleep(2 * time.Second)
	for i := 0; i < 10; i++ {
		resp, err := client.Get("https://management.azure.com")
		if err != nil {
			fmt.Println(err)
		} else {
			defer resp.Body.Close()
			fmt.Println(resp.StatusCode)
		}
	}

	conns2 := listConnections(minConnTransport, "management.azure.com:443")
	fmt.Println("Connections: \n" + strings.Join(conns2, "\n"))
	assert.Equal(t, 5, len(conns2))

	// sleep some time to have server close the connection
	// a quicker way need human intervention: add a breakpoint here, and then toggle your wifi
	time.Sleep(185 * time.Second)

	// make sure we can recover from broken connections
	for i := 0; i < 20; i++ {
		resp, err := client.Get("https://management.azure.com")
		if err != nil {
			fmt.Println(err)
		} else {
			defer resp.Body.Close()
			fmt.Println(resp.StatusCode)
		}
	}

	conns3 := listConnections(minConnTransport, "management.azure.com:443")
	fmt.Println("Connections: \n" + strings.Join(conns3, "\n"))
	assert.Equal(t, 5, len(conns3))
}

func listConnections(minConnTransport *roundTripperWithConnectionManager, host string) []string {
	perHostCM, ok := minConnTransport.cm.connections[host]
	if !ok {
		return []string{}
	}

	// TODO: assert on unique connection count
	conns := make([]string, 0)
	for _, conn := range perHostCM.connections {
		if conn != nil && conn.conn != nil {
			cs := conn.conn.ConnectionState()
			conns = append(conns, fmt.Sprintf("Local: %s \t -- %s --> \t Remote: %s", conn.conn.LocalAddr(), cs.NegotiatedProtocol, conn.conn.RemoteAddr()))
		}
	}

	return conns
}
