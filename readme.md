minconn transport is trying to replace default connection pooling, the key different is:
- default transport prefer idle connection
- minConn transpoprt prefer new connection, until it hit the specified limit

this is to help dealing with server throttling, by make more connections, higher chance we hit more different server instance, and hopefully less throttling.


Status:
- min connection pool is ready.
- figure out broken connection and replace them is key issue facing now.