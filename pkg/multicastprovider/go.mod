module github.com/tripleclabs/westcoast/pkg/multicastprovider

go 1.26.1

require (
	github.com/tripleclabs/westcoast v0.0.0
	golang.org/x/net v0.52.0
)

require golang.org/x/sys v0.42.0 // indirect

replace github.com/tripleclabs/westcoast => ../..
