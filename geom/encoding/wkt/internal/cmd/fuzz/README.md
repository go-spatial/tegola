# Fuzz

This command generate 100,000 random geometries trying to find one that panics the wkt.Encode() function. If there is 
a panic, it will print out the geometry and the message. 

To run the command:
```
go run main.go
```
